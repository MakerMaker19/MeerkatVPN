package client

import (
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ClientConfig holds user preferences for the client CLI / daemon.
// Env vars always override these when both are set.
type ClientConfig struct {
	NostrPrivKey    string   `yaml:"nostr_privkey"`
	Relays          []string `yaml:"relays"`
	PoolPubKey      string   `yaml:"pool_pubkey"`
	PreferredRegion string   `yaml:"preferred_region"`
	Backend         string   `yaml:"backend"`
	NodeURL         string   `yaml:"node_url"`
}

// clientConfig is a package-global, filled at init time.
var clientConfig *ClientConfig

// Config returns the loaded client config, or nil if none.
func Config() *ClientConfig {
	return clientConfig
}

var defaultRelays = []string{
    "wss://relay.damus.io",
    "wss://relay.primal.net",
    "wss://nos.lol",
    "wss://nostr.wine",
    "wss://relay.current.fyi",
}

func DefaultRelays() []string {
    out := make([]string, len(defaultRelays))
    copy(out, defaultRelays)
    return out
}


func init() {
	cfg, err := loadClientConfig()
	if err != nil {
		log.Printf("[client] warning: failed to load meerkat-client.yaml: %v", err)
	}
	clientConfig = cfg
}

// loadClientConfig attempts to load meerkat-client.yaml from:
//
//   1) $MEERKAT_CLIENT_CONFIG if set
//   2) os.UserConfigDir()/meerkat-client.yaml (eg ~/.config/meerkat-client.yaml)
//   3) $HOME/meerkat-client.yaml
//
// If no file is found, it returns (nil, nil).
func loadClientConfig() (*ClientConfig, error) {
	// 1) Explicit path
	if p := os.Getenv("MEERKAT_CLIENT_CONFIG"); p != "" {
		return readConfigFile(p)
	}

	// 2) User config dir
	if dir, err := os.UserConfigDir(); err == nil {
		if cfg, err := readConfigFile(filepath.Join(dir, "meerkat-client.yaml")); err == nil || !os.IsNotExist(err) {
			return cfg, err
		}
	}

	// 3) Home dir fallback
	if home, err := os.UserHomeDir(); err == nil {
		if cfg, err := readConfigFile(filepath.Join(home, "meerkat-client.yaml")); err == nil || !os.IsNotExist(err) {
			return cfg, err
		}
	}

	// No config file, not an error
	return nil, nil
}

func readConfigFile(path string) (*ClientConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg ClientConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
