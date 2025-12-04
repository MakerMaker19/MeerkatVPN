// pkg/discovery/static.go
package discovery

import (
    "context"
    "errors"
    "strings"
)

// staticNodes is a simple, in-memory registry.
// You can later replace this with Nostr-based discovery or a pool API.
var staticNodes = []NodeInfo{
    {
        ID:       "local-dev",
        APIURL:   "http://localhost:9090",
        Region:   "local",
        Country:  "",
        City:     "",
        Backends: []string{"openvpn", "wireguard"},
        Healthy:  true,
    },
    // Example for your VPS:
    // {
    //     ID:       "node-us-east-1",
    //     APIURL:   "http://46.62.204.11:9090",
    //     Region:   "us-east-1",
    //     Country:  "US",
    //     City:     "NYC",
    //     Backends: []string{"openvpn"},
    //     Healthy:  true,
    // },
}

// staticFinder implements Finder using staticNodes.
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

// internal helper with the previous selection logic
func findNodeStatic(preferredRegion, backend string) (*NodeInfo, error) {
    if backend == "" {
        backend = "openvpn"
    }
    preferredRegion = strings.ToLower(strings.TrimSpace(preferredRegion))

    // 1) Filter by healthy + backend support
    candidates := filterByBackend(staticNodes, backend)
    if len(candidates) == 0 {
        return nil, errors.New("no nodes support backend " + backend)
    }

    // 2) If region is "auto" or empty, just pick first healthy candidate for now.
    if preferredRegion == "" || preferredRegion == "auto" {
        return &candidates[0], nil
    }

    // 3) Try to find exact region match.
    for _, n := range candidates {
        if strings.EqualFold(n.Region, preferredRegion) {
            return &n, nil
        }
    }

    // 4) Fallback: just return the first candidate.
    return &candidates[0], nil
}

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
