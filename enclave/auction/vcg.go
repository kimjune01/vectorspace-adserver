package auction

import "math"

// ComputeVCGPayment determines the payment for the auction winner using
// VCG-style pricing adjusted for embedding-space distance.
// Uses the global LogBase. For per-publisher log base, use ComputeVCGPaymentWithBase.
func ComputeVCGPayment(result *AuctionResult, queryEmbedding []float64) float64 {
	return ComputeVCGPaymentWithBase(result, queryEmbedding, LogBase)
}

// ComputeVCGPaymentWithBase determines the payment for the auction winner using
// VCG-style pricing adjusted for embedding-space distance.
//
// When a runner-up exists, payment = runner-up price × B^(distW²/σW² - distR²/σR²),
// where B = logBase. This accounts for the embedding distance differential between
// winner and runner-up at the query location. The winner pays only enough to beat
// the runner-up in embedding-adjusted score space.
//
// Caps:
//   - Individual rationality: payment ≤ winner's bid price
//   - Sanity cap: payment ≤ 10× winner's bid price
func ComputeVCGPaymentWithBase(result *AuctionResult, queryEmbedding []float64, logBase float64) float64 {
	if result.Winner == nil {
		return 0
	}

	winner := result.Winner
	var payment float64

	if result.RunnerUp != nil {
		ru := result.RunnerUp
		distW2 := SquaredEuclideanDistance(winner.Embedding, queryEmbedding)
		distR2 := SquaredEuclideanDistance(ru.Embedding, queryEmbedding)
		sigmaW := winner.Sigma
		sigmaR := ru.Sigma

		if sigmaW > 0 && sigmaR > 0 {
			payment = ru.Price * math.Pow(logBase, distW2/(sigmaW*sigmaW)-distR2/(sigmaR*sigmaR))
		} else {
			payment = ru.Price
		}
	} else {
		payment = winner.Price
	}

	// Individual rationality: never pay more than your bid
	if payment > winner.Price {
		payment = winner.Price
	}
	// Sanity cap
	if payment > winner.Price*10 {
		payment = winner.Price * 10
	}

	return payment
}
