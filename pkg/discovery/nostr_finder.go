package discovery

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/nbd-wtf/go-nostr"
)

// NostrNodeAnnouncementKind is the Nostr kind used for node announcements.
const NostrNodeAnnouncementKind = 38383

// NostrNodeAnnouncement models the JSON content of a node-announcement event.
type NostrNodeAnnouncement struct {
	APIURL   string   `json:"api_url"`
	Region   string   `json:"region"`
	Country  string   `json:"country,omitempty"`
	City     string   `json:"city,omitempty"`
	Backends []string `json:"backends,omitempty"`
	Version  string   `json:"version,omitempty"`
}

// nostrFinder implements Finder by subscribing to Nostr events and
// building a dynamic list of NodeInfo. If no dynamic nodes are
// available, it falls back to a static Finder.
type nostrFinder struct {
	relays  []string
	poolPub string

	fallback Finder

	mu    sync.RWMutex
	nodes []NodeInfo

	startOnce sync.Once
}

// NewNostrFinder creates a Finder that uses Nostr-based discovery,
// with a fallback Finder used when no Nostr data is available.
func NewNostrFinder(relays []string, poolPubKey string, fallback Finder) Finder {
	if fallback == nil {
		fallback = NewStaticFinder()
	}
	return &nostrFinder{
		relays:   relays,
		poolPub:  poolPubKey,
		fallback: fallback,
	}
}

func (f *nostrFinder) ensureStarted() {
	f.startOnce.Do(func() {
		go f.run()
	})
}

// run connects to Nostr relays and subscribes for node-announcement events.
func (f *nostrFinder) run() {
	if len(f.relays) == 0 {
		log.Println("[discovery/nostr] no relays configured; using fallback only")
		return
	}

	ctx := context.Background()
	pool := nostr.NewSimplePool(ctx)

	debug := os.Getenv("MEERKAT_DEBUG_DISCOVERY") == "1"

	// Connect relays
	for _, addr := range f.relays {
		if _, err := pool.EnsureRelay(addr); err != nil {
			if debug {
				log.Printf("[discovery/nostr] relay error for %s: %v\n", addr, err)
			}
		} else if debug {
			log.Printf("[discovery/nostr] connected relay %s\n", addr)
		}
	}

	if f.poolPub == "" && debug {
		log.Printf("[discovery/nostr] WARNING: poolPubKey is empty; subscription will match all pools")
	}

	// Subscribe for announcements of our kind, filtered by pool tag.
	filters := nostr.Filters{
		{
			Kinds: []int{NostrNodeAnnouncementKind},
			Tags:  nostr.TagMap{"pool": []string{f.poolPub}}, // empty string matches any pool
		},
	}

	ch := pool.SubMany(ctx, f.relays, filters)
	log.Printf("[discovery/nostr] started subscription (kind=%d, pool=%s, relays=%v)\n",
		NostrNodeAnnouncementKind, f.poolPub, f.relays)

	for ev := range ch {
		if ev.Event == nil {
			continue
		}
		f.updateFromEvent(ev.Event)
	}

	if debug {
		log.Println("[discovery/nostr] event channel closed; Nostr discovery loop ended")
	}
}

