# MeerkatVPN MVP Runbook

Goal: from zero → **working tunnel** using:

- 1 **pool**: issues subscription tokens over Nostr when invoices settle (simulated)
- 1 **node**: accepts tokens, provisions WireGuard sessions, announces itself on Nostr
- 1 **client**: receives tokens via Nostr, discovers nodes via Nostr, connects via HTTP to fetch a VPN config

This is intentionally opinionated and minimal. It’s not production, just a reproducible path.

---

## 0. Topology + names

We’ll assume:

- Pool VPS: `POOL_HOST` (e.g. `203.0.113.10`)
- Node VPS: `NODE_HOST` (e.g. `46.62.204.11`)
- Client: your Windows machine

Ports:

- Pool LN webhook: `:8080`
- Node API: `:9090`
- WireGuard server on node: `wg0` on `10.8.0.1/24`, port `51820`

Relays (you can adjust later):

```text
wss://relay.damus.io,wss://relay.primal.net
1. Prerequisites
On all three machines
Go installed (1.21+ recommended)

Git (if you’re cloning directly on the VPS)

On the node VPS
Linux with root or sudo

WireGuard tools:

bash
Copy code
sudo apt update
sudo apt install -y wireguard iproute2 iptables
On the Windows client
Go installed

OpenVPN or WireGuard client installed
For now the code is friendliest for OpenVPN (it writes meerkat.ovpn and on some OSes auto-copies it to the right folder).

2. Clone and build
You can either develop on Windows and scp binaries, or just clone on each server.

On each machine (pool, node, client):

bash
Copy code
git clone <your-repo-url> MeerkatVPN
cd MeerkatVPN/meerkatvpn

go build ./cmd/client-cli
go build ./cmd/poold
go build ./cmd/noded
go build ./cmd/node-announce
On Windows you’ll get client-cli.exe etc.

3. Generate Nostr keys (pool, node, client)
You need three Nostr identities:

Pool identity: POOL_NSEC / POOL_PUB_HEX

Node identity: NODE_NSEC / NODE_PUB_HEX

Client identity: CLIENT_NSEC / CLIENT_PUB_HEX

You can:

Use any external Nostr key tool you like, or

Use a small Go helper (you already have genkey.go.bak in the repo if you want to resurrect it).

For this runbook, we’ll assume you now have:

text
Copy code
POOL_NSEC        = nsec1...
POOL_PUB_HEX     = 64-char hex of pool pubkey

NODE_NSEC        = nsec1...
NODE_PUB_HEX     = 64-char hex of node pubkey

CLIENT_NSEC      = nsec1...
CLIENT_PUB_HEX   = 64-char hex of client pubkey
4. Start the pool (poold)
4.1 Environment variables
On the pool VPS:

bash
Copy code
cd ~/MeerkatVPN/meerkatvpn

export MEERKAT_POOL_NOSTR_PRIVKEY="$POOL_NSEC"        # nsec or hex; main.go handles both
export MEERKAT_CLIENT_POOL_PUBKEY="$POOL_PUB_HEX"     # the pool's pubkey in hex; clients will use this

# Where to listen for LN webhooks
export MEERKAT_POOL_LN_WEBHOOK_ADDR=":8080"

# Shared secret for the webhook (header X-Meerkat-Secret)
export MEERKAT_POOL_LN_WEBHOOK_SECRET="supersecretstring"

# Optional: sats pricing (defaults exist if not set)
export MEERKAT_POOL_WEEKLY_SATS="5000"
export MEERKAT_POOL_MONTHLY_SATS="15000"
export MEERKAT_POOL_YEARLY_SATS="150000"

# Optional: override relays (defaults to damus + primal)
export MEERKAT_POOL_RELAYS="wss://relay.damus.io,wss://relay.primal.net"
4.2 Run the pool
bash
Copy code
./poold
# or: go run ./cmd/poold
You should see logs along the lines of:

Connecting to relays

Starting pricing publisher

Listening on :8080 for /ln/webhook

5. Start the node (WireGuard + noded)
You already have a helper script: setup-meerkat-node.txt.

5.1 Run the node setup script
On the node VPS:

bash
Copy code
cd ~/MeerkatVPN

chmod +x setup-meerkat-node.txt
./setup-meerkat-node.txt
Before running, edit the top of the script:

Set PUBLIC_IP to the node’s actual public IP.

Set MEERKAT_NODE_ALLOWED_POOL_PUBKEY="$POOL_PUB_HEX" so this node only trusts tokens issued by your pool.

The script will:

Install and configure the WireGuard interface (wg0) on 10.8.0.1/24.

Set up IP forwarding + iptables MASQUERADE.

Print out the node’s WireGuard public key.

At the bottom, it prints export lines like:

bash
Copy code
export MEERKAT_NODE_LISTEN_ADDR="0.0.0.0:9090"
export MEERKAT_NODE_ALLOWED_POOL_PUBKEY="63f0..."
export MEERKAT_NODE_WG_PUBKEY="NODE_WG_PUBLIC_KEY"
export MEERKAT_NODE_WG_ENDPOINT="PUBLIC_IP:51820"
export MEERKAT_NODE_WG_INTERFACE="wg0"
export MEERKAT_NODE_WG_NETWORK="10.8.0.1/24"
export MEERKAT_NODE_WG_APPLY="1"

go run ./cmd/noded
Copy those export lines and run them in your shell:

bash
Copy code
export MEERKAT_NODE_LISTEN_ADDR="0.0.0.0:9090"
export MEERKAT_NODE_ALLOWED_POOL_PUBKEY="$POOL_PUB_HEX"
export MEERKAT_NODE_WG_PUBKEY="..."      # from script output
export MEERKAT_NODE_WG_ENDPOINT="PUBLIC_IP:51820"
export MEERKAT_NODE_WG_INTERFACE="wg0"
export MEERKAT_NODE_WG_NETWORK="10.8.0.1/24"
export MEERKAT_NODE_WG_APPLY="1"
5.2 Run the node daemon
Still on the node VPS:

bash
Copy code
cd ~/MeerkatVPN/meerkatvpn
./noded
# or: go run ./cmd/noded
This exposes an HTTP API on 0.0.0.0:9090 that:

Accepts a SubscriptionToken

Provisions a WireGuard peer

Returns a client config

(Exact JSON is in cmd/noded/main.go and pkg/vpn if you want to inspect it.)

6. Start Nostr node announcements (node-announce)
This is what lets the client discover the node via Nostr.

On the node VPS (another shell):

bash
Copy code
cd ~/MeerkatVPN/meerkatvpn

export MEERKAT_NODE_NSEC="$NODE_NSEC"
export MEERKAT_NODE_API_URL="http://$PUBLIC_IP:9090"
export MEERKAT_POOL_PUBKEY="$POOL_PUB_HEX"
export MEERKAT_NOSTR_RELAYS="wss://relay.damus.io,wss://relay.primal.net"

# Optional metadata
export MEERKAT_NODE_REGION="us-east"
export MEERKAT_NODE_COUNTRY="US"
export MEERKAT_NODE_CITY="Miami"
export MEERKAT_NODE_BACKENDS="wireguard,openvpn"
export MEERKAT_NODE_VERSION="0.0.1-mvp"
export MEERKAT_NODE_SCHEMA="meerkat-node-announcement/v1"
export MEERKAT_NODE_WEIGHT="100"
export MEERKAT_NODE_ANNOUNCE_INTERVAL_SECS="60"

./node-announce
# or: go run ./cmd/node-announce
node-announce will:

Convert MEERKAT_NODE_NSEC → hex privkey internally.

Derive the node pubkey.

Periodically publish node announcement events (kind 38383) to the relays, tagged with your pool pubkey.

7. Configure the client (Windows)
On your Windows machine:

powershell
Copy code
cd C:\path\to\MeerkatVPN\meerkatvpn
7.1 Subscription token listener (Nostr DM)
Set environment variables (PowerShell syntax):

powershell
Copy code
$env:MEERKAT_CLIENT_NOSTR_PRIVKEY = "<CLIENT_NSEC>"
$env:MEERKAT_CLIENT_RELAYS        = "wss://relay.damus.io,wss://relay.primal.net"
$env:MEERKAT_CLIENT_POOL_PUBKEY   = "<POOL_PUB_HEX>"
Start the listener:

powershell
Copy code
go run ./cmd/client-cli receive-tokens
This will:

Connect to the relays.

Subscribe to DMs.

Accept any valid SubscriptionToken signed by MEERKAT_CLIENT_POOL_PUBKEY.

Save them in a local token store.

Leave this running when you simulate the LN webhook in the next step so you can see a token arrive live.

8. Simulate Lightning webhook → subscription token
On the pool VPS, in another shell:

You’ll call the webhook endpoint exposed by poold.

The exact JSON shape is defined in pkg/pool/types.go as InvoiceWebhook. A typical payload looks like:

jsonc
Copy code
{
  "id": "dummy-invoice-id",
  "settled": true,
  "amount_sats": 15000,
  "metadata": {
    "purpose": "vpn-subscription",
    "plan": "monthly",
    "nostr_pubkey": "<CLIENT_PUB_HEX>"
  }
}
Send it with curl:

bash
Copy code
export WEBHOOK_SECRET="supersecretstring"   # same as MEERKAT_POOL_LN_WEBHOOK_SECRET

curl -v \
  -X POST "http://127.0.0.1:8080/ln/webhook" \
  -H "Content-Type: application/json" \
  -H "X-Meerkat-Secret: $WEBHOOK_SECRET" \
  -d '{
    "id": "mvp-test-invoice",
    "settled": true,
    "amount_sats": 15000,
    "metadata": {
      "purpose": "vpn-subscription",
      "plan": "monthly",
      "nostr_pubkey": "'$CLIENT_PUB_HEX'"
    }
  }'
If everything is wired correctly, on the client (where receive-tokens is running) you should see logs like:

Received DM event

Parsed subscription token

Verified signature

Stored token with expiry

You can then stop receive-tokens (Ctrl-C) once you’ve got a token.

Check stored tokens:

powershell
Copy code
go run ./cmd/client-cli list-tokens
You should see a list with your new TokenID, plan, expiry, and issuer equal to your pool pubkey.

9. Discover the node (watch-nodes / list-nodes)
Now test discovery and health.

On the client:

powershell
Copy code
$env:MEERKAT_NOSTR_RELAYS      = "wss://relay.damus.io,wss://relay.primal.net"
$env:MEERKAT_CLIENT_POOL_PUBKEY = "<POOL_PUB_HEX>"

go run ./cmd/client-cli watch-nodes
This will:

Call configureFinderFromEnv():

Start Nostr discovery: NewNostrFinder(relays, poolPub, GlobalRegistry())

Set RegistryFinder as the default finder.

Print a table of nodes (every few seconds) including:

ID, region, country, city

backends

health status

latency

last announce time

expiry / whether expired

You should see your node announced from NODE_HOST and marked healthy once health checks succeed.

You can also do a one-shot list:

powershell
Copy code
go run ./cmd/client-cli list-nodes
10. Connect and bring up the tunnel
Finally, use the client to request a session from the node and write an OpenVPN config.

On the client:

powershell
Copy code
# Tell the client which node URL to talk to if discovery isn't yet fully plugged into connect()
$env:MEERKAT_NODE_URL          = "http://<NODE_HOST>:9090"

# Choose backend
$env:MEERKAT_TUNNEL_BACKEND    = "openvpn"   # or "wireguard" to just save meerkat-wg.conf

go run ./cmd/client-cli connect
What happens:

Client loads the local token store and picks the latest valid SubscriptionToken.

Calls discovery to select a node (Nostr + registry).

Makes an HTTP request to the node’s /session/create API, passing the token.

Node:

Verifies the token (signature + expiry + issuer pool pubkey).

Allocates a WireGuard peer IP.

Returns a client config (OpenVPN or WireGuard).

Client writes:

meerkat.ovpn (for MEERKAT_TUNNEL_BACKEND=openvpn), possibly copying it to your OpenVPN profiles folder depending on OS, or

meerkat-wg.conf (for wireguard backend; currently just saved to disk, not auto-applied).

Then:

Import meerkat.ovpn into your OpenVPN client and connect, or

Load meerkat-wg.conf into your WireGuard client and activate it.

If everything’s correct, traffic from your Windows box will egress via the node VPS.

11. Sanity checks
From the node: sudo wg show wg0
You should see your client peer with some RX/TX bytes when connected.

From the client (while VPN is up):

powershell
Copy code
curl https://ifconfig.me   # or https://ipinfo.io/ip
The IP should match NODE_HOST (or its outbound IP), not your home IP.

From the pool logs: you should see:

Webhook processed

Subscription token issued

DM sent to the client