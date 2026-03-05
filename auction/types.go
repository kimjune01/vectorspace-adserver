package auction

// CoreBid represents a single bid in the auction system.
type CoreBid struct {
	ID       string  `json:"id"`
	Bidder   string  `json:"bidder"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`

	// Embedding-space auction fields (all optional; zero values = pure price bid)
	Embedding []float64 `json:"embedding,omitempty"`
	Sigma     float64   `json:"sigma,omitempty"`
}

// ScoredBid pairs a CoreBid with a pre-computed embedding score.
type ScoredBid struct {
	CoreBid
	Score float64
}

// CoreRankingResult contains the ranked bidders and their highest bids.
type CoreRankingResult struct {
	Ranks         map[string]int      `json:"ranks"`
	HighestBids   map[string]*CoreBid `json:"highest_bids"`
	SortedBidders []string            `json:"sorted_bidders"`
}

// AuctionResult contains the complete results of running an auction.
type AuctionResult struct {
	Winner              *CoreBid
	RunnerUp            *CoreBid
	EligibleBids        []CoreBid
	ScoredBids          []ScoredBid // all bids with scores, sorted by rank
	PriceRejectedBidIDs []string
	FloorRejectedBidIDs []string
}
