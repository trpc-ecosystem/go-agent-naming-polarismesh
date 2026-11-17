#!/bin/bash
set -euo pipefail
COMMON_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../internal" && pwd)"
source "${COMMON_DIR}/e2e_common.sh"
feature_common_init mcp
feature_usage() {
    cat <<'EOF'
Usage: ./examples/mcp/cleanup.sh [options]
  -f, --force             apply without prompting
  --dry-run               show planned cleanup
  --keep-logs             keep .build/.logs
  --delete-service        also delete the demo MCP service
  --polaris-server HOST   required with --delete-service
EOF
}
SERVICE_CONTRACT_CLEANUP=true
feature_cleanup "trpc.polaris.demo.mcp" "" "$@"
