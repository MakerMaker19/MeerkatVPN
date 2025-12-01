package pool

import "time"

type Pricing struct {
    WeeklyPriceSats  int64
    MonthlyPriceSats int64
    YearlyPriceSats  int64
}

type InvoiceMetadata struct {
    Purpose    string `json:"purpose"`
    Plan       string `json:"plan"`        // "weekly"|"monthly"|"yearly"
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
    switch plan {
    case "weekly":
        return 7 * 24 * time.Hour
    case "yearly":
        return 365 * 24 * time.Hour
    default:
        return 30 * 24 * time.Hour
    }
}
