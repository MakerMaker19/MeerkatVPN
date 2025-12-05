package discovery

import "context"

// NodeInfo describes a MeerkatVPN node that a client can connect to.
type NodeInfo struct {
	ID       string   // stable node ID (can be hostname, pubkey, etc.)
	APIURL   string   // base URL for noded, e.g. "http://46.62.204.11:9090"
	Region   string   // logical region, e.g. "us-east-1", "eu-west-1"
	Country  string   // "US", "DE", etc.
	City     string   // optional, e.g. "NYC", "Frankfurt"
	Backends []string // e.g. []string{"openvpn", "wireguard"}
	Healthy  bool     // static flag: whether node is enabled at config time
}

// Finder is an interface for any node discovery backend
// (static list, Nostr registry, HTTP pool API, etc.).
type Finder interface {
	// FindNode returns the "best" node for the given pool/region/backend.
	FindNode(ctx context.Context, poolPubKey, preferredRegion, backend string) (*NodeInfo, error)

	// ListNodes returns all nodes the finder knows about.
	ListNodes(ctx context.Context) ([]NodeInfo, error)
}

// defaultFinder is what the rest of the code uses.
// Right now it's backed by a static in-memory list (wrapped in a staticFinder).
var defaultFinder Finder = NewStaticFinder()

// NewStaticFinder returns a Finder implementation that uses staticNodes.
func NewStaticFinder() Finder {
	return staticFinder{}
}

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

// ListNodes exposes whatever the current finder knows about.
func ListNodes(ctx context.Context) ([]NodeInfo, error) {
	return defaultFinder.ListNodes(ctx)
}
