// pkg/discovery/types.go
package discovery

// NodeInfo describes a MeerkatVPN node that a client can connect to.
type NodeInfo struct {
    ID       string   // stable node ID (can be hostname, pubkey, etc.)
    APIURL   string   // base URL for noded, e.g. "http://46.62.204.11:9090"
    Region   string   // logical region, e.g. "us-east-1", "eu-west-1"
    Country  string   // "US", "DE", etc.
    City     string   // optional, e.g. "NYC", "Frankfurt"
    Backends []string // e.g. []string{"openvpn", "wireguard"}
    Healthy  bool     // whether this node is currently considered usable
}
