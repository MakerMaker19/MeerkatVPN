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

func clientRelayURLsFromEnv() []string {
	v := os.Getenv("MEERKAT_CLIENT_RELAYS")
	if v == "" {
		return []string{"wss://relay.damus.io", "wss://relay.primal.net"}
	}
	parts := strings.Split(v, ",")
	out := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ListenForTokens connects to relays and stores subscription tokens found in kind-4 DMs.
//
// Env vars:
//   MEERKAT_CLIENT_NOSTR_PRIVKEY  (hex or nsec)
//   MEERKAT_CLIENT_RELAYS         (optional, comma-separated)
//   MEERKAT_CLIENT_POOL_PUBKEY    (optional, hex or npub; if set, only accept tokens from this pubkey)
func ListenForTokens(ctx context.Context) error {
	priv := os.Getenv("MEERKAT_CLIENT_NOSTR_PRIVKEY")
	if priv == "" {
		return fmt.Errorf("MEERKAT_CLIENT_NOSTR_PRIVKEY not set")
	}
	relays := clientRelayURLsFromEnv()

	poolPubFilter := os.Getenv("MEERKAT_CLIENT_POOL_PUBKEY")
	var poolPubHex string
	if poolPubFilter != "" {
		parsed, err := nostrutil.ParsePubKey(poolPubFilter)
		if err != nil {
			return fmt.Errorf("failed to parse MEERKAT_CLIENT_POOL_PUBKEY: %w", err)
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
		Limit:  0,                                      // no explicit limit
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
