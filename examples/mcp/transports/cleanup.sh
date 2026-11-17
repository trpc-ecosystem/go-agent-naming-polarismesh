#!/bin/bash
set -euo pipefail
COMMON_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../internal" && pwd)"
source "${COMMON_DIR}/e2e_common.sh"
feature_common_init mcp-transports
feature_usage() {
    echo "Usage: ./examples/mcp/transports/cleanup.sh [--force] [--dry-run] [--delete-service --polaris-server HOST]"
}
SERVICE_CONTRACT_CLEANUP=true
feature_cleanup "trpc.polaris.demo.mcp-transports" "" "$@"
