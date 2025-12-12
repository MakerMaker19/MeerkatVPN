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
# Prompt for node Nostr keypair and public IP/DNS if not set.
if [ -z "${MEERKAT_NODE_NSEC:-}" ]; then
  read -rp "Enter MEERKAT_NODE_NSEC (nsec...): " MEERKAT_NODE_NSEC
fi
if [ -z "${MEERKAT_NODE_NSEC:-}" ]; then
  echo "MEERKAT_NODE_NSEC is required. Aborting." >&2
  exit 1
fi

if [ -z "${MEERKAT_NODE_IP:-}" ]; then
  read -rp "Enter public IP or DNS for this node (reachable by clients): " MEERKAT_NODE_IP
fi
if [ -z "${MEERKAT_NODE_IP:-}" ]; then
  echo "MEERKAT_NODE_IP is required. Aborting." >&2
  exit 1
fi

if [ -z "${MEERKAT_NODE_ID:-}" ]; then
  read -rp "Enter MEERKAT_NODE_ID (optional, leave blank to use pubkey): " MEERKAT_NODE_ID
fi

export MEERKAT_NODE_API_URL="http://${MEERKAT_NODE_IP}:9090"

# Start node API in background so announcer can run in foreground.
cd "$REPO_ROOT"
go run ./cmd/noded &
NODE_PID=$!
echo "Node API started with PID ${NODE_PID}"
trap 'kill ${NODE_PID} 2>/dev/null || true' EXIT

# ===== RUN =====
# Run announcer in foreground (will log to stdout).
"$SCRIPT_DIR/start-announce.sh"
