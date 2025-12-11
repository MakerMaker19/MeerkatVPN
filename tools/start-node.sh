#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load secrets (never committed)
source "$SCRIPT_DIR/env.local"

echo "Starting Meerkat Node API (OpenVPN)..."

# ===== HARD ENFORCED ENV =====
export MEERKAT_NODE_LISTEN_ADDR="0.0.0.0:9090"
export MEERKAT_NODE_ALLOWED_POOL_PUBKEY="4cb03ad56b84dc22f4870a9a7412412bebce44d3ce7bf3233513478aaac31aaa"
export MEERKAT_NODE_OVPN_PROFILE_PATH="/etc/openvpn/meerkat-client.ovpn"

# ===== RUN =====
cd "$REPO_ROOT"
go run ./cmd/noded
