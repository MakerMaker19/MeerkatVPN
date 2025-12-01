package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/MakerMaker19/meerkatvpn/pkg/vpn"
)

type sessionCreateRequest struct {
	Token vpn.SubscriptionToken `json:"token"`
}

type sessionCreateResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`

	// WireGuard parameters for the client to build a config.
	ServerPubKey string   `json:"server_pubkey,omitempty"`
	Endpoint     string   `json:"endpoint,omitempty"`
	ClientIP     string   `json:"client_ip,omitempty"`
	AllowedIPs   string   `json:"allowed_ips,omitempty"`
	DNS          []string `json:"dns,omitempty"`
}

func main() {
	addr := os.Getenv("MEERKAT_NODE_LISTEN_ADDR")
	if addr == "" {
		addr = ":9090"
	}

	allowedPool := os.Getenv("MEERKAT_NODE_ALLOWED_POOL_PUBKEY") // optional hex pubkey filter

	http.HandleFunc("/session/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req sessionCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Println("session create decode error:", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		tok := req.Token

		// Optional issuer pubkey filter.
		if allowedPool != "" && tok.Payload.IssuerPubKey != allowedPool {
			log.Printf("session create: issuer mismatch (got %s, expected %s)\n",
				tok.Payload.IssuerPubKey, allowedPool)
			writeJSON(w, http.StatusForbidden, sessionCreateResponse{
				Status:  "error",
				Message: "token issuer not allowed",
			})
			return
		}

		// Verify signature + expiry.
		if err := vpn.VerifySubscription(tok, time.Now()); err != nil {
			log.Println("session create: token invalid:", err)
			writeJSON(w, http.StatusForbidden, sessionCreateResponse{
				Status:  "error",
				Message: "invalid token: " + err.Error(),
			})
			return
		}

		// TODO: here we would allocate a WireGuard peer, IP, etc.
		log.Printf("session create: accepted token %s for user %s\n",
			tok.Payload.TokenID, tok.Payload.UserPubKey)

		// Read WG-related env vars or use some simple defaults.
		serverPub := os.Getenv("MEERKAT_NODE_WG_PUBKEY")
		if serverPub == "" {
			// placeholder / fake
			serverPub = "SERVER_WG_PUBKEY_PLACEHOLDER"
		}
		endpoint := os.Getenv("MEERKAT_NODE_WG_ENDPOINT")
		if endpoint == "" {
			endpoint = "127.0.0.1:51820"
		}
		clientIP := "10.8.0.2/32" // later: allocate dynamically
		allowed := "0.0.0.0/0, ::/0"
		dns := []string{"1.1.1.1"}

		writeJSON(w, http.StatusOK, sessionCreateResponse{
			Status:      "ok",
			Message:     "session accepted (WireGuard config TBD)",
			ServerPubKey: serverPub,
			Endpoint:     endpoint,
			ClientIP:     clientIP,
			AllowedIPs:   allowed,
			DNS:          dns,
		})

	})

	log.Printf("noded: listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
