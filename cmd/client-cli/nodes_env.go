// cmd/client-cli/nodes_env.go
package main

import (
	"log"
	"os"
	"strings"

	"github.com/MakerMaker19/meerkatvpn/pkg/client"
	"github.com/MakerMaker19/meerkatvpn/pkg/discovery"
	"github.com/MakerMaker19/meerkatvpn/pkg/nostrutil"
)

func configureFinderFromEnv() {
	// Pool pubkey: env -> config -> bail.
	poolRaw := os.Getenv("MEERKAT_CLIENT_POOL_PUBKEY")
	if poolRaw == "" {
		if cfg := client.Config(); cfg != nil && cfg.PoolPubKey != "" {
			poolRaw = cfg.PoolPubKey
		}
	}
	if poolRaw == "" {
		log.Println("[watch-nodes] no MEERKAT_CLIENT_POOL_PUBKEY env or pool_pubkey in config; skipping Nostr finder")
		return
	}

	poolPubHex, err := nostrutil.ParsePubKey(poolRaw)
	if err != nil {
		log.Printf("[watch-nodes] failed to parse pool pubkey (%q): %v; skipping Nostr finder\n", poolRaw, err)
		return
	}

	// Relays: env -> config -> defaults.
	var relays []string
	if v := os.Getenv("MEERKAT_NOSTR_RELAYS"); v != "" {
		parts := strings.Split(v, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				relays = append(relays, p)
			}
		}
	} else if cfg := client.Config(); cfg != nil && len(cfg.Relays) > 0 {
		relays = cfg.Relays
	} else {
		relays = client.DefaultRelays()
	}

	if len(relays) == 0 {
		log.Println("[watch-nodes] no relays from env/config; skipping Nostr finder")
		return
	}

	log.Printf("[watch-nodes] enabling Nostr discovery: pool=%s relays=%v\n", poolPubHex, relays)

	// Start Nostr discovery feeding the global registry.
	_ = discovery.NewNostrFinder(relays, poolPubHex, nil)

	// Use the registry-backed finder for all list/find calls.
	discovery.SetDefaultFinder(discovery.NewRegistryFinder(discovery.GlobalRegistry()))
}
