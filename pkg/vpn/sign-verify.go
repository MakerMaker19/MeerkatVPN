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

	// 2) Parse issuer pubkey from hex (Nostr style x-only pubkey).
	pubBytes, err := hex.DecodeString(tok.Payload.IssuerPubKey)
	if err != nil {
		return fmt.Errorf("invalid issuer pubkey hex: %w", err)
	}
	pub, err := schnorr.ParsePubKey(pubBytes)
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
