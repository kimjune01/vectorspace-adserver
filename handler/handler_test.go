package handler

import (
	"bytes"
	"cloudx-adserver/platform"
	"encoding/json"
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

	router := NewRouter(RouterConfig{
		Registry: registry,
		Budgets:  budgets,
		Engine:   engine,
		DB:       db,
	})
	return router, db
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
	router, _ := setupTestRouter(t)

	// Register 2 advertisers so we get a winner + runner-up
	registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	registerAdvertiser(t, router, "Adv2", "intent two", 0.5, 3.0, 1000.0)

	body, _ := json.Marshal(map[string]interface{}{"intent": "query intent"})
	req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ad-request status = %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["winner"] == nil {
		t.Error("expected winner in response")
	}
	if resp["runner_up"] == nil {
		t.Error("expected runner_up with 2 advertisers")
	}
	if resp["all_bidders"] == nil {
		t.Error("expected all_bidders array")
	}
	bidders, ok := resp["all_bidders"].([]interface{})
	if !ok {
		t.Fatal("expected all_bidders array in response")
	}
	if len(bidders) != 2 {
		t.Errorf("all_bidders len = %d, want 2", len(bidders))
	}
	if payment, ok := resp["payment"].(float64); !ok || payment <= 0 {
		t.Errorf("payment = %v, want > 0", resp["payment"])
	}
	if resp["intent"] != "query intent" {
		t.Errorf("intent = %v, want %q", resp["intent"], "query intent")
	}
}

func TestAdRequestNoAdvertisers(t *testing.T) {
	router, _ := setupTestRouter(t)
	body, _ := json.Marshal(map[string]interface{}{"intent": "query"})
	req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 with no advertisers, got %d", w.Code)
	}
}

func TestAdRequestMissingIntent(t *testing.T) {
	router, _ := setupTestRouter(t)
	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing intent, got %d", w.Code)
	}
}

func TestStatsEndpoint(t *testing.T) {
	router, _ := setupTestRouter(t)

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

	// Do an ad request
	registerAdvertiser(t, router, "Adv1", "intent", 0.5, 2.0, 1000.0)
	body, _ := json.Marshal(map[string]interface{}{"intent": "query"})
	adReq := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	adW := httptest.NewRecorder()
	router.ServeHTTP(adW, adReq)
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

func TestAdRequestWithTau(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Register 2 advertisers (both get same embedding from fakeSidecar, so dist²=0)
	registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	registerAdvertiser(t, router, "Adv2", "intent two", 0.5, 3.0, 1000.0)

	// tau=1.0 — both pass since dist²=0 < 1.0
	body, _ := json.Marshal(map[string]interface{}{"intent": "query", "tau": 1.0})
	req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ad-request with tau status = %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	bidders, ok := resp["all_bidders"].([]interface{})
	if !ok {
		t.Fatal("expected all_bidders array")
	}
	if len(bidders) != 2 {
		t.Errorf("all_bidders len = %d, want 2 (both should pass tau with dist²=0)", len(bidders))
	}
}

func TestAdRequestWithTauOmitted(t *testing.T) {
	router, _ := setupTestRouter(t)

	registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)

	// No tau field — should default to no filtering
	body, _ := json.Marshal(map[string]interface{}{"intent": "query"})
	req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ad-request without tau status = %d: %s", w.Code, w.Body.String())
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
	router, _ := setupTestRouter(t)

	// Run an auction to generate stats
	registerAdvertiser(t, router, "Adv1", "intent", 0.5, 2.0, 1000.0)
	body, _ := json.Marshal(map[string]interface{}{"intent": "query"})
	adReq := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	adW := httptest.NewRecorder()
	router.ServeHTTP(adW, adReq)
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

// --- /embeddings tests ---

func TestEmbeddingsReturnsFormat(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Register an advertiser so there's at least one embedding
	registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 100.0)

	req := httptest.NewRequest("GET", "/embeddings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Version    string `json:"version"`
		Embeddings []struct {
			ID        string    `json:"id"`
			Embedding []float64 `json:"embedding"`
		} `json:"embeddings"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Version == "" {
		t.Error("expected non-empty version")
	}
	if len(resp.Embeddings) != 1 {
		t.Fatalf("embeddings len = %d, want 1", len(resp.Embeddings))
	}
	if resp.Embeddings[0].ID == "" {
		t.Error("expected non-empty embedding id")
	}
	if len(resp.Embeddings[0].Embedding) != 3 {
		t.Errorf("embedding dim = %d, want 3", len(resp.Embeddings[0].Embedding))
	}
}

func TestEmbeddingsETag(t *testing.T) {
	router, _ := setupTestRouter(t)
	registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 100.0)

	// First request: should return ETag
	req := httptest.NewRequest("GET", "/embeddings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header")
	}

	// Second request with If-None-Match: should return 304
	req = httptest.NewRequest("GET", "/embeddings", nil)
	req.Header.Set("If-None-Match", etag)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotModified {
		t.Errorf("expected 304, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("expected empty body on 304, got %d bytes", w.Body.Len())
	}
}

func TestEmbeddingsVersionChangesOnRegister(t *testing.T) {
	router, _ := setupTestRouter(t)
	registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 100.0)

	// Get initial version
	req := httptest.NewRequest("GET", "/embeddings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	etag1 := w.Header().Get("ETag")

	// Register another advertiser
	registerAdvertiser(t, router, "Adv2", "intent two", 0.5, 3.0, 100.0)

	// Version should change
	req = httptest.NewRequest("GET", "/embeddings", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	etag2 := w.Header().Get("ETag")

	if etag1 == etag2 {
		t.Error("ETag should change after registering a new advertiser")
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

func TestAdRequestRevenueDistribution(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Register 2 advertisers so VCG payment < bid_price (creates exchange revenue)
	registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 5.0, 1000.0)
	registerAdvertiser(t, router, "Adv2", "intent two", 0.5, 3.0, 1000.0)

	body, _ := json.Marshal(map[string]interface{}{"intent": "query"})
	req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ad-request status = %d", w.Code)
	}

	// Check stats: publisher_revenue + exchange_revenue = total_spend
	req = httptest.NewRequest("GET", "/stats", nil)
	w = httptest.NewRecorder()
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
