package auction

// validateBidPrices filters bids with invalid (non-positive) prices.
func validateBidPrices(bids []CoreBid) (valid []CoreBid, rejectedBidIDs []string) {
	validBids := make([]CoreBid, 0, len(bids))
	rejectedIDs := make([]string, 0)

	for _, bid := range bids {
		if bid.Price > 0.0 {
			validBids = append(validBids, bid)
		} else {
			rejectedIDs = append(rejectedIDs, bid.ID)
		}
	}

	return validBids, rejectedIDs
}

// RunAuction executes the core auction logic using the global LogBase.
// For per-publisher log base, use RunAuctionWithBase.
func RunAuction(
	bids []CoreBid,
	bidFloor float64,
	queryEmbedding ...[]float64,
) *AuctionResult {
	var qe []float64
	if len(queryEmbedding) > 0 {
		qe = queryEmbedding[0]
	}
	return RunAuctionWithBase(bids, bidFloor, LogBase, qe)
}

// RunAuctionWithBase executes the core auction logic with a configurable log base:
// price validation → floor enforcement → ranking.
func RunAuctionWithBase(
	bids []CoreBid,
	bidFloor float64,
	logBase float64,
	queryEmbedding []float64,
) *AuctionResult {
	// Step 1: Validate bid prices
	validBids, priceRejectedBids := validateBidPrices(bids)

	// Step 2: Enforce floor price
	eligibleBids, floorRejectedBids := EnforceBidFloor(validBids, bidFloor)

	// Step 4: Rank eligible bids
	var ranking *CoreRankingResult
	var scoredBids []ScoredBid
	if len(queryEmbedding) > 0 {
		qe := queryEmbedding
		scoredBids = make([]ScoredBid, len(eligibleBids))
		for i, bid := range eligibleBids {
			scoredBids[i] = ScoredBid{
				CoreBid: bid,
				Score:   ComputeEmbeddingScoreWithBase(bid.Price, bid.Embedding, bid.Sigma, qe, logBase),
			}
		}
		ranking = RankScoredBids(scoredBids, defaultRandSource)
	} else {
		ranking = RankCoreBids(eligibleBids, defaultRandSource)
	}

	// Build sorted scored bids list in rank order
	var sortedScoredBids []ScoredBid
	if len(scoredBids) > 0 {
		scoredByID := make(map[string]ScoredBid, len(scoredBids))
		for _, sb := range scoredBids {
			scoredByID[sb.ID] = sb
		}
		for _, bidderID := range ranking.SortedBidders {
			bid := ranking.HighestBids[bidderID]
			if sb, ok := scoredByID[bid.ID]; ok {
				sortedScoredBids = append(sortedScoredBids, sb)
			}
		}
	}

	// Step 5: Extract winner and runner-up
	var winner, runnerUp *CoreBid
	if len(ranking.SortedBidders) > 0 {
		winner = ranking.HighestBids[ranking.SortedBidders[0]]
	}
	if len(ranking.SortedBidders) > 1 {
		runnerUp = ranking.HighestBids[ranking.SortedBidders[1]]
	}

	return &AuctionResult{
		Winner:              winner,
		RunnerUp:            runnerUp,
		EligibleBids:        eligibleBids,
		ScoredBids:          sortedScoredBids,
		PriceRejectedBidIDs: priceRejectedBids,
		FloorRejectedBidIDs: floorRejectedBids,
	}
}
