// pkg/discovery/nostr_finder.go

// NOTE: In the current architecture we treat nostrFinder primarily as a
// "feeder" for the global Registry. The Finder interface methods are kept
// for future use, but all production reads go through the Registry-backed
// default finder (see discovery.SetDefaultFinder in cmd/client-cli/nodes_env.go).


package discovery

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

// nostrFinder implements Finder by subscribing to Nostr events and
// building a dynamic list of NodeInfo. If no dynamic nodes are available,
// it falls back to a static Finder.
type nostrFinder struct {
	relays  []string
	poolPub string

	fallback Finder    // keep this for now
	registry *Registry // shared view

	startOnce sync.Once
}

// NewNostrFinder creates a Finder that uses Nostr-based discovery,
// with a fallback Finder used when no Nostr data is available.
func NewNostrFinder(relays []string, poolPubKey string, fallback Finder) Finder {
	if fallback == nil {
		fallback = NewStaticFinder()
	}
	f := &nostrFinder{
		relays:   relays,
		poolPub:  poolPubKey,
		fallback: fallback,
		registry: GlobalRegistry(),
	}
	// Start the Nostr subscription immediately so announcements feed the registry
	// even if callers only use the registry-backed finder.
	f.ensureStarted()
	return f
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

	// Subscribe for announcements of our kind. If poolPub is non-empty,
	// we still re-check inside the parser (to avoid relays rejecting unknown tags).
	filter := nostr.Filter{
		Kinds: []int{NostrNodeAnnouncementKind},
	}
	filters := nostr.Filters{filter}

	ch := pool.SubMany(ctx, f.relays, filters)
	log.Printf("[discovery/nostr] started subscription (kind=%d, pool=%s, relays=%v)\n",
		NostrNodeAnnouncementKind, f.poolPub, f.relays)

	for msg := range ch {
		if msg.Event == nil {
			continue
		}
		f.updateFromEvent(msg.Event)
	}

	if debug {
		log.Println("[discovery/nostr] event channel closed; Nostr discovery loop ended")
	}
}

// updateFromEvent parses a Nostr event, converts it into an Announcement,
// and merges it into the in-memory node map.
func (f *nostrFinder) updateFromEvent(ev *nostr.Event) {
	if ev == nil {
		return
	}

	now := time.Now()
	a, ok := announcementFromNostrEvent(ev, f.poolPub, now)
	if !ok {
		return
	}

	f.registry.UpsertAnnouncement(*a, "nostr", now, DefaultAnnouncementTTL)
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

	now := time.Now()
	slice := f.registry.Snapshot(now, true)

	if len(slice) == 0 {
		// No Nostr nodes yet; fall back to staticFinder
		return f.fallback.FindNode(ctx, poolPubKey, preferredRegion, backend)
	}

	// Reuse the same selection logic as staticFinder, but applied to our Nostr nodes.
	return findNodeFromList(slice, preferredRegion, backend)
}

func (f *nostrFinder) ListNodes(ctx context.Context) ([]NodeInfo, error) {
	f.ensureStarted()

	now := time.Now()
	slice := f.registry.Snapshot(now, false)

	if len(slice) == 0 {
		return f.fallback.ListNodes(ctx)
	}
	return slice, nil
}
