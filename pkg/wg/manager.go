package wg

import (
    "os/exec"
    "time"
)

type Config struct {
    ClientPrivateKey string `json:"client_private_key"`
    ClientAddress    string `json:"client_address"`
    DNS              string `json:"dns"`
    ServerPublicKey  string `json:"server_public_key"`
    ServerEndpoint   string `json:"server_endpoint"`
    AllowedIPs       string `json:"allowed_ips"`
    KeepaliveSec     int    `json:"keepalive_sec"`
}

type Manager struct {
    InterfaceName string
    ServerPubKey  string
    Endpoint      string
}

func (m *Manager) NewSession(sessionID string, expiry time.Time) (Config, error) {
    // NOTE: In real code, generate keys and call `wg set` or use wireguard-go.
    // For skeleton:
    cfg := Config{
        ClientPrivateKey: "CLIENT_PRIV_KEY_PLACEHOLDER",
        ClientAddress:    "10.66.66.2/32",
        DNS:              "1.1.1.1",
        ServerPublicKey:  m.ServerPubKey,
        ServerEndpoint:   m.Endpoint,
        AllowedIPs:       "0.0.0.0/0, ::/0",
        KeepaliveSec:     25,
    }

    // Example: schedule removal
    go func() {
        <-time.After(time.Until(expiry))
        _ = m.RemoveSession(sessionID)
    }()

    return cfg, nil
}

func (m *Manager) RemoveSession(sessionID string) error {
    // Here you’d call wg / ip commands to remove peer by pubkey/IP.
    cmd := exec.Command("echo", "remove-session", sessionID)
    return cmd.Run()
}
