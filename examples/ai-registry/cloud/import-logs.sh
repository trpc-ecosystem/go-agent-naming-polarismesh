#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "${SCRIPT_DIR}/examples/ai-registry/verify_all.sh" ]]; then
    ROOT="$SCRIPT_DIR"
    OUTPUT_ROOT="${ROOT}/imported"
else
    ROOT="$(cd -- "${SCRIPT_DIR}/../../.." && pwd)"
    OUTPUT_ROOT="${ROOT}/examples/ai-registry/.cloud-runs/imported"
fi

ARCHIVE=""
OUTPUT_DIR=""
FORCE=false

usage() {
    cat <<EOF
Usage: $0 ARCHIVE [options]

Options:
  --output-dir DIR    Extraction directory
  --force             Replace an existing extraction directory
  -h, --help          Show this help

An adjacent ARCHIVE.sha256 file is verified when present.
EOF
}

die() {
    echo "ERROR: $*" >&2
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --output-dir)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --force)
            FORCE=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        -*)
            die "unknown argument: $1"
            ;;
        *)
            [[ -z "$ARCHIVE" ]] || die "only one archive may be imported"
            ARCHIVE="$1"
            shift
            ;;
    esac
done

[[ -n "$ARCHIVE" ]] || die "archive is required"
[[ -f "$ARCHIVE" ]] || die "archive not found: $ARCHIVE"
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
    die "sha256sum or shasum is required"
fi
ARCHIVE="$(cd -- "$(dirname "$ARCHIVE")" && pwd)/$(basename "$ARCHIVE")"
base="$(basename "$ARCHIVE")"
base="${base%.tar.gz}"
base="${base%.tgz}"
OUTPUT_DIR="${OUTPUT_DIR:-${OUTPUT_ROOT}/${base}}"

if [[ -e "$OUTPUT_DIR" ]]; then
    [[ "$FORCE" == true ]] || die "output exists: $OUTPUT_DIR (use --force)"
    rm -rf -- "$OUTPUT_DIR"
fi
mkdir -p "$OUTPUT_DIR"
chmod 700 "$OUTPUT_DIR"

sidecar="${ARCHIVE}.sha256"
if [[ -f "$sidecar" ]]; then
    expected="$(awk 'NR == 1 {print $1}' "$sidecar")"
    if command -v sha256sum >/dev/null 2>&1; then
        actual="$(sha256sum "$ARCHIVE" | awk '{print $1}')"
    else
        actual="$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')"
    fi
    [[ "$expected" == "$actual" ]] || die "checksum mismatch for $ARCHIVE"
fi

while IFS= read -r member; do
    normalized="${member#./}"
    if [[ -z "$normalized" || "$normalized" == /* || "$normalized" == *\\* ||
        "/$normalized/" == *"/../"* || "$normalized" == ".." || "$normalized" == */.. ]]; then
        die "unsafe archive path: $member"
    fi
done < <(tar -tzf "$ARCHIVE")

while IFS= read -r line; do
    mode="${line:0:1}"
    [[ "$mode" == "-" || "$mode" == "d" ]] ||
        die "archive may contain only regular files and directories"
    permissions="${line%% *}"
    if [[ ${permissions:3:1} == [Ss] || ${permissions:6:1} == [Ss] ]]; then
        die "archive setuid/setgid entries are not accepted"
    fi
done < <(tar -tvzf "$ARCHIVE")

(
    umask 077
    tar -xzf "$ARCHIVE" -C "$OUTPUT_DIR"
)

entries=()
while IFS= read -r path; do
    entries+=("$path")
done < <(find "$OUTPUT_DIR" -mindepth 1 -maxdepth 1 | LC_ALL=C sort)
RUN_DIR="$OUTPUT_DIR"
if [[ ${#entries[@]} -eq 1 && -d "${entries[0]}" ]]; then
    RUN_DIR="${entries[0]}"
fi

SUMMARIZER="${SCRIPT_DIR}/summarize-results.sh"
[[ -f "$SUMMARIZER" ]] || SUMMARIZER="${ROOT}/examples/ai-registry/cloud/summarize-results.sh"
if [[ -f "$SUMMARIZER" ]]; then
    "$SUMMARIZER" "$RUN_DIR" >/dev/null || true
fi

echo "Imported: ${OUTPUT_DIR}"
echo "Analysis: ${RUN_DIR}/analysis.md"
