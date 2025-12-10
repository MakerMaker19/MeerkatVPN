#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load shared env (keys, relays, etc.)
source "$SCRIPT_DIR/env.local"

echo "=== Meerkat DEV RESET (local) ==="

cd "$REPO_ROOT"

# 1) Kill any stale client-cli processes (ignore errors if none)
echo "[*] Killing old client-cli/watch-nodes/receive-tokens (if any)..."
if command -v pkill >/dev/null 2>&1; then
  pkill -f "cmd/client-cli" || true
  pkill -f "client-cli watch-nodes" || true
  pkill -f "client-cli receive-tokens" || true
else
  echo "pkill not found; skipping process cleanup"
fi

# 2) (Optional) Mint a fresh dev token if you want
# Uncomment if you ever want it automatic:
# echo "[*] Minting fresh dev token..."
# go run ./tools/mint_dev_token/main.go

# 3) Connect via discovery (with fallback baked into start-client.sh)
echo "[*] Starting client via discovery..."
"$SCRIPT_DIR/start-client.sh"
