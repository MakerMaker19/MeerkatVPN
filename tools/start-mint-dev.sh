#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load secrets (never committed)
source "$SCRIPT_DIR/env.local"

echo "Minting Meerkat Dev Token..."

cd "$REPO_ROOT/tools/mint_dev_token"
go run ./main.go
