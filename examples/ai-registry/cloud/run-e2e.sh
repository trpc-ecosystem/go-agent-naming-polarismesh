#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "${SCRIPT_DIR}/examples/ai-registry/verify_all.sh" ]]; then
    ROOT="$SCRIPT_DIR"
else
    ROOT="$(cd -- "${SCRIPT_DIR}/../../.." && pwd)"
fi
AI_DIR="${ROOT}/examples/ai-registry"

SERVER="${POLARIS_SERVER:-}"
DISCOVER="${POLARIS_SERVER_ADDRESS:-}"
CONSOLE="${POLARIS_CONSOLE_ADDRESS:-}"
SKILL_ADDRESS="${POLARIS_SKILL_ADDRESS:-}"
NAMESPACE="${POLARIS_E2E_NAMESPACE:-go-agent-naming-e2e}"
RESOURCES="${POLARIS_E2E_RESOURCES:-keep}"
RUN_ID=""
RUNS_ROOT="${CLOUD_RUNS_DIR:-${AI_DIR}/.cloud-runs}"
TIMEOUT=""
RUN_TIMEOUT=""
CREATE_ARCHIVE=true
DEBUG=false
EXTRA_ARGS=()

usage() {
    cat <<EOF
Usage: $0 --server HOST [options] [-- extra E2E arguments]

Options:
  --server HOST          Polaris host; default: POLARIS_SERVER
  --discover HOST:PORT   Discovery gRPC; default: derived from --server
  --console URL          Console origin; default: derived from --server
  --skill-address HOST:PORT
                         Skill gRPC; default: derived from --server
  --namespace NAME       Isolated non-default namespace
  --resources POLICY     keep or cleanup; default: ${RESOURCES}
  --timeout DURATION     Per-operation timeout, for example 5s
  --run-timeout DURATION Full suite timeout, for example 12m
  --run-id ID            Stable run directory name
  --runs-root DIR        Run root; default: ${RUNS_ROOT}
  --debug                Enable SDK debug logs
  --no-archive           Do not create a log archive automatically
  -h, --help             Show this help

Tokens are read only from POLARIS_TOKEN, POLARIS_CONSOLE_TOKEN,
POLARIS_SKILL_TOKEN, and POLARIS_SERVICE_TOKEN.

The default keep policy preserves remote services, Skills, and the final
Provider so the run can be inspected. Use --resources cleanup only after
confirming that automatic ownership-scoped cleanup is desired.
EOF
}

die() {
    echo "ERROR: $*" >&2
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --server)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            SERVER="$2"
            shift 2
            ;;
        --discover)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            DISCOVER="$2"
            shift 2
            ;;
        --console)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            CONSOLE="$2"
            shift 2
            ;;
        --skill-address)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            SKILL_ADDRESS="$2"
            shift 2
            ;;
        --namespace)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            NAMESPACE="$2"
            shift 2
            ;;
        --resources)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            RESOURCES="$2"
            shift 2
            ;;
        --timeout)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            TIMEOUT="$2"
            shift 2
            ;;
        --run-timeout)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            RUN_TIMEOUT="$2"
            shift 2
            ;;
        --run-id)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            RUN_ID="$2"
            shift 2
            ;;
        --runs-root)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            RUNS_ROOT="$2"
            shift 2
            ;;
        --debug)
            DEBUG=true
            shift
            ;;
        --no-archive)
            CREATE_ARCHIVE=false
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        --)
            shift
            EXTRA_ARGS=("$@")
            break
            ;;
        *)
            die "unknown argument: $1"
            ;;
    esac
done

[[ -n "$SERVER" ]] || die "--server or POLARIS_SERVER is required"
[[ "$NAMESPACE" != "default" ]] || die "namespace must not be default"
[[ "$RESOURCES" == "keep" || "$RESOURCES" == "cleanup" ]] ||
    die "--resources must be keep or cleanup"
[[ -f "${AI_DIR}/verify_all.sh" ]] || die "verify_all.sh not found below ${ROOT}"

for command in bash tar jq ps; do
    command -v "$command" >/dev/null 2>&1 || die "$command is required"
done
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
    die "sha256sum or shasum is required"
fi

if [[ -n "$RUN_ID" && ! "$RUN_ID" =~ ^[A-Za-z0-9_.-]+$ ]]; then
    die "--run-id may contain only letters, digits, dot, underscore and dash"
