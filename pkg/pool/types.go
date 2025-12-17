package pool

import "time"

type Pricing struct {
	WeeklyPriceSats  int64
	MonthlyPriceSats int64
	YearlyPriceSats  int64
}

type InvoiceMetadata struct {
	Purpose     string `json:"purpose"`
	Plan        string `json:"plan"` // "weekly"|"monthly"|"yearly"
	NostrPubKey string `json:"nostr_pubkey"`
}

type InvoiceWebhook struct {
	InvoiceID  string          `json:"invoice_id"`
	AmountSats int64           `json:"amount_sats"`
	Settled    bool            `json:"settled"`
	Metadata   InvoiceMetadata `json:"metadata"`
}

// Subscription duration lookup
func PlanDuration(plan string) time.Duration {
	// Dev-shortened expiry for fast testing: 5 minutes for all plans.
	return 5 * time.Minute
}
