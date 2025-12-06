package discovery

import (
	"time"
)

// DefaultAnnouncementTTL is how long we consider a node announcement "fresh".
// Nostr-based nodes with LastAnnouncedAt+TTL < now are treated as expired
// for selection (but may still be shown in debug UIs).
const DefaultAnnouncementTTL = 15 * time.Minute

// NodeInfo describes everything the client needs to know about a VPN node.
//
// It intentionally separates:
//   - identity & pool info
//   - address/capabilities
//   - geo/descriptive metadata
//   - dynamic runtime state (health/latency)
//   - discovery bookkeeping (source, freshness)
type NodeInfo struct {
	// -------- Identity / keying --------

	// ID is the stable identifier we use to merge announcements and health checks.
	// Typically this comes from the "node" tag in Nostr, or the announcer's pubkey,
	// or as a last resort, the API URL.
	ID         string
	PoolPubKey string // pool this node belongs to (from config or "pool" tag)

	// -------- Address / capabilities --------

	APIURL   string   // e.g. "https://meerkat-eu.example.com/api"
	Backends []string // e.g. ["wireguard","openvpn"]

	// -------- Geo / descriptive --------

	Region  string // e.g. "eu-central"
	Country string // e.g. "DE"
	City    string // e.g. "Frankfurt"

	Version string            // node software version ("0.1.5")
	Schema  int               // announcement schema version (1, 2, ...)
	Meta    map[string]string // flattened metadata (provider, tier, etc.)

	// -------- Dynamic / runtime state --------

	Healthy         bool          // result of last health check / self-report
	LastError       string        // last failure reason (for logging/UI)
	LastHealthCheck time.Time     // when we last probed this node
	LastLatency     time.Duration // most recent measured RTT
	AverageLatency  time.Duration // optional rolling average
	Weight          float64       // preference hint (larger = more preferred, by convention)

	// -------- Discovery / bookkeeping --------

	Source          string    // "static", "nostr", "static+nostr", etc.
	FirstSeen       time.Time // when we first created this NodeInfo
	LastAnnouncedAt time.Time // timestamp of the latest announcement we accepted
	ExpiresAt       time.Time // when this announcement should be considered stale
}

// Announcement is a normalized view of a "node announcement" coming from any
// discovery mechanism (Nostr, static config, config file, etc.).
//
// Nostr-specific code should parse nostr.Event into this type, then let the
// generic merge helpers below apply it to the registry.
type Announcement struct {
	// Identity / scoping
	ID          string    // canonical node ID (already resolved via tags/pubkey)
	PoolPubKey  string    // pool this node belongs to
	AnnouncedAt time.Time // when this announcement was created (e.g. Nostr created_at)

	// Address / capabilities
	APIURL   string
	Backends []string

	// Geo / descriptive
	Region  string
	Country string
	City    string

	Version string            // node software version
	Schema  int               // announcement schema version
	Meta    map[string]string // metadata (provider, tier, etc.)

	// Hints
	Weight float64
}

// NewNodeFromAnnouncement constructs a fresh NodeInfo from a discovery
// announcement. It does NOT run health checks; dynamic fields are initialized
// to safe defaults.
//
// 'source' is typically "static" or "nostr".
// 'ttl' controls how long the announcement is considered fresh.
func NewNodeFromAnnouncement(a Announcement, source string, now time.Time, ttl time.Duration) *NodeInfo {
	if ttl <= 0 {
		ttl = DefaultAnnouncementTTL
	}
	if a.Meta == nil {
		a.Meta = make(map[string]string)
	}

	n := &NodeInfo{
		// identity
		ID:         a.ID,
		PoolPubKey: a.PoolPubKey,

		// address / capabilities
		APIURL:   a.APIURL,
		Backends: cloneStrings(a.Backends),

		// geo / descriptive
		Region:  a.Region,
		Country: a.Country,
		City:    a.City,
		Version: a.Version,
		Meta:    cloneStringMap(a.Meta),
		Weight:  a.Weight,
		Source:  source,

		FirstSeen:       now,
		LastAnnouncedAt: a.AnnouncedAt,
		ExpiresAt:       a.AnnouncedAt.Add(ttl),
	}

	// Schema handling; keep it simple for now.
	if a.Schema > 0 {
		n.Schema = a.Schema
	}

	return n
}

