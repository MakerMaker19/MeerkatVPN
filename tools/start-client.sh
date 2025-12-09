#!/usr/bin/env bash
set -e

# Load secrets (never committed)
source "$(dirname "$0")/env.local"

echo "Starting Meerkat Client (Discovery Mode)..."

# ===== FORCE CONFIG PATH (OVERRIDES ALL HIDDEN FILES) =====
export MEERKAT_CLIENT_CONFIG="/dev/null"

# ===== FORCE CLIENT ID =====
export MEERKAT_CLIENT_NOSTR_PRIVKEY="2f919a8ded33fe98fb99a8ae5e81191f9f1144a54fcf654c33753afe940d6f32"

# ===== FORCE POOL TRUST =====
export MEERKAT_CLIENT_POOL_PUBKEY="4cb03ad56b84dc22f4870a9a7412412bebce44d3ce7bf3233513478aaac31aaa"

# ===== FORCE DISCOVERY RELAYS =====
export MEERKAT_NOSTR_RELAYS="wss://relay.damus.io,wss://relay.primal.net,wss://nos.lol"

# ===== FORCE BACKEND =====
export MEERKAT_TUNNEL_BACKEND="openvpn"

# ===== FORCE DISCOVERY MODE (NO DIRECT NODE) =====
unset MEERKAT_NODE_URL

# ===== RUN =====
cd ~/onedrive/Desktop/MeerkatVPN/meerkatvpn
go run ./cmd/client-cli connect
