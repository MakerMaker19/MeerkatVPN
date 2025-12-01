package client

import "context"

// NodeOffer represents a VPN node's published offer. We'll flesh this out later.
type NodeOffer struct {
	PubKey string
	// TODO: add bandwidth, latency, country, pricing, etc.
}

// FetchNodeOffers will eventually query relays over Nostr for node offers.
//
// For now it just returns nil so we can get a clean build.
func FetchNodeOffers(ctx context.Context, relays []string) ([]NodeOffer, error) {
	return nil, nil
}




// package client

// import (
// 	"context"
// 	"encoding/json"
// 	"time"

// 	"github.com/nbd-wtf/go-nostr"

// 	"github.com/MakerMaker19/meerkatvpn/pkg/vpn"
// 	"github.com/MakerMaker19/meerkatvpn/pkg/nostrutil"
// )

// type NodeOffer struct {
// 	NodePubKey    string `json:"node_pubkey"`
// 	Region        string `json:"region"`
// 	Country       string `json:"country"`
// 	City          string `json:"city"`
// 	BandwidthMbps int    `json:"bandwidth_mbps"`
// 	LatencyMsHint int    `json:"latency_ms_hint"`
// 	MaxSessions   int    `json:"max_sessions"`
// 	AcceptingNew  bool   `json:"accepting_new"`
// 	WGEndpoint    string `json:"wg_endpoint"`
// 	LastUpdated   int64  `json:"last_updated"`
// }

// // FetchNodeOffers queries relays for vpn-node-offer events, optionally filtered by region.
// func FetchNodeOffers(ctx context.Context, c *nostrutil.Client, region string, limit int) ([]NodeOffer, error) {
// 	f := nostr.Filter{
// 		Kinds: []int{30060},
// 		Tags:  nostr.TagMap{"t": []string{"vpn-node-offer"}},
// 	}
// 	if region != "" && region != "auto" {
// 		f.Tags["region"] = []string{region}
// 	}
// 	if limit <= 0 {
// 		limit = 100
// 	}
// 	f.Limit = limit

// 	offers := []NodeOffer{}

// 	for _, relay := range c.Relays {
// 		events, err := relay.QuerySync(ctx, nostr.Filters{f})
// 		if err != nil {
// 			continue
// 		}
// 		for _, ev := range events {
// 			var offer NodeOffer
// 			if err := json.Unmarshal([]byte(ev.Content), &offer); err != nil {
// 				continue
// 			}
// 			// fill NodePubKey from author if missing
// 			if offer.NodePubKey == "" {
// 				offer.NodePubKey = ev.PubKey
// 			}
// 			offers = append(offers, offer)
// 		}
// 	}

// 	// TODO: dedupe by NodePubKey, pick latest LastUpdated, sort by latency/bw/etc.
// 	return offers, nil
// }
