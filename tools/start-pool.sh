#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load secrets (never committed)
source "$SCRIPT_DIR/env.local"

echo "Starting Meerkat Pool..."

if [ -z "${MEERKAT_POOL_NOSTR_PRIVKEY:-}" ]; then
  echo "MEERKAT_POOL_NOSTR_PRIVKEY is required. Aborting." >&2
  exit 1
fi

if [ -z "${MEERKAT_POOL_RELAYS:-}" ]; then
  echo "MEERKAT_POOL_RELAYS is required. Aborting." >&2
  exit 1
fi

export MEERKAT_POOL_LN_WEBHOOK_SECRET="devsecret"
export MEERKAT_POOL_LN_WEBHOOK_ADDR=":8080"

export MEERKAT_POOL_WEEKLY_SATS="1500"
export MEERKAT_POOL_MONTHLY_SATS="5000"
export MEERKAT_POOL_YEARLY_SATS="45000"

cd "$REPO_ROOT"
go run ./cmd/poold
