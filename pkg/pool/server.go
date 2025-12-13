package pool

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/google/uuid"
	"github.com/nbd-wtf/go-nostr"

	"github.com/MakerMaker19/meerkatvpn/pkg/nostrutil"
	"github.com/MakerMaker19/meerkatvpn/pkg/vpn"
)

type Server struct {
	Nostr         *nostrutil.Client
	PoolPrivKey   *btcec.PrivateKey
	PoolPubHex    string
	Pricing       Pricing
	WebhookSecret string
}

func NewServer(nostrClient *nostrutil.Client, poolPriv *btcec.PrivateKey, pricing Pricing, webhookSecret string) *Server {
	return &Server{
		Nostr:         nostrClient,
		PoolPrivKey:   poolPriv,
		PoolPubHex:    nostrClient.PubKey,
		Pricing:       pricing,
		WebhookSecret: webhookSecret,
	}
}

// Middleware: simply check a header secret
func (s *Server) validateSecret(r *http.Request) bool {
	sec := r.Header.Get("X-Meerkat-Secret")
	return sec != "" && sec == s.WebhookSecret
}

func (s *Server) LNWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if !s.validateSecret(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var inv InvoiceWebhook
	if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
		log.Println("LN webhook decode error:", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if !inv.Settled {
		w.WriteHeader(http.StatusOK)
		return
	}

	if inv.Metadata.Purpose != "vpn-subscription" {
		w.WriteHeader(http.StatusOK)
		return
	}

	userPub := inv.Metadata.NostrPubKey
	plan := inv.Metadata.Plan
	if userPub == "" || plan == "" {
		log.Println("missing nostr_pubkey or plan in metadata")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Compute expiry
	now := time.Now().Unix()
	duration := PlanDuration(plan)
	expires := time.Now().Add(duration).Unix()

	payload := vpn.SubscriptionPayload{
		TokenID:          "sub_" + uuid.New().String(),
		UserPubKey:       userPub,
		SubscriptionType: plan,
		Tier:             "full",
		IssuedAt:         now,
		ExpiresAt:        expires,
		Nonce:            uuid.New().String(),
		IssuerPubKey:     s.PoolPubHex, // pool's nostr pubkey
	}

	token, err := vpn.SignSubscription(s.PoolPrivKey, payload)
	if err != nil {
		log.Println("failed to sign subscription:", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("LN webhook: issued subscription token %s for user %s plan=%s (expires=%d)\n",
		token.Payload.TokenID,
		token.Payload.UserPubKey,
		token.Payload.SubscriptionType,
		token.Payload.ExpiresAt,
	)

	if err := s.sendSubscriptionDM(userPub, token); err != nil {
		log.Println("failed to send sub DM:", err)
		// Don't fail webhook: Lightning side already settled
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) sendSubscriptionDM(userPubKey string, token vpn.SubscriptionToken) error {
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}

	log.Printf("Subscription token JSON: %s\n", string(data))
	log.Printf("Attempting to send subscription DM to %s\n", userPubKey)

	tags := nostr.Tags{
		{"t", "vpn-subscription"},
	}

	return s.Nostr.SendDM(context.Background(), userPubKey, string(data), tags)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// DevInvoiceHandler is a simple dev-only endpoint to issue a token on demand.
// It simulates invoice creation + immediate settlement.
//
// POST /invoice with JSON: { "nostr_pubkey": "...", "plan": "weekly|monthly|yearly" }
func (s *Server) DevInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		NostrPubKey string `json:"nostr_pubkey"`
		Plan        string `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	req.Plan = strings.TrimSpace(strings.ToLower(req.Plan))
	if req.Plan == "" {
		req.Plan = "monthly"
	}
	if req.NostrPubKey == "" {
		http.Error(w, "nostr_pubkey is required", http.StatusBadRequest)
		return
	}

	// Compute expiry based on plan.
	duration := PlanDuration(req.Plan)
	now := time.Now()
	expires := now.Add(duration).Unix()

	payload := vpn.SubscriptionPayload{
		TokenID:          "sub_" + uuid.New().String(),
		UserPubKey:       req.NostrPubKey,
		SubscriptionType: req.Plan,
		Tier:             "full",
		IssuedAt:         now.Unix(),
		ExpiresAt:        expires,
		Nonce:            uuid.New().String(),
		IssuerPubKey:     s.PoolPubHex,
	}

	token, err := vpn.SignSubscription(s.PoolPrivKey, payload)
	if err != nil {
		log.Println("failed to sign subscription:", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := s.sendSubscriptionDM(req.NostrPubKey, token); err != nil {
		log.Println("failed to send sub DM:", err)
		// Still return success; the caller can wait for DM separately.
	}

	resp := struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		TokenID string `json:"token_id"`
		Invoice string `json:"invoice"` // placeholder/dev
		Plan    string `json:"plan"`
		Expires int64  `json:"expires_at"`
		PubKey  string `json:"issuer_pubkey"`
	}{
		Status:  "ok",
		Message: "dev invoice simulated; token issued via DM",
		TokenID: token.Payload.TokenID,
		Invoice: "dev-" + token.Payload.TokenID,
		Plan:    req.Plan,
		Expires: token.Payload.ExpiresAt,
		PubKey:  s.PoolPubHex,
	}
	writeJSON(w, http.StatusOK, resp)
}

// Pricing publisher: emits kind=30070 periodically
func (s *Server) StartPricingPublisher(interval time.Duration) {
	go func() {
		for {
			s.publishPricing()
			time.Sleep(interval)
		}
	}()
}

func (s *Server) publishPricing() {
	contentMap := map[string]interface{}{
		"currency":           "sats",
		"weekly_price_sats":  s.Pricing.WeeklyPriceSats,
		"monthly_price_sats": s.Pricing.MonthlyPriceSats,
		"yearly_price_sats":  s.Pricing.YearlyPriceSats,
		"price_last_updated": time.Now().Unix(),
	}
	data, err := json.Marshal(contentMap)
	if err != nil {
		log.Println("pricing marshal error:", err)
		return
	}

	ev := nostr.Event{
		PubKey:    s.Nostr.PubKey,
		CreatedAt: nostr.Now(),
		Kind:      30070,
		Tags: nostr.Tags{
			{"t", "vpn-network-pricing"},
		},
		Content: string(data),
	}

	if err := s.Nostr.Publish(context.Background(), ev); err != nil {
		log.Println("publish pricing error:", err)
	} else {
		log.Println("published pricing event")
	}
}

// Simple helper to load pricing from env
func LoadPricingFromEnv() Pricing {
	w, _ := strconv.ParseInt(os.Getenv("MEERKAT_POOL_WEEKLY_SATS"), 10, 64)
	m, _ := strconv.ParseInt(os.Getenv("MEERKAT_POOL_MONTHLY_SATS"), 10, 64)
	y, _ := strconv.ParseInt(os.Getenv("MEERKAT_POOL_YEARLY_SATS"), 10, 64)
	if w == 0 {
		w = 1500
	}
	if m == 0 {
		m = 5000
	}
	if y == 0 {
		y = 45000
	}
	return Pricing{
		WeeklyPriceSats:  w,
		MonthlyPriceSats: m,
		YearlyPriceSats:  y,
	}
}

// Useful helper to parse relays from env
func RelayURLsFromEnv() []string {
	v := os.Getenv("MEERKAT_POOL_RELAYS")
	if v == "" {
		return []string{"wss://relay.damus.io", "wss://relay.primal.net"}
	}
	parts := strings.Split(v, ",")
	out := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
