#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load secrets (never committed)
source "$SCRIPT_DIR/env.local"

echo "Starting Meerkat Node Announcer (Discovery)..."

# ===== HARD ENFORCED IDENTITY =====
if [ -z "${MEERKAT_NODE_NSEC:-}" ]; then
  read -rp "Enter MEERKAT_NODE_NSEC (nsec...): " MEERKAT_NODE_NSEC
fi
if [ -z "${MEERKAT_NODE_NSEC:-}" ]; then
  echo "MEERKAT_NODE_NSEC is required. Aborting." >&2
  exit 1
fi

# ===== PUBLIC API =====
# If MEERKAT_NODE_IP is not set (env/local), prompt for it interactively.
if [ -z "${MEERKAT_NODE_IP:-}" ]; then
  read -rp "Enter public IP or DNS for this node (reachable by clients): " MEERKAT_NODE_IP
fi

if [ -z "${MEERKAT_NODE_IP:-}" ]; then
  echo "MEERKAT_NODE_IP is required (public IP or DNS). Aborting." >&2
  exit 1
fi

export MEERKAT_NODE_API_URL="http://${MEERKAT_NODE_IP}:9090"

# ===== POOL WE SERVE =====
if [ -z "${MEERKAT_POOL_PUBKEY:-}" ]; then
  echo "MEERKAT_POOL_PUBKEY is required. Aborting." >&2
  exit 1
fi

# ===== RELAYS (ONLY WORKING ONES) =====
if [ -z "${MEERKAT_NOSTR_RELAYS:-}" ]; then
  echo "MEERKAT_NOSTR_RELAYS is required. Aborting." >&2
  exit 1
fi

# ===== NODE METADATA =====
export MEERKAT_NODE_REGION="us-east-1"
export MEERKAT_NODE_COUNTRY="US"
export MEERKAT_NODE_CITY="nyc"
export MEERKAT_NODE_VERSION="0.1.0"
export MEERKAT_NODE_SCHEMA="1"
export MEERKAT_NODE_WEIGHT="1.0"

# ===== BACKEND ADVERTISEMENT =====
if [ -z "${MEERKAT_NODE_BACKENDS:-}" ]; then
  export MEERKAT_NODE_BACKENDS="openvpn"
fi

# ===== RE-ANNOUNCE EVERY 60s =====
export MEERKAT_NODE_ANNOUNCE_INTERVAL_SECS="60"

cd "$REPO_ROOT"
go run ./cmd/node-announce
