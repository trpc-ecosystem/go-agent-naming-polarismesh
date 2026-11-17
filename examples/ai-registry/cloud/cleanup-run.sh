#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "${SCRIPT_DIR}/examples/ai-registry/cleanup.sh" ]]; then
    ROOT="$SCRIPT_DIR"
else
    ROOT="$(cd -- "${SCRIPT_DIR}/../../.." && pwd)"
fi
AI_DIR="${ROOT}/examples/ai-registry"
RUNS_ROOT="${CLOUD_RUNS_DIR:-${AI_DIR}/.cloud-runs}"
RUN_ID=""
RUN_DIR=""
APPLY=false
DRY_RUN=false
DELETE_SERVICE=false

usage() {
    cat <<EOF
Usage: $0 (--run-id ID | --run-dir DIR) [options]

Options:
  --runs-root DIR       Run root; default: ${RUNS_ROOT}
  --apply               Apply cleanup
  --dry-run             Preview cleanup; takes precedence over --apply
  --delete-service      Also delete owned services and Skills
  -h, --help            Show this help
EOF
}

die() {
    echo "ERROR: $*" >&2
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --run-id)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            RUN_ID="$2"
            shift 2
            ;;
        --run-dir)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            RUN_DIR="$2"
            shift 2
            ;;
        --runs-root)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            RUNS_ROOT="$2"
            shift 2
            ;;
        --apply)
            APPLY=true
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --delete-service)
            DELETE_SERVICE=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            die "unknown argument: $1"
            ;;
    esac
done

[[ -z "$RUN_ID" || -z "$RUN_DIR" ]] || die "--run-id and --run-dir are mutually exclusive"
if [[ -z "$RUN_DIR" ]]; then
    [[ -n "$RUN_ID" ]] || die "--run-id or --run-dir is required"
    RUN_DIR="${RUNS_ROOT}/${RUN_ID}"
fi
[[ -d "$RUN_DIR" ]] || die "run directory not found: $RUN_DIR"
STATE_FILE="${RUN_DIR}/state.json"
[[ -f "$STATE_FILE" ]] || die "state file not found: $STATE_FILE"

export GO_AGENT_E2E_SKIP_BUILD=true
COMMAND=(bash "${AI_DIR}/cleanup.sh" "--state=${STATE_FILE}")
[[ "$APPLY" == true ]] && COMMAND+=(--apply)
[[ "$DRY_RUN" == true ]] && COMMAND+=(--dry-run)
[[ "$DELETE_SERVICE" == true ]] && COMMAND+=(--delete-service)
"${COMMAND[@]}"
