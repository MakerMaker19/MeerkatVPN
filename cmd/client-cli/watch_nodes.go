// cmd/client-cli/watch_nodes.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MakerMaker19/meerkatvpn/pkg/discovery"
)

func cmdWatchNodes() error {
	// Turn on verbose logs from discovery (nostr + health, etc.).
	_ = os.Setenv("MEERKAT_DEBUG_DISCOVERY", "1")

	// Configure the global Finder (static + Nostr, etc.) from env.
	// This should wire up Nostr relays, pool pubkey, static config, etc.
	configureFinderFromEnv()

	// ctx will be cancelled when the user hits Ctrl+C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("[watch-nodes] starting; press Ctrl+C to exit")

	// Periodically dump the node list so you can see updates as events arrive.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[watch-nodes] shutting down")
			return nil

		case <-ticker.C:
			nodes, err := discovery.ListNodes(ctx)
			if err != nil {
				log.Printf("[watch-nodes] error listing nodes: %v\n", err)
				continue
			}

			log.Printf("[watch-nodes] currently %d nodes:\n", len(nodes))
			now := time.Now()

			for _, n := range nodes {
				expired := n.IsExpired(now)

				log.Printf(
					"  - id=%s src=%s api=%s region=%s country=%s city=%s backends=%v healthy=%v latency=%s lastHealth=%s lastAnnounced=%s expires=%s expired=%v\n",
					n.ID,
					n.Source,
					n.APIURL,
					n.Region,
					n.Country,
					n.City,
					n.Backends,
					n.Healthy,
					n.LastLatency,
					n.LastHealthCheck.Format(time.RFC3339),
					n.LastAnnouncedAt.Format(time.RFC3339),
					n.ExpiresAt.Format(time.RFC3339),
					expired,
				)
			}
		}
	}
}
