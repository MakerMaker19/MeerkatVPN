package client

import (
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// WGKeypair represents a WireGuard keypair in string form (base64-encoded).
type WGKeypair struct {
	Private string
	Public  string
}

// GenerateWGKeypair creates a new WireGuard private/public keypair.
func GenerateWGKeypair() (WGKeypair, error) {
	priv, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return WGKeypair{}, err
	}
	pub := priv.PublicKey()
	return WGKeypair{
		Private: priv.String(),
		Public:  pub.String(),
	}, nil
}
