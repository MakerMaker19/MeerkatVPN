#!/usr/bin/env bash
set -e

# Load secrets (never committed)
source "$(dirname "$0")/env.local"

echo "Minting Meerkat Dev Token..."

cd ~/onedrive/Desktop/MeerkatVPN/meerkatvpn/tools/mint_dev_token
go run ./main.go

