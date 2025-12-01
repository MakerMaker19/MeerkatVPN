package nostrutil

import (
	"context"
	"fmt"
	"strings"

	"github.com/nbd-wtf/go-nostr"
)

// Client wraps a basic nostr relay set + keys.
type Client struct {
	PrivKey string
	PubKey  string
	Relays  []*nostr.Relay
}

// NewClient connects to the given relay URLs and parses the privkey
// (which may be hex or nsec).
func NewClient(ctx context.Context, priv string, relayURLs []string) (*Client, error) {
	parsed, err := ParsePrivKey(priv)
	if err != nil {
		return nil, err
	}

	c := &Client{
		PrivKey: parsed.PrivHex,
		PubKey:  parsed.PubHex,
		Relays:  []*nostr.Relay{},
	}

	for _, raw := range relayURLs {
		url := strings.TrimSpace(raw)
		if url == "" {
			continue
		}
		relay, err := nostr.RelayConnect(ctx, url)
		if err != nil {
			fmt.Println("failed to connect to relay", url, ":", err)
			continue
		}
		c.Relays = append(c.Relays, relay)
	}

	if len(c.Relays) == 0 {
		return nil, fmt.Errorf("no relays connected")
	}

	return c, nil
}

// SendDM sends a simple (currently plaintext) kind-4 DM to the target pubkey.
//
// NOTE: For now this does NOT do NIP-04/44 encryption. It just signs and
// publishes the event so we can focus on plumbing. We can harden this later.
func (c *Client) SendDM(ctx context.Context, toPub string, content string, extraTags nostr.Tags) error {
	pubHex, err := ParsePubKey(toPub)
	if err != nil {
		return err
	}

	tags := nostr.Tags{
		{"p", pubHex},
	}
	if extraTags != nil {
		tags = append(tags, extraTags...)
	}

	ev := nostr.Event{
		PubKey:    c.PubKey,
		CreatedAt: nostr.Now(),
		Kind:      nostr.KindEncryptedDirectMessage, // kind 4
		Tags:      tags,
		Content:   content, // plaintext for now
	}

	if err := ev.Sign(c.PrivKey); err != nil {
		return err
	}

	for _, r := range c.Relays {
		if err := r.Publish(ctx, ev); err != nil {
			fmt.Println("failed to publish DM:", err)
		}
	}

	return nil
}

// Publish broadcasts a generic event to all connected relays.
func (c *Client) Publish(ctx context.Context, ev nostr.Event) error {
	var lastErr error
	for _, r := range c.Relays {
		if err := r.Publish(ctx, ev); err != nil {
			fmt.Println("failed to publish event:", err)
			lastErr = err
		}
	}
	return lastErr
}



