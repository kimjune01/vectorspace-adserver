package auction

import "math"

// ComputeVCGPayment determines the payment for the auction winner using
// VCG-style pricing adjusted for embedding-space distance.
//
// When a runner-up exists, payment = runner-up price × B^(distW²/σW² - distR²/σR²),
// where B = LogBase. This accounts for the embedding distance differential between
// winner and runner-up at the query location. The winner pays only enough to beat
// the runner-up in embedding-adjusted score space.
//
// The payment is the winner's critical value: the larger of the
// competitor-implied threshold and the reserve (bid floor). With no
// runner-up, the critical value is the reserve alone — zero if the
// publisher set none.
//
// Individual rationality: payment ≤ winner's bid price.
func ComputeVCGPayment(result *AuctionResult, queryEmbedding []float64, bidFloor float64) float64 {
	if result.Winner == nil {
		return 0
	}

	winner := result.Winner
	var payment float64

	if result.RunnerUp != nil {
		ru := result.RunnerUp
		// Use the same distance term as ComputeScore so the payment is the
		// exact limit of the scoring rule, including the σ = 0 keyword case
		// (an exact-match winner carries zero penalty; a runner-up cannot
		// have an infinite penalty, or it would have scored -Inf and been
		// excluded from ranking).
		termW := scoreDistanceTerm(winner, queryEmbedding)
		termR := scoreDistanceTerm(ru, queryEmbedding)
		payment = ru.Price * math.Pow(LogBase, termW-termR)
	}

	// Reserve: the floor is a price-space threshold the winner had to clear.
	if payment < bidFloor {
		payment = bidFloor
	}

	// Individual rationality: never pay more than your bid
	if payment > winner.Price {
		payment = winner.Price
	}

	return payment
}
