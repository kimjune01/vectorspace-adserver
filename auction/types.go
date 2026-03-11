package auction

// CoreBid represents an advertiser's position in embedding space.
// Fields map to the three numbers an advertiser declares:
// center (Embedding), reach (Sigma), and value (Price).
// See: june.kim/buying-space-not-keywords
type CoreBid struct {
	ID        string    `json:"id"`
	Bidder    string    `json:"bidder"`
	Price     float64   `json:"price"`
	Currency  string    `json:"currency,omitempty"`
	Embedding []float64 `json:"embedding,omitempty"`
	Sigma     float64   `json:"sigma,omitempty"`
}

// ScoredBid pairs a bid with its computed score at a query point.
// score_i(x) = log_B(b_i) - ||x - c_i||² / σ_i²
type ScoredBid struct {
	CoreBid
	Score float64 `json:"score"`
}

// AuctionResult holds the outcome of a single sealed-bid auction.
type AuctionResult struct {
	Winner               *CoreBid    `json:"winner"`
	RunnerUp             *CoreBid    `json:"runner_up"`
	EligibleBids         []CoreBid   `json:"eligible_bids"`
	ScoredBids           []ScoredBid `json:"scored_bids"`
	PriceRejectedBidIDs  []string    `json:"price_rejected_bid_ids"`
	FloorRejectedBidIDs  []string    `json:"floor_rejected_bid_ids"`
}
