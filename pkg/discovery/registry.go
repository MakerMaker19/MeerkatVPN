package discovery

import (
	"sync"
	"time"
)

// Registry is a concurrency-safe store for NodeInfo entries.
// It is designed so multiple discovery sources (static, Nostr, etc.)
// and health checkers can share a single view of the world.
type Registry struct {
	mu    sync.RWMutex
	nodes map[string]*NodeInfo
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		nodes: make(map[string]*NodeInfo),
	}
}

// UpsertAnnouncement applies a discovery announcement for a node identified
// by Announcement.ID. If the node is new, it is created; otherwise the
// announcement is merged into the existing NodeInfo using ApplyAnnouncement.
//
// 'source' is typically "static" or "nostr".
// 'ttl' controls how long the announcement is considered fresh.
func (r *Registry) UpsertAnnouncement(a Announcement, source string, now time.Time, ttl time.Duration) *NodeInfo {
	if a.ID == "" {
		return nil
	}
	if ttl <= 0 {
		ttl = DefaultAnnouncementTTL
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	node, exists := r.nodes[a.ID]
	if exists {
		node.ApplyAnnouncement(a, now, ttl)
		return node
	}

	node = NewNodeFromAnnouncement(a, source, now, ttl)
	r.nodes[a.ID] = node
	return node
}

// ApplyHealth updates the runtime state of a node by ID based on a health probe.
// If no such node exists, this is a no-op.
func (r *Registry) ApplyHealth(id string, ok bool, latency time.Duration, err error, now time.Time) {
	if id == "" {
		return
	}

	r.mu.RLock()
	node, exists := r.nodes[id]
	r.mu.RUnlock()
	if !exists {
		return
	}

	// Let NodeInfo own the update logic.
	node.ApplyHealthProbe(ok, latency, err, now)
}

// Snapshot returns a slice copy of the nodes currently in the registry.
// If excludeExpired is true, nodes whose IsExpired(now) is true are omitted.
func (r *Registry) Snapshot(now time.Time, excludeExpired bool) []NodeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]NodeInfo, 0, len(r.nodes))
	for _, n := range r.nodes {
		if excludeExpired && n.IsExpired(now) {
			continue
		}
		out = append(out, *n)
	}
	return out
}
