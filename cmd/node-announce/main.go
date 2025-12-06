package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MakerMaker19/meerkatvpn/pkg/discovery"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

// payload mirrors discovery.nostrNodePayload JSON schema.
// We duplicate it here to avoid exporting internals; JSON tags
// MUST stay in sync with what the client expects.
type nodeAnnouncementPayload struct {
	Schema   int               `json:"schema,omitempty"`
	APIURL   string            `json:"api_url"`
	Region   string            `json:"region,omitempty"`
	Country  string            `json:"country,omitempty"`
	City     string            `json:"city,omitempty"`
	Backends []string          `json:"backends,omitempty"`
	Version  string            `json:"version,omitempty"`
	Weight   float64           `json:"weight,omitempty"`
	Meta     map[string]string `json:"meta,omitempty"`
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("[meerkat-node] error: %v", err)
	}
}

func run() error {
	// Required envs.
	nsec := strings.TrimSpace(os.Getenv("MEERKAT_NODE_NSEC"))
	apiURL := strings.TrimSpace(os.Getenv("MEERKAT_NODE_API_URL"))
	poolPub := strings.TrimSpace(os.Getenv("MEERKAT_POOL_PUBKEY"))
	relaysEnv := strings.TrimSpace(os.Getenv("MEERKAT_NOSTR_RELAYS"))

	if nsec == "" || apiURL == "" || poolPub == "" || relaysEnv == "" {
		return fmt.Errorf("MEERKAT_NODE_NSEC, MEERKAT_NODE_API_URL, MEERKAT_POOL_PUBKEY, and MEERKAT_NOSTR_RELAYS must be set")
	}

	relays := splitList(relaysEnv)
	if len(relays) == 0 {
		return fmt.Errorf("MEERKAT_NOSTR_RELAYS is empty after parsing")
	}

	// Optional envs.
	region := strings.TrimSpace(os.Getenv("MEERKAT_NODE_REGION"))
	country := strings.TrimSpace(os.Getenv("MEERKAT_NODE_COUNTRY"))
	city := strings.TrimSpace(os.Getenv("MEERKAT_NODE_CITY"))

	backends := splitList(os.Getenv("MEERKAT_NODE_BACKENDS"))
	if len(backends) == 0 {
		backends = []string{"wireguard"}
	}

	version := strings.TrimSpace(os.Getenv("MEERKAT_NODE_VERSION"))

	schema := 1
	if v := strings.TrimSpace(os.Getenv("MEERKAT_NODE_SCHEMA")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			schema = n
		}
	}

	weight := 1.0
	if v := strings.TrimSpace(os.Getenv("MEERKAT_NODE_WEIGHT")); v != "" {
		if w, err := strconv.ParseFloat(v, 64); err == nil {
			weight = w
		}
	}

	intervalSecs := 0
	if v := strings.TrimSpace(os.Getenv("MEERKAT_NODE_ANNOUNCE_INTERVAL_SECS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			intervalSecs = n
		}
	}

	// Parse private key (nsec -> hex) and derive pubkey.
	skHex, err := parseNsecToHex(nsec)
	if err != nil {
		return fmt.Errorf("invalid MEERKAT_NODE_NSEC: %w", err)
	}
	pubKey, err := nostr.GetPublicKey(skHex)
	if err != nil {
		return fmt.Errorf("derive pubkey: %w", err)
	}

	log.Printf("[meerkat-node] starting announcer: api_url=%s pool=%s relays=%v pubkey=%s",
		apiURL, poolPub, relays, pubKey)

	ctx := context.Background()
	pool := nostr.NewSimplePool(ctx)

	// Ensure relays are reachable.
	for _, addr := range relays {
		if _, err := pool.EnsureRelay(addr); err != nil {
			log.Printf("[meerkat-node] warning: relay %s error: %v", addr, err)
		} else {
			log.Printf("[meerkat-node] connected relay %s", addr)
		}
	}

	announceOnce := func() {
		if err := publishAnnouncement(ctx, pool, relays, skHex, pubKey, poolPub, apiURL, region, country, city, backends, version, schema, weight); err != nil {
			log.Printf("[meerkat-node] error publishing announcement: %v", err)
		}
	}

	// Single-shot vs periodic mode.
	if intervalSecs <= 0 {
		announceOnce()
		return nil
	}

	interval := time.Duration(intervalSecs) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	announceOnce()
	log.Printf("[meerkat-node] now re-announcing every %s (Ctrl+C to exit)", interval)

	for range ticker.C {
		announceOnce()
	}

	// unreachable
	//nolint
	return nil
}

