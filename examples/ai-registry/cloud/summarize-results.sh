#!/usr/bin/env bash
set -euo pipefail

RUN_DIR=""
OUTPUT=""

usage() {
    cat <<'EOF'
Usage: ./summarize-results.sh RUN_DIR [--output FILE]

Writes a Markdown summary from manifest.json and results.json.
EOF
}

die() {
    echo "ERROR: $*" >&2
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --output)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            OUTPUT="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        -*)
            die "unknown argument: $1"
            ;;
        *)
            [[ -z "$RUN_DIR" ]] || die "only one run directory may be summarized"
            RUN_DIR="$1"
            shift
            ;;
    esac
done

[[ -n "$RUN_DIR" ]] || die "run directory is required"
[[ -d "$RUN_DIR" ]] || die "run directory not found: $RUN_DIR"
RUN_DIR="$(cd -- "$RUN_DIR" && pwd)"
OUTPUT="${OUTPUT:-${RUN_DIR}/analysis.md}"
mkdir -p "$(dirname "$OUTPUT")"

manifest_value() {
    local key="$1"
    if [[ -f "${RUN_DIR}/manifest.json" ]]; then
        jq -r --arg key "$key" '.[$key] // "<unknown>"' "${RUN_DIR}/manifest.json" 2>/dev/null
    else
        echo "<unknown>"
    fi
}

{
    echo "# Remote AI Registry E2E"
    echo
    echo "- Run ID: $(manifest_value run_id)"
    echo "- Package: $(manifest_value package)"
    echo "- Server: $(manifest_value server)"
    echo "- Namespace: $(manifest_value namespace)"
    echo "- Resources: $(manifest_value resources)"
    echo "- Started: $(manifest_value started_at)"
    echo "- Finished: $(manifest_value finished_at)"
    echo "- Exit code: $(manifest_value exit_code)"
    echo

    if [[ ! -f "${RUN_DIR}/results.json" ]]; then
        echo "No results.json was produced. Inspect run.log and provider logs."
        exit 0
    fi

    echo "- Passed: $(jq -r '.Passed // 0' "${RUN_DIR}/results.json")"
    echo "- Failed: $(jq -r '.Failed // 0' "${RUN_DIR}/results.json")"
    echo
    echo "| Case | Name | Status | Duration | Error |"
    echo "| --- | --- | --- | --- | --- |"
    jq -r '
        .Cases[]? |
        [
            (.ID // ""),
            (.Name // ""),
            (.Status // ""),
            (.Duration // ""),
            ((.Error // "") | gsub("\\|"; "\\|") | gsub("[\\r\\n]+"; " "))
        ] |
        "| " + join(" | ") + " |"
    ' "${RUN_DIR}/results.json"
} > "$OUTPUT"

cat "$OUTPUT"
