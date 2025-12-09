package main

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"

	"github.com/MakerMaker19/meerkatvpn/pkg/client"
	"github.com/MakerMaker19/meerkatvpn/pkg/vpn"
)

// Hard-code your pool + client keys for now.
// These are the HEX keys output by genkey.go (NOT npub1/nsec1 bech32).
const poolPrivHex = "b3fd885722af2e29f340cb1bddc8e420ad3fb6003fc828844c00b0e6b5200cd5"
const poolPubHex = "4cb03ad56b84dc22f4870a9a7412412bebce44d3ce7bf3233513478aaac31aaa"

const clientPubHex = "3b748c6c7e338eca9b0f3768d9a68a2b973e91c6ac180f80bac33baa1ef4452b"

func main() {
	// 1) Parse pool private key from hex
	privBytes, err := hex.DecodeString(poolPrivHex)
	if err != nil {
		panic(fmt.Errorf("decode poolPrivHex: %w", err))
	}

	// btcec/v2 API: only takes []byte, returns (*PrivateKey, PublicKey)
	priv, _ := btcec.PrivKeyFromBytes(privBytes)

	// 2) Build a payload valid for ~30 days
	now := time.Now()

	payload := vpn.SubscriptionPayload{
		// Use seconds + a slice of nanos to avoid collisions if you mint twice quickly.
		TokenID: fmt.Sprintf("dev-%d-%d", now.Unix(), now.UnixNano()%1e6),

		UserPubKey:       clientPubHex,
		SubscriptionType: "vpn",
		Tier:             "dev",

		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(30 * 24 * time.Hour).Unix(),

		Nonce:        fmt.Sprintf("dev-%d", now.UnixNano()),
		IssuerPubKey: poolPubHex,
	}

	// 3) Sign with pool key
	tok, err := vpn.SignSubscription(priv, payload)
	if err != nil {
		panic(fmt.Errorf("SignSubscription: %w", err))
	}

	// 4) Load existing store, add/update this token, and save
	ts, err := client.LoadTokenStore()
	if err != nil {
		panic(fmt.Errorf("LoadTokenStore: %w", err))
	}

	ts.AddOrUpdate(tok)

	if err := ts.Save(); err != nil {
		panic(fmt.Errorf("Save token store: %w", err))
	}

	fmt.Println("Minted dev token:")
	fmt.Println("  token_id     :", tok.Payload.TokenID)
	fmt.Println("  issuer_pub   :", tok.Payload.IssuerPubKey)
	fmt.Println("  user_pub     :", tok.Payload.UserPubKey)
	fmt.Println("  expires_at   :", time.Unix(tok.Payload.ExpiresAt, 0))
	fmt.Println()
	fmt.Println("Saved into ~/.meerkatvpn/tokens.json")
}
