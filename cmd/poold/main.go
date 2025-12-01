package main

import (
	"context"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	// "time"

	"github.com/btcsuite/btcd/btcec/v2"

	"github.com/MakerMaker19/meerkatvpn/pkg/nostrutil"
	"github.com/MakerMaker19/meerkatvpn/pkg/pool"
)

func main() {
	// ---- 1. Read environment variables ----

	nostrPriv := os.Getenv("MEERKAT_POOL_NOSTR_PRIVKEY")
	if nostrPriv == "" {
		log.Fatal("MEERKAT_POOL_NOSTR_PRIVKEY not set (nsec or hex)")
	}

	webhookAddr := os.Getenv("MEERKAT_POOL_LN_WEBHOOK_ADDR")
	if webhookAddr == "" {
		webhookAddr = ":8080"
	}

	webhookSecret := os.Getenv("MEERKAT_POOL_LN_WEBHOOK_SECRET")
	if webhookSecret == "" {
		log.Println("WARNING: MEERKAT_POOL_LN_WEBHOOK_SECRET not set – webhook will accept any request with any secret")
	}

	pricing := pool.LoadPricingFromEnv()
	relayURLs := pool.RelayURLsFromEnv()

	// ---- 2. Create Nostr client ----

	ctx := context.Background()
	nostrClient, err := nostrutil.NewClient(ctx, nostrPriv, relayURLs)
	if err != nil {
		log.Fatalf("failed to init Nostr client: %v", err)
	}

	// ---- 3. Build btcec private key from same nostr priv ----

	parsed, err := nostrutil.ParsePrivKey(nostrPriv)
	if err != nil {
		log.Fatalf("failed to parse pool nostr privkey: %v", err)
	}

	privBytes, err := hex.DecodeString(parsed.PrivHex)
	if err != nil {
		log.Fatalf("failed to decode pool priv hex: %v", err)
	}

	poolPrivKey, _ := btcec.PrivKeyFromBytes(privBytes)

	// ---- 4. Create pool server ----

	srv := pool.NewServer(nostrClient, poolPrivKey, pricing, webhookSecret)

	// Periodically publish pricing as a Nostr event (optional)
	// srv.StartPricingPublisher(10 * time.Minute)

	// ---- 5. HTTP webhook handler ----

	http.HandleFunc("/ln/webhook", srv.LNWebhookHandler)

	log.Printf("poold: listening on %s for LN webhooks...", webhookAddr)
	if err := http.ListenAndServe(webhookAddr, nil); err != nil {
		log.Fatalf("ListenAndServe error: %v", err)
	}
}



// package main

// import (
//     "context"
//     "log"
//     "net/http"
//     "os"
//     "time"

//     "github.com/btcsuite/btcd/btcec/v2"
//     "github.com/nbd-wtf/go-nostr"
// 	"encoding/hex"

// 	"github.com/MakerMaker19/meerkatvpn/pkg/vpn"
// 	"github.com/MakerMaker19/meerkatvpn/pkg/nostrutil"
// 	"github.com/MakerMaker19/meerkatvpn/pkg/pool"
// 	"github.com/MakerMaker19/meerkatvpn/pkg/client"
// 	"github.com/MakerMaker19/meerkatvpn/pkg/wg"
// )

// func main() {
//     ctx := context.Background()

//     nostrPriv := os.Getenv("MEERKAT_POOL_NOSTR_PRIVKEY")
//     if nostrPriv == "" {
//         log.Fatal("MEERKAT_POOL_NOSTR_PRIVKEY not set")
//     }

//     relayURLs := pool.RelayURLsFromEnv()

//     nostrClient, err := nostrutil.NewClient(ctx, nostrPriv, relayURLs)
//     if err != nil {
//         log.Fatal("failed to init nostr client:", err)
//     }

//     // Derive btcec private key from nostrPriv
//     // If nostrPriv is hex secp256k1 secret
//     privKey, _ := btcec.PrivKeyFromBytes([]byte{}) // <-- replace with real decode
//     // NOTE: for real code, decode hex or nsec here. Left placeholder to avoid
//     // accidentally using raw bytes.

//     pricing := pool.LoadPricingFromEnv()
//     webhookSecret := os.Getenv("MEERKAT_POOL_LN_WEBHOOK_SECRET")
//     if webhookSecret == "" {
//         log.Println("WARNING: MEERKAT_POOL_LN_WEBHOOK_SECRET not set, using insecure default")
//         webhookSecret = "insecure-default"
//     }

//     srv := pool.NewServer(nostrClient, privKey, pricing, webhookSecret)

//     // Publish pricing every 10 minutes
//     srv.StartPricingPublisher(10 * time.Minute)

//     // HTTP server for LN webhook
//     mux := http.NewServeMux()
//     mux.HandleFunc("/ln/webhook", srv.LNWebhookHandler)

//     addr := os.Getenv("MEERKAT_POOL_LN_WEBHOOK_ADDR")
//     if addr == "" {
//         addr = ":8080"
//     }

//     log.Println("Meerkat poold listening on", addr)
//     if err := http.ListenAndServe(addr, mux); err != nil {
//         log.Fatal(err)
//     }

//     _ = nostr.ErrTimeout // avoid import pruning
// }



// // poolPubRaw := os.Getenv("MEERKAT_NODE_POOL_PUBKEY")
// // poolPubHex, err := nostrutil.ParsePubKey(poolPubRaw)
// // if err != nil { ... }


// // poolPubRaw := os.Getenv("MEERKAT_CLIENT_POOL_PUBKEY")
// // poolPubHex, err := nostrutil.ParsePubKey(poolPubRaw)

// // rawPriv := os.Getenv("MEERKAT_POOL_NOSTR_PRIVKEY")
// // parsed, err := nostrutil.ParsePrivKey(rawPriv) // handles nsec / hex
// // if err != nil { log.Fatal(err) }

// // privBytes, err := hex.DecodeString(parsed.PrivHex)
// // if err != nil { log.Fatal(err) }

// // btcecPriv, _ := btcec.PrivKeyFromBytes(privBytes)
