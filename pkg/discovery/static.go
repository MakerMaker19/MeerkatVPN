package discovery

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
)

// staticNodes is a simple, in-memory registry.
// You can later replace this with Nostr-based discovery or a pool API.
var staticNodes = []NodeInfo{
	{
		ID:       "vps-us-east-1",
		APIURL:   "http://46.62.204.11:9090",
		Region:   "us-east-1",
		Country:  "US",
		City:     "nyc",
		Backends: []string{"openvpn"},
		Healthy:  true,
	},
	{
		ID:       "local-dev",
		APIURL:   "http://localhost:9090",
		Region:   "local",
		Country:  "",
		City:     "",
		Backends: []string{"openvpn", "wireguard"},
		Healthy:  true,
	},
}

// staticFinder implements Finder using staticNodes + runtime health data.
type staticFinder struct{}

func (staticFinder) FindNode(
	ctx context.Context,
	poolPubKey string,
	preferredRegion string,
	backend string,
) (*NodeInfo, error) {
	_ = ctx
	_ = poolPubKey
	return findNodeStatic(preferredRegion, backend)
}

func (staticFinder) ListNodes(ctx context.Context) ([]NodeInfo, error) {
	_ = ctx
	out := make([]NodeInfo, len(staticNodes))
	copy(out, staticNodes)
	return out, nil
}

// internal helper with the selection logic
func findNodeStatic(preferredRegion, backend string) (*NodeInfo, error) {
	if backend == "" {
		backend = "openvpn"
	}
	preferredRegion = strings.ToLower(strings.TrimSpace(preferredRegion))

	debug := os.Getenv("MEERKAT_DEBUG_DISCOVERY") == "1"

	if debug {
		log.Printf("[discovery] starting static discovery: backend=%s region=%s\n", backend, preferredRegion)
		log.Printf("[discovery] staticNodes=%+v\n", staticNodes)
	}

	// 1) Filter by static healthy flag + backend support
	candidates := filterByBackend(staticNodes, backend)
	if debug {
		log.Printf("[discovery] candidates after backend filter=%+v\n", candidates)
	}

	if len(candidates) == 0 {
		return nil, errors.New("no nodes support backend " + backend)
	}

	// 1.5) Rank candidates by runtime health/latency.
	candidates = rankByLatency(candidates)
	if debug {
		log.Printf("[discovery] candidates after latency ranking=%+v\n", candidates)
	}

	// 2) If region is "auto" or empty, just pick first candidate (best latency).
	if preferredRegion == "" || preferredRegion == "auto" {
		if debug {
			log.Printf("[discovery] region=auto -> picking first candidate: %s\n", candidates[0].ID)
		}
		return &candidates[0], nil
	}

	// 3) Try to find exact region match within ranked candidates.
	for _, n := range candidates {
		if strings.EqualFold(n.Region, preferredRegion) {
			if debug {
				log.Printf("[discovery] exact region match found: %s (%s)\n", n.ID, n.Region)
			}
			return &n, nil
		}
	}

	// 4) Fallback: just return the top-ranked candidate.
	if debug {
		log.Printf("[discovery] no exact region match for %s; falling back to %s\n",
			preferredRegion, candidates[0].ID)
	}
	return &candidates[0], nil
}

// filterByBackend returns only nodes that are statically healthy
// and support the given backend.
func filterByBackend(nodes []NodeInfo, backend string) []NodeInfo {
	backend = strings.ToLower(backend)

	var out []NodeInfo
	for _, n := range nodes {
		if !n.Healthy {
			continue
		}
		if backend == "" {
			out = append(out, n)
			continue
		}
		for _, b := range n.Backends {
			if strings.EqualFold(b, backend) {
				out = append(out, n)
				break
			}
		}
	}
	return out
}
