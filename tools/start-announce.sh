#!/usr/bin/env bash
set -e

# Load secrets (never committed)
source "$(dirname "$0")/env.local"

echo "Starting Meerkat Node Announcer (Discovery)..."

# ===== HARD ENFORCED IDENTITY =====
export MEERKAT_NODE_NSEC="ce2903ed9ae571d7f706b35540ef917231099d27b3992f0a8b4536e71e61401e"

# ===== PUBLIC API =====
export MEERKAT_NODE_API_URL="http://46.62.204.11:9090"

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