fi
if [[ -n "$RUN_ID" && ${#RUN_ID} -gt 32 ]]; then
    die "--run-id must be at most 32 characters"
fi
RUN_ID="${RUN_ID:-cloud-$(date -u '+%Y%m%d-%H%M%S')-$$}"
RUN_DIR="${RUNS_ROOT}/${RUN_ID}"
[[ ! -e "$RUN_DIR" ]] || die "run directory already exists: $RUN_DIR"
mkdir -p "$RUN_DIR"
chmod 700 "$RUN_DIR"
RUN_LOG="${RUN_DIR}/run.log"
STARTED_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

log() {
    printf '%s\n' "$*" | tee -a "$RUN_LOG"
}

BIN_DIR="${AI_DIR}/.build"
if [[ -x "${BIN_DIR}/provider" && -x "${BIN_DIR}/consumer" && -x "${BIN_DIR}/e2e" ]]; then
    export GO_AGENT_E2E_SKIP_BUILD=true
else
    command -v go >/dev/null 2>&1 ||
        die "prebuilt binaries are missing and Go is not installed"
fi

export POLARIS_SERVER="$SERVER"
export POLARIS_E2E_NAMESPACE="$NAMESPACE"
export POLARIS_E2E_RESOURCES="$RESOURCES"

PACKAGE_NAME="$(basename "$ROOT")"
{
    echo "started_at=${STARTED_AT}"
    echo "hostname=$(hostname 2>/dev/null || true)"
    echo "uname=$(uname -a 2>/dev/null || true)"
    echo "server=${SERVER}"
    echo "namespace=${NAMESPACE}"
    echo "resources=${RESOURCES}"
    echo "run_id=${RUN_ID}"
    echo "POLARIS_TOKEN_set=$([[ -n "${POLARIS_TOKEN:-}" ]] && echo true || echo false)"
    echo "POLARIS_CONSOLE_TOKEN_set=$([[ -n "${POLARIS_CONSOLE_TOKEN:-}" ]] && echo true || echo false)"
    echo "POLARIS_SKILL_TOKEN_set=$([[ -n "${POLARIS_SKILL_TOKEN:-}" ]] && echo true || echo false)"
    echo "POLARIS_SERVICE_TOKEN_set=$([[ -n "${POLARIS_SERVICE_TOKEN:-}" ]] && echo true || echo false)"
} > "${RUN_DIR}/environment.txt"

jq -n \
    --arg run_id "$RUN_ID" \
    --arg package "$PACKAGE_NAME" \
    --arg server "$SERVER" \
    --arg namespace "$NAMESPACE" \
    --arg resources "$RESOURCES" \
    --arg started_at "$STARTED_AT" \
    '{
        run_id: $run_id,
        package: $package,
        server: $server,
        namespace: $namespace,
        resources: $resources,
        started_at: $started_at
    }' > "${RUN_DIR}/manifest.json"

append_secret_safe_base_opts() {
    local path="$1"
    printf '%s' "$path" | grep -Eq '^[^[:space:]]+$' ||
        die "access-bearing or whitespace-containing URL is not allowed"
}

COMMAND=(bash "${AI_DIR}/verify_all.sh"
    "--server=${SERVER}"
    "--resources=${RESOURCES}"
    "--run-id=${RUN_ID}"
    "--run-dir=${RUN_DIR}")
if [[ -n "$DISCOVER" ]]; then
    append_secret_safe_base_opts "$DISCOVER"
    COMMAND+=("--discover=${DISCOVER}")
fi
if [[ -n "$CONSOLE" ]]; then
    append_secret_safe_base_opts "$CONSOLE"
    COMMAND+=("--console=${CONSOLE}")
fi
if [[ -n "$SKILL_ADDRESS" ]]; then
    append_secret_safe_base_opts "$SKILL_ADDRESS"
    COMMAND+=("--skill-address=${SKILL_ADDRESS}")
fi
[[ -n "$TIMEOUT" ]] && COMMAND+=("--timeout=${TIMEOUT}")
[[ -n "$RUN_TIMEOUT" ]] && COMMAND+=("--run-timeout=${RUN_TIMEOUT}")
[[ "$DEBUG" == true ]] && COMMAND+=(--debug)
if [[ ${#EXTRA_ARGS[@]} -gt 0 ]]; then
    COMMAND+=("${EXTRA_ARGS[@]}")
fi

log "Run ID: ${RUN_ID}"
log "Package: ${PACKAGE_NAME}"
log "Server: ${SERVER}"
log "Namespace: ${NAMESPACE}"
log "Resources: ${RESOURCES}"
log "Started: ${STARTED_AT}"

set +e
"${COMMAND[@]}" 2>&1 | tee -a "$RUN_LOG"
EXIT_CODE=${PIPESTATUS[0]}
set -e

FINISHED_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
TMP_MANIFEST="${RUN_DIR}/manifest.json.tmp"
jq \
    --arg finished_at "$FINISHED_AT" \
    --argjson exit_code "$EXIT_CODE" \
    '. + {finished_at: $finished_at, exit_code: $exit_code}' \
    "${RUN_DIR}/manifest.json" > "$TMP_MANIFEST"
mv "$TMP_MANIFEST" "${RUN_DIR}/manifest.json"

SUMMARIZER="${SCRIPT_DIR}/summarize-results.sh"
[[ -f "$SUMMARIZER" ]] || SUMMARIZER="${AI_DIR}/cloud/summarize-results.sh"
"$SUMMARIZER" "$RUN_DIR" >/dev/null || true

log "Finished: ${FINISHED_AT}"
log "Exit code: ${EXIT_CODE}"
log "Run directory: ${RUN_DIR}"
log "Analysis: ${RUN_DIR}/analysis.md"

if [[ "$CREATE_ARCHIVE" == true ]]; then
    PACKER="${SCRIPT_DIR}/pack-logs.sh"
    [[ -f "$PACKER" ]] || PACKER="${AI_DIR}/cloud/pack-logs.sh"
    ARCHIVE="${RUNS_ROOT}/${RUN_ID}-logs.tar.gz"
    "$PACKER" --run-dir "$RUN_DIR" --output "$ARCHIVE" | tee -a "$RUN_LOG"
fi

exit "$EXIT_CODE"
