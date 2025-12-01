package nostrutil

import (
	"encoding/hex"
	"errors"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

// ParsedKey holds a hex private+public key pair.
type ParsedKey struct {
	PrivHex string
	PubHex  string
}

// ParsePrivKey accepts:
// - nsec1...  (NIP-19)
// - hex-encoded 32-byte privkey
func ParsePrivKey(s string) (*ParsedKey, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("empty key")
	}

	// NIP-19 nsec
	if strings.HasPrefix(s, "nsec1") {
		_, data, err := nip19.Decode(s)
		if err != nil {
			return nil, err
		}
		privBytes, ok := data.([]byte)
		if !ok {
			return nil, errors.New("invalid nsec payload")
		}
		privHex := hex.EncodeToString(privBytes)
		pubHex, err := nostr.GetPublicKey(privHex)
		if err != nil {
			return nil, err
		}
		return &ParsedKey{PrivHex: privHex, PubHex: pubHex}, nil
	}

	// Assume raw hex
	if _, err := hex.DecodeString(s); err != nil {
		return nil, errors.New("invalid privkey: not nsec and not hex")
	}
	pubHex, err := nostr.GetPublicKey(s)
	if err != nil {
		return nil, err
	}
	return &ParsedKey{PrivHex: s, PubHex: pubHex}, nil
}

// ParsePubKey accepts:
// - npub1...
// - hex
func ParsePubKey(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("empty pubkey")
	}

	if strings.HasPrefix(s, "npub1") {
		_, data, err := nip19.Decode(s)
		if err != nil {
			return "", err
		}
		pubHex, ok := data.(string)
		if !ok {
			return "", errors.New("invalid npub payload")
		}
		return pubHex, nil
	}

	// Assume hex
	if _, err := hex.DecodeString(s); err != nil {
		return "", errors.New("invalid pubkey: not npub and not hex")
	}
	return s, nil
}
