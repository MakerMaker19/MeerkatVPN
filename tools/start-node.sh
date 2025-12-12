#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load secrets (never committed)
source "$SCRIPT_DIR/env.local"

echo "Starting Meerkat Node API (OpenVPN)..."

# ===== HARD ENFORCED ENV =====
export MEERKAT_NODE_LISTEN_ADDR="0.0.0.0:9090"
export MEERKAT_NODE_OVPN_PROFILE_PATH="/etc/openvpn/meerkat-client.ovpn"

if [ -z "${MEERKAT_NODE_ALLOWED_POOL_PUBKEY:-}" ]; then
  if [ -n "${MEERKAT_POOL_PUBKEY:-}" ]; then
    export MEERKAT_NODE_ALLOWED_POOL_PUBKEY="$MEERKAT_POOL_PUBKEY"
  else
    echo "MEERKAT_NODE_ALLOWED_POOL_PUBKEY (or MEERKAT_POOL_PUBKEY) is required. Aborting." >&2
    exit 1
  fi
fi

# ===== RUN =====
cd "$REPO_ROOT"

# Start node API in background so announcer can run in foreground.
# Send node logs to a file so prompts stay readable.
NODE_LOG="${REPO_ROOT}/noded.log"
go run ./cmd/noded >"$NODE_LOG" 2>&1 &
NODE_PID=$!
echo "Node API started with PID ${NODE_PID} (logs: $NODE_LOG)"
trap 'kill ${NODE_PID} 2>/dev/null || true' EXIT

# Run announcer in foreground (handles nsec/IP prompts).
"$SCRIPT_DIR/start-announce.sh"
