package discovery

import (
	"context"
	"log"
	"os"
	"sync"
	"strings"
	//"time"

	"github.com/nbd-wtf/go-nostr"
)

// NostrNodeAnnouncementKind is the Nostr kind we use for node announcements.
// You can change this later; just keep it app-specific.
const NostrNodeAnnouncementKind = 38383

// NostrNodeAnnouncement models the content of a node-announcement event.
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
	relays    []string
	poolPub   string
	fallback  Finder

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
// For now this is a stub that logs a TODO and does not break anything.
func (f *nostrFinder) run() {
	if len(f.relays) == 0 {
		log.Println("[discovery/nostr] no relays configured; using fallback only")
		return
	}

	// Skeleton for future implementation:
	// - connect to relays
	// - subscribe for kind=NostrNodeAnnouncementKind
	// - filter by tag pool=<f.poolPub>
	// - parse events into NodeInfo and store in f.nodes

	log.Printf("[discovery/nostr] starting Nostr discovery (relays=%v, pool=%s)\n",
		f.relays, f.poolPub)

	// Example pseudocode (commented so it doesn't run yet):
	/*
		ctx := context.Background()
		relayPool := nostr.NewSimplePool(ctx)

		go func() {
			defer relayPool.Close()

			for _, addr := range f.relays {
				_, err := relayPool.EnsureRelay(addr)
				if err != nil {
					log.Println("[discovery/nostr] relay error:", err)
				}
			}

			filters := []nostr.Filter{
				{
					Kinds: []int{NostrNodeAnnouncementKind},
					Tags: nostr.TagMap{
						"pool": []string{f.poolPub},
					},
				},
			}

			evChan := relayPool.SubManyEose(ctx, f.relays, filters)

			for ev := range evChan {
				// TODO: parse event into NodeInfo and call f.updateFromEvent(ev)
			}
		}()
	*/
}

// updateFromEvent would parse a Nostr event and turn it into a NodeInfo.
// Left as TODO for now so we don't break anything.
func (f *nostrFinder) updateFromEvent(ev *nostr.Event) {
	_ = ev
	// TODO: parse tags + JSON content into NodeInfo and merge into f.nodes.
}

func (f *nostrFinder) FindNode(
	ctx context.Context,
	poolPubKey string,
	preferredRegion string,
	backend string,
) (*NodeInfo, error) {
	f.ensureStarted()

	_ = poolPubKey // you may use this later to filter by pool

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

// findNodeFromList mirrors findNodeStatic but takes an explicit list.
func findNodeFromList(nodes []NodeInfo, preferredRegion, backend string) (*NodeInfo, error) {
	if backend == "" {
		backend = "openvpn"
	}
	preferredRegion = strings.ToLower(strings.TrimSpace(preferredRegion))

	debug := os.Getenv("MEERKAT_DEBUG_DISCOVERY") == "1"

	if debug {
		log.Printf("[discovery/nostr] starting discovery: backend=%s region=%s\n", backend, preferredRegion)
		log.Printf("[discovery/nostr] nodes=%+v\n", nodes)
	}

	candidates := filterByBackend(nodes, backend)
	if debug {
		log.Printf("[discovery/nostr] candidates after backend filter=%+v\n", candidates)
	}

	if len(candidates) == 0 {
		return nil, context.DeadlineExceeded // or errors.New("no nodes")
	}

	candidates = rankByLatency(candidates)
	if debug {
		log.Printf("[discovery/nostr] candidates after latency ranking=%+v\n", candidates)
	}

	if preferredRegion == "" || preferredRegion == "auto" {
		if debug {
			log.Printf("[discovery/nostr] region=auto -> picking first candidate: %s\n", candidates[0].ID)
		}
		return &candidates[0], nil
	}

	for _, n := range candidates {
		if strings.EqualFold(n.Region, preferredRegion) {
			if debug {
				log.Printf("[discovery/nostr] exact region match found: %s (%s)\n", n.ID, n.Region)
			}
			return &n, nil
		}
	}

	if debug {
		log.Printf("[discovery/nostr] no exact region match for %s; falling back to %s\n",
			preferredRegion, candidates[0].ID)
	}
	return &candidates[0], nil
}
