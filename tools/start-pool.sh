#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load secrets (never committed)
source "$SCRIPT_DIR/env.local"

echo "Starting Meerkat Pool..."

export MEERKAT_POOL_NOSTR_PRIVKEY="b3fd885722af2e29f340cb1bddc8e420ad3fb6003fc828844c00b0e6b5200cd5"
export MEERKAT_POOL_RELAYS="wss://relay.damus.io,wss://relay.primal.net,wss://nos.lol"
export MEERKAT_POOL_LN_WEBHOOK_SECRET="devsecret"
export MEERKAT_POOL_LN_WEBHOOK_ADDR=":8080"

export MEERKAT_POOL_WEEKLY_SATS="1500"
export MEERKAT_POOL_MONTHLY_SATS="5000"
export MEERKAT_POOL_YEARLY_SATS="45000"

cd "$REPO_ROOT"
go run ./cmd/poold
