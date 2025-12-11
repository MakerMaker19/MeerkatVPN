#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load shared secrets/env
source "$SCRIPT_DIR/env.local"

echo "Starting Meerkat Client (Discovery Mode)..."

if [ -z "${MEERKAT_CLIENT_POOL_PUBKEY:-}" ]; then
  echo "MEERKAT_CLIENT_POOL_PUBKEY is required. Aborting." >&2
  exit 1
fi

if [ -z "${MEERKAT_NOSTR_RELAYS:-}" ]; then
  echo "MEERKAT_NOSTR_RELAYS is required. Aborting." >&2
  exit 1
fi

# Use the repo-local client YAML (kills the hidden AppData config)
export MEERKAT_CLIENT_CONFIG="$REPO_ROOT/meerkat-client.yaml"

# Force identity + pool + relays + backend
export MEERKAT_CLIENT_NOSTR_PRIVKEY="$MEERKAT_CLIENT_NOSTR_PRIVKEY"
export MEERKAT_CLIENT_POOL_PUBKEY="$MEERKAT_POOL_PUBKEY"
export MEERKAT_NOSTR_RELAYS="$MEERKAT_NOSTR_RELAYS"
export MEERKAT_TUNNEL_BACKEND="openvpn"
export MEERKAT_DEBUG_DISCOVERY="${MEERKAT_DEBUG_DISCOVERY:-1}"

# Discovery mode (no hard-coded node)
unset MEERKAT_NODE_URL

cd "$REPO_ROOT"
go run ./cmd/client-cli connect
