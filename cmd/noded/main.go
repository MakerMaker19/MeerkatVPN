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
	// TODO: add WireGuard config fields here later.
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

		writeJSON(w, http.StatusOK, sessionCreateResponse{
			Status:  "ok",
			Message: "session accepted (WireGuard config TBD)",
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
