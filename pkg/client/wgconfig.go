package client

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

const tokenDirName = ".meerkatvpn"

func WriteWGConfig(creds WGCredentials) (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(u.HomeDir, tokenDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("meerkat-%s.conf", creds.SessionID)
	path := filepath.Join(dir, filename)

	content := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
DNS = %s

[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = %s
PersistentKeepalive = %d
`,
		creds.Wireguard.ClientPrivateKey,
		creds.Wireguard.ClientAddress,
		creds.Wireguard.DNS,
		creds.Wireguard.ServerPublicKey,
		creds.Wireguard.ServerEndpoint,
		creds.Wireguard.AllowedIPs,
		creds.Wireguard.KeepaliveSec,
	)

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}
	return path, nil
}
