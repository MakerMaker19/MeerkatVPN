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

# Start announcer in the background (will prompt for node nsec/IP as needed).
echo "Starting node announcer in background..."
"$SCRIPT_DIR/start-announce.sh" &

# ===== RUN =====
cd "$REPO_ROOT"
go run ./cmd/noded
