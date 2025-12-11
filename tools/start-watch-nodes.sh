#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$SCRIPT_DIR/env.local"

echo "Starting Meerkat Client (watch-nodes)..."

if [ -z "${MEERKAT_CLIENT_POOL_PUBKEY:-}" ]; then
  echo "MEERKAT_CLIENT_POOL_PUBKEY is required. Aborting." >&2
  exit 1
fi

if [ -z "${MEERKAT_NOSTR_RELAYS:-}" ]; then
  echo "MEERKAT_NOSTR_RELAYS is required. Aborting." >&2
  exit 1
fi

export MEERKAT_CLIENT_CONFIG="$REPO_ROOT/meerkat-client.yaml"
export MEERKAT_CLIENT_NOSTR_PRIVKEY="$MEERKAT_CLIENT_NOSTR_PRIVKEY"
export MEERKAT_CLIENT_POOL_PUBKEY="$MEERKAT_POOL_PUBKEY"
export MEERKAT_NOSTR_RELAYS="$MEERKAT_NOSTR_RELAYS"
export MEERKAT_DEBUG_DISCOVERY="${MEERKAT_DEBUG_DISCOVERY:-1}"

cd "$REPO_ROOT"
go run ./cmd/client-cli watch-nodes
