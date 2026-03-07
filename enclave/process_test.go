package enclave

import (
	"cloudx-adserver/enclave/auction"
	"encoding/json"
	"math"
	"testing"
)

// makeTestPositions returns a set of positions with known embeddings for testing.
func makeTestPositions() ([]PositionSnapshot, []BudgetSnapshot) {
	positions := []PositionSnapshot{
		{ID: "adv-1", Name: "Close Ad", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
		{ID: "adv-2", Name: "Far Ad", Embedding: []float64{1.0, 1.0, 1.0}, Sigma: 0.5, BidPrice: 5.0, Currency: "USD"},
		{ID: "adv-3", Name: "Mid Ad", Embedding: []float64{0.5, 0.5, 0.5}, Sigma: 0.45, BidPrice: 3.0, Currency: "USD"},
	}
	budgets := []BudgetSnapshot{
		{AdvertiserID: "adv-1", Total: 1000, Spent: 0, Currency: "USD"},
		{AdvertiserID: "adv-2", Total: 1000, Spent: 0, Currency: "USD"},
		{AdvertiserID: "adv-3", Total: 1000, Spent: 0, Currency: "USD"},
	}
	return positions, budgets
}

// TestSameResultAsDirectAuction verifies that ProcessPrivateAuction produces
// bit-identical winner + payment as calling auction.RunAuction + ComputeVCGPayment directly.
func TestSameResultAsDirectAuction(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	positions, budgetSnapshots := makeTestPositions()
	posStore := NewPositionStore()
	posStore.ReplaceAll(positions)
	budgetStore := NewBudgetStore()
	budgetStore.ReplaceAll(budgetSnapshots)

	queryEmbedding := []float64{0.01, 0.02, 0.03}

	// --- Direct auction path (what the server does) ---
	var bids []auction.CoreBid
	for _, pos := range positions {
		bids = append(bids, auction.CoreBid{
			ID:        pos.ID,
			Bidder:    pos.Name,
			Price:     pos.BidPrice,
			Currency:  pos.Currency,
			Embedding: pos.Embedding,
			Sigma:     pos.Sigma,
		})
	}
	directResult := auction.RunAuction(bids, 0, queryEmbedding)
	directPayment := auction.ComputeVCGPayment(directResult, queryEmbedding)

	// --- Enclave path ---
	embJSON, _ := json.Marshal(queryEmbedding)
	aesKeyEnc, payloadEnc, nonce, err := EncryptHybrid(embJSON, &km.PrivateKey().PublicKey, HashAlgorithmSHA256)
	if err != nil {
		t.Fatalf("EncryptHybrid: %v", err)
	}

	enclaveResp, err := ProcessPrivateAuction(
		&AuctionRequest{
			EncryptedEmbedding: EncryptedEmbedding{
				AESKeyEncrypted:  aesKeyEnc,
				EncryptedPayload: payloadEnc,
				Nonce:            nonce,
				HashAlgorithm:    "SHA-256",
			},
		},
		km.PrivateKey(),
		posStore,
		budgetStore,
	)
	if err != nil {
		t.Fatalf("ProcessPrivateAuction: %v", err)
	}

	// Compare: same winner
	if enclaveResp.WinnerID != directResult.Winner.ID {
		t.Errorf("winner: enclave=%q, direct=%q", enclaveResp.WinnerID, directResult.Winner.ID)
	}

	// Compare: same payment (bit-identical)
	if math.Abs(enclaveResp.Payment-directPayment) > 1e-10 {
		t.Errorf("payment: enclave=%.10f, direct=%.10f", enclaveResp.Payment, directPayment)
	}

	// Compare: same bid count
	if enclaveResp.BidCount != len(bids) {
		t.Errorf("bid_count: enclave=%d, direct=%d", enclaveResp.BidCount, len(bids))
	}
}

func TestProcessPrivateAuctionWithTau(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	positions, budgetSnapshots := makeTestPositions()
	posStore := NewPositionStore()
	posStore.ReplaceAll(positions)
	budgetStore := NewBudgetStore()
	budgetStore.ReplaceAll(budgetSnapshots)

	queryEmbedding := []float64{0.01, 0.02, 0.03}
	embJSON, _ := json.Marshal(queryEmbedding)
	aesKeyEnc, payloadEnc, nonce, _ := EncryptHybrid(embJSON, &km.PrivateKey().PublicKey, HashAlgorithmSHA256)

	// tau=0.5: only "Close Ad" (dist²=0) passes; "Mid Ad" (dist²≈0.72) and "Far Ad" (dist²≈2.88) don't
	resp, err := ProcessPrivateAuction(
		&AuctionRequest{
			EncryptedEmbedding: EncryptedEmbedding{
				AESKeyEncrypted:  aesKeyEnc,
				EncryptedPayload: payloadEnc,
				Nonce:            nonce,
				HashAlgorithm:    "SHA-256",
			},
			Tau: 0.5,
		},
		km.PrivateKey(),
		posStore,
		budgetStore,
	)
	if err != nil {
		t.Fatalf("ProcessPrivateAuction: %v", err)
	}

	if resp.WinnerID != "adv-1" {
		t.Errorf("winner = %q, want %q (only close ad should pass tau)", resp.WinnerID, "adv-1")
	}
	if resp.BidCount != 1 {
		t.Errorf("bid_count = %d, want 1", resp.BidCount)
	}
}

func TestProcessPrivateAuctionBudgetFilter(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	positions, _ := makeTestPositions()
	posStore := NewPositionStore()
	posStore.ReplaceAll(positions)

	// adv-2 (Far Ad, bid=5.0) is out of budget
	budgetStore := NewBudgetStore()
	budgetStore.ReplaceAll([]BudgetSnapshot{
		{AdvertiserID: "adv-1", Total: 1000, Spent: 0, Currency: "USD"},
		{AdvertiserID: "adv-2", Total: 4, Spent: 0, Currency: "USD"}, // can't afford 5.0
		{AdvertiserID: "adv-3", Total: 1000, Spent: 0, Currency: "USD"},
	})

	queryEmbedding := []float64{0.01, 0.02, 0.03}
	embJSON, _ := json.Marshal(queryEmbedding)
	aesKeyEnc, payloadEnc, nonce, _ := EncryptHybrid(embJSON, &km.PrivateKey().PublicKey, HashAlgorithmSHA256)

	resp, err := ProcessPrivateAuction(
		&AuctionRequest{
			EncryptedEmbedding: EncryptedEmbedding{
				AESKeyEncrypted:  aesKeyEnc,
				EncryptedPayload: payloadEnc,
				Nonce:            nonce,
				HashAlgorithm:    "SHA-256",
			},
		},
		km.PrivateKey(),
		posStore,
		budgetStore,
	)
	if err != nil {
		t.Fatalf("ProcessPrivateAuction: %v", err)
	}

	if resp.BidCount != 2 {
		t.Errorf("bid_count = %d, want 2 (adv-2 should be filtered by budget)", resp.BidCount)
	}
	if resp.WinnerID == "adv-2" {
		t.Error("adv-2 should not win (out of budget)")
	}
}

func TestProcessPrivateAuctionNoPositions(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	posStore := NewPositionStore()
	budgetStore := NewBudgetStore()

	queryEmbedding := []float64{0.01, 0.02, 0.03}
	embJSON, _ := json.Marshal(queryEmbedding)
	aesKeyEnc, payloadEnc, nonce, _ := EncryptHybrid(embJSON, &km.PrivateKey().PublicKey, HashAlgorithmSHA256)

	_, err = ProcessPrivateAuction(
		&AuctionRequest{
			EncryptedEmbedding: EncryptedEmbedding{
				AESKeyEncrypted:  aesKeyEnc,
				EncryptedPayload: payloadEnc,
				Nonce:            nonce,
				HashAlgorithm:    "SHA-256",
			},
		},
		km.PrivateKey(),
		posStore,
		budgetStore,
	)
	if err == nil {
		t.Fatal("expected error with no positions")
	}
}
