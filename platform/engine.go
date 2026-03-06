package platform

import (
	"cloudx-adserver/auction"
	"fmt"
	"math"
	"net/url"
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
}

// RichAdResponse is the enriched response returned to publishers.
type RichAdResponse struct {
	AuctionID     int64          `json:"auction_id"`
	Intent        string         `json:"intent"`
	Winner        *BidderDetail  `json:"winner"`
	RunnerUp      *BidderDetail  `json:"runner_up,omitempty"`
	AllBidders    []BidderDetail `json:"all_bidders"`
	Payment       float64        `json:"payment"`
	Currency      string         `json:"currency"`
	BidCount      int            `json:"bid_count"`
	EligibleCount int            `json:"eligible_count"`
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

// RunAdRequest processes a publisher's ad request and returns a rich response
// with all bidder scores.
func (e *AuctionEngine) RunAdRequest(intent string) (*RichAdResponse, error) {
	return e.RunAdRequestWithTau(intent, 0)
}

// RunAdRequestWithTau processes a publisher's ad request with an optional relevance
// threshold tau. Only ads whose squared distance to the query falls below tau are
// eligible. If tau <= 0, all ads pass (no filtering).
func (e *AuctionEngine) RunAdRequestWithTau(intent string, tau float64) (*RichAdResponse, error) {
	return e.RunAdRequestFull(intent, tau, "")
}

// RunAdRequestFull processes an ad request with optional tau and publisher_id.
func (e *AuctionEngine) RunAdRequestFull(intent string, tau float64, publisherID string) (*RichAdResponse, error) {
	queryEmbedding, err := e.Embedder.Embed(intent)
	if err != nil {
		return nil, fmt.Errorf("embed query intent: %w", err)
	}

	positions := e.Registry.GetAll()
	if len(positions) == 0 {
		return nil, fmt.Errorf("no registered advertisers")
	}

	// Build index of position intents and URLs for the response
	positionIntents := make(map[string]string, len(positions))
	positionURLs := make(map[string]string)
	for _, pos := range positions {
		positionIntents[pos.ID] = pos.Intent
		if pos.URL != "" {
			positionURLs[pos.ID] = pos.URL
		}
	}

	// Build bids from positions that can afford their bid price
	var bids []auction.CoreBid
	for _, pos := range positions {
		if !e.Budgets.CanAfford(pos.ID, pos.BidPrice) {
			continue
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

	if len(bids) == 0 {
		return nil, fmt.Errorf("no eligible bidders (all out of budget)")
	}

	// Phase 1: Apply publisher's relevance gate (τ)
	// Only ads whose squared distance to the query falls below τ pass through.
	if tau > 0 {
		var filtered []auction.CoreBid
		for _, bid := range bids {
			distSq := auction.SquaredEuclideanDistance(bid.Embedding, queryEmbedding)
			if distSq <= tau {
				filtered = append(filtered, bid)
			}
		}
		bids = filtered
		if len(bids) == 0 {
			return nil, fmt.Errorf("no bidders passed relevance threshold (tau=%.4f)", tau)
		}
	}

	bidCount := len(bids)

	// Phase 2: Rank by log(b) among ads that passed the relevance gate
	result := auction.RunAuction(bids, 0, queryEmbedding)
	if result.Winner == nil {
		return nil, fmt.Errorf("auction produced no winner")
	}

	// Compute VCG payment (charged on click, not here)
	payment := auction.ComputeVCGPayment(result, queryEmbedding)

	winnerID := result.Winner.ID

	// Log auction to DB
	var auctionID int64
	if e.DB != nil {
		var err error
		if publisherID != "" {
			auctionID, err = e.DB.LogAuctionReturningIDWithPublisher(intent, winnerID, payment, result.Winner.Currency, bidCount, publisherID)
		} else {
			auctionID, err = e.DB.LogAuctionReturningID(intent, winnerID, payment, result.Winner.Currency, bidCount)
		}
		if err != nil {
			// Fallback to non-returning version
			e.DB.LogAuction(intent, winnerID, payment, result.Winner.Currency, bidCount)
		}
	}

	// Build bidder details from scored bids
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
			LogBid:     math.Log(sb.Price) / math.Log(auction.LogBase),
		})
	}

	var winnerDetail *BidderDetail
	if len(allBidders) > 0 {
		winnerDetail = &allBidders[0]
		if baseURL, ok := positionURLs[winnerDetail.ID]; ok {
			winnerDetail.ClickURL = buildClickURL(baseURL, publisherID, winnerDetail.ID, auctionID, intent)
		}
	}
	var runnerUpDetail *BidderDetail
	if len(allBidders) > 1 {
		runnerUpDetail = &allBidders[1]
	}

	return &RichAdResponse{
		AuctionID:     auctionID,
		Intent:        intent,
		Winner:        winnerDetail,
		RunnerUp:      runnerUpDetail,
		AllBidders:    allBidders,
		Payment:       payment,
		Currency:      result.Winner.Currency,
		BidCount:      bidCount,
		EligibleCount: len(result.EligibleBids),
	}, nil
}

// buildClickURL appends UTM params to an advertiser's base URL for click tracking.
func buildClickURL(baseURL, publisherID, advertiserID string, auctionID int64, intent string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	q := u.Query()
	q.Set("utm_source", publisherID)
	q.Set("utm_medium", "cpc")
	q.Set("utm_campaign", advertiserID)
	q.Set("utm_content", fmt.Sprintf("%d", auctionID))
	q.Set("utm_term", intent)
	u.RawQuery = q.Encode()
	return u.String()
}
