package auction

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sort"
)

// RandSource provides random number generation for tie-breaking.
type RandSource interface {
	Intn(n int) int
}

type cryptoRandSource struct{}

func (cryptoRandSource) Intn(n int) int {
	if n <= 0 {
		panic(fmt.Sprintf("cryptoRandSource.Intn: n must be positive, got %d", n))
	}
	nBig, _ := rand.Int(rand.Reader, big.NewInt(int64(n)))
	return int(nBig.Int64())
}

var defaultRandSource RandSource = cryptoRandSource{}

// RankScoredBids ranks bids by Score (descending).
// Per-bidder highest is chosen by Score. Tie-breaking uses random shuffle.
func RankScoredBids(bids []ScoredBid, randSource RandSource) *CoreRankingResult {
	if len(bids) == 0 {
		return &CoreRankingResult{
			Ranks:         make(map[string]int),
			HighestBids:   make(map[string]*CoreBid),
			SortedBidders: make([]string, 0),
		}
	}

	type ScoredEntry struct {
		bidder string
		bid    *CoreBid
		score  float64
	}

	bidderMap := make(map[string]*ScoredEntry)
	bidderOrder := make([]string, 0, len(bids))
	seenBidders := make(map[string]bool)

	for i := range bids {
		sb := &bids[i]
		if !seenBidders[sb.Bidder] {
			bidderOrder = append(bidderOrder, sb.Bidder)
			seenBidders[sb.Bidder] = true
		}
		existing, exists := bidderMap[sb.Bidder]
		if !exists || sb.Score > existing.score {
			bid := sb.CoreBid
			bidderMap[sb.Bidder] = &ScoredEntry{bidder: sb.Bidder, bid: &bid, score: sb.Score}
		}
	}

	entries := make([]ScoredEntry, 0, len(bidderOrder))
	for _, bidder := range bidderOrder {
		entries = append(entries, *bidderMap[bidder])
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].score > entries[j].score
	})

	if randSource == nil {
		randSource = defaultRandSource
	}

	// Break ties randomly (Fisher-Yates shuffle on equal-score groups)
	i := 0
	for i < len(entries) {
		score := entries[i].score
		j := i + 1
		for j < len(entries) && entries[j].score == score {
			j++
		}
		if j-i > 1 {
			for k := j - 1; k > i; k-- {
				randIdx := i + randSource.Intn(k-i+1)
				entries[k], entries[randIdx] = entries[randIdx], entries[k]
			}
		}
		i = j
	}

	result := &CoreRankingResult{
		Ranks:         make(map[string]int, len(entries)),
		HighestBids:   make(map[string]*CoreBid, len(entries)),
		SortedBidders: make([]string, len(entries)),
	}

	for rank, entry := range entries {
		result.Ranks[entry.bidder] = rank + 1
		result.HighestBids[entry.bidder] = entry.bid
		result.SortedBidders[rank] = entry.bidder
	}

	return result
}

// RankCoreBids ranks bids by Price (descending).
// Per-bidder highest is chosen by Price. Tie-breaking uses random shuffle.
func RankCoreBids(bids []CoreBid, randSource RandSource) *CoreRankingResult {
	if len(bids) == 0 {
		return &CoreRankingResult{
			Ranks:         make(map[string]int),
			HighestBids:   make(map[string]*CoreBid),
			SortedBidders: make([]string, 0),
		}
	}

	type BidEntry struct {
		bidder string
		bid    *CoreBid
	}

	bidderMap := make(map[string]*CoreBid)
	bidderOrder := make([]string, 0, len(bids))
	seenBidders := make(map[string]bool)

	for i := range bids {
		bid := &bids[i]
		if !seenBidders[bid.Bidder] {
			bidderOrder = append(bidderOrder, bid.Bidder)
			seenBidders[bid.Bidder] = true
		}
		existing, exists := bidderMap[bid.Bidder]
		if !exists || bid.Price > existing.Price {
			bidderMap[bid.Bidder] = bid
		}
	}

	entries := make([]BidEntry, 0, len(bidderOrder))
	for _, bidder := range bidderOrder {
		entries = append(entries, BidEntry{bidder: bidder, bid: bidderMap[bidder]})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].bid.Price > entries[j].bid.Price
	})

	if randSource == nil {
		randSource = defaultRandSource
	}

	i := 0
	for i < len(entries) {
		price := entries[i].bid.Price
		j := i + 1
		for j < len(entries) && entries[j].bid.Price == price {
			j++
		}
		if j-i > 1 {
			for k := j - 1; k > i; k-- {
				randIdx := i + randSource.Intn(k-i+1)
				entries[k], entries[randIdx] = entries[randIdx], entries[k]
			}
		}
		i = j
	}

	result := &CoreRankingResult{
		Ranks:         make(map[string]int, len(entries)),
		HighestBids:   make(map[string]*CoreBid, len(entries)),
		SortedBidders: make([]string, len(entries)),
	}

	for rank, entry := range entries {
		result.Ranks[entry.bidder] = rank + 1
		result.HighestBids[entry.bidder] = entry.bid
		result.SortedBidders[rank] = entry.bidder
	}

	return result
}
