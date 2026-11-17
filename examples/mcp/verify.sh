#!/bin/bash

set -euo pipefail

COMMON_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../internal" && pwd)"
source "${COMMON_DIR}/e2e_common.sh"

feature_common_init mcp
feature_usage() {
    echo "Usage: ./examples/mcp/verify.sh --polaris-server <host> [--debug]"
}
feature_parse_transport_args "$@"

SERVICE_NAME="trpc.polaris.demo.mcp"
PROVIDER_PORT="${PROVIDER_PORT:-28214}"
MCP_PROTOCOL_VERSION="${MCP_PROTOCOL_VERSION:-2025-03-26}"
MCP_TRANSPORT="${MCP_TRANSPORT:-mcp-streamable-http}"

build_demo() {
    (
        cd "${REPO_ROOT}"
        env GOCACHE="${GOCACHE:-/private/tmp/go-agent-naming-demo-cache}" \
            go build -o "${BUILD_DIR}/provider" ./examples/mcp/provider
        env GOCACHE="${GOCACHE:-/private/tmp/go-agent-naming-demo-cache}" \
            go build -o "${BUILD_DIR}/client" ./examples/mcp/client
    )
}

start_provider() {
    local debug_flag=()
    if [[ "${DEBUG_MODE:-false}" == "true" ]]; then
        debug_flag=(-debug)
    fi
    (
        cd "${BUILD_DIR}"
        exec ./provider \
            -server "${POLARIS_SERVER}" \
            -polaris-port "${POLARIS_GRPC_PORT}" \
            -namespace "${NAMESPACE}" \
            -service "${SERVICE_NAME}" \
            -port "${PROVIDER_PORT}" \
            -protocol-version "${MCP_PROTOCOL_VERSION}" \
            -ready "${BUILD_DIR}/ready.json" \
            ${debug_flag[@]+"${debug_flag[@]}"}
    ) > "${LOG_DIR}/provider.log" 2>&1 &
    local pid=$!
    if ! record_process "${pid}" provider "${PROVIDER_PORT}"; then
        stop_pid "${pid}"
        return 1
    fi
    PROVIDER_PID="${pid}"
}

verify_contract_features() {
    local response
    if ! response="$(api_request GET \
        "/naming/v1/service/contract/features?namespace=${NAMESPACE}&service=${SERVICE_NAME}&protocol=${MCP_TRANSPORT}&version=1.0.0")"; then
        echo "MCP Service Contract feature query failed" >&2
        return 1
    fi
    if ! jq -e '.code == 200000 and ([.data[]? | select(.name == "echo")] | length == 1)' \
        >/dev/null <<<"${response}"; then
        echo "MCP Service Contract feature was not persisted: ${response}" >&2
        return 1
    fi
}

build_demo
ensure_namespace
ensure_mcp_service "${SERVICE_NAME}"
start_provider
wait_for_instances "${SERVICE_NAME}" 1 "${PROVIDER_PID}"

(
    cd "${BUILD_DIR}"
    exec ./client \
        -server "${POLARIS_SERVER}" \
        -polaris-port "${POLARIS_GRPC_PORT}" \
        -namespace "${NAMESPACE}" \
        -service "${SERVICE_NAME}" \
        -protocol-version "${MCP_PROTOCOL_VERSION}" \
        -message hello
) > "${LOG_DIR}/client.json" 2> "${LOG_DIR}/client.log"

result="$(cat "${LOG_DIR}/client.json")"
if ! jq -e '
    .success == true and
    (.content | contains("mcp|trpc.polaris.demo.mcp|hello"))
' >/dev/null <<<"${result}"; then
    echo "MCP Polaris discovery/call failed: ${result}" >&2
    exit 1
fi
if ! jq -e '.contract | length > 0' "${LOG_DIR}/client.json" >/dev/null; then
    echo "MCP Service Contract was not queried through ConsumerAPI" >&2
    exit 1
fi
if ! jq -e '.contract | length > 0' "${BUILD_DIR}/ready.json" >/dev/null; then
    echo "MCP Service Contract was not reported" >&2
    exit 1
fi
verify_contract_features
echo "[case 4.1 MCP metadata registration and contract report] PASS"
echo "[case 4.2 MCP Polaris discovery, pool, and tool call] PASS"
echo "verification complete; Provider remains running until cleanup.sh"
