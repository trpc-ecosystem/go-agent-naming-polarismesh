#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "${SCRIPT_DIR}/examples/ai-registry/verify_all.sh" ]]; then
    ROOT="$SCRIPT_DIR"
else
    ROOT="$(cd -- "${SCRIPT_DIR}/../../.." && pwd)"
fi
AI_DIR="${ROOT}/examples/ai-registry"
RUNS_ROOT="${CLOUD_RUNS_DIR:-${AI_DIR}/.cloud-runs}"
RUN_DIR=""
OUTPUT=""
PACK_ALL=false

usage() {
    cat <<EOF
Usage: $0 [options]

Options:
  --run-dir DIR       Pack one remote run
  --runs-root DIR     Run root; default: ${RUNS_ROOT}
  --all               Pack every run below --runs-root
  --output FILE       Archive path
  -h, --help          Show this help
EOF
}

die() {
    echo "ERROR: $*" >&2
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
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
        --all)
            PACK_ALL=true
            shift
            ;;
        --output)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            OUTPUT="$2"
            shift 2
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

command -v tar >/dev/null 2>&1 || die "tar is required"
command -v jq >/dev/null 2>&1 || die "jq is required"
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
    die "sha256sum or shasum is required"
fi

ENTRIES=()
BASE_DIR=""
if [[ -n "$RUN_DIR" ]]; then
    [[ -d "$RUN_DIR" ]] || die "run directory not found: $RUN_DIR"
    RUN_DIR="$(cd -- "$RUN_DIR" && pwd)"
    BASE_DIR="$(dirname "$RUN_DIR")"
    ENTRIES=("$(basename "$RUN_DIR")")
else
    [[ -d "$RUNS_ROOT" ]] || die "no cloud runs found below $RUNS_ROOT"
    RUNS_ROOT="$(cd -- "$RUNS_ROOT" && pwd)"
    while IFS= read -r directory; do
        ENTRIES+=("$(basename "$directory")")
    done < <(find "$RUNS_ROOT" -mindepth 1 -maxdepth 1 -type d | LC_ALL=C sort)
    [[ ${#ENTRIES[@]} -gt 0 ]] || die "no cloud runs found below $RUNS_ROOT"
    BASE_DIR="$RUNS_ROOT"
    PACK_ALL=true
fi

SUMMARIZER="${SCRIPT_DIR}/summarize-results.sh"
[[ -f "$SUMMARIZER" ]] || SUMMARIZER="${AI_DIR}/cloud/summarize-results.sh"
for entry in "${ENTRIES[@]}"; do
    if [[ -f "${BASE_DIR}/${entry}/results.json" ]]; then
        "$SUMMARIZER" "${BASE_DIR}/${entry}" >/dev/null || true
    fi
done

if [[ -z "$OUTPUT" ]]; then
    stamp="$(date '+%Y%m%d_%H%M%S')"
    if [[ "$PACK_ALL" == true ]]; then
        OUTPUT="${AI_DIR}/.cloud-runs/go-agent-naming-cloud-logs-all-${stamp}.tar.gz"
    else
        OUTPUT="${AI_DIR}/.cloud-runs/${ENTRIES[0]}-logs.tar.gz"
    fi
fi
mkdir -p "$(dirname "$OUTPUT")"
OUTPUT="$(cd -- "$(dirname "$OUTPUT")" && pwd)/$(basename "$OUTPUT")"

tar -czf "$OUTPUT" -C "$BASE_DIR" "${ENTRIES[@]}"
if command -v sha256sum >/dev/null 2>&1; then
    (
        cd -- "$(dirname "$OUTPUT")"
        sha256sum "$(basename "$OUTPUT")" > "$(basename "$OUTPUT").sha256"
    )
else
    (
        cd -- "$(dirname "$OUTPUT")"
        shasum -a 256 "$(basename "$OUTPUT")" > "$(basename "$OUTPUT").sha256"
    )
fi

echo "Log archive: ${OUTPUT}"
echo "Checksum:    ${OUTPUT}.sha256"
echo "Transfer:    scp $(whoami 2>/dev/null || echo user)@<cloud-host>:${OUTPUT} ."
