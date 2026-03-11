package auction

import (
	"crypto/rand"
	"math/big"
	"sort"
)

// RankByScore scores all bids against a query point and returns them
// sorted by descending score. Ties are broken by cryptographic coin flip
// to prevent deterministic ordering exploits.
func RankByScore(bids []CoreBid, queryEmbedding []float64) []ScoredBid {
	scored := make([]ScoredBid, len(bids))
	for i, bid := range bids {
		scored[i] = ScoredBid{
			CoreBid: bid,
			Score:   ComputeScore(bid, queryEmbedding),
		}
	}

	// Sort descending by score
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// Break ties with Fisher-Yates using crypto/rand
	breakTies(scored)

	return scored
}

// breakTies shuffles adjacent entries that share the same score
// using cryptographic randomness.
func breakTies(scored []ScoredBid) {
	i := 0
	for i < len(scored) {
		j := i + 1
		for j < len(scored) && scored[j].Score == scored[i].Score {
			j++
		}
		// [i, j) is a run of equal scores
		if j-i > 1 {
			shuffleRange(scored, i, j)
		}
		i = j
	}
}

// shuffleRange performs Fisher-Yates shuffle on scored[lo:hi]
// using crypto/rand for unbiased randomness.
func shuffleRange(scored []ScoredBid, lo, hi int) {
	for k := hi - 1; k > lo; k-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(k-lo+1)))
		if err != nil {
			continue
		}
		swap := lo + int(n.Int64())
		scored[k], scored[swap] = scored[swap], scored[k]
	}
}
