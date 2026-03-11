package handler

import (
	"bytes"
	"vectorspace/enclave"
	"vectorspace/platform"
	"vectorspace/tee"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTEETestRouter(t *testing.T) (http.Handler, *platform.DB, *tee.MockTEEProxy) {
	t.Helper()
	sidecar := fakeSidecar(3)
	t.Cleanup(sidecar.Close)

	db, err := platform.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	embedder := platform.NewEmbedder(sidecar.URL)
	registry := platform.NewPositionRegistry(embedder)
	if err := registry.SetDB(db); err != nil {
		t.Fatal(err)
	}
	budgets := platform.NewBudgetTracker()
	if err := budgets.SetDB(db); err != nil {
		t.Fatal(err)
	}
	engine := platform.NewAuctionEngine(registry, budgets, embedder)
	engine.DB = db

	proxy, err := tee.NewMockTEEProxy()
	if err != nil {
		t.Fatalf("NewMockTEEProxy: %v", err)
	}

	router := NewRouter(RouterConfig{
		Registry: registry,
		Budgets:  budgets,
		Engine:   engine,
		DB:       db,
		TEEProxy: proxy,
	})
	return router, db, proxy
}

func TestTEEAttestation(t *testing.T) {
	router, _, _ := setupTEETestRouter(t)

	req := httptest.NewRequest("GET", "/tee/attestation", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	var resp enclave.AttestationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.PublicKey == "" {
		t.Error("expected non-empty public key")
	}
	if resp.AttestationB64 == "" {
		t.Error("expected non-empty attestation")
	}
}

func TestTEEAttestationMethodNotAllowed(t *testing.T) {
	router, _, _ := setupTEETestRouter(t)
	req := httptest.NewRequest("POST", "/tee/attestation", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestTEEAdRequestPrivate(t *testing.T) {
	router, _, proxy := setupTEETestRouter(t)

	// Sync positions + budgets to mock proxy
	proxy.SyncPositions([]enclave.PositionSnapshot{
		{ID: "adv-1", Name: "Close Ad", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
		{ID: "adv-2", Name: "Far Ad", Embedding: []float64{1.0, 1.0, 1.0}, Sigma: 0.5, BidPrice: 5.0, Currency: "USD"},
	})
	proxy.SyncBudgets([]enclave.BudgetSnapshot{
		{AdvertiserID: "adv-1", Total: 1000, Spent: 0, Currency: "USD"},
		{AdvertiserID: "adv-2", Total: 1000, Spent: 0, Currency: "USD"},
	})

	// Encrypt embedding
	queryEmbedding := []float64{0.01, 0.02, 0.03}
	embJSON, _ := json.Marshal(queryEmbedding)
	pubKey := proxy.KeyManagerPublicKey()
	aesKeyEnc, payloadEnc, nonce, err := enclave.EncryptHybrid(embJSON, &pubKey, enclave.HashAlgorithmSHA256)
	if err != nil {
		t.Fatalf("EncryptHybrid: %v", err)
	}

	body, _ := json.Marshal(enclave.AuctionRequest{
		EncryptedEmbedding: enclave.EncryptedEmbedding{
			AESKeyEncrypted:  aesKeyEnc,
			EncryptedPayload: payloadEnc,
			Nonce:            nonce,
			HashAlgorithm:    "SHA-256",
		},
	})

	req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["winner_id"] == nil || resp["winner_id"] == "" {
		t.Error("expected winner_id in response")
	}
	if payment, ok := resp["payment"].(float64); !ok || payment <= 0 {
		t.Errorf("payment = %v, want > 0", resp["payment"])
	}
	if resp["currency"] != "USD" {
		t.Errorf("currency = %v, want USD", resp["currency"])
	}
	if bidCount, ok := resp["bid_count"].(float64); !ok || bidCount != 2 {
		t.Errorf("bid_count = %v, want 2", resp["bid_count"])
	}
}

func TestTEEAdRequestPrivateMethodNotAllowed(t *testing.T) {
	router, _, _ := setupTEETestRouter(t)
	req := httptest.NewRequest("GET", "/ad-request", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestTEEAdRequestRejectsPlaintextEmbedding(t *testing.T) {
	router, _, _ := setupTEETestRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"embedding": []float64{0.01, 0.02, 0.03},
	})
	req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("plaintext embeddings are not accepted")) {
		t.Errorf("expected rejection message, got: %s", w.Body.String())
	}
}

func TestTEEAdRequestPrivateMissingFields(t *testing.T) {
	router, _, _ := setupTEETestRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"encrypted_embedding": map[string]string{},
	})
	req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
