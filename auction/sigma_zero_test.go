package auction

import (
	"math"
	"testing"
)

// σ = 0 is the keyword limit: full score at the exact point, -Inf elsewhere.
// See: june.kim/keywords-are-tiny-circles

func TestSigmaZeroScoresAtExactPoint(t *testing.T) {
	point := []float64{0.1, 0.2, 0.3}
	bid := CoreBid{ID: "kw", Bidder: "kw-adv", Price: 2.0, Embedding: point, Sigma: 0}

	got := ComputeScore(bid, point)
	want := math.Log(2.0) / math.Log(LogBase)
	if math.Abs(got-want) > 1e-12 {
		t.Errorf("exact point: got %v, want %v", got, want)
	}
}

func TestSigmaZeroLosesOffPoint(t *testing.T) {
	bid := CoreBid{ID: "kw", Bidder: "kw-adv", Price: 100.0, Embedding: []float64{0.1, 0.2, 0.3}, Sigma: 0}
	query := []float64{0.1, 0.2, 0.3000001}

	if got := ComputeScore(bid, query); !math.IsInf(got, -1) {
		t.Errorf("off point: got %v, want -Inf", got)
	}
}

func TestKeywordBidBeatsRicherVectorBidAtItsPoint(t *testing.T) {
	point := []float64{0.1, 0.2, 0.3}
	keyword := CoreBid{ID: "kw", Bidder: "kw-adv", Price: 1.0, Embedding: point, Sigma: 0}
	// Higher price but centered elsewhere: distance penalty must dominate.
	vector := CoreBid{ID: "vec", Bidder: "vec-adv", Price: 50.0, Embedding: []float64{0.9, 0.9, 0.9}, Sigma: 0.3}

	result := RunAuction([]CoreBid{keyword, vector}, 0, point)
	if result.Winner == nil || result.Winner.ID != "kw" {
		t.Fatalf("winner = %+v, want keyword bid", result.Winner)
	}
	if result.RunnerUp == nil || result.RunnerUp.ID != "vec" {
		t.Fatalf("runner-up = %+v, want vector bid", result.RunnerUp)
	}

	// Payment is the score-rule limit: ru.Price * B^(0 - distR²/σR²) ≤ ru.Price,
	// and strictly positive.
	payment := ComputeVCGPayment(result, point, 0)
	if payment <= 0 || payment > vector.Price {
		t.Errorf("payment = %v, want in (0, %v]", payment, vector.Price)
	}
}

func TestSigmaZeroOffPointCannotWinDefaultsToNoWinner(t *testing.T) {
	keyword := CoreBid{ID: "kw", Bidder: "kw-adv", Price: 100.0, Embedding: []float64{0.1, 0.2, 0.3}, Sigma: 0}
	query := []float64{0.5, 0.5, 0.5}

	result := RunAuction([]CoreBid{keyword}, 0, query)
	if result.Winner != nil {
		t.Errorf("winner = %+v, want nil (keyword away from its point)", result.Winner)
	}
}

func TestRunnerUpSkipsWinnersOwnKeywords(t *testing.T) {
	point := []float64{0.1, 0.2, 0.3}
	// Same advertiser holds two keyword positions; one matches exactly.
	kw1 := CoreBid{ID: "kw1", Bidder: "group-adv", Price: 2.0, Embedding: point, Sigma: 0.4}
	kw2 := CoreBid{ID: "kw2", Bidder: "group-adv", Price: 2.0, Embedding: []float64{0.11, 0.21, 0.31}, Sigma: 0.4}
	rival := CoreBid{ID: "rival", Bidder: "rival-adv", Price: 1.0, Embedding: []float64{0.2, 0.3, 0.4}, Sigma: 0.4}

	result := RunAuction([]CoreBid{kw1, kw2, rival}, 0, point)
	if result.Winner == nil || result.Winner.Bidder != "group-adv" {
		t.Fatalf("winner = %+v, want group-adv", result.Winner)
	}
	if result.RunnerUp == nil || result.RunnerUp.ID != "rival" {
		t.Fatalf("runner-up = %+v, want rival (not the winner's other keyword)", result.RunnerUp)
	}
}

func TestVCGPaymentBothExactMatch(t *testing.T) {
	point := []float64{0.1, 0.2, 0.3}
	a := CoreBid{ID: "a", Bidder: "a-adv", Price: 5.0, Embedding: point, Sigma: 0}
	b := CoreBid{ID: "b", Bidder: "b-adv", Price: 2.0, Embedding: point, Sigma: 0}

	result := RunAuction([]CoreBid{a, b}, 0, point)
	if result.Winner == nil || result.Winner.ID != "a" {
		t.Fatalf("winner = %+v, want a (higher price at same point)", result.Winner)
	}
	payment := ComputeVCGPayment(result, point, 0)
	if math.Abs(payment-2.0) > 1e-12 {
		t.Errorf("payment = %v, want 2.0 (pure second price when both exact)", payment)
	}
}
