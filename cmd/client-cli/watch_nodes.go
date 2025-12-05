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
	// Turn on verbose logs from discovery.
	_ = os.Setenv("MEERKAT_DEBUG_DISCOVERY", "1")

	configureFinderFromEnv()

	// ctx will be cancelled when the user hits Ctrl+C
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
			for _, n := range nodes {
				log.Printf("  - id=%s api=%s region=%s country=%s city=%s backends=%v healthy=%v\n",
					n.ID, n.APIURL, n.Region, n.Country, n.City, n.Backends, n.Healthy)
			}
		}
	}
}
