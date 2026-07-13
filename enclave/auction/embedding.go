package auction

import "math"

// LogBase is the fixed base of the logarithm used in bid scoring.
// See "Three Levers": log base is not a meaningful lever because
// advertisers adjust σ to compensate for any compression.
const LogBase = 5.0

// logB computes log_base(x) = ln(x) / ln(LogBase).
func logB(x float64) float64 {
	return math.Log(x) / math.Log(LogBase)
}

// SquaredEuclideanDistance computes ||a - b||² between two vectors.
// Returns +Inf if dimensions do not match.
func SquaredEuclideanDistance(a, b []float64) float64 {
	if len(a) != len(b) {
		return math.Inf(1)
	}
	sum := 0.0
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
func scoreDistanceTerm(bidEmbedding []float64, sigma float64, queryEmbedding []float64) float64 {
	if len(bidEmbedding) == 0 || len(queryEmbedding) == 0 {
		return 0
	}
	dist2 := SquaredEuclideanDistance(bidEmbedding, queryEmbedding)
	if sigma == 0 {
		if dist2 == 0 {
			return 0
		}
		return math.Inf(1)
	}
	return dist2 / (sigma * sigma)
}

// ComputeEmbeddingScore returns log_B(price) - distance²/σ².
// If bidEmbedding is nil/empty, returns log_B(price) (pure price ranking).
// σ = 0 is the keyword limit: log_B(price) at the exact point, -Inf elsewhere.
func ComputeEmbeddingScore(price float64, bidEmbedding []float64, sigma float64, queryEmbedding []float64) float64 {
	term := scoreDistanceTerm(bidEmbedding, sigma, queryEmbedding)
	if math.IsInf(term, 1) {
		return math.Inf(-1)
	}
	return logB(price) - term
}
