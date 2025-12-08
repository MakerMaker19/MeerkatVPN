package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/nbd-wtf/go-nostr"

	"github.com/MakerMaker19/meerkatvpn/pkg/nostrutil"
	"github.com/MakerMaker19/meerkatvpn/pkg/vpn"
)

func clientRelayURLs() []string {
	// 1) Env override
	if v := os.Getenv("MEERKAT_CLIENT_RELAYS"); v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			return out
		}
	}

	// 2) Config fallback
	if cfg := Config(); cfg != nil && len(cfg.Relays) > 0 {
		return cfg.Relays
	}

	// 3) Hard-coded defaults
	return DefaultRelays()
}

// ListenForTokens connects to relays and stores subscription tokens found in kind-4 DMs.
//
// Env vars:
//   MEERKAT_CLIENT_NOSTR_PRIVKEY  (hex or nsec)
//   MEERKAT_CLIENT_RELAYS         (optional, comma-separated)
//   MEERKAT_CLIENT_POOL_PUBKEY    (optional, hex or npub; if set, only accept tokens from this pubkey)
func ListenForTokens(ctx context.Context) error {
	// 1) Nostr privkey: env overrides config.
	priv := os.Getenv("MEERKAT_CLIENT_NOSTR_PRIVKEY")
	if priv == "" {
		if cfg := Config(); cfg != nil && cfg.NostrPrivKey != "" {
			priv = cfg.NostrPrivKey
		}
	}
	if priv == "" {
		return fmt.Errorf("MEERKAT_CLIENT_NOSTR_PRIVKEY not set and no nostr_privkey in meerkat-client.yaml")
	}

	relays := clientRelayURLs()

	// 2) Pool pubkey filter: env overrides config.
	poolPubFilter := os.Getenv("MEERKAT_CLIENT_POOL_PUBKEY")
	if poolPubFilter == "" {
		if cfg := Config(); cfg != nil && cfg.PoolPubKey != "" {
			poolPubFilter = cfg.PoolPubKey
		}
	}

	var poolPubHex string
	if poolPubFilter != "" {
		parsed, err := nostrutil.ParsePubKey(poolPubFilter)
		if err != nil {
			return fmt.Errorf("failed to parse pool pubkey (%q): %w", poolPubFilter, err)
		}
		poolPubHex = parsed
	}

	// Nostr client using same helper as pool.
	nc, err := nostrutil.NewClient(ctx, priv, relays)
	if err != nil {
		return fmt.Errorf("failed to init nostr client: %w", err)
	}
	log.Printf("Client pubkey (hex): %s\n", nc.PubKey)

	var wg sync.WaitGroup
	for _, r := range nc.Relays {
		wg.Add(1)
		go func(relay *nostr.Relay) {
			defer wg.Done()
			if err := listenOnRelay(ctx, relay, nc.PubKey, poolPubHex); err != nil {
				log.Println("relay listener error:", err)
			}
		}(r)
	}

	log.Println("Listening for subscription tokens over Nostr DMs. Ctrl+C to stop.")
	<-ctx.Done()
	log.Println("Context canceled, waiting for relay listeners to exit...")
	wg.Wait()
	return nil
}

func listenOnRelay(ctx context.Context, relay *nostr.Relay, myPubHex, poolPubHex string) error {
	filter := nostr.Filter{
		Kinds: []int{nostr.KindEncryptedDirectMessage}, // kind 4
		Limit: 0,                                       // no explicit limit
	}

	sub, err := relay.Subscribe(ctx, nostr.Filters{filter})
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-sub.Events:
			if !ok {
				return nil
			}

			// We only care about DMs where our pubkey appears in a "p" tag.
			if !ev.Tags.ContainsAny("p", []string{myPubHex}) {
				continue
			}

			// If poolPubHex set, only accept from that issuer.
			if poolPubHex != "" && ev.PubKey != poolPubHex {
				continue
			}

			if err := handleIncomingTokenEvent(ev); err != nil {
				log.Println("failed to handle DM:", err)
			}
		}
	}
}

func handleIncomingTokenEvent(ev *nostr.Event) error {
	// For now, we assume plaintext JSON content (no encryption yet).
	var tok vpn.SubscriptionToken
	if err := json.Unmarshal([]byte(ev.Content), &tok); err != nil {
		return fmt.Errorf("invalid token JSON: %w", err)
	}

	store, err := LoadTokenStore()
	if err != nil {
		return err
	}
	store.AddOrUpdate(tok)
	if err := store.Save(); err != nil {
		return err
	}

	log.Printf("Stored subscription token %s (expires %d) from %s\n",
		tok.Payload.TokenID, tok.Payload.ExpiresAt, ev.PubKey)
	return nil
}
