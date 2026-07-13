package platform

import (
	"vectorspace/auction"
	"errors"
	"fmt"
	"math"
	"strings"
)

// ErrNoBid signals a well-formed request that produced no winner (empty
// registry, exhausted budgets, or no bid matched the query point). Callers
// map it to a no-bid response; any other error is an operational failure.
var ErrNoBid = errors.New("no bid")

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
	positionNames := make(map[string]string, len(positions))
	for _, pos := range positions {
		positionIntents[pos.ID] = pos.Intent
		positionNames[pos.ID] = pos.Name
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
			Bidder:    pos.BudgetKey(), // stable owner identity, not display name
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

	payment := auction.ComputeVCGPayment(result, queryEmbedding, 0)

	allBidders := make([]BidderDetail, 0, len(result.ScoredBids))
	for rank, sb := range result.ScoredBids {
		distSq := auction.SquaredEuclideanDistance(sb.Embedding, queryEmbedding)
		allBidders = append(allBidders, BidderDetail{
			ID:         sb.ID,
			Rank:       rank + 1,
			Name:       positionNames[sb.ID],
			Intent:     positionIntents[sb.ID],
			BidPrice:   sb.Price,
			Sigma:      sb.Sigma,
			Score:      sb.Score,
			DistanceSq: distSq,
			LogBid:     math.Log(sb.Price) / math.Log(auction.LogBase),
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

// ORTBAuctionResult is the outcome of a live (logged, budget-filtered) auction
// run on behalf of an OpenRTB bid request.
type ORTBAuctionResult struct {
	AuctionID  int64
	Winner     *BidderDetail
	Payment    float64
	Currency   string
	BidCount   int
	ClickURL   string
	AdTitle    string
	AdSubtitle string
}

// RunORTBAuction runs a live auction for an OpenRTB bid request: budget
// filter → auction per query point → VCG → log. The query is either a
// precomputed embedding or a list of texts (e.g. the comma-separated ORTB
// keywords field, already split). With several texts, an auction runs at
// each point and the highest-scoring winner takes the impression — so an
// imported σ = 0 keyword matches when ANY request keyword is its exact
// text, which is how keyword lists behave on the platforms they come from.
//
// currencies, when non-empty, restricts eligible positions (ORTB cur field).
// test suppresses logging: a test=1 request can never become billable.
//
// This is the interop path: the query reaches the host in plaintext, as
// OpenRTB carries it. The private path is POST /ad-request, where the query
// embedding is encrypted to the enclave's attested key.
func (e *AuctionEngine) RunORTBAuction(queryTexts []string, queryEmbedding []float64, floor float64, publisherID string, currencies []string, test bool) (*ORTBAuctionResult, error) {
	var queryPoints [][]float64
	if len(queryEmbedding) > 0 {
		queryPoints = [][]float64{queryEmbedding}
	} else {
		if len(queryTexts) == 0 {
			return nil, fmt.Errorf("no query: provide keywords, content, or an embedding")
		}
		for _, text := range queryTexts {
			emb, err := e.Embedder.Embed(text)
			if err != nil {
				return nil, fmt.Errorf("embed query %q: %w", text, err)
			}
			queryPoints = append(queryPoints, emb)
		}
	}

	positions := e.Registry.GetAll()
	if len(positions) == 0 {
		return nil, ErrNoBid
	}

	// An absent cur list means USD, not "anything": prices in different
	// currencies must never compete numerically by default.
	if len(currencies) == 0 {
		currencies = []string{"USD"}
	}
	curAllowed := func(c string) bool {
		if c == "" {
			c = "USD"
		}
		for _, allowed := range currencies {
			if c == allowed {
				return true
			}
		}
		return false
	}

	bids := make([]auction.CoreBid, 0, len(positions))
	for _, pos := range positions {
		if !curAllowed(pos.Currency) {
			continue
		}
		if e.Budgets != nil && !e.Budgets.CanAfford(pos.BudgetKey(), pos.BidPrice) {
			continue
		}
		bids = append(bids, auction.CoreBid{
			ID:        pos.ID,
			Bidder:    pos.BudgetKey(), // stable owner identity, not display name
			Price:     pos.BidPrice,
			Currency:  pos.Currency,
			Embedding: pos.Embedding,
			Sigma:     pos.Sigma,
		})
	}
	if len(bids) == 0 {
		return nil, ErrNoBid
	}

	// One auction per query point; the highest-scoring winner takes it.
	var result *auction.AuctionResult
	var winningPoint []float64
	bestScore := math.Inf(-1)
	for _, qp := range queryPoints {
		r := auction.RunAuction(bids, floor, qp)
		if r.Winner == nil || len(r.ScoredBids) == 0 {
			continue
		}
		if r.ScoredBids[0].Score > bestScore {
			bestScore = r.ScoredBids[0].Score
			result = r
			winningPoint = qp
		}
	}
	if result == nil {
		return nil, ErrNoBid
	}
	payment := auction.ComputeVCGPayment(result, winningPoint, floor)

	winnerPos := e.Registry.Get(result.Winner.ID)
	detail := &BidderDetail{
		ID:       result.Winner.ID,
		Rank:     1,
		BidPrice: result.Winner.Price,
		Sigma:    result.Winner.Sigma,
		Score:    result.ScoredBids[0].Score,
		LogBid:   math.Log(result.Winner.Price) / math.Log(auction.LogBase),
	}
	out := &ORTBAuctionResult{
		Winner:   detail,
		Payment:  payment,
		Currency: result.Winner.Currency,
		BidCount: len(bids),
	}
	if winnerPos != nil {
		detail.Name = winnerPos.Name
		detail.Intent = winnerPos.Intent
		out.ClickURL = winnerPos.URL
	}
	if e.DB != nil && !test {
		if creative, err := e.DB.GetActiveCreative(result.Winner.ID); err == nil && creative != nil {
			out.AdTitle = creative.Title
			out.AdSubtitle = creative.Subtitle
		}
		intent := strings.Join(queryTexts, ", ")
		if intent == "" {
			intent = "[ortb-embedding]"
		}
		id, err := e.DB.LogAuctionReturningIDWithPublisher(intent, result.Winner.ID, payment, out.Currency, len(bids), publisherID)
		if err != nil {
			// An unlogged auction cannot be settled; that is an operational
			// failure, not a no-bid.
			return nil, fmt.Errorf("log auction: %w", err)
		}
		out.AuctionID = id
	}

	return out, nil
}
