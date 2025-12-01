# MeerkatVPN (prototype)

MeerkatVPN is a decentralized VPN subscription prototype built on top of:

- ⚡ Lightning-style webhooks for payments
- 🐀 Nostr for distributing subscription tokens
- 🧾 Cryptographically signed subscription tokens
- 🦫 WireGuard (planned) for actual VPN tunnels

This repo currently implements:

1. A **pool daemon** (`poold`) that:
   - Accepts a fake Lightning invoice webhook
   - Mints a signed subscription token
   - Sends it as a Nostr DM to the user

2. A **client CLI** (`client-cli`) that:
   - Listens for Nostr DMs
   - Extracts subscription tokens
   - Stores them under `~/.meerkatvpn/tokens.json`
   - Lists stored tokens

3. A **node daemon** (`noded`) – TODO / in progress:
   - Verifies subscription tokens
   - Issues (or will issue) WireGuard configs

> ⚠ This is **prototype / playground** code, not production-ready VPN software.

---

## Prerequisites

- Go 1.21+  
- git  
- A working internet connection (for Nostr relays)

---

## Key Concepts

### Subscription token

A subscription token is a signed structure:

```jsonc
{
  "payload": {
    "token_id": "sub_...",
    "user_pubkey": "hex pubkey",
    "subscription_type": "monthly|weekly|yearly",
    "tier": "full",
    "issued_at": 1234567890,
    "expires_at": 1234569999,
    "nonce": "uuid",
    "issuer_pubkey": "pool pubkey hex"
  },
  "signature": "hex-encoded Schnorr signature"
}


The pool signs payload with its private key; nodes will verify it.

Tokens are stored client-side at:

~/.meerkatvpn/tokens.json

Environment variables
Pool daemon (poold)
export MEERKAT_POOL_NOSTR_PRIVKEY="HEX_PRIVKEY"    # 64-char hex
export MEERKAT_POOL_LN_WEBHOOK_SECRET="testsecret" # shared secret for webhook auth
export MEERKAT_POOL_LN_WEBHOOK_ADDR=":8080"        # listen address
export MEERKAT_POOL_RELAYS="wss://relay.damus.io,wss://relay.primal.net"

# Optional pricing overrides
export MEERKAT_POOL_WEEKLY_SATS="1500"
export MEERKAT_POOL_MONTHLY_SATS="5000"
export MEERKAT_POOL_YEARLY_SATS="45000"

Client (client-cli)
export MEERKAT_CLIENT_NOSTR_PRIVKEY="HEX_PRIVKEY"           # same hex priv as pool in this test
export MEERKAT_CLIENT_RELAYS="wss://relay.damus.io,wss://relay.primal.net"
export MEERKAT_CLIENT_POOL_PUBKEY="POOL_PUBKEY_HEX"         # hex pubkey of pool (issuer)


In the current dev setup, pool and client share the same keypair for simplicity.

Running the pool + client

From repo root:

1. Start the pool daemon
go run ./cmd/poold
# logs:
# poold: listening on :8080 for LN webhooks...

2. Start the client listener

In another terminal:

go run ./cmd/client-cli receive-tokens
# logs:
# Client pubkey (hex): ...
# Listening for subscription tokens over Nostr DMs. Ctrl+C to stop.

3. Simulate a Lightning payment (fake webhook)

In a third terminal:

curl -X POST http://localhost:8080/ln/webhook \
  -H "Content-Type: application/json" \
  -H "X-Meerkat-Secret: testsecret" \
  -d '{
    "invoice_id": "test-123",
    "amount_sats": 5000,
    "settled": true,
    "metadata": {
      "purpose": "vpn-subscription",
      "plan": "monthly",
      "nostr_pubkey": "POOL_OR_CLIENT_PUBKEY_HEX"
    }
  }'


The flow:

poold logs token creation + DM send

client-cli receive-tokens logs Stored subscription token ...

Tokens accumulate in ~/.meerkatvpn/tokens.json

4. List stored tokens
go run ./cmd/client-cli list-tokens

Node daemon (noded)

noded (in progress) will:

Accept a subscription token over HTTP

Verify its signature and expiry

Eventually issue WireGuard credentials

See cmd/noded/main.go for details once implemented.


You can tweak references to your GitHub path or add architecture diagrams later.

---

## 2️⃣ Add token verification & a minimal `noded`

Next: node daemon that can **verify** the tokens your pool is issuing.

We’ll:

1. Make sure `vpn.SignSubscription` and `vpn.VerifySubscription` exist and match.
2. Add `cmd/noded/main.go` for a simple HTTP API:
   - `POST /session/create` with `{ "token": <SubscriptionToken JSON> }`
   - Verifies signature + expiry
   - Returns JSON `{"status":"ok"}` or `{"status":"error"...}`

### 2.1. Replace `pkg/vpn/sign_verify.go` with a consistent version

Open:

```text
pkg/vpn/sign-verify.go


