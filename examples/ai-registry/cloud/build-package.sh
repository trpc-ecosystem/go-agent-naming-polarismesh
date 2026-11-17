#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../../.." && pwd)"
BUILD_ROOT="${SCRIPT_DIR}/.build"
OUTPUT_DIR="${SCRIPT_DIR}/dist"
PACKAGE_NAME=""
PACKAGE_NAME_EXPLICIT=false
TARGET_GOOS="linux"
TARGET_GOARCH="amd64"
KEEP_OLD=false

usage() {
    cat <<EOF
Usage: $0 [options]

Options:
  --output-dir DIR      Archive output directory; default: ${OUTPUT_DIR}
  --build-root DIR      Temporary build root; default: ${BUILD_ROOT}
  --name NAME           Package basename
  --goos OS             Target operating system; default: ${TARGET_GOOS}
  --goarch ARCH         Target architecture; default: ${TARGET_GOARCH}
  --keep-old            Keep older archives for this commit
  --clean               Remove local package build and output directories
  -h, --help            Show this help
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
        --build-root)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            BUILD_ROOT="$2"
            shift 2
            ;;
        --name)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            PACKAGE_NAME="$2"
            PACKAGE_NAME_EXPLICIT=true
            shift 2
            ;;
        --goos)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            TARGET_GOOS="$2"
            shift 2
            ;;
        --goarch)
            [[ $# -ge 2 ]] || die "$1 requires a value"
            TARGET_GOARCH="$2"
            shift 2
            ;;
        --keep-old)
            KEEP_OLD=true
            shift
            ;;
        --clean)
            rm -rf -- "$BUILD_ROOT" "$OUTPUT_DIR"
            echo "Removed ${BUILD_ROOT} and ${OUTPUT_DIR}"
            exit 0
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

for command in go git tar jq sed; do
    command -v "$command" >/dev/null 2>&1 || die "$command is required"
done
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
    die "sha256sum or shasum is required"
fi

VERSION="$(
    sed -n 's/^const Version = "\(.*\)"$/\1/p' "${REPO_ROOT}/version/version.go" |
        head -n 1
)"
[[ -n "$VERSION" ]] || die "cannot read adapter version"
GIT_COMMIT="$(git -C "$REPO_ROOT" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)"
if [[ -n "$(git -C "$REPO_ROOT" status --porcelain 2>/dev/null || true)" ]]; then
    GIT_DIRTY=true
else
    GIT_DIRTY=false
fi
STAMP="$(date '+%Y%m%d_%H%M%S')"
PACKAGE_NAME="${PACKAGE_NAME:-go-agent-naming-polarismesh-cloud-${VERSION}-${GIT_COMMIT}-${STAMP}}"
STAGE_DIR="${BUILD_ROOT}/${PACKAGE_NAME}"

rm -rf -- "$STAGE_DIR"
mkdir -p \
    "${STAGE_DIR}/examples/ai-registry/.build" \
    "${OUTPUT_DIR}"

echo "[1/5] Building target binaries"
(
    cd "$REPO_ROOT"
    export CGO_ENABLED=0
    export GOOS="$TARGET_GOOS"
    export GOARCH="$TARGET_GOARCH"
    go build -trimpath -ldflags="-s -w" \
        -o "${STAGE_DIR}/examples/ai-registry/.build/provider" \
        ./examples/ai-registry/provider
    go build -trimpath -ldflags="-s -w" \
        -o "${STAGE_DIR}/examples/ai-registry/.build/consumer" \
        ./examples/ai-registry/consumer
    go build -trimpath -ldflags="-s -w" \
        -o "${STAGE_DIR}/examples/ai-registry/.build/e2e" \
        ./examples/ai-registry/e2e
)
install -m 0755 \
    "${STAGE_DIR}/examples/ai-registry/.build/e2e" \
    "${STAGE_DIR}/examples/ai-registry/.build/cleanup"

echo "[2/5] Copying cloud entrypoints"
install -m 0755 "${SCRIPT_DIR}/run-e2e.sh" "${STAGE_DIR}/run-e2e.sh"
install -m 0755 "${SCRIPT_DIR}/cleanup-run.sh" "${STAGE_DIR}/cleanup-run.sh"
install -m 0755 "${SCRIPT_DIR}/pack-logs.sh" "${STAGE_DIR}/pack-logs.sh"
install -m 0755 "${SCRIPT_DIR}/import-logs.sh" "${STAGE_DIR}/import-logs.sh"
install -m 0755 "${SCRIPT_DIR}/summarize-results.sh" "${STAGE_DIR}/summarize-results.sh"
install -m 0755 "${REPO_ROOT}/examples/ai-registry/verify_all.sh" \
    "${STAGE_DIR}/examples/ai-registry/verify_all.sh"
