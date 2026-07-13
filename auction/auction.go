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

	// Collapse to each bidder's best bid before ranking, mirroring the
	// enclave copy's RankScoredBids semantics. This matters for keyword
	// groups, where one bidder holds several positions: the group competes
	// as one bidder, and VCG prices against the competition, not against
	// the winner's other keywords.
	best := make(map[string]ScoredBid, len(eligible))
	order := make([]string, 0, len(eligible))
	for _, bid := range eligible {
		score := ComputeScore(bid, qe)
		cur, seen := best[bid.Bidder]
		if !seen {
			order = append(order, bid.Bidder)
			best[bid.Bidder] = ScoredBid{CoreBid: bid, Score: score}
		} else if score > cur.Score {
			best[bid.Bidder] = ScoredBid{CoreBid: bid, Score: score}
		}
	}
	collapsed := make([]CoreBid, 0, len(order))
	for _, bidder := range order {
		collapsed = append(collapsed, best[bidder].CoreBid)
	}

	// Score and rank per-bidder best bids
	ranked := RankByScore(collapsed, qe)
	result.ScoredBids = ranked

	// Winner: highest score (must be finite)
	if len(ranked) > 0 && !math.IsInf(ranked[0].Score, -1) {
		winner := ranked[0].CoreBid
		result.Winner = &winner
	}

	// Runner-up: second-highest score (a different bidder by construction)
	if result.Winner != nil && len(ranked) > 1 && !math.IsInf(ranked[1].Score, -1) {
		runnerUp := ranked[1].CoreBid
		result.RunnerUp = &runnerUp
	}

	return result
}