// updateFromEvent parses a Nostr event and turns it into a NodeInfo,
// merging it into the in-memory node list.
func (f *nostrFinder) updateFromEvent(ev *nostr.Event) {
	if ev == nil {
		return
	}

	debug := os.Getenv("MEERKAT_DEBUG_DISCOVERY") == "1"

	if ev.Kind != NostrNodeAnnouncementKind {
		return
	}

	// Filter on pool tag, if configured.
	poolTag := firstTagValue(ev.Tags, "pool")
	if f.poolPub != "" {
		if poolTag == "" {
			if debug {
				log.Printf("[discovery/nostr] ignoring event %s: missing pool tag\n", ev.ID)
			}
			return
		}
		if !strings.EqualFold(poolTag, f.poolPub) {
			if debug {
				log.Printf("[discovery/nostr] ignoring event %s: pool mismatch (%s != %s)\n",
					ev.ID, poolTag, f.poolPub)
			}
			return
		}
	}

	var ann NostrNodeAnnouncement
	if err := json.Unmarshal([]byte(ev.Content), &ann); err != nil {
		if debug {
			log.Printf("[discovery/nostr] invalid JSON content in event %s: %v\n", ev.ID, err)
		}
		return
	}

	if ann.APIURL == "" {
		if debug {
			log.Printf("[discovery/nostr] ignoring event %s: api_url missing in content\n", ev.ID)
		}
		return
	}

	node := NodeInfo{
		ID:      ev.PubKey,
		APIURL:  ann.APIURL,
		Region:  strings.TrimSpace(ann.Region),
		Country: strings.TrimSpace(ann.Country),
		City:    strings.TrimSpace(ann.City),
		Backends: append([]string(nil), ann.Backends...),
		Healthy:  true,
	}

	// Tags can override JSON content.
	if v := firstTagValue(ev.Tags, "region"); v != "" {
		node.Region = v
	}
	if v := firstTagValue(ev.Tags, "country"); v != "" {
		node.Country = v
	}
	if v := firstTagValue(ev.Tags, "city"); v != "" {
		node.City = v
	}

	backendTags := allTagValues(ev.Tags, "backend")
	if len(backendTags) > 0 {
		node.Backends = backendTags
	}
	if len(node.Backends) == 0 {
		node.Backends = []string{"openvpn"}
	}

	if debug {
		log.Printf("[discovery/nostr] announcement from %s: api=%s region=%s country=%s city=%s backends=%v\n",
			node.ID, node.APIURL, node.Region, node.Country, node.City, node.Backends)
	}

	// Merge into our node list.
	f.mu.Lock()
	defer f.mu.Unlock()

	for i := range f.nodes {
		if f.nodes[i].ID == node.ID {
			f.nodes[i] = node
			if debug {
				log.Printf("[discovery/nostr] updated node %s\n", node.ID)
			}
			return
		}
	}

	f.nodes = append(f.nodes, node)
	if debug {
		log.Printf("[discovery/nostr] now tracking %d nostr nodes\n", len(f.nodes))
	}
}

// Helpers for working with Nostr tags.

func firstTagValue(tags nostr.Tags, name string) string {
	for _, t := range tags {
		if len(t) >= 2 && t[0] == name {
			return t[1]
		}
	}
	return ""
}

func allTagValues(tags nostr.Tags, name string) []string {
	var out []string
	for _, t := range tags {
		if len(t) >= 2 && t[0] == name {
			out = append(out, t[1])
		}
	}
	return out
}

// Finder implementation ///////////////////////////////////////////////////////

func (f *nostrFinder) FindNode(
	ctx context.Context,
	poolPubKey string,
	preferredRegion string,
	backend string,
) (*NodeInfo, error) {
	f.ensureStarted()

	_ = poolPubKey // may be used later to filter by pool

	f.mu.RLock()
	nodes := make([]NodeInfo, len(f.nodes))
	copy(nodes, f.nodes)
	f.mu.RUnlock()

	if len(nodes) == 0 {
		// No Nostr nodes yet; fall back to staticFinder
		return f.fallback.FindNode(ctx, poolPubKey, preferredRegion, backend)
	}

	// Reuse the same selection logic as staticFinder, but applied to our Nostr nodes.
	return findNodeFromList(nodes, preferredRegion, backend)
}

func (f *nostrFinder) ListNodes(ctx context.Context) ([]NodeInfo, error) {
	f.ensureStarted()

	f.mu.RLock()
	nodes := make([]NodeInfo, len(f.nodes))
	copy(nodes, f.nodes)
	f.mu.RUnlock()

	if len(nodes) == 0 {
		return f.fallback.ListNodes(ctx)
	}
	return nodes, nil
}
