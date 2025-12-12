#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load secrets (never committed)
source "$SCRIPT_DIR/env.local"

install_tmux() {
  if command -v tmux >/dev/null 2>&1; then
    return 0
  fi
  if [ -f /etc/debian_version ]; then
    sudo apt-get update && sudo apt-get install -y tmux
  elif [ -f /etc/redhat-release ] || [ -f /etc/centos-release ]; then
    sudo yum install -y tmux
  else
    return 1
  fi
}

echo "Starting Meerkat Node API (OpenVPN)..."

# ===== HARD ENFORCED ENV =====
export MEERKAT_NODE_LISTEN_ADDR="0.0.0.0:9090"
export MEERKAT_NODE_OVPN_PROFILE_PATH="/etc/openvpn/meerkat-client.ovpn"

if [ -z "${MEERKAT_NODE_ALLOWED_POOL_PUBKEY:-}" ]; then
  if [ -n "${MEERKAT_POOL_PUBKEY:-}" ]; then
    export MEERKAT_NODE_ALLOWED_POOL_PUBKEY="$MEERKAT_POOL_PUBKEY"
  else
    echo "MEERKAT_NODE_ALLOWED_POOL_PUBKEY (or MEERKAT_POOL_PUBKEY) is required. Aborting." >&2
    exit 1
  fi
fi
# Prompt for node Nostr keypair and public IP/DNS if not set.
if [ -z "${MEERKAT_NODE_NSEC:-}" ]; then
  read -rp "Enter MEERKAT_NODE_NSEC (nsec...): " MEERKAT_NODE_NSEC
fi
if [ -z "${MEERKAT_NODE_NSEC:-}" ]; then
  echo "MEERKAT_NODE_NSEC is required. Aborting." >&2
  exit 1
fi

if [ -z "${MEERKAT_NODE_IP:-}" ]; then
  read -rp "Enter public IP or DNS for this node (reachable by clients): " MEERKAT_NODE_IP
fi
if [ -z "${MEERKAT_NODE_IP:-}" ]; then
  echo "MEERKAT_NODE_IP is required. Aborting." >&2
  exit 1
fi

if [ -z "${MEERKAT_NODE_ID:-}" ]; then
  read -rp "Enter MEERKAT_NODE_ID (optional, leave blank to use pubkey): " MEERKAT_NODE_ID
fi

export MEERKAT_NODE_API_URL="http://${MEERKAT_NODE_IP}:9090"

# ===== RUN =====
cd "$REPO_ROOT"

# Try tmux; auto-install if possible, else fall back to single terminal.
if ! command -v tmux >/dev/null 2>&1; then
  install_tmux || true
fi

if command -v tmux >/dev/null 2>&1; then
  SESSION="meerkat-node"
  tmux kill-session -t "$SESSION" 2>/dev/null || true
  tmux new-session -d -s "$SESSION" -n api "cd \"$REPO_ROOT\" && go run ./cmd/noded"
  tmux new-window -t "$SESSION" -n announce "cd \"$REPO_ROOT\" && \"$SCRIPT_DIR/start-announce.sh\""
  echo "Started tmux session '$SESSION' with windows: api, announce"
  echo "Attach with: tmux attach -t $SESSION"
  tmux attach -t "$SESSION"
else
  echo "tmux not available; starting node API in background and announcer in foreground."
  go run ./cmd/noded &
  NODE_PID=$!
  echo "Node API started with PID ${NODE_PID}"
  trap 'kill ${NODE_PID} 2>/dev/null || true' EXIT
  "$SCRIPT_DIR/start-announce.sh"
fi
