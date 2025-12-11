#!/usr/bin/env bash
set -e

# Load secrets (never committed)
source "$(dirname "$0")/env.local"

echo "Starting Meerkat Node Announcer (Discovery)..."

# ===== HARD ENFORCED IDENTITY =====
export MEERKAT_NODE_NSEC="ce2903ed9ae571d7f706b35540ef917231099d27b3992f0a8b4536e71e61401e"

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
export MEERKAT_POOL_PUBKEY="4cb03ad56b84dc22f4870a9a7412412bebce44d3ce7bf3233513478aaac31aaa"

# ===== RELAYS (ONLY WORKING ONES) =====
export MEERKAT_NOSTR_RELAYS="wss://relay.damus.io,wss://relay.primal.net,wss://nos.lol"

# ===== NODE METADATA =====
export MEERKAT_NODE_REGION="us-east-1"
export MEERKAT_NODE_COUNTRY="US"
export MEERKAT_NODE_CITY="nyc"
export MEERKAT_NODE_VERSION="0.1.0"
export MEERKAT_NODE_SCHEMA="1"
export MEERKAT_NODE_WEIGHT="1.0"

# ===== BACKEND ADVERTISEMENT =====
export MEERKAT_NODE_BACKENDS="openvpn"

# ===== RE-ANNOUNCE EVERY 60s =====
export MEERKAT_NODE_ANNOUNCE_INTERVAL_SECS="60"

cd /root/meerkatvpn/meerkatvpn
go run ./cmd/node-announce
