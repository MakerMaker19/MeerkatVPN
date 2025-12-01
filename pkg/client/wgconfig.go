package client

import (
	"fmt"
	"os"
	"path/filepath"
)

const tokenDirName = ".meerkatvpn"

type WGConfigParams struct {
	PrivateKey string
	Address    string
	DNS        []string
	ServerPub  string
	Endpoint   string
	AllowedIPs string
	Keepalive  int
}

func BuildWGConfig(p WGConfigParams) string {
	if p.AllowedIPs == "" {
		p.AllowedIPs = "0.0.0.0/0, ::/0"
	}
	if p.Keepalive == 0 {
		p.Keepalive = 25
	}

	dnsLine := ""
	if len(p.DNS) > 0 {
		dnsLine = fmt.Sprintf("DNS = %s\n", joinStrings(p.DNS, ", "))
	}

	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
%v[Peer]
PublicKey = %s
AllowedIPs = %s
Endpoint = %s
PersistentKeepalive = %d
`, p.PrivateKey, p.Address, dnsLine, p.ServerPub, p.AllowedIPs, p.Endpoint, p.Keepalive)
}

func DefaultWGConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, tokenDirName, "wg")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "meerkat.conf"), nil
}

func WriteWGConfig(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for i := 1; i < len(ss); i++ {
		out += sep + ss[i]
	}
	return out
}