Delete its contents and paste this (it keeps signing compatible and adds verify):

package vpn

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// SubscriptionPayload is the data that gets signed by the pool.
type SubscriptionPayload struct {
	TokenID          string `json:"token_id"`
	UserPubKey       string `json:"user_pubkey"`
	SubscriptionType string `json:"subscription_type"`
	Tier             string `json:"tier"`
	IssuedAt         int64  `json:"issued_at"`
	ExpiresAt        int64  `json:"expires_at"`
	Nonce            string `json:"nonce"`
	IssuerPubKey     string `json:"issuer_pubkey"`
}

// SubscriptionToken = payload + signature.
type SubscriptionToken struct {
	Payload   SubscriptionPayload `json:"payload"`
	Signature string              `json:"signature"` // hex-encoded Schnorr signature
}

// SignSubscription signs the payload with the pool's private key.
func SignSubscription(poolPriv *btcec.PrivateKey, payload SubscriptionPayload) (SubscriptionToken, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return SubscriptionToken{}, err
	}
	hash := sha256.Sum256(payloadBytes)

	sig, err := schnorr.Sign(poolPriv, hash[:])
	if err != nil {
		return SubscriptionToken{}, err
	}

	return SubscriptionToken{
		Payload:   payload,
		Signature: hex.EncodeToString(sig.Serialize()),
	}, nil
}

// VerifySubscription verifies the signature and basic validity (expiry).
//
// It uses Payload.IssuerPubKey as the public key.
func VerifySubscription(tok SubscriptionToken, now time.Time) error {
	// 1) Recreate the hash of the payload.
	payloadBytes, err := json.Marshal(tok.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	hash := sha256.Sum256(payloadBytes)

	// 2) Parse issuer pubkey from hex.
	pubBytes, err := hex.DecodeString(tok.Payload.IssuerPubKey)
	if err != nil {
		return fmt.Errorf("invalid issuer pubkey hex: %w", err)
	}
	pub, err := btcec.ParsePubKey(pubBytes)
	if err != nil {
		return fmt.Errorf("parse issuer pubkey: %w", err)
	}

	// 3) Parse signature.
	sigBytes, err := hex.DecodeString(tok.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature hex: %w", err)
	}
	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		return fmt.Errorf("parse signature: %w", err)
	}

	// 4) Verify Schnorr signature.
	if ok := sig.Verify(hash[:], pub); !ok {
		return fmt.Errorf("invalid subscription signature")
	}

	// 5) Check expiry.
	if tok.Payload.ExpiresAt <= now.Unix() {
		return fmt.Errorf("subscription expired at %d", tok.Payload.ExpiresAt)
	}

	return nil
}


Rebuild to ensure no errors:

go build ./...


(Existing tokens will still verify with this scheme because we’re hashing the same JSON payload the same way we sign it.)

2.2. Implement noded – token-verifying node daemon

Now we add a simple node HTTP server that:

Listens on MEERKAT_NODE_LISTEN_ADDR (e.g. :9090)

Accepts POST /session/create

Body: {"token": { ...SubscriptionToken... }}

Verifies with vpn.VerifySubscription

Returns JSON with status and reason

Create / replace:

cmd/noded/main.go


With:

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/MakerMaker19/meerkatvpn/pkg/vpn"
)

type sessionCreateRequest struct {
	Token vpn.SubscriptionToken `json:"token"`
}

type sessionCreateResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	// TODO: add WireGuard config fields here later.
}

