#!/bin/bash

set -euo pipefail

COMMON_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../internal" && pwd)"
source "${COMMON_DIR}/e2e_common.sh"
feature_common_init mcp-transports
feature_usage() {
    echo "Usage: ./examples/mcp/transports/verify.sh --polaris-server <host> [--debug]"
}
feature_parse_transport_args "$@"

SERVICE_NAME="trpc.polaris.demo.mcp-transports"
HTTP_PORT="${HTTP_PORT:-28216}"
SSE_PORT="${SSE_PORT:-28217}"
MCP_PROTOCOL_VERSION="${MCP_PROTOCOL_VERSION:-2025-03-26}"

build_demo() {
    (cd "${REPO_ROOT}"
        go build -o "${BUILD_DIR}/provider" ./examples/mcp/provider
        go build -o "${BUILD_DIR}/client" ./examples/mcp/client
    )
}

start_provider() {
    rm -f "${BUILD_DIR}/ready.json"
    local extra=()
    if [[ "${DEBUG_MODE:-false}" == "true" ]]; then extra+=(-debug); fi
    (cd "${BUILD_DIR}"
        exec ./provider -server "${POLARIS_SERVER}" -polaris-port "${POLARIS_GRPC_PORT}" \
            -namespace "${NAMESPACE}" -service "${SERVICE_NAME}" -port "${HTTP_PORT}" \
            -sse-port "${SSE_PORT}" -multi-transport -server-version 1.0.0 -sse-version 2.0.0 \
            -protocol-version "${MCP_PROTOCOL_VERSION}" -ready "${BUILD_DIR}/ready.json" \
            ${extra[@]+"${extra[@]}"}
    ) >> "${LOG_DIR}/provider.log" 2>&1 &
    PROVIDER_PID=$!
    if ! record_process "${PROVIDER_PID}" provider "${HTTP_PORT}"; then
        stop_pid "${PROVIDER_PID}"
        return 1
    fi
    for _ in $(seq 1 120); do
        if [[ -s "${BUILD_DIR}/ready.json" ]]; then return 0; fi
        if ! process_alive "${PROVIDER_PID}"; then
            echo "Provider exited before both contracts were reported" >&2
            return 1
        fi
        sleep 0.25
    done
    echo "Provider readiness timed out" >&2
    return 1
}

instances() {
    local response
    response="$(api_request GET "/naming/v1/instances?namespace=${NAMESPACE}&service=${SERVICE_NAME}")"
    jq -e '.code == 200000 and ((.instances // []) | type == "array")' >/dev/null <<<"${response}" || return 1
    printf '%s\n' "${response}"
}

