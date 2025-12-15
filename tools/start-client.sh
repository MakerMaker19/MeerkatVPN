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

# Force prompt for client Nostr privkey every run.
unset MEERKAT_CLIENT_NOSTR_PRIVKEY
read -rp "Enter MEERKAT_CLIENT_NOSTR_PRIVKEY (nsec...): " MEERKAT_CLIENT_NOSTR_PRIVKEY
if [ -z "${MEERKAT_CLIENT_NOSTR_PRIVKEY:-}" ]; then
  echo "MEERKAT_CLIENT_NOSTR_PRIVKEY is required. Aborting." >&2
  exit 1
fi
export MEERKAT_CLIENT_NOSTR_PRIVKEY

# Use the repo-local client YAML (kills the hidden AppData config)
export MEERKAT_CLIENT_CONFIG="$REPO_ROOT/meerkat-client.yaml"

# Force identity + pool + relays + backend
export MEERKAT_CLIENT_NOSTR_PRIVKEY="$MEERKAT_CLIENT_NOSTR_PRIVKEY"
export MEERKAT_CLIENT_POOL_PUBKEY="$MEERKAT_POOL_PUBKEY"
export MEERKAT_NOSTR_RELAYS="$MEERKAT_NOSTR_RELAYS"
export MEERKAT_TUNNEL_BACKEND="openvpn"
export MEERKAT_DEBUG_DISCOVERY="${MEERKAT_DEBUG_DISCOVERY:-1}"
# Auto-wait for a token if none are present (seconds); can override via env.
export MEERKAT_WAIT_FOR_TOKEN_SECS="${MEERKAT_WAIT_FOR_TOKEN_SECS:-30}"

# Discovery mode (no hard-coded node)
unset MEERKAT_NODE_URL

# If no token is present and pool API is configured, auto-run subscribe first.
if [ -n "${MEERKAT_POOL_API_URL:-}" ]; then
  # Try to see if we already have a valid token; if not, invoke subscribe.
  if ! go run ./cmd/client-cli check-tokens >/dev/null 2>&1; then
    echo "No valid token detected; calling subscribe (pool API: $MEERKAT_POOL_API_URL)..."
    MEERKAT_SUBSCRIBE_PLAN="${MEERKAT_SUBSCRIBE_PLAN:-monthly}" go run ./cmd/client-cli subscribe
  fi
fi

cd "$REPO_ROOT"
go run ./cmd/client-cli connect
