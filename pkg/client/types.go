package client

import (
	"github.com/MakerMaker19/meerkatvpn/pkg/vpn"
	)
	
type WGCredentials struct {
	SessionID string `json:"session_id"`
	Protocol  string `json:"protocol"`
	ExpiresAt int64  `json:"expires_at"`
	Wireguard struct {
		ClientPrivateKey string `json:"client_private_key"`
		ClientAddress    string `json:"client_address"`
		DNS              string `json:"dns"`
		ServerPublicKey  string `json:"server_public_key"`
		ServerEndpoint   string `json:"server_endpoint"`
		AllowedIPs       string `json:"allowed_ips"`
		KeepaliveSec     int    `json:"persistent_keepalive"`
	} `json:"wireguard"`
}

// A thin wrapper to expose tokens to UI/CLI.
type SubscriptionToken = vpn.SubscriptionToken
