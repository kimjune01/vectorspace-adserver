package tee

import (
	"cloudx-adserver/enclave"
	"encoding/json"
	"math"
	"testing"
)

func TestMockTEEProxyFullCycle(t *testing.T) {
	proxy, err := NewMockTEEProxy()
	if err != nil {
		t.Fatalf("NewMockTEEProxy: %v", err)
	}

	// 1. Sync positions + budgets
	positions := []enclave.PositionSnapshot{
		{ID: "adv-1", Name: "Close Ad", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
		{ID: "adv-2", Name: "Far Ad", Embedding: []float64{1.0, 1.0, 1.0}, Sigma: 0.5, BidPrice: 5.0, Currency: "USD"},
	}
	budgets := []enclave.BudgetSnapshot{
		{AdvertiserID: "adv-1", Total: 1000, Spent: 0, Currency: "USD"},
		{AdvertiserID: "adv-2", Total: 1000, Spent: 0, Currency: "USD"},
	}

	if err := proxy.SyncPositions(positions); err != nil {
		t.Fatalf("SyncPositions: %v", err)
	}
	if err := proxy.SyncBudgets(budgets); err != nil {
		t.Fatalf("SyncBudgets: %v", err)
	}

	// 2. Get attestation
	attest, err := proxy.GetAttestation()
	if err != nil {
		t.Fatalf("GetAttestation: %v", err)
	}
	if attest.PublicKey == "" {
		t.Error("expected non-empty public key")
	}

	// 3. Encrypt embedding (simulating SDK)
	queryEmbedding := []float64{0.01, 0.02, 0.03}
	embJSON, _ := json.Marshal(queryEmbedding)

	// Use the enclave's EncryptHybrid with the public key from attestation
	// (In real flow, SDK uses Web Crypto to encrypt with the PEM key)
	// For test, we use the Go enclave.EncryptHybrid
	aesKeyEnc, payloadEnc, nonce, err := enclave.EncryptHybrid(embJSON, &proxy.keyManager.PrivateKey().PublicKey, enclave.HashAlgorithmSHA256)
	if err != nil {
		t.Fatalf("EncryptHybrid: %v", err)
	}

	// 4. Run auction
	resp, err := proxy.RunAuction(&enclave.AuctionRequest{
		EncryptedEmbedding: enclave.EncryptedEmbedding{
			AESKeyEncrypted:  aesKeyEnc,
			EncryptedPayload: payloadEnc,
			Nonce:            nonce,
			HashAlgorithm:    "SHA-256",
		},
	})
	if err != nil {
		t.Fatalf("RunAuction: %v", err)
	}

	if resp.WinnerID == "" {
		t.Error("expected a winner")
	}
	if resp.Payment <= 0 {
		t.Errorf("payment = %f, want > 0", resp.Payment)
	}
	if resp.Currency != "USD" {
		t.Errorf("currency = %q, want USD", resp.Currency)
	}
	if resp.BidCount != 2 {
		t.Errorf("bid_count = %d, want 2", resp.BidCount)
	}
}

func TestMockTEEProxyAttestation(t *testing.T) {
	proxy, err := NewMockTEEProxy()
	if err != nil {
		t.Fatalf("NewMockTEEProxy: %v", err)
	}

	attest, err := proxy.GetAttestation()
	if err != nil {
		t.Fatalf("GetAttestation: %v", err)
	}
	if attest.PublicKey == "" {
		t.Error("expected non-empty public key")
	}
	if attest.AttestationB64 == "" {
		t.Error("expected non-empty attestation")
	}
}

func TestMockTEEProxyNoPositions(t *testing.T) {
	proxy, err := NewMockTEEProxy()
	if err != nil {
		t.Fatalf("NewMockTEEProxy: %v", err)
	}

	queryEmbedding := []float64{0.01, 0.02, 0.03}
	embJSON, _ := json.Marshal(queryEmbedding)
	aesKeyEnc, payloadEnc, nonce, _ := enclave.EncryptHybrid(embJSON, &proxy.keyManager.PrivateKey().PublicKey, enclave.HashAlgorithmSHA256)

	_, err = proxy.RunAuction(&enclave.AuctionRequest{
		EncryptedEmbedding: enclave.EncryptedEmbedding{
			AESKeyEncrypted:  aesKeyEnc,
			EncryptedPayload: payloadEnc,
			Nonce:            nonce,
			HashAlgorithm:    "SHA-256",
		},
	})
	if err == nil {
		t.Fatal("expected error with no positions")
	}
}

func TestMockTEEProxyPaymentMatchesDirect(t *testing.T) {
	proxy, err := NewMockTEEProxy()
	if err != nil {
		t.Fatalf("NewMockTEEProxy: %v", err)
	}

	positions := []enclave.PositionSnapshot{
		{ID: "adv-1", Name: "A", Embedding: []float64{0.1, 0.2, 0.3}, Sigma: 0.5, BidPrice: 10.0, Currency: "USD"},
		{ID: "adv-2", Name: "B", Embedding: []float64{0.4, 0.5, 0.6}, Sigma: 0.45, BidPrice: 8.0, Currency: "USD"},
	}
	budgets := []enclave.BudgetSnapshot{
		{AdvertiserID: "adv-1", Total: 1000, Spent: 0, Currency: "USD"},
		{AdvertiserID: "adv-2", Total: 1000, Spent: 0, Currency: "USD"},
	}
	proxy.SyncPositions(positions)
	proxy.SyncBudgets(budgets)

	queryEmbedding := []float64{0.1, 0.2, 0.3}
	embJSON, _ := json.Marshal(queryEmbedding)
	aesKeyEnc, payloadEnc, nonce, _ := enclave.EncryptHybrid(embJSON, &proxy.keyManager.PrivateKey().PublicKey, enclave.HashAlgorithmSHA256)

	resp, err := proxy.RunAuction(&enclave.AuctionRequest{
		EncryptedEmbedding: enclave.EncryptedEmbedding{
			AESKeyEncrypted:  aesKeyEnc,
			EncryptedPayload: payloadEnc,
			Nonce:            nonce,
			HashAlgorithm:    "SHA-256",
		},
	})
	if err != nil {
		t.Fatalf("RunAuction: %v", err)
	}

	// Payment should be reasonable (between 0 and winner's bid)
	if resp.Payment <= 0 || resp.Payment > 10.0 {
		t.Errorf("payment = %f, want 0 < p <= 10.0", resp.Payment)
	}
	if math.IsNaN(resp.Payment) {
		t.Error("payment is NaN")
	}
}
