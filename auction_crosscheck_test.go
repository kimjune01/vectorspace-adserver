package main

import (
	original "cloudx-adserver/auction"
	vendored "cloudx-adserver/enclave/auction"
	"math"
	"testing"
)

// TestAuctionCrossCheck runs identical inputs through both the original
// auction package and the vendored enclave copy, asserting bit-identical
// results. This catches drift if one copy is updated without the other.
func TestAuctionCrossCheck(t *testing.T) {
	queryEmbedding := []float64{0.01, 0.02, 0.03}

	origBids := []original.CoreBid{
		{ID: "adv-1", Bidder: "Close Ad", Price: 2.0, Currency: "USD", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5},
		{ID: "adv-2", Bidder: "Far Ad", Price: 5.0, Currency: "USD", Embedding: []float64{1.0, 1.0, 1.0}, Sigma: 0.5},
		{ID: "adv-3", Bidder: "Mid Ad", Price: 3.0, Currency: "USD", Embedding: []float64{0.5, 0.5, 0.5}, Sigma: 0.45},
	}

	vendBids := []vendored.CoreBid{
		{ID: "adv-1", Bidder: "Close Ad", Price: 2.0, Currency: "USD", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5},
		{ID: "adv-2", Bidder: "Far Ad", Price: 5.0, Currency: "USD", Embedding: []float64{1.0, 1.0, 1.0}, Sigma: 0.5},
		{ID: "adv-3", Bidder: "Mid Ad", Price: 3.0, Currency: "USD", Embedding: []float64{0.5, 0.5, 0.5}, Sigma: 0.45},
	}

	origResult := original.RunAuction(origBids, 0, queryEmbedding)
	vendResult := vendored.RunAuction(vendBids, 0, queryEmbedding)

	// Same winner
	if origResult.Winner == nil || vendResult.Winner == nil {
		t.Fatal("both auctions should produce a winner")
	}
	if origResult.Winner.ID != vendResult.Winner.ID {
		t.Errorf("winner: original=%q, vendored=%q", origResult.Winner.ID, vendResult.Winner.ID)
	}

	// Same runner-up
	if (origResult.RunnerUp == nil) != (vendResult.RunnerUp == nil) {
		t.Fatalf("runner-up mismatch: original=%v, vendored=%v", origResult.RunnerUp, vendResult.RunnerUp)
	}
	if origResult.RunnerUp != nil && origResult.RunnerUp.ID != vendResult.RunnerUp.ID {
		t.Errorf("runner-up: original=%q, vendored=%q", origResult.RunnerUp.ID, vendResult.RunnerUp.ID)
	}

	// Same VCG payment
	origPayment := original.ComputeVCGPayment(origResult, queryEmbedding)
	vendPayment := vendored.ComputeVCGPayment(vendResult, queryEmbedding)
	if math.Abs(origPayment-vendPayment) > 1e-15 {
		t.Errorf("payment: original=%.15f, vendored=%.15f", origPayment, vendPayment)
	}

	// Same eligible bid count
	if len(origResult.EligibleBids) != len(vendResult.EligibleBids) {
		t.Errorf("eligible bids: original=%d, vendored=%d", len(origResult.EligibleBids), len(vendResult.EligibleBids))
	}

	// Same embedding scores
	if len(origResult.ScoredBids) != len(vendResult.ScoredBids) {
		t.Errorf("scored bids: original=%d, vendored=%d", len(origResult.ScoredBids), len(vendResult.ScoredBids))
	} else {
		for i := range origResult.ScoredBids {
			if origResult.ScoredBids[i].ID != vendResult.ScoredBids[i].ID {
				t.Errorf("scored bid[%d] ID: original=%q, vendored=%q", i, origResult.ScoredBids[i].ID, vendResult.ScoredBids[i].ID)
			}
			if math.Abs(origResult.ScoredBids[i].Score-vendResult.ScoredBids[i].Score) > 1e-15 {
				t.Errorf("scored bid[%d] score: original=%.15f, vendored=%.15f", i, origResult.ScoredBids[i].Score, vendResult.ScoredBids[i].Score)
			}
		}
	}

	// Same embedding distance function
	for i, ob := range origBids {
		origDist := original.SquaredEuclideanDistance(ob.Embedding, queryEmbedding)
		vendDist := vendored.SquaredEuclideanDistance(vendBids[i].Embedding, queryEmbedding)
		if origDist != vendDist {
			t.Errorf("distance[%s]: original=%.15f, vendored=%.15f", ob.ID, origDist, vendDist)
		}
	}

	// Same floor enforcement
	origFloor, origRejected := original.EnforceBidFloor(origBids, 2.5)
	vendFloor, vendRejected := vendored.EnforceBidFloor(vendBids, 2.5)
	if len(origFloor) != len(vendFloor) {
		t.Errorf("floor eligible: original=%d, vendored=%d", len(origFloor), len(vendFloor))
	}
	if len(origRejected) != len(vendRejected) {
		t.Errorf("floor rejected: original=%d, vendored=%d", len(origRejected), len(vendRejected))
	}
}