install -m 0755 "${REPO_ROOT}/examples/ai-registry/cleanup.sh" \
    "${STAGE_DIR}/examples/ai-registry/cleanup.sh"
cp "${SCRIPT_DIR}/README.md" "${STAGE_DIR}/CLOUD-README.md"
cp "${SCRIPT_DIR}/README.zh_CN.md" "${STAGE_DIR}/CLOUD-README.zh_CN.md"
cp "${REPO_ROOT}/LICENSE" "${STAGE_DIR}/LICENSE"
cp "${REPO_ROOT}/README.md" "${STAGE_DIR}/README.md"
cp "${REPO_ROOT}/README.zh_CN.md" "${STAGE_DIR}/README.zh_CN.md"

echo "[3/5] Writing package manifest"
BUILD_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
GO_VERSION="$(go version)"
jq -n \
    --arg package "$PACKAGE_NAME" \
    --arg version "$VERSION" \
    --arg git_commit "$GIT_COMMIT" \
    --argjson git_dirty "$GIT_DIRTY" \
    --arg built_at "$BUILD_AT" \
    --arg target_goos "$TARGET_GOOS" \
    --arg target_goarch "$TARGET_GOARCH" \
    --arg go_version "$GO_VERSION" \
    '{
        package: $package,
        version: $version,
        git_commit: $git_commit,
        git_dirty: $git_dirty,
        built_at: $built_at,
        target_goos: $target_goos,
        target_goarch: $target_goarch,
        go_version: $go_version,
        runtime_bundled: true,
        entrypoints: [
            "run-e2e.sh",
            "cleanup-run.sh",
            "pack-logs.sh",
            "import-logs.sh",
            "summarize-results.sh"
        ]
    }' > "${STAGE_DIR}/manifest.json"

echo "[4/5] Writing checksums"
CHECKSUM_FILE="${STAGE_DIR}/SHA256SUMS"
while IFS= read -r file; do
    [[ "$file" == "$CHECKSUM_FILE" ]] && continue
    if command -v sha256sum >/dev/null 2>&1; then
        digest="$(sha256sum "$file" | awk '{print $1}')"
    else
        digest="$(shasum -a 256 "$file" | awk '{print $1}')"
    fi
    printf '%s  %s\n' "$digest" "${file#${STAGE_DIR}/}"
done < <(find "$STAGE_DIR" -type f | LC_ALL=C sort) > "$CHECKSUM_FILE"

echo "[5/5] Creating archive"
ARCHIVE="${OUTPUT_DIR}/${PACKAGE_NAME}.tar.gz"
if [[ "$KEEP_OLD" == false && "$PACKAGE_NAME_EXPLICIT" == false ]]; then
    find "$OUTPUT_DIR" -maxdepth 1 -type f \
        \( -name "go-agent-naming-polarismesh-cloud-${VERSION}-${GIT_COMMIT}-*.tar.gz" \
        -o -name "go-agent-naming-polarismesh-cloud-${VERSION}-${GIT_COMMIT}-*.tar.gz.sha256" \) \
        -delete
fi
tar -czf "$ARCHIVE" -C "$BUILD_ROOT" "$PACKAGE_NAME"
if command -v sha256sum >/dev/null 2>&1; then
    (
        cd -- "$OUTPUT_DIR"
        sha256sum "$(basename "$ARCHIVE")" > "$(basename "$ARCHIVE").sha256"
    )
else
    (
        cd -- "$OUTPUT_DIR"
        shasum -a 256 "$(basename "$ARCHIVE")" > "$(basename "$ARCHIVE").sha256"
    )
fi
rm -rf -- "$STAGE_DIR"

cat <<EOF

Package:  ${ARCHIVE}
Checksum: ${ARCHIVE}.sha256
Target:   ${TARGET_GOOS}/${TARGET_GOARCH}

Upload:
  scp ${ARCHIVE} ${ARCHIVE}.sha256 <user>@<cloud-host>:~/

Cloud host:
  tar xzf $(basename "$ARCHIVE")
  cd ${PACKAGE_NAME}
  ./run-e2e.sh --server <polaris-host> --resources keep
EOF