// ApplyAnnouncement merges a new discovery announcement into an existing node.
//
// Rules:
//   - If the announcement is older than the node's LastAnnouncedAt, it is ignored.
//   - Static-like fields (APIURL, Region, Backends, Meta, Weight, etc.) are
//     overwritten by the new announcement.
//   - Dynamic runtime state (Healthy, latency, LastHealthCheck, LastError) is
//     preserved, unless the address fundamentally changes (APIURL change).
//   - Source is upgraded (e.g. "static" -> "static+nostr") if needed.
func (n *NodeInfo) ApplyAnnouncement(a Announcement, now time.Time, ttl time.Duration) {
	if ttl <= 0 {
		ttl = DefaultAnnouncementTTL
	}

	// Ignore strictly older announcements.
	if !n.LastAnnouncedAt.IsZero() && !a.AnnouncedAt.IsZero() && !a.AnnouncedAt.After(n.LastAnnouncedAt) {
		return
	}

	// Identity / pool
	if a.PoolPubKey != "" {
		n.PoolPubKey = a.PoolPubKey
	}

	// Detect meaningful address change.
	apiChanged := a.APIURL != "" && a.APIURL != n.APIURL

	// Address / capabilities
	if a.APIURL != "" {
		n.APIURL = a.APIURL
	}
	if len(a.Backends) > 0 {
		n.Backends = cloneStrings(a.Backends)
	}

	// Geo / descriptive
	if a.Region != "" {
		n.Region = a.Region
	}
	if a.Country != "" {
		n.Country = a.Country
	}
	if a.City != "" {
		n.City = a.City
	}
	if a.Version != "" {
		n.Version = a.Version
	}
	if a.Meta != nil {
		n.Meta = cloneStringMap(a.Meta)
	}
	if a.Weight != 0 {
		n.Weight = a.Weight
	}
	if a.Schema > 0 {
		n.Schema = a.Schema
	}

	// Update announcement bookkeeping.
	if !a.AnnouncedAt.IsZero() {
		n.LastAnnouncedAt = a.AnnouncedAt
	} else {
		n.LastAnnouncedAt = now
	}
	n.ExpiresAt = n.LastAnnouncedAt.Add(ttl)

	// Upgrade source label if we now know more.
	if n.Source == "" {
		n.Source = "unknown"
	}
	switch n.Source {
	case "static":
		n.Source = "static+dynamic"
	case "nostr":
		// already dynamic, keep
	default:
		// leave as-is for "static+nostr", "mixed", etc.
	}

	// If the API endpoint changed, it can be reasonable to reset latency/health
	// so the health checker can re-evaluate this node from scratch.
	if apiChanged {
		n.Healthy = false
		n.LastLatency = 0
		n.AverageLatency = 0
		n.LastError = ""
		n.LastHealthCheck = time.Time{}
	}
}

// ApplyHealthProbe updates the dynamic runtime state of a node based on a
// health check result. It never mutates identity, capabilities, or geo fields.
func (n *NodeInfo) ApplyHealthProbe(ok bool, latency time.Duration, err error, now time.Time) {
	n.LastHealthCheck = now

	if ok {
		n.Healthy = true
		n.LastError = ""

		// Update latencies.
		n.LastLatency = latency
		if n.AverageLatency == 0 {
			n.AverageLatency = latency
		} else {
			// Simple EMA-style smoothing; tweak alpha as desired.
			const alpha = 0.3
			n.AverageLatency = time.Duration(
				(1-alpha)*float64(n.AverageLatency) + alpha*float64(latency),
			)
		}
		return
	}

	// Failure path.
	n.Healthy = false
	n.LastLatency = latency
	if err != nil {
		n.LastError = err.Error()
	}
}

// IsExpired reports whether this node's announcement should be considered
// stale for selection purposes. Nodes discovered purely via static config
// may have ExpiresAt zero and are never expired.
func (n *NodeInfo) IsExpired(now time.Time) bool {
	if n.ExpiresAt.IsZero() {
		return false
	}
	return now.After(n.ExpiresAt)
}

// cloneStrings and cloneStringMap are tiny helpers to avoid sharing slices/maps
// across NodeInfo and Announcement; they keep merges side-effect free.

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
