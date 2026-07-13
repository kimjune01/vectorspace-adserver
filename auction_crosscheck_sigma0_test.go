package main

import (
	"math"
	"testing"

	original "vectorspace/auction"
	vendored "vectorspace/enclave/auction"
)

// TestAuctionCrossCheckSigmaZero runs σ = 0 (keyword-limit) bids through both
// the original auction package and the vendored enclave copy, asserting
// identical winners, runner-ups, and payments. The two copies once disagreed
// on this exact case (original: -Inf everywhere; vendored: match everywhere).
func TestAuctionCrossCheckSigmaZero(t *testing.T) {
	point := []float64{0.1, 0.2, 0.3}

	cases := []struct {
		name  string
		query []float64
	}{
		{"query at keyword point", point},
		{"query off keyword point", []float64{0.5, 0.5, 0.5}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			origBids := []original.CoreBid{
				{ID: "kw", Bidder: "kw-adv", Price: 2.0, Currency: "USD", Embedding: point, Sigma: 0},
				{ID: "vec", Bidder: "vec-adv", Price: 3.0, Currency: "USD", Embedding: []float64{0.4, 0.4, 0.4}, Sigma: 0.5},
			}
			vendBids := []vendored.CoreBid{
				{ID: "kw", Bidder: "kw-adv", Price: 2.0, Currency: "USD", Embedding: point, Sigma: 0},
				{ID: "vec", Bidder: "vec-adv", Price: 3.0, Currency: "USD", Embedding: []float64{0.4, 0.4, 0.4}, Sigma: 0.5},
			}

			origResult := original.RunAuction(origBids, 0, tc.query)
			vendResult := vendored.RunAuction(vendBids, 0, tc.query)

			if (origResult.Winner == nil) != (vendResult.Winner == nil) {
				t.Fatalf("winner presence: original=%v, vendored=%v", origResult.Winner, vendResult.Winner)
			}
			if origResult.Winner != nil && origResult.Winner.ID != vendResult.Winner.ID {
				t.Errorf("winner: original=%q, vendored=%q", origResult.Winner.ID, vendResult.Winner.ID)
			}
			if (origResult.RunnerUp == nil) != (vendResult.RunnerUp == nil) {
				t.Fatalf("runner-up presence: original=%v, vendored=%v", origResult.RunnerUp, vendResult.RunnerUp)
			}
			if origResult.RunnerUp != nil && origResult.RunnerUp.ID != vendResult.RunnerUp.ID {
				t.Errorf("runner-up: original=%q, vendored=%q", origResult.RunnerUp.ID, vendResult.RunnerUp.ID)
			}

			origPayment := original.ComputeVCGPayment(origResult, tc.query, 0)
			vendPayment := vendored.ComputeVCGPayment(vendResult, tc.query, 0)
			if math.Abs(origPayment-vendPayment) > 1e-15 {
				t.Errorf("payment: original=%.15f, vendored=%.15f", origPayment, vendPayment)
			}
		})
	}
}

// TestAuctionCrossCheckKeywordGroup runs a multi-position bidder (a keyword
// group) through both copies: the group must compete as one bidder, with
// identical winner, runner-up, and payment.
func TestAuctionCrossCheckKeywordGroup(t *testing.T) {
	point := []float64{0.1, 0.2, 0.3}
	near := []float64{0.11, 0.21, 0.31}
	far := []float64{0.6, 0.6, 0.6}

	origBids := []original.CoreBid{
		{ID: "kw1", Bidder: "group", Price: 2.0, Currency: "USD", Embedding: point, Sigma: 0},
		{ID: "kw2", Bidder: "group", Price: 2.0, Currency: "USD", Embedding: near, Sigma: 0.4},
		{ID: "rival", Bidder: "rival", Price: 1.5, Currency: "USD", Embedding: far, Sigma: 0.8},
	}
	vendBids := []vendored.CoreBid{
		{ID: "kw1", Bidder: "group", Price: 2.0, Currency: "USD", Embedding: point, Sigma: 0},
		{ID: "kw2", Bidder: "group", Price: 2.0, Currency: "USD", Embedding: near, Sigma: 0.4},
		{ID: "rival", Bidder: "rival", Price: 1.5, Currency: "USD", Embedding: far, Sigma: 0.8},
	}

	origResult := original.RunAuction(origBids, 0, point)
	vendResult := vendored.RunAuction(vendBids, 0, point)

	if origResult.Winner == nil || vendResult.Winner == nil {
		t.Fatal("both auctions should produce a winner")
	}
	if origResult.Winner.ID != vendResult.Winner.ID {
		t.Errorf("winner: original=%q, vendored=%q", origResult.Winner.ID, vendResult.Winner.ID)
	}
	if origResult.Winner.ID != "kw1" {
		t.Errorf("winner = %q, want kw1 (exact match beats near sibling and far rival)", origResult.Winner.ID)
	}
	if origResult.RunnerUp == nil || vendResult.RunnerUp == nil {
		t.Fatal("both auctions should produce a runner-up")
	}
	if origResult.RunnerUp.ID != "rival" || vendResult.RunnerUp.ID != "rival" {
		t.Errorf("runner-up: original=%q, vendored=%q, want rival in both (never the winner's own keyword)",
			origResult.RunnerUp.ID, vendResult.RunnerUp.ID)
	}
	if len(origResult.ScoredBids) != len(vendResult.ScoredBids) {
		t.Errorf("scored bids: original=%d, vendored=%d (per-bidder collapse must match)",
			len(origResult.ScoredBids), len(vendResult.ScoredBids))
	}

	origPayment := original.ComputeVCGPayment(origResult, point, 0)
	vendPayment := vendored.ComputeVCGPayment(vendResult, point, 0)
	if math.Abs(origPayment-vendPayment) > 1e-15 {
		t.Errorf("payment: original=%.15f, vendored=%.15f", origPayment, vendPayment)
	}
}
