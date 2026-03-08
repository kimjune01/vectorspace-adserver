package auction

import "math"

// DefaultLogBase is the default base of the logarithm used in bid scoring.
// Higher values compress price differences, making distance matter more.
// Default 5.0 balances bid vs relevance so tau changes outcomes ~50% of the time.
// Configurable per-publisher: range b=5 (balanced) to b=50 (quality absolutist).
const DefaultLogBase = 5.0

// LogBase is kept for backwards compatibility. Use DefaultLogBase for new code.
var LogBase = DefaultLogBase

// logBWithBase computes log_base(x) = ln(x) / ln(base).
func logBWithBase(x, base float64) float64 {
	return math.Log(x) / math.Log(base)
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
// Uses the global LogBase. For per-publisher log base, use ComputeEmbeddingScoreWithBase.
func ComputeEmbeddingScore(price float64, bidEmbedding []float64, sigma float64, queryEmbedding []float64) float64 {
	return ComputeEmbeddingScoreWithBase(price, bidEmbedding, sigma, queryEmbedding, LogBase)
}

// ComputeEmbeddingScoreWithBase returns log_base(price) - distance²/σ² using the given log base.
func ComputeEmbeddingScoreWithBase(price float64, bidEmbedding []float64, sigma float64, queryEmbedding []float64, logBase float64) float64 {
	logPrice := logBWithBase(price, logBase)
	if len(bidEmbedding) == 0 || len(queryEmbedding) == 0 || sigma == 0 {
		return logPrice
	}
	dist2 := SquaredEuclideanDistance(bidEmbedding, queryEmbedding)
	return logPrice - dist2/(sigma*sigma)
}
