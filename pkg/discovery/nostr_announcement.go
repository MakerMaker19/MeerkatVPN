package discovery

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

// NostrNodeAnnouncementKind is the Nostr kind we use for node announcements.
// Must match what nodes publish.
const NostrNodeAnnouncementKind = 38383

// nostrNodePayload models the JSON content of a node announcement event.
// It mirrors the schema we designed earlier.
type nostrNodePayload struct {
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

// nodeIDFromEvent computes the stable node ID:
//  1. "node" tag, if present
//  2. event.PubKey
//  3. apiURL (as last resort; only used if content parses)
func nodeIDFromEvent(ev *nostr.Event, apiURL string) string {
	if ev == nil {
		return ""
	}

	if id := firstTagValue(ev.Tags, "node"); id != "" {
		return id
	}
	if ev.PubKey != "" {
		return ev.PubKey
	}
	return apiURL
}

// announcementFromNostrEvent converts a Nostr node-announcement event into an
// Announcement suitable for merging into NodeInfo instances.
//
// poolFilter:
//   - if non-empty, only events with ["pool", poolFilter] will be accepted.
//   - if empty, any pool is accepted.
//
// Returns (*Announcement, true) on success, or (nil, false) if the event should
// be ignored (wrong kind, wrong pool, bad JSON, missing api_url, etc.).
func announcementFromNostrEvent(ev *nostr.Event, poolFilter string, now time.Time) (*Announcement, bool) {
	if ev == nil {
		return nil, false
	}
	if ev.Kind != NostrNodeAnnouncementKind {
		return nil, false
	}

	debug := false // if you want, hook this to an env var later

	// Pool filtering.
	poolTag := firstTagValue(ev.Tags, "pool")
	if poolFilter != "" {
		if poolTag == "" {
			if debug {
				log.Printf("[nostr-ann] ignoring event %s: missing pool tag", ev.ID)
			}
			return nil, false
		}
		if !strings.EqualFold(poolTag, poolFilter) {
			if debug {
				log.Printf("[nostr-ann] ignoring event %s: pool mismatch: got %s, want %s", ev.ID, poolTag, poolFilter)
			}
			return nil, false
		}
	}

	// Parse JSON content.
	var payload nostrNodePayload
	if err := json.Unmarshal([]byte(ev.Content), &payload); err != nil {
		if debug {
			log.Printf("[nostr-ann] invalid JSON in event %s: %v", ev.ID, err)
		}
		return nil, false
	}

	if payload.APIURL == "" {
		if debug {
			log.Printf("[nostr-ann] ignoring event %s: api_url missing", ev.ID)
		}
		return nil, false
	}

	// Backends: JSON first, then override with tags if any.
	backends := cloneStrings(payload.Backends)
	backendTags := allTagValues(ev.Tags, "backend")
	if len(backendTags) > 0 {
		backends = cloneStrings(backendTags)
	}
	if len(backends) == 0 {
		backends = []string{"wireguard"}
	}

	// Region / country / city: JSON with tag overrides.
	region := payload.Region
	if v := firstTagValue(ev.Tags, "region"); v != "" {
		region = v
	}
	country := payload.Country
	if v := firstTagValue(ev.Tags, "country"); v != "" {
		country = v
	}
	city := payload.City
	if v := firstTagValue(ev.Tags, "city"); v != "" {
		city = v
	}

	// Version: JSON with tag override.
	version := payload.Version
	if v := firstTagValue(ev.Tags, "version"); v != "" {
		version = v
	}

	// Schema: JSON with tag override (parsed as int).
	schema := payload.Schema
	if v := firstTagValue(ev.Tags, "schema"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			schema = n
		}
	}

	// Weight: JSON with tag override (parsed as float).
	weight := payload.Weight
	if v := firstTagValue(ev.Tags, "weight"); v != "" {
		if w, err := strconv.ParseFloat(v, 64); err == nil {
			weight = w
		}
	}
	if weight == 0 {
		weight = 1.0
	}

	// Meta: start from payload.Meta; you could in the future also
	// derive meta from tags (e.g. "meta:provider:hetzner"), but we'll
	// keep it simple for now.
	meta := cloneStringMap(payload.Meta)

	// AnnouncedAt from created_at; fall back to now if zero.
	announcedAt := time.Unix(int64(ev.CreatedAt), 0)
	if announcedAt.IsZero() {
		announcedAt = now
	}

	// Compute node ID using our NodeKey rule.
	id := nodeIDFromEvent(ev, payload.APIURL)
	if id == "" {
		// Extremely unlikely, but be safe.
		if debug {
			log.Printf("[nostr-ann] ignoring event %s: could not derive node ID", ev.ID)
		}
		return nil, false
	}

	a := &Announcement{
		ID:          id,
		PoolPubKey:  poolTag,
		AnnouncedAt: announcedAt,

		APIURL:   payload.APIURL,
		Backends: backends,

		Region:  region,
		Country: country,
		City:    city,

		Version: version,
		Schema:  schema,
		Meta:    meta,

		Weight: weight,
	}

	return a, true
}
