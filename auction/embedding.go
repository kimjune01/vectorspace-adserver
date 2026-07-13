package auction

import "math"

// LogBase controls bid compression in the scoring function.
// score_i(x) = log_B(price) - distance² / sigma²
// See: june.kim/three-levers
const LogBase = 5.0

// logB computes log base B of x.
func logB(x float64) float64 {
	return math.Log(x) / math.Log(LogBase)
}

// SquaredEuclideanDistance computes ||a - b||² in embedding space.
// Returns +Inf if dimensions don't match.
func SquaredEuclideanDistance(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return math.Inf(1)
	}
	var sum float64
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return sum
}

// scoreDistanceTerm computes the distance penalty ||x - c||² / σ² for a bid
// at a query point. Bids without an embedding (or with no query point) carry
// no penalty. σ = 0 is the keyword limit: zero penalty at the exact point,
// infinite penalty anywhere else. Exact equality is meaningful because the
// embedder is deterministic and cached — identical text yields identical
// vectors. See: june.kim/keywords-are-tiny-circles
func scoreDistanceTerm(bid *CoreBid, queryEmbedding []float64) float64 {
	if len(bid.Embedding) == 0 || len(queryEmbedding) == 0 {
		return 0
	}
	distSq := SquaredEuclideanDistance(bid.Embedding, queryEmbedding)
	if bid.Sigma == 0 {
		if distSq == 0 {
			return 0
		}
		return math.Inf(1)
	}
	return distSq / (bid.Sigma * bid.Sigma)
}

// ComputeScore calculates an advertiser's score at a query point.
// score_i(x) = log_B(b_i) - ||x - c_i||² / σ_i²
// See: june.kim/power-diagrams-ad-auctions
func ComputeScore(bid CoreBid, queryEmbedding []float64) float64 {
	if bid.Price <= 0 {
		return math.Inf(-1)
	}
	term := scoreDistanceTerm(&bid, queryEmbedding)
	if math.IsInf(term, 1) {
		return math.Inf(-1)
	}
	return logB(bid.Price) - term
}
