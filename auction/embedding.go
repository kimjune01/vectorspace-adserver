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

// ComputeScore calculates an advertiser's score at a query point.
// score_i(x) = log_B(b_i) - ||x - c_i||² / σ_i²
// See: june.kim/power-diagrams-ad-auctions
func ComputeScore(bid CoreBid, queryEmbedding []float64) float64 {
	if bid.Price <= 0 {
		return math.Inf(-1)
	}
	if len(bid.Embedding) == 0 || len(queryEmbedding) == 0 {
		return logB(bid.Price)
	}
	if bid.Sigma == 0 {
		return math.Inf(-1)
	}
	distSq := SquaredEuclideanDistance(bid.Embedding, queryEmbedding)
	return logB(bid.Price) - distSq/(bid.Sigma*bid.Sigma)
}
