package client

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/MakerMaker19/meerkatvpn/pkg/vpn"
)

type TokenStore struct {
	Tokens []vpn.SubscriptionToken `json:"tokens"`
}

func tokenStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".meerkatvpn")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "tokens.json"), nil
}

func LoadTokenStore() (*TokenStore, error) {
	path, err := tokenStorePath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &TokenStore{Tokens: []vpn.SubscriptionToken{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var ts TokenStore
	if err := json.Unmarshal(b, &ts); err != nil {
		return nil, err
	}
	ts.CleanExpired(time.Now())
	return &ts, nil
}

func (ts *TokenStore) Save() error {
	ts.CleanExpired(time.Now())

	path, err := tokenStorePath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(ts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// AddOrUpdate inserts or replaces a token by TokenID.
func (ts *TokenStore) AddOrUpdate(tok vpn.SubscriptionToken) {
	for i, existing := range ts.Tokens {
		if existing.Payload.TokenID == tok.Payload.TokenID {
			ts.Tokens[i] = tok
			return
		}
	}
	ts.Tokens = append(ts.Tokens, tok)
}

// CleanExpired drops expired tokens in-place.
func (ts *TokenStore) CleanExpired(now time.Time) {
	out := ts.Tokens[:0]
	for _, t := range ts.Tokens {
		if t.Payload.ExpiresAt > now.Unix() {
			out = append(out, t)
		}
	}
	ts.Tokens = out
}

// LatestValid returns the latest non-expired token from a given issuer.
func (ts *TokenStore) LatestValid(poolPub string, now time.Time) (*vpn.SubscriptionToken, error) {
	var best *vpn.SubscriptionToken
	for i := range ts.Tokens {
		t := &ts.Tokens[i]
		if poolPub != "" && t.Payload.IssuerPubKey != poolPub {
			continue
		}
		if t.Payload.ExpiresAt <= now.Unix() {
			continue
		}
		if best == nil || t.Payload.ExpiresAt > best.Payload.ExpiresAt {
			best = t
		}
	}
	if best == nil {
		return nil, errors.New("no valid tokens")
	}
	return best, nil
}
