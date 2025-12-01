package client

import (
	"context"
	"fmt"
)

// ConnectToNode is currently a stub. It will eventually:
//   - discover node offers
//   - pick a node
//   - negotiate WireGuard credentials via Nostr
//   - write wg config and bring up the tunnel.
//
// For now it just logs so the project can build and run.
func ConnectToNode(ctx context.Context, nodePubKey string) error {
	fmt.Println("ConnectToNode: stub implementation – not wired up yet")
	fmt.Println("Requested node pubkey:", nodePubKey)
	return nil
}




// package client

// import (
// 	"context"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"log"
// 	"time"

// 	"github.com/google/uuid"
// 	"github.com/nbd-wtf/go-nostr"

// 	"github.com/MakerMaker19/meerkatvpn/pkg/nostrutil"
// )

// var ErrTimeout = errors.New("timed out waiting for vpn-credentials")

// // ConnectToNode:
// // - picks latest valid token from store
// // - sends vpn-session-request to nodePubHex
// // - waits for vpn-credentials DM
// // - writes WG config file
// // Returns the path to the config.
// func ConnectToNode(
// 	ctx context.Context,
// 	nc *nostrutil.Client,
// 	poolPubHex string,
// 	nodePubHex string,
// 	region string,
// 	timeout time.Duration,
// ) (string, error) {
// 	if region == "" {
// 		region = "auto"
// 	}
// 	if timeout <= 0 {
// 		timeout = 2 * time.Minute
// 	}

// 	store, err := LoadTokenStore()
// 	if err != nil {
// 		return "", fmt.Errorf("load token store: %w", err)
// 	}

// 	token, err := store.LatestValid(poolPubHex, time.Now())
// 	if err != nil {
// 		return "", fmt.Errorf("no valid subscription token: %w", err)
// 	}
// 	if token.Payload.UserPubKey != nc.PubKey {
// 		log.Println("WARNING: token user_pubkey != client Nostr pubkey; node may reject")
// 	}

// 	sessionID := "sess_" + uuid.New().String()
// 	log.Println("Session ID:", sessionID)

// 	// Build request
// 	req := map[string]interface{}{
// 		"session_id":         sessionID,
// 		"protocol":           "wireguard",
// 		"region":             region,
// 		"subscription_token": token,
// 	}
// 	reqJSON, _ := json.Marshal(req)

// 	if err := nc.SendDM(nodePubHex, string(reqJSON), nostr.Tags{
// 		{"t", "vpn-session-request"},
// 		{"session", sessionID},
// 	}); err != nil {
// 		return "", fmt.Errorf("send session request: %w", err)
// 	}

// 	log.Println("Sent session request; waiting for vpn-credentials DM...")

// 	// Subscribe to credentials for this session
// 	filter := nostr.Filters{
// 		{
// 			Kinds: []int{nostr.KindEncryptedDirectMessage},
// 			Tags: nostr.TagMap{
// 				"p":       []string{nc.PubKey},
// 				"session": []string{sessionID},
// 			},
// 		},
// 	}

// 	credsChan := make(chan WGCredentials, 1)

// 	for _, r := range nc.Relays {
// 		go func(relay *nostr.Relay) {
// 			ch, subErr := relay.Subscribe(ctx, filter)
// 			if subErr != nil {
// 				log.Println("subscribe error:", subErr)
// 				return
// 			}
// 			for ev := range ch {
// 				e := ev.Event
// 				if !e.Tags.Contains("t", "vpn-credentials") {
// 					continue
// 				}
// 				plain, decErr := nostr.Decrypt(nc.PrivKey, e.PubKey, e.Content)
// 				if decErr != nil {
// 					log.Println("decrypt credentials DM error:", decErr)
// 					continue
// 				}
// 				var creds WGCredentials
// 				if err := json.Unmarshal([]byte(plain), &creds); err != nil {
// 					log.Println("parse credentials error:", err)
// 					continue
// 				}
// 				if creds.SessionID != sessionID {
// 					continue
// 				}
// 				select {
// 				case credsChan <- creds:
// 				default:
// 				}
// 				return
// 			}
// 		}(r)
// 	}

// 	select {
// 	case creds := <-credsChan:
// 		path, err := WriteWGConfig(creds)
// 		if err != nil {
// 			return "", fmt.Errorf("write wg config: %w", err)
// 		}
// 		return path, nil

// 	case <-time.After(timeout):
// 		return "", ErrTimeout

// 	case <-ctx.Done():
// 		return "", ctx.Err()
// 	}
// }
