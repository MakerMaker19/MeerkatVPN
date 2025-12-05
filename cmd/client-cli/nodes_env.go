// cmd/client-cli/nodes_env.go
package main

import (
	"log"
	"os"
	"strings"

	"github.com/MakerMaker19/meerkatvpn/pkg/discovery"
)

func configureFinderFromEnv() {
	relaysEnv := os.Getenv("MEERKAT_NOSTR_RELAYS")
	poolPub := os.Getenv("MEERKAT_CLIENT_POOL_PUBKEY")

	if relaysEnv == "" || poolPub == "" {
		log.Println("[watch-nodes] MEERKAT_NOSTR_RELAYS or MEERKAT_CLIENT_POOL_PUBKEY not set; using static discovery only")
		return
	}

	parts := strings.Split(relaysEnv, ",")
	var relays []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			relays = append(relays, p)
		}
	}
	if len(relays) == 0 {
		log.Println("[watch-nodes] MEERKAT_NOSTR_RELAYS parsed to empty set; skipping Nostr finder")
		return
	}

	log.Printf("[watch-nodes] enabling Nostr discovery: pool=%s relays=%v\n", poolPub, relays)

	nf := discovery.NewNostrFinder(relays, poolPub, discovery.NewStaticFinder())
	discovery.SetDefaultFinder(nf)
}
