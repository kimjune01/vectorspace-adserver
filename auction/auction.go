package auction

import "math"

// RunAuction executes a single sealed-bid auction in embedding space.
// One-shot: advertisers declare center, sigma, bid. No iterative rounds.
// Winner is argmax of score_i(x) = log_B(b_i) - ||x - c_i||² / σ_i²
// Payment determined by VCG (see ComputeVCGPayment).
// See: june.kim/one-shot-bidding, june.kim/power-diagrams-ad-auctions
func RunAuction(bids []CoreBid, bidFloor float64, queryEmbedding ...[]float64) *AuctionResult {
	result := &AuctionResult{}

	// Reject non-positive bids
	var valid []CoreBid
	for _, bid := range bids {
		if bid.Price <= 0 {
			result.PriceRejectedBidIDs = append(result.PriceRejectedBidIDs, bid.ID)
		} else {
			valid = append(valid, bid)
		}
	}

	// Enforce bid floor (publisher's τ for price)
	eligible, floorRejected := EnforceBidFloor(valid, bidFloor)
	result.FloorRejectedBidIDs = floorRejected
	result.EligibleBids = eligible

	if len(eligible) == 0 {
		return result
	}

	// Determine query point
	var qe []float64
	if len(queryEmbedding) > 0 {
		qe = queryEmbedding[0]
	}

	// Score and rank all eligible bids
	ranked := RankByScore(eligible, qe)
	result.ScoredBids = ranked

	// Winner: highest score (must be finite)
	if len(ranked) > 0 && !math.IsInf(ranked[0].Score, -1) {
		winner := ranked[0].CoreBid
		result.Winner = &winner
	}

	// Runner-up: second highest score
	if len(ranked) > 1 && !math.IsInf(ranked[1].Score, -1) {
		runnerUp := ranked[1].CoreBid
		result.RunnerUp = &runnerUp
	}

	return result
}