assert_instances() {
    local healthy="$1" response
    response="$(instances)"
    jq -e --arg service "${SERVICE_NAME}" --argjson http_port "${HTTP_PORT}" \
        --argjson sse_port "${SSE_PORT}" --argjson healthy "${healthy}" \
        --arg protocol_version "${MCP_PROTOCOL_VERSION}" '
        def value: if type == "object" and has("value") then .value else . end;
        [.instances[]? | select(. != null)] as $all
        | ($all | length) == 2
        and ([$all[] | select(.id == ($service + "-http") and .port == $http_port
            and .protocol == "mcp-streamable-http" and .version == "1.0.0" and .weight == 80
            and .metadata["mcp-endpoint-path"] == "/mcp")] | length == 1)
        and ([$all[] | select(.id == ($service + "-sse") and .port == $sse_port
            and .protocol == "mcp-sse" and .version == "2.0.0" and .weight == 120
            and .metadata["mcp-endpoint-path"] == "/sse")] | length == 1)
        and all($all[]; .host == "127.0.0.1" and ((.healthy | value) == $healthy)
            and ((.enable_health_check // .enableHealthCheck | value) == true)
            and (((.health_check // .healthCheck).heartbeat.ttl | value) == 6)
            and .metadata["mcp-server-name"] == $service
            and .metadata["mcp-protocol-version"] == $protocol_version)
    ' >/dev/null <<<"${response}"
}

wait_health() {
    for _ in $(seq 1 120); do
        if assert_instances "$1"; then return 0; fi
        sleep 0.25
    done
    echo "MCP instances did not reach healthy=$1" >&2
    instances >&2
    return 1
}

wait_zero() {
    local response
    for _ in $(seq 1 120); do
        response="$(instances)"
        if jq -e '[.instances[]? | select(. != null)] | length == 0' >/dev/null <<<"${response}"; then return 0; fi
        sleep 0.25
    done
    echo "MCP instances were not deregistered: ${response}" >&2
    return 1
}

verify_client() {
    local transport="$1" version="$2" response
    (cd "${BUILD_DIR}"
        ./client -server "${POLARIS_SERVER}" -polaris-port "${POLARIS_GRPC_PORT}" \
            -namespace "${NAMESPACE}" -service "${SERVICE_NAME}" -transport "${transport}" \
            -server-version "${version}" -protocol-version "${MCP_PROTOCOL_VERSION}" -message hello
    ) > "${LOG_DIR}/${transport}.json" 2> "${LOG_DIR}/${transport}.log"
    jq -e --arg expected "mcp|${SERVICE_NAME}|hello" \
        '.success == true and (.content | contains($expected)) and (.contract | length > 0)' \
        "${LOG_DIR}/${transport}.json" >/dev/null
    response="$(api_request GET "/naming/v1/service/contract/features?namespace=${NAMESPACE}&service=${SERVICE_NAME}&protocol=${transport}&version=${version}")"
    jq -e '.code == 200000 and ([.data[]? | select(.name == "echo")] | length == 1)' >/dev/null <<<"${response}"
}

revision() {
    local response
    response="$(instances)"
    jq -r --arg id "${SERVICE_NAME}-$1" '.instances[]? | select(.id == $id) | .revision // empty' <<<"${response}"
}

build_demo
ensure_namespace
ensure_mcp_service "${SERVICE_NAME}"
response="$(instances)"
if ! jq -e '[.instances[]? | select(. != null)] | length == 0' >/dev/null <<<"${response}"; then
    echo "Existing MCP instances must be inspected or explicitly cleaned first" >&2
    exit 1
fi
start_provider
wait_health true
echo "[case 5.1 MCP metadata and two protocols] PASS"
response="$(api_request GET "/naming/v1/services?namespace=${NAMESPACE}&name=${SERVICE_NAME}")"
jq -e --arg service "${SERVICE_NAME}" '
    .code == 200000 and ([.services[]? | select(.name == $service and .service_type == "SERVICE_TYPE_MCP_SERVER")] | length == 1)
' >/dev/null <<<"${response}"
echo "[case 5.2 MCP service type] PASS"
sleep 7
wait_health true
kill -STOP "${PROVIDER_PID}"
negative_status=0
set +e
wait_health false || negative_status=$?
kill -CONT "${PROVIDER_PID}"
set -e
if [[ "${negative_status}" -ne 0 ]]; then exit "${negative_status}"; fi
wait_health true
echo "[case 5.3 TTL and heartbeat] PASS"
verify_client mcp-streamable-http 1.0.0
verify_client mcp-sse 2.0.0
jq -e '(.contract | length > 0) and (.sseContract | length > 0)' "${BUILD_DIR}/ready.json" >/dev/null
echo "[case 5.4 two wire calls and Provider/Consumer contracts] PASS"
http_before="$(revision http)"
sse_before="$(revision sse)"
if [[ -z "${http_before}" || -z "${sse_before}" ]]; then echo "Empty initial revisions" >&2; exit 1; fi
stop_feature_processes
wait_zero
response="$(api_request GET "/naming/v1/services?namespace=${NAMESPACE}&name=${SERVICE_NAME}")"
jq -e --arg service "${SERVICE_NAME}" '
    .code == 200000 and ([.services[]? | select(.name == $service and .service_status == "OFFLINE" and .total_instance_count == 0)] | length == 1)
' >/dev/null <<<"${response}"
echo "[case 5.5 deregistration and offline service] PASS"
start_provider
wait_health true
http_after="$(revision http)"
sse_after="$(revision sse)"
if [[ -z "${http_after}" || -z "${sse_after}" || "${http_before}" == "${http_after}" || "${sse_before}" == "${sse_after}" ]]; then
    echo "Stable IDs did not receive new revisions after re-registration" >&2
    exit 1
fi
echo "[case 5.6 stable instance IDs and revision after re-registration] PASS"
stop_feature_processes
wait_zero
echo "[case 5.7 repeat deregistration] PASS"