func main() {
	addr := os.Getenv("MEERKAT_NODE_LISTEN_ADDR")
	if addr == "" {
		addr = ":9090"
	}

	allowedPool := os.Getenv("MEERKAT_NODE_ALLOWED_POOL_PUBKEY") // optional hex pubkey filter

	http.HandleFunc("/session/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req sessionCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Println("session create decode error:", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		tok := req.Token

		// Optional issuer pubkey filter.
		if allowedPool != "" && tok.Payload.IssuerPubKey != allowedPool {
			log.Printf("session create: issuer mismatch (got %s, expected %s)\n",
				tok.Payload.IssuerPubKey, allowedPool)
			writeJSON(w, http.StatusForbidden, sessionCreateResponse{
				Status:  "error",
				Message: "token issuer not allowed",
			})
			return
		}

		// Verify signature + expiry.
		if err := vpn.VerifySubscription(tok, time.Now()); err != nil {
			log.Println("session create: token invalid:", err)
			writeJSON(w, http.StatusForbidden, sessionCreateResponse{
				Status:  "error",
				Message: "invalid token: " + err.Error(),
			})
			return
		}

		// TODO: here we would allocate a WireGuard peer, IP, etc.
		log.Printf("session create: accepted token %s for user %s\n",
			tok.Payload.TokenID, tok.Payload.UserPubKey)

		writeJSON(w, http.StatusOK, sessionCreateResponse{
			Status:  "ok",
			Message: "session accepted (WireGuard config TBD)",
		})
	})

	log.Printf("noded: listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}


Rebuild to be safe:

go build ./...

2.3. Run noded and test it with a stored token

Start noded:

cd ~/onedrive/Desktop/MeerkatVPN/meerkatvpn

export MEERKAT_NODE_LISTEN_ADDR=":9090"
export MEERKAT_NODE_ALLOWED_POOL_PUBKEY="63f013dc88ab98befb662f278d938493bd0e44cde71afdbc5a69677325b498ad"

go run ./cmd/noded


You should see:

noded: listening on :9090


Grab a stored token from ~/.meerkatvpn/tokens.json

Open that file in an editor; you’ll see something like:

{
  "tokens": [
    {
      "payload": {
        "token_id": "sub_082a145b-e98f-48b6-9305-7938664da7e5",
        "user_pubkey": "63f0...",
        "subscription_type": "monthly",
        "tier": "full",
        "issued_at": 1764546375,
        "expires_at": 1767138375,
        "nonce": "23cf2acf-...",
        "issuer_pubkey": "63f0..."
      },
      "signature": "64af51..."
    }
  ]
}


Copy one full token object (just the { ... } inside tokens[0]).

Test /session/create with curl

In another terminal:

TOKEN_JSON='{
  "payload": {
    "token_id": "sub_082a145b-e98f-48b6-9305-7938664da7e5",
    "user_pubkey": "63f013dc88ab98befb662f278d938493bd0e44cde71afdbc5a69677325b498ad",
    "subscription_type": "monthly",
    "tier": "full",
    "issued_at": 1764546375,
    "expires_at": 1767138375,
    "nonce": "23cf2acf-798b-484d-bebe-a7ef776dee12",
    "issuer_pubkey": "63f013dc88ab98befb662f278d938493bd0e44cde71afdbc5a69677325b498ad"
  },
  "signature": "64af51794535a3f6a3cc3ef90f36f7e5e5e98de156bf756337c9e2fbed6d182ae240dc47332f27fc1464d7ea8ce94def6fb0dada37c04ead10e94f2af71f2ce9"
}'

curl -X POST http://localhost:9090/session/create \
  -H "Content-Type: application/json" \
  -d "{\"token\": $TOKEN_JSON}"


You should get back:

{"status":"ok","message":"session accepted (WireGuard config TBD)"}


And in the noded logs:

session create: accepted token sub_082a145b-e98f-48b6-9305-7938664da7e5 for user 63f0...


If you intentionally change expires_at to something in the past, or change the signature, you should see "status":"error" and the log explain why. That proves verification is actually happening.

3️⃣ Nostr DM reliability (what to improve next)

You already have basic success with Damus/Primal relays, but to harden things later:

Multiple relays + retries:

You already connect to multiple relays.

You could add backoff/retry on failed Publish / SendDM.

Health checks / pruning:

On repeated errors from a relay, drop it from the active list.

Queueing:

If all relays are temporarily down, queue DMs in memory (or on disk) to retry later.

Structured logging:

Add event IDs or correlation IDs in logs to trace a token from:

webhook → token → DM → client → token store → node verify

Your current behavior is perfectly fine for dev; these are “Phase 2” hardening tasks.

4️⃣ Upgrade to encrypted DMs (NIP-44) later

Right now, tokens are sent as plaintext JSON in kind-4 events. To move to NIP-44 (or NIP-04), the shape will be:

In nostrutil.Client.SendDM:

Derive shared secret between pool privkey and client pubkey.

Encrypt token JSON with NIP-44 scheme.

Store ciphertext base64 (or nip44 spec format) in Content.

In client.handleIncomingTokenEvent:

Decrypt event content using client privkey + pool pubkey.

Then json.Unmarshal the plaintext as you’re doing now.

We’d update:

pkg/nostrutil/client.go → SendDM to encrypt

pkg/client/listen.go → handleIncomingTokenEvent to decrypt

But the rest of the pipeline (poold → client store → noded verify) stays the same.