func publishAnnouncement(
	ctx context.Context,
	pool *nostr.SimplePool,
	relays []string,
	skHex string,
	pubKey string,
	poolPub string,
	apiURL string,
	region string,
	country string,
	city string,
	backends []string,
	version string,
	schema int,
	weight float64,
) error {
	now := nostr.Now()

	payload := nodeAnnouncementPayload{
		Schema:   schema,
		APIURL:   apiURL,
		Region:   region,
		Country:  country,
		City:     city,
		Backends: backends,
		Version:  version,
		Weight:   weight,
		Meta:     map[string]string{}, // extend as needed
	}

	contentBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	ev := nostr.Event{
		PubKey:    pubKey,
		CreatedAt: now,
		Kind:      discovery.NostrNodeAnnouncementKind,
		Tags:      make(nostr.Tags, 0, 8),
		Content:   string(contentBytes),
	}

	// Required tags.
	ev.Tags = append(ev.Tags,
		nostr.Tag{"pool", poolPub},
	)

	// Optional tags (mirror what the client expects).
	if region != "" {
		ev.Tags = append(ev.Tags, nostr.Tag{"region", region})
	}
	if country != "" {
		ev.Tags = append(ev.Tags, nostr.Tag{"country", country})
	}
	if city != "" {
		ev.Tags = append(ev.Tags, nostr.Tag{"city", city})
	}
	if version != "" {
		ev.Tags = append(ev.Tags, nostr.Tag{"version", version})
	}
	if schema > 0 {
		ev.Tags = append(ev.Tags, nostr.Tag{"schema", strconv.Itoa(schema)})
	}
	if weight != 0 {
		ev.Tags = append(ev.Tags, nostr.Tag{"weight", strconv.FormatFloat(weight, 'f', -1, 64)})
	}
	for _, b := range backends {
		if b = strings.TrimSpace(b); b != "" {
			ev.Tags = append(ev.Tags, nostr.Tag{"backend", b})
		}
	}

	if err := ev.Sign(skHex); err != nil {
		return fmt.Errorf("sign event: %w", err)
	}

	log.Printf("[meerkat-node] announcing node: kind=%d api=%s region=%s country=%s city=%s backends=%v",
		ev.Kind, apiURL, region, country, city, backends)

	// Publish to all relays via the pool.
	for _, addr := range relays {
		relay, err := pool.EnsureRelay(addr)
		if err != nil {
			log.Printf("[meerkat-node] relay %s error: %v", addr, err)
			continue
		}
		if err := relay.Publish(ctx, ev); err != nil {
			log.Printf("[meerkat-node] publish to %s error: %v", addr, err)
		}
	}

	return nil
}

func splitList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// parseNsecToHex converts an nsec string to a hex private key.
// If the input is already hex, it is returned unchanged.
func parseNsecToHex(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty key")
	}

	if strings.HasPrefix(s, "nsec1") {
		_, data, err := nip19.Decode(s)
		if err != nil {
			return "", err
		}
		privBytes, ok := data.([]byte)
		if !ok {
			return "", fmt.Errorf("invalid nsec payload")
		}
		return fmt.Sprintf("%x", privBytes), nil
	}

	// Assume raw hex
	return s, nil
}
