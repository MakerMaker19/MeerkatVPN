package discovery

import (
	"context"
	"errors"
	"time"
)

// ErrNoNodes indicates the registry has no entries available.
var ErrNoNodes = errors.New("no nodes available")

// registryFinder is a Finder that simply reads from a Registry snapshot.
// It assumes other components (static seeders, Nostr, health probes) keep
// the registry up to date.
type registryFinder struct {
	registry *Registry
}

// NewRegistryFinder returns a Finder that serves data from the given registry.
// If reg is nil, the global registry is used.
func NewRegistryFinder(reg *Registry) Finder {
	if reg == nil {
		reg = GlobalRegistry()
	}
	return &registryFinder{registry: reg}
}

func (f *registryFinder) FindNode(
	ctx context.Context,
	poolPubKey string,
	preferredRegion string,
	backend string,
) (*NodeInfo, error) {
	_ = ctx // reserved for future cancellation/timeouts

	now := time.Now()
	nodes := f.registry.Snapshot(now, true) // exclude expired nodes
	if len(nodes) == 0 {
		return nil, ErrNoNodes
	}
	return findNodeFromList(nodes, preferredRegion, backend)
}

func (f *registryFinder) ListNodes(ctx context.Context) ([]NodeInfo, error) {
	_ = ctx
	now := time.Now()
	return f.registry.Snapshot(now, false), nil
}
