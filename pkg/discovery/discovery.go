// pkg/discovery/discovery.go
package discovery

import "context"

// Finder is an interface for any node discovery backend
// (static list, Nostr registry, HTTP pool API, etc.).
type Finder interface {
    FindNode(ctx context.Context, poolPubKey, preferredRegion, backend string) (*NodeInfo, error)
}

// defaultFinder is what the rest of the code uses.
// Right now it's backed by a static in-memory list.
var defaultFinder Finder = staticFinder{}

// SetDefaultFinder lets you swap in a different implementation
// (e.g., a Nostr-based finder) in the future.
func SetDefaultFinder(f Finder) {
    if f != nil {
        defaultFinder = f
    }
}

// FindNode is the package-level helper used by the client.
// Under the hood it delegates to defaultFinder.
func FindNode(
    ctx context.Context,
    poolPubKey string,
    preferredRegion string,
    backend string,
) (*NodeInfo, error) {
    return defaultFinder.FindNode(ctx, poolPubKey, preferredRegion, backend)
}
