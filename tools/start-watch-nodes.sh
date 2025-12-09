#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$SCRIPT_DIR/env.local"

echo "Starting Meerkat Client (watch-nodes)..."

export MEERKAT_CLIENT_CONFIG="$REPO_ROOT/meerkat-client.yaml"
export MEERKAT_CLIENT_NOSTR_PRIVKEY="$MEERKAT_CLIENT_NOSTR_PRIVKEY"
export MEERKAT_CLIENT_POOL_PUBKEY="$MEERKAT_POOL_PUBKEY"
export MEERKAT_NOSTR_RELAYS="$MEERKAT_NOSTR_RELAYS"

cd "$REPO_ROOT"
go run ./cmd/client-cli watch-nodes
