package auction

import "math"

// LogBase is the base of the logarithm used in bid scoring.
// Higher values compress price differences, making distance matter more.
// Default 5.0 balances bid vs relevance so tau changes outcomes ~50% of the time.
var LogBase = 5.0

// logB computes log_LogBase(x) = ln(x) / ln(LogBase).
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

// ComputeEmbeddingScore returns log_B(price) - distance²/σ².
// If bidEmbedding is nil/empty or sigma is 0, returns log_B(price) (pure price ranking).
func ComputeEmbeddingScore(price float64, bidEmbedding []float64, sigma float64, queryEmbedding []float64) float64 {
	logPrice := logB(price)
	if len(bidEmbedding) == 0 || len(queryEmbedding) == 0 || sigma == 0 {
		return logPrice
	}
	dist2 := SquaredEuclideanDistance(bidEmbedding, queryEmbedding)
	return logPrice - dist2/(sigma*sigma)
}
