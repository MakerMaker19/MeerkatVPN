#!/usr/bin/env bash

#!/usr/bin/env bash
set -e

# Load secrets (never committed)
source "$(dirname "$0")/env.local"

echo "Starting Meerkat Node API (OpenVPN)..."

# ===== HARD ENFORCED ENV =====
export MEERKAT_NODE_LISTEN_ADDR="0.0.0.0:9090"
export MEERKAT_NODE_ALLOWED_POOL_PUBKEY="4cb03ad56b84dc22f4870a9a7412412bebce44d3ce7bf3233513478aaac31aaa"
export MEERKAT_NODE_OVPN_PROFILE_PATH="/etc/openvpn/meerkat-client.ovpn"

# ===== RUN =====
cd /root/meerkatvpn/meerkatvpn
go run ./cmd/noded
