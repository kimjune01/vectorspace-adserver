package handler

import (
	"bytes"
	"vectorspace/enclave"
	"vectorspace/platform"
	"vectorspace/tee"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func init() {
	log.SetOutput(io.Discard)
}

// fakeSidecar returns a test HTTP server that produces deterministic embeddings.
func fakeSidecar(embDim int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Text  string   `json:"text"`
			Texts []string `json:"texts"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		makeEmb := func(seed int) []float64 {
			emb := make([]float64, embDim)
			for d := range emb {
				emb[d] = float64(seed+1) * 0.01 * float64(d+1)
			}
			return emb
		}

		w.Header().Set("Content-Type", "application/json")
		if req.Texts != nil {
			embeddings := make([][]float64, len(req.Texts))
			for i := range req.Texts {
				embeddings[i] = makeEmb(i)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"embeddings": embeddings,
				"dim":        embDim,
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"embedding": makeEmb(0),
				"dim":       embDim,
			})
		}
	}))
}

func setupTestRouter(t *testing.T) (http.Handler, *platform.DB) {
	router, db, _ := setupTestRouterWithProxy(t)
	return router, db
}

func setupTestRouterWithProxy(t *testing.T) (http.Handler, *platform.DB, *tee.MockTEEProxy) {
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

// teeAdRequest creates an encrypted ad-request body for the TEE endpoint.
// It syncs positions/budgets to the proxy, encrypts a query embedding, and POSTs.
func teeAdRequest(t *testing.T, router http.Handler, proxy *tee.MockTEEProxy, positions []enclave.PositionSnapshot, budgets []enclave.BudgetSnapshot, publisherID string) *httptest.ResponseRecorder {
	t.Helper()
	proxy.SyncPositions(positions)
	proxy.SyncBudgets(budgets)

	queryEmbedding := []float64{0.01, 0.02, 0.03}
	embJSON, _ := json.Marshal(queryEmbedding)
	pubKey := proxy.KeyManagerPublicKey()
	aesKeyEnc, payloadEnc, nonce, err := enclave.EncryptHybrid(embJSON, &pubKey, enclave.HashAlgorithmSHA256)
	if err != nil {
		t.Fatalf("EncryptHybrid: %v", err)
	}

	reqBody := map[string]interface{}{
		"encrypted_embedding": enclave.EncryptedEmbedding{
			AESKeyEncrypted:  aesKeyEnc,
			EncryptedPayload: payloadEnc,
			Nonce:            nonce,
			HashAlgorithm:    "SHA-256",
		},
	}
	if publisherID != "" {
		reqBody["publisher_id"] = publisherID
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func registerAdvertiser(t *testing.T, router http.Handler, name, intent string, sigma, bidPrice, budget float64) map[string]interface{} {
	t.Helper()
	body := map[string]interface{}{
		"name":      name,
		"intent":    intent,
		"sigma":     sigma,
		"bid_price": bidPrice,
		"budget":    budget,
		"currency":  "USD",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/advertiser/register", bytes.NewReader(b))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("register failed: status %d, body: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	return result
}

func TestRegisterSuccess(t *testing.T) {
	router, _ := setupTestRouter(t)

	result := registerAdvertiser(t, router, "Test Adv", "test intent", 0.5, 2.0, 100.0)

	if result["id"] == nil {
		t.Error("expected id in response")
	}
	if result["name"] != "Test Adv" {
		t.Errorf("name = %v, want %q", result["name"], "Test Adv")
	}
	if result["intent"] != "test intent" {
		t.Errorf("intent = %v, want %q", result["intent"], "test intent")
	}
}

func TestRegisterValidationErrors(t *testing.T) {
	router, _ := setupTestRouter(t)

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{"missing name", map[string]interface{}{"intent": "i", "sigma": 0.5, "bid_price": 1.0, "budget": 100.0}},
		{"missing intent", map[string]interface{}{"name": "n", "sigma": 0.5, "bid_price": 1.0, "budget": 100.0}},
		{"sigma <= 0", map[string]interface{}{"name": "n", "intent": "i", "sigma": 0, "bid_price": 1.0, "budget": 100.0}},
		{"bid_price <= 0", map[string]interface{}{"name": "n", "intent": "i", "sigma": 0.5, "bid_price": 0, "budget": 100.0}},
		{"budget <= 0", map[string]interface{}{"name": "n", "intent": "i", "sigma": 0.5, "bid_price": 1.0, "budget": 0}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.body)
			req := httptest.NewRequest("POST", "/advertiser/register", bytes.NewReader(b))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestGetPositions(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Initially empty
	req := httptest.NewRequest("GET", "/positions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var positions []interface{}
	json.NewDecoder(w.Body).Decode(&positions)
	if len(positions) != 0 {
		t.Errorf("expected 0 positions initially, got %d", len(positions))
	}

	// Register one
	registerAdvertiser(t, router, "Adv1", "intent1", 0.5, 2.0, 100.0)

	req = httptest.NewRequest("GET", "/positions", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	json.NewDecoder(w.Body).Decode(&positions)
	if len(positions) != 1 {
		t.Errorf("expected 1 position, got %d", len(positions))
	}
}

func TestGetBudget(t *testing.T) {
	router, _ := setupTestRouter(t)
	result := registerAdvertiser(t, router, "Adv1", "intent1", 0.5, 2.0, 100.0)
	id, ok := result["id"].(string)
	if !ok {
		t.Fatal("expected string id in register response")
	}

	req := httptest.NewRequest("GET", "/budget/"+id, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	var budget map[string]interface{}
	json.NewDecoder(w.Body).Decode(&budget)
	if total, ok := budget["total"].(float64); !ok || total != 100.0 {
		t.Errorf("total = %v, want 100.0", budget["total"])
	}
	if remaining, ok := budget["remaining"].(float64); !ok || remaining != 100.0 {
		t.Errorf("remaining = %v, want 100.0", budget["remaining"])
	}
}

func TestGetBudgetNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)
	req := httptest.NewRequest("GET", "/budget/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateAdvertiser(t *testing.T) {
	router, _ := setupTestRouter(t)
	result := registerAdvertiser(t, router, "Original", "original intent", 0.5, 2.0, 100.0)
	id, ok := result["id"].(string)
	if !ok {
		t.Fatal("expected string id in register response")
	}

	updateBody := map[string]interface{}{
		"name":      "Updated",
		"bid_price": 3.0,
	}
	b, _ := json.Marshal(updateBody)
	req := httptest.NewRequest("PUT", "/advertiser/"+id, bytes.NewReader(b))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", w.Code, w.Body.String())
	}

	var updated map[string]interface{}
	json.NewDecoder(w.Body).Decode(&updated)
	if updated["name"] != "Updated" {
		t.Errorf("name = %v, want %q", updated["name"], "Updated")
	}
}

func TestUpdateAdvertiserNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)
	b, _ := json.Marshal(map[string]interface{}{"name": "x"})
	req := httptest.NewRequest("PUT", "/advertiser/nonexistent", bytes.NewReader(b))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteAdvertiser(t *testing.T) {
	router, _ := setupTestRouter(t)
	result := registerAdvertiser(t, router, "ToDelete", "intent", 0.5, 2.0, 100.0)
	id, ok := result["id"].(string)
	if !ok {
		t.Fatal("expected string id in register response")
	}

	req := httptest.NewRequest("DELETE", "/advertiser/"+id, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete status = %d: %s", w.Code, w.Body.String())
	}

	// Verify positions shrinks
	req = httptest.NewRequest("GET", "/positions", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var positions []interface{}
	json.NewDecoder(w.Body).Decode(&positions)
	if len(positions) != 0 {
		t.Errorf("expected 0 positions after delete, got %d", len(positions))
	}
}

func TestDeleteAdvertiserNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)
	req := httptest.NewRequest("DELETE", "/advertiser/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestAdRequest(t *testing.T) {
	router, _, proxy := setupTestRouterWithProxy(t)

	// Register 2 advertisers so we get a winner + runner-up
	registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	registerAdvertiser(t, router, "Adv2", "intent two", 0.5, 3.0, 1000.0)

	w := teeAdRequest(t, router, proxy,
		[]enclave.PositionSnapshot{
			{ID: "adv-1", Name: "Adv1", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
			{ID: "adv-2", Name: "Adv2", Embedding: []float64{1.0, 1.0, 1.0}, Sigma: 0.5, BidPrice: 3.0, Currency: "USD"},
		},
		[]enclave.BudgetSnapshot{
			{AdvertiserID: "adv-1", Total: 1000, Spent: 0, Currency: "USD"},
			{AdvertiserID: "adv-2", Total: 1000, Spent: 0, Currency: "USD"},
		},
		"",
	)
	if w.Code != http.StatusOK {
		t.Fatalf("ad-request status = %d: %s", w.Code, w.Body.String())
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
}

func TestAdRequestNoAdvertisers(t *testing.T) {
	router, _, proxy := setupTestRouterWithProxy(t)

	w := teeAdRequest(t, router, proxy,
		[]enclave.PositionSnapshot{},
		[]enclave.BudgetSnapshot{},
		"",
	)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 with no advertisers, got %d", w.Code)
	}
}

func TestAdRequestMissingEncryptedEmbedding(t *testing.T) {
	router, _ := setupTestRouter(t)
	body, _ := json.Marshal(map[string]interface{}{
		"encrypted_embedding": map[string]string{},
	})
	req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing encrypted_embedding fields, got %d", w.Code)
	}
}

// plainEmbeddingAdRequest creates a plain-vector ad-request (no encryption).
// It syncs positions/budgets to the proxy, then POSTs a raw embedding vector.
func plainEmbeddingAdRequest(t *testing.T, router http.Handler, proxy *tee.MockTEEProxy, positions []enclave.PositionSnapshot, budgets []enclave.BudgetSnapshot, publisherID string) *httptest.ResponseRecorder {
	t.Helper()
	proxy.SyncPositions(positions)
	proxy.SyncBudgets(budgets)

	reqBody := map[string]interface{}{
		"embedding": []float64{0.01, 0.02, 0.03},
	}
	if publisherID != "" {
		reqBody["publisher_id"] = publisherID
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestAdRequestPlainEmbeddingRejected(t *testing.T) {
	router, _, proxy := setupTestRouterWithProxy(t)

	w := plainEmbeddingAdRequest(t, router, proxy,
		[]enclave.PositionSnapshot{
			{ID: "adv-1", Name: "Adv1", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
			{ID: "adv-2", Name: "Adv2", Embedding: []float64{1.0, 1.0, 1.0}, Sigma: 0.5, BidPrice: 3.0, Currency: "USD"},
		},
		[]enclave.BudgetSnapshot{
			{AdvertiserID: "adv-1", Total: 1000, Spent: 0, Currency: "USD"},
			{AdvertiserID: "adv-2", Total: 1000, Spent: 0, Currency: "USD"},
		},
		"pub-1",
	)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for plain embedding, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdRequestNeitherEmbeddingProvided(t *testing.T) {
	router, _ := setupTestRouter(t)
	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when neither embedding provided, got %d", w.Code)
	}
}

func TestStatsEndpoint(t *testing.T) {
	router, _, proxy := setupTestRouterWithProxy(t)

	// Initial stats should be zeroed
	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("stats status = %d", w.Code)
	}

	var stats map[string]interface{}
	json.NewDecoder(w.Body).Decode(&stats)
	if count, ok := stats["auction_count"].(float64); !ok || count != 0 {
		t.Errorf("initial auction_count = %v, want 0", stats["auction_count"])
	}

	// Do a TEE ad request
	registerAdvertiser(t, router, "Adv1", "intent", 0.5, 2.0, 1000.0)
	adW := teeAdRequest(t, router, proxy,
		[]enclave.PositionSnapshot{
			{ID: "adv-1", Name: "Adv1", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
		},
		[]enclave.BudgetSnapshot{
			{AdvertiserID: "adv-1", Total: 1000, Spent: 0, Currency: "USD"},
		},
		"",
	)
	if adW.Code != http.StatusOK {
		t.Fatalf("ad-request failed: %d", adW.Code)
	}

	// Stats should now show 1 auction
	req = httptest.NewRequest("GET", "/stats", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	json.NewDecoder(w.Body).Decode(&stats)
	if count, ok := stats["auction_count"].(float64); !ok || count != 1 {
		t.Errorf("auction_count = %v, want 1", stats["auction_count"])
	}
	if spend, ok := stats["total_spend"].(float64); !ok || spend <= 0 {
		t.Errorf("total_spend = %v, want > 0", stats["total_spend"])
	}
}

func TestChatNoAPIKey(t *testing.T) {
	router, _ := setupTestRouter(t) // no AnthropicKey set

	body, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	req := httptest.NewRequest("POST", "/chat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 without API key, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCORSHeaders(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("CORS Allow-Origin = %q, want %q", w.Header().Get("Access-Control-Allow-Origin"), "*")
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("CORS Allow-Methods header missing")
	}
	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("CORS Allow-Headers header missing")
	}
}

func TestCORSPreflight(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest("OPTIONS", "/ad-request", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("OPTIONS status = %d, want 200", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS headers missing on OPTIONS")
	}
}

func TestHealthEndpoint(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("health status = %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
	if _, ok := resp["gitHash"]; !ok {
		t.Error("gitHash field missing from health response")
	}
}

func TestRegisterMethodNotAllowed(t *testing.T) {
	router, _ := setupTestRouter(t)
	req := httptest.NewRequest("GET", "/advertiser/register", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestAdRequestMethodNotAllowed(t *testing.T) {
	router, _ := setupTestRouter(t)
	req := httptest.NewRequest("GET", "/ad-request", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestStatsReset(t *testing.T) {
	router, _, proxy := setupTestRouterWithProxy(t)

	// Run an auction to generate stats
	registerAdvertiser(t, router, "Adv1", "intent", 0.5, 2.0, 1000.0)
	adW := teeAdRequest(t, router, proxy,
		[]enclave.PositionSnapshot{
			{ID: "adv-1", Name: "Adv1", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
		},
		[]enclave.BudgetSnapshot{
			{AdvertiserID: "adv-1", Total: 1000, Spent: 0, Currency: "USD"},
		},
		"",
	)
	if adW.Code != http.StatusOK {
		t.Fatalf("ad-request failed: %d", adW.Code)
	}

	// Verify stats are non-zero
	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var stats map[string]interface{}
	json.NewDecoder(w.Body).Decode(&stats)
	if count, _ := stats["auction_count"].(float64); count != 1 {
		t.Fatalf("pre-reset auction_count = %v, want 1", stats["auction_count"])
	}

	// Reset stats
	req = httptest.NewRequest("DELETE", "/stats", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DELETE /stats status = %d, want 204", w.Code)
	}

	// Verify stats are zeroed
	req = httptest.NewRequest("GET", "/stats", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	json.NewDecoder(w.Body).Decode(&stats)
	if count, _ := stats["auction_count"].(float64); count != 0 {
		t.Errorf("post-reset auction_count = %v, want 0", stats["auction_count"])
	}
	if spend, _ := stats["total_spend"].(float64); spend != 0 {
		t.Errorf("post-reset total_spend = %v, want 0", stats["total_spend"])
	}
}

func TestStatsMethodNotAllowed(t *testing.T) {
	router, _ := setupTestRouter(t)
	req := httptest.NewRequest("POST", "/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// --- /embed tests ---

func TestEmbedReturnsVector(t *testing.T) {
	router, _ := setupTestRouter(t)

	body, _ := json.Marshal(map[string]string{"text": "back pain from sitting"})
	req := httptest.NewRequest("POST", "/embed", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Embedding) != 3 {
		t.Errorf("embedding dim = %d, want 3", len(resp.Embedding))
	}
}

func TestEmbedMissingText(t *testing.T) {
	router, _ := setupTestRouter(t)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/embed", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestEmbedMethodNotAllowed(t *testing.T) {
	router, _ := setupTestRouter(t)
	req := httptest.NewRequest("GET", "/embed", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestEmbeddingsEndpoint(t *testing.T) {
	router, _ := setupTestRouter(t)

	// No advertisers yet — should return empty list with ETag
	req := httptest.NewRequest("GET", "/embeddings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header")
	}
	var resp struct {
		Embeddings []struct {
			ID        string    `json:"id"`
			Name      string    `json:"name"`
			Embedding []float64 `json:"embedding"`
			BidPrice  float64   `json:"bid_price"`
			Sigma     float64   `json:"sigma"`
			Currency  string    `json:"currency"`
		} `json:"embeddings"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Embeddings) != 0 {
		t.Errorf("expected 0 embeddings, got %d", len(resp.Embeddings))
	}

	// Register an advertiser
	registerAdvertiser(t, router, "Dog Trainer", "dog training", 1.0, 5.0, 100.0)

	// Should now return 1 embedding
	req2 := httptest.NewRequest("GET", "/embeddings", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
	newEtag := w2.Header().Get("ETag")
	if newEtag == etag {
		t.Error("expected ETag to change after registration")
	}
	var resp2 struct {
		Embeddings []struct {
			ID        string    `json:"id"`
			Embedding []float64 `json:"embedding"`
		} `json:"embeddings"`
	}
	json.NewDecoder(w2.Body).Decode(&resp2)
	if len(resp2.Embeddings) != 1 {
		t.Fatalf("expected 1 embedding, got %d", len(resp2.Embeddings))
	}
	if len(resp2.Embeddings[0].Embedding) == 0 {
		t.Error("expected non-empty embedding vector")
	}

	// If-None-Match should return 304
	req3 := httptest.NewRequest("GET", "/embeddings", nil)
	req3.Header.Set("If-None-Match", newEtag)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	if w3.Code != http.StatusNotModified {
		t.Errorf("expected 304, got %d", w3.Code)
	}
}

func TestEmbeddingsMethodNotAllowed(t *testing.T) {
	router, _ := setupTestRouter(t)
	req := httptest.NewRequest("POST", "/embeddings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestCreativeCreateAndList(t *testing.T) {
	router, db := setupTestRouter(t)
	result := registerAdvertiser(t, router, "Adv1", "intent", 0.5, 2.0, 100.0)
	advID := result["id"].(string)
	token, err := db.GenerateToken(advID)
	if err != nil {
		t.Fatal(err)
	}

	// POST: create creative
	body, _ := json.Marshal(map[string]string{"title": "Buy Now", "subtitle": "Best deal"})
	req := httptest.NewRequest("POST", "/portal/me/creatives?token="+token, bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create creative status = %d: %s", w.Code, w.Body.String())
	}
	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	if created["title"] != "Buy Now" {
		t.Errorf("title = %v, want %q", created["title"], "Buy Now")
	}
	if created["subtitle"] != "Best deal" {
		t.Errorf("subtitle = %v, want %q", created["subtitle"], "Best deal")
	}

	// GET: list creatives
	req = httptest.NewRequest("GET", "/portal/me/creatives?token="+token, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list creatives status = %d", w.Code)
	}
	var list []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 1 {
		t.Fatalf("expected 1 creative, got %d", len(list))
	}
	if list[0]["title"] != "Buy Now" {
		t.Errorf("listed title = %v, want %q", list[0]["title"], "Buy Now")
	}
}

func TestCreativeUpdateAndDelete(t *testing.T) {
	router, db := setupTestRouter(t)
	result := registerAdvertiser(t, router, "Adv1", "intent", 0.5, 2.0, 100.0)
	advID := result["id"].(string)
	token, err := db.GenerateToken(advID)
	if err != nil {
		t.Fatal(err)
	}

	// Create a creative
	body, _ := json.Marshal(map[string]string{"title": "Old Title", "subtitle": "Old Sub"})
	req := httptest.NewRequest("POST", "/portal/me/creatives?token="+token, bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create creative status = %d", w.Code)
	}
	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	creativeID := int64(created["id"].(float64))

	// PUT: update creative
	body, _ = json.Marshal(map[string]string{"title": "New Title", "subtitle": "New Sub"})
	req = httptest.NewRequest("PUT", fmt.Sprintf("/portal/me/creatives/%d?token=%s", creativeID, token), bytes.NewReader(body))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update creative status = %d: %s", w.Code, w.Body.String())
	}
	var updated map[string]interface{}
	json.NewDecoder(w.Body).Decode(&updated)
	if updated["title"] != "New Title" {
		t.Errorf("updated title = %v, want %q", updated["title"], "New Title")
	}

	// DELETE: remove creative
	req = httptest.NewRequest("DELETE", fmt.Sprintf("/portal/me/creatives/%d?token=%s", creativeID, token), nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete creative status = %d", w.Code)
	}

	// Verify gone
	req = httptest.NewRequest("GET", "/portal/me/creatives?token="+token, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var list []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 0 {
		t.Errorf("expected 0 creatives after delete, got %d", len(list))
	}
}

func TestCreativeInAdResponse(t *testing.T) {
	router, db, proxy := setupTestRouterWithProxy(t)

	// Register 2 advertisers
	result1 := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)

	// Add creative to Adv1
	advID := result1["id"].(string)
	db.InsertCreative(advID, "My Ad Title", "My Ad Subtitle")

	// Run TEE ad request — creative data is not in TEE response (only winner_id/payment)
	// but verify the auction still runs successfully
	w := teeAdRequest(t, router, proxy,
		[]enclave.PositionSnapshot{
			{ID: advID, Name: "Adv1", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
		},
		[]enclave.BudgetSnapshot{
			{AdvertiserID: advID, Total: 1000, Spent: 0, Currency: "USD"},
		},
		"",
	)
	if w.Code != http.StatusOK {
		t.Fatalf("ad-request status = %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["winner_id"] != advID {
		t.Errorf("winner_id = %v, want %q", resp["winner_id"], advID)
	}
}

func TestAdRequestRevenueDistribution(t *testing.T) {
	router, _, proxy := setupTestRouterWithProxy(t)

	// Register 2 advertisers so VCG payment < bid_price (creates exchange revenue)
	registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 5.0, 1000.0)
	registerAdvertiser(t, router, "Adv2", "intent two", 0.5, 3.0, 1000.0)

	adW := teeAdRequest(t, router, proxy,
		[]enclave.PositionSnapshot{
			{ID: "adv-1", Name: "Adv1", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 5.0, Currency: "USD"},
			{ID: "adv-2", Name: "Adv2", Embedding: []float64{1.0, 1.0, 1.0}, Sigma: 0.5, BidPrice: 3.0, Currency: "USD"},
		},
		[]enclave.BudgetSnapshot{
			{AdvertiserID: "adv-1", Total: 1000, Spent: 0, Currency: "USD"},
			{AdvertiserID: "adv-2", Total: 1000, Spent: 0, Currency: "USD"},
		},
		"",
	)
	if adW.Code != http.StatusOK {
		t.Fatalf("ad-request status = %d", adW.Code)
	}

	// Check stats: publisher_revenue + exchange_revenue = total_spend
	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var stats map[string]interface{}
	json.NewDecoder(w.Body).Decode(&stats)

	totalSpend, _ := stats["total_spend"].(float64)
	pubRevenue, _ := stats["publisher_revenue"].(float64)
	exchRevenue, _ := stats["exchange_revenue"].(float64)

	if totalSpend <= 0 {
		t.Errorf("total_spend = %v, want > 0", totalSpend)
	}
	if pubRevenue <= 0 {
		t.Errorf("publisher_revenue = %v, want > 0", pubRevenue)
	}
	// publisher + exchange should sum to total spend
	sum := pubRevenue + exchRevenue
	if diff := totalSpend - sum; diff > 0.01 || diff < -0.01 {
		t.Errorf("publisher(%.4f) + exchange(%.4f) = %.4f, want %.4f", pubRevenue, exchRevenue, sum, totalSpend)
	}
}
