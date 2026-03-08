package platform

import (
	"vectorspace/auction"
	"fmt"
	"math"
)

// BidderDetail contains scoring details for a single bidder in the auction.
type BidderDetail struct {
	ID         string  `json:"id"`
	Rank       int     `json:"rank"`
	Name       string  `json:"name"`
	Intent     string  `json:"intent"`
	BidPrice   float64 `json:"bid_price"`
	Sigma      float64 `json:"sigma"`
	Score      float64 `json:"score"`
	DistanceSq float64 `json:"distance_sq"`
	LogBid     float64 `json:"log_bid"`
	ClickURL   string  `json:"click_url,omitempty"`
	AdTitle    string  `json:"ad_title,omitempty"`
	AdSubtitle string  `json:"ad_subtitle,omitempty"`
}

// AuctionEngine orchestrates the full ad-request flow:
// registry → budget filter → auction → VCG payment → log (charge happens on click)
type AuctionEngine struct {
	Registry *PositionRegistry
	Budgets  *BudgetTracker
	Embedder *Embedder
	DB       *DB
}

func NewAuctionEngine(registry *PositionRegistry, budgets *BudgetTracker, embedder *Embedder) *AuctionEngine {
	return &AuctionEngine{
		Registry: registry,
		Budgets:  budgets,
		Embedder: embedder,
	}
}

// TauBucket represents how many advertisers pass at a given tau threshold.
type TauBucket struct {
	Tau   float64 `json:"tau"`
	Count int     `json:"count"`
}

// SimulationResult is the response for a simulated auction (no logging, no billing).
type SimulationResult struct {
	Intent        string         `json:"intent"`
	Winner        *BidderDetail  `json:"winner"`
	AllBidders    []BidderDetail `json:"all_bidders"`
	Payment       float64        `json:"payment"`
	BidCount      int            `json:"bid_count"`
	TauThresholds []TauBucket    `json:"tau_thresholds"`
}

// SimulateAuction runs a simulated auction for the given intent.
// Includes ALL advertisers regardless of budget and does not log to
// the database. Used for the /explore debug tool.
// If tau > 0, only advertisers whose squared Euclidean distance to the
// query embedding is <= tau are included.
func (e *AuctionEngine) SimulateAuction(intent string, tau float64) (*SimulationResult, error) {
	queryEmbedding, err := e.Embedder.Embed(intent)
	if err != nil {
		return nil, fmt.Errorf("embed query intent: %w", err)
	}

	positions := e.Registry.GetAll()
	if len(positions) == 0 {
		return nil, fmt.Errorf("no registered advertisers")
	}

	positionIntents := make(map[string]string, len(positions))
	for _, pos := range positions {
		positionIntents[pos.ID] = pos.Intent
	}

	// Build bids, optionally filtering by tau (distance threshold)
	bids := make([]auction.CoreBid, 0, len(positions))
	for _, pos := range positions {
		if tau > 0 {
			distSq := auction.SquaredEuclideanDistance(pos.Embedding, queryEmbedding)
			if distSq > tau {
				continue
			}
		}
		bids = append(bids, auction.CoreBid{
			ID:        pos.ID,
			Bidder:    pos.Name,
			Price:     pos.BidPrice,
			Currency:  pos.Currency,
			Embedding: pos.Embedding,
			Sigma:     pos.Sigma,
		})
	}

	result := auction.RunAuction(bids, 0, queryEmbedding)
	if result.Winner == nil {
		return nil, fmt.Errorf("auction produced no winner")
	}

	payment := auction.ComputeVCGPayment(result, queryEmbedding)

	allBidders := make([]BidderDetail, 0, len(result.ScoredBids))
	for rank, sb := range result.ScoredBids {
		distSq := auction.SquaredEuclideanDistance(sb.Embedding, queryEmbedding)
		allBidders = append(allBidders, BidderDetail{
			ID:         sb.ID,
			Rank:       rank + 1,
			Name:       sb.Bidder,
			Intent:     positionIntents[sb.ID],
			BidPrice:   sb.Price,
			Sigma:      sb.Sigma,
			Score:      sb.Score,
			DistanceSq: distSq,
			LogBid:     math.Log(sb.Price) / math.Log(auction.DefaultLogBase),
		})
	}

	// Compute tau threshold buckets
	tauValues := []float64{0.1, 0.25, 0.5, 1.0, 2.0, 5.0}
	tauThresholds := make([]TauBucket, len(tauValues))
	for i, tau := range tauValues {
		count := 0
		for _, b := range allBidders {
			if b.DistanceSq <= tau {
				count++
			}
		}
		tauThresholds[i] = TauBucket{Tau: tau, Count: count}
	}

	var winner *BidderDetail
	if len(allBidders) > 0 {
		winner = &allBidders[0]
		if e.DB != nil {
			if creative, err := e.DB.GetActiveCreative(winner.ID); err == nil && creative != nil {
				winner.AdTitle = creative.Title
				winner.AdSubtitle = creative.Subtitle
			}
		}
	}

	return &SimulationResult{
		Intent:        intent,
		Winner:        winner,
		AllBidders:    allBidders,
		Payment:       payment,
		BidCount:      len(bids),
		TauThresholds: tauThresholds,
	}, nil
}
