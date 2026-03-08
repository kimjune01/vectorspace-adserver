package handler

import (
	"bytes"
	"vectorspace/enclave"
	"vectorspace/platform"
	"vectorspace/tee"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Event Tracking Tests ---

func TestImpressionEvent(t *testing.T) {
	router, _, proxy := setupTestRouterWithProxy(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	advID := result["id"].(string)

	// Run a TEE auction to get an auction_id
	adW := teeAdRequest(t, router, proxy,
		[]enclave.PositionSnapshot{
			{ID: advID, Name: "Adv1", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
		},
		[]enclave.BudgetSnapshot{
			{AdvertiserID: advID, Total: 1000, Spent: 0, Currency: "USD"},
		},
		"",
	)
	if adW.Code != http.StatusOK {
		t.Fatalf("ad-request failed: %d", adW.Code)
	}
	var adResp map[string]interface{}
	json.NewDecoder(adW.Body).Decode(&adResp)
	auctionID := adResp["auction_id"].(float64)

	// Log impression
	body, _ := json.Marshal(map[string]interface{}{
		"auction_id":    auctionID,
		"advertiser_id": advID,
		"user_id":       "u1",
	})
	req := httptest.NewRequest("POST", "/event/impression", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("impression status = %d: %s", w.Code, w.Body.String())
	}
}

func TestImpressionFrequencyCap(t *testing.T) {
	router, _ := setupTestRouter(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	advID := result["id"].(string)

	// Hit the frequency cap (default 3)
	for i := 0; i < 3; i++ {
		body, _ := json.Marshal(map[string]interface{}{
			"auction_id":    1,
			"advertiser_id": advID,
			"user_id":       "u1",
		})
		req := httptest.NewRequest("POST", "/event/impression", bytes.NewReader(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("impression %d status = %d: %s", i+1, w.Code, w.Body.String())
		}
	}

	// 4th should be capped
	body, _ := json.Marshal(map[string]interface{}{
		"auction_id":    1,
		"advertiser_id": advID,
		"user_id":       "u1",
	})
	req := httptest.NewRequest("POST", "/event/impression", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestImpressionNoUserIDBypassesCap(t *testing.T) {
	router, _ := setupTestRouter(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	advID := result["id"].(string)

	// Without user_id, no frequency cap applies — should always succeed
	for i := 0; i < 5; i++ {
		body, _ := json.Marshal(map[string]interface{}{
			"auction_id":    1,
			"advertiser_id": advID,
		})
		req := httptest.NewRequest("POST", "/event/impression", bytes.NewReader(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("impression %d without user_id: status = %d", i+1, w.Code)
		}
	}
}

func TestClickEvent(t *testing.T) {
	router, _ := setupTestRouter(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	advID := result["id"].(string)

	body, _ := json.Marshal(map[string]interface{}{
		"auction_id":    1,
		"advertiser_id": advID,
		"user_id":       "u1",
	})
	req := httptest.NewRequest("POST", "/event/click", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("click status = %d: %s", w.Code, w.Body.String())
	}
}

func TestViewableEvent(t *testing.T) {
	router, _ := setupTestRouter(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	advID := result["id"].(string)

	body, _ := json.Marshal(map[string]interface{}{
		"auction_id":    1,
		"advertiser_id": advID,
		"user_id":       "u1",
	})
	req := httptest.NewRequest("POST", "/event/viewable", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("viewable status = %d: %s", w.Code, w.Body.String())
	}
}

func TestEventMissingAdvertiserID(t *testing.T) {
	router, _ := setupTestRouter(t)

	body, _ := json.Marshal(map[string]interface{}{"auction_id": 1})
	for _, path := range []string{"/event/impression", "/event/click", "/event/viewable"} {
		req := httptest.NewRequest("POST", path, bytes.NewReader(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", path, w.Code)
		}
	}
}

// --- Portal Tests ---

func TestPortalMeWithToken(t *testing.T) {
	router, _ := setupTestRouter(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	token, ok := result["token"].(string)
	if !ok || token == "" {
		t.Fatal("expected token in register response")
	}

	req := httptest.NewRequest("GET", "/portal/me?token="+token, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("portal/me status = %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["name"] != "Adv1" {
		t.Errorf("name = %v, want Adv1", resp["name"])
	}
	if resp["budget_total"] == nil {
		t.Error("expected budget_total in response")
	}
}

func TestPortalMeInvalidToken(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/portal/me?token=invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPortalMeNoToken(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/portal/me", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPortalMeUpdate(t *testing.T) {
	router, _ := setupTestRouter(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	token := result["token"].(string)

	body, _ := json.Marshal(map[string]interface{}{"name": "Updated Name"})
	req := httptest.NewRequest("PUT", "/portal/me?token="+token, bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("portal/me PUT status = %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["name"] != "Updated Name" {
		t.Errorf("name = %v, want Updated Name", resp["name"])
	}
}

func TestPortalAuctions(t *testing.T) {
	router, _, proxy := setupTestRouterWithProxy(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	token := result["token"].(string)
	advID := result["id"].(string)

	// Run a TEE auction
	teeAdRequest(t, router, proxy,
		[]enclave.PositionSnapshot{
			{ID: advID, Name: "Adv1", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
		},
		[]enclave.BudgetSnapshot{
			{AdvertiserID: advID, Total: 1000, Spent: 0, Currency: "USD"},
		},
		"",
	)

	req := httptest.NewRequest("GET", "/portal/me/auctions?token="+token, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("portal/me/auctions status = %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	total, _ := resp["total"].(float64)
	if total != 1 {
		t.Errorf("total = %v, want 1", total)
	}
}

func TestPortalEvents(t *testing.T) {
	router, _ := setupTestRouter(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	token := result["token"].(string)
	advID := result["id"].(string)

	// Log an event
	body, _ := json.Marshal(map[string]interface{}{
		"auction_id": 1, "advertiser_id": advID, "user_id": "u1",
	})
	eventReq := httptest.NewRequest("POST", "/event/impression", bytes.NewReader(body))
	eventW := httptest.NewRecorder()
	router.ServeHTTP(eventW, eventReq)

	req := httptest.NewRequest("GET", "/portal/me/events?token="+token, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("portal/me/events status = %d: %s", w.Code, w.Body.String())
	}

	var stats map[string]interface{}
	json.NewDecoder(w.Body).Decode(&stats)
	if imps, _ := stats["impressions"].(float64); imps != 1 {
		t.Errorf("impressions = %v, want 1", imps)
	}
}

// --- Admin Tests ---

func TestAdminAuctions(t *testing.T) {
	router, _, proxy := setupTestRouterWithProxy(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	advID := result["id"].(string)
	teeAdRequest(t, router, proxy,
		[]enclave.PositionSnapshot{
			{ID: advID, Name: "Adv1", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
		},
		[]enclave.BudgetSnapshot{
			{AdvertiserID: advID, Total: 1000, Spent: 0, Currency: "USD"},
		},
		"",
	)

	req := httptest.NewRequest("GET", "/admin/auctions?limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin/auctions status = %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	total, _ := resp["total"].(float64)
	if total < 1 {
		t.Errorf("total = %v, want >= 1", total)
	}
}

func TestAdminAuctionsCSV(t *testing.T) {
	router, _, proxy := setupTestRouterWithProxy(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	advID := result["id"].(string)
	teeAdRequest(t, router, proxy,
		[]enclave.PositionSnapshot{
			{ID: advID, Name: "Adv1", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
		},
		[]enclave.BudgetSnapshot{
			{AdvertiserID: advID, Total: 1000, Spent: 0, Currency: "USD"},
		},
		"",
	)

	req := httptest.NewRequest("GET", "/admin/auctions?format=csv", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin/auctions CSV status = %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}
	if !strings.Contains(w.Body.String(), "id,intent,winner_id") {
		t.Error("CSV missing header row")
	}
}

func TestAdminRevenue(t *testing.T) {
	router, _, proxy := setupTestRouterWithProxy(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	advID := result["id"].(string)
	teeAdRequest(t, router, proxy,
		[]enclave.PositionSnapshot{
			{ID: advID, Name: "Adv1", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
		},
		[]enclave.BudgetSnapshot{
			{AdvertiserID: advID, Total: 1000, Spent: 0, Currency: "USD"},
		},
		"",
	)

	req := httptest.NewRequest("GET", "/admin/revenue?group_by=day", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin/revenue status = %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["group_by"] != "day" {
		t.Errorf("group_by = %v, want day", resp["group_by"])
	}
}

func TestAdminTopAdvertisers(t *testing.T) {
	router, _, proxy := setupTestRouterWithProxy(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	advID := result["id"].(string)
	teeAdRequest(t, router, proxy,
		[]enclave.PositionSnapshot{
			{ID: advID, Name: "Adv1", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
		},
		[]enclave.BudgetSnapshot{
			{AdvertiserID: advID, Total: 1000, Spent: 0, Currency: "USD"},
		},
		"",
	)

	req := httptest.NewRequest("GET", "/admin/top-advertisers?limit=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin/top-advertisers status = %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminAdvertisers(t *testing.T) {
	router, _ := setupTestRouter(t)

	registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)

	req := httptest.NewRequest("GET", "/admin/advertisers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin/advertisers status = %d: %s", w.Code, w.Body.String())
	}

	var resp []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 1 {
		t.Errorf("len = %d, want 1", len(resp))
	}
}

func TestAdminEvents(t *testing.T) {
	router, _ := setupTestRouter(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	advID := result["id"].(string)

	// Log events
	for _, evType := range []string{"/event/impression", "/event/click", "/event/viewable"} {
		body, _ := json.Marshal(map[string]interface{}{
			"auction_id": 1, "advertiser_id": advID, "user_id": "u1",
		})
		eventReq := httptest.NewRequest("POST", evType, bytes.NewReader(body))
		eventW := httptest.NewRecorder()
		router.ServeHTTP(eventW, eventReq)
	}

	req := httptest.NewRequest("GET", "/admin/events", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin/events status = %d: %s", w.Code, w.Body.String())
	}

	var stats map[string]interface{}
	json.NewDecoder(w.Body).Decode(&stats)
	if imps, _ := stats["impressions"].(float64); imps != 1 {
		t.Errorf("impressions = %v, want 1", imps)
	}
	if clicks, _ := stats["clicks"].(float64); clicks != 1 {
		t.Errorf("clicks = %v, want 1", clicks)
	}
	if viewable, _ := stats["viewable"].(float64); viewable != 1 {
		t.Errorf("viewable = %v, want 1", viewable)
	}
}

func TestAdRequestReturnsAuctionID(t *testing.T) {
	router, _, proxy := setupTestRouterWithProxy(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	advID := result["id"].(string)

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
	auctionID, ok := resp["auction_id"].(float64)
	if !ok || auctionID <= 0 {
		t.Errorf("auction_id = %v, want > 0", resp["auction_id"])
	}
}

func TestRegisterReturnsToken(t *testing.T) {
	router, _ := setupTestRouter(t)

	result := registerAdvertiser(t, router, "TokenTest", "intent", 0.5, 2.0, 100.0)
	token, ok := result["token"].(string)
	if !ok || token == "" {
		t.Error("expected token in register response")
	}
	if len(token) != 32 {
		t.Errorf("token length = %d, want 32", len(token))
	}
}

// --- Publisher Registration Tests ---

func registerPublisher(t *testing.T, router http.Handler, name, domain string) map[string]interface{} {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{"name": name, "domain": domain})
	req := httptest.NewRequest("POST", "/publisher/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("publisher register failed: status %d, body: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	return result
}

func TestPublisherRegister(t *testing.T) {
	router, _ := setupTestRouter(t)

	result := registerPublisher(t, router, "TechBlog", "techblog.com")
	if result["id"] == nil {
		t.Error("expected id in response")
	}
	if result["name"] != "TechBlog" {
		t.Errorf("name = %v, want TechBlog", result["name"])
	}
	if result["domain"] != "techblog.com" {
		t.Errorf("domain = %v, want techblog.com", result["domain"])
	}
	token, ok := result["token"].(string)
	if !ok || len(token) != 32 {
		t.Errorf("expected 32-char token, got %q", token)
	}
}

func TestPublisherRegisterMissingName(t *testing.T) {
	router, _ := setupTestRouter(t)

	body, _ := json.Marshal(map[string]interface{}{"domain": "test.com"})
	req := httptest.NewRequest("POST", "/publisher/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- Publisher Portal Tests ---

func TestPublisherPortalMe(t *testing.T) {
	router, _ := setupTestRouter(t)

	result := registerPublisher(t, router, "TechBlog", "techblog.com")
	token := result["token"].(string)

	req := httptest.NewRequest("GET", "/portal/publisher/me?token="+token, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	var pub map[string]interface{}
	json.NewDecoder(w.Body).Decode(&pub)
	if pub["name"] != "TechBlog" {
		t.Errorf("name = %v, want TechBlog", pub["name"])
	}
}

func TestPublisherPortalMeInvalidToken(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/portal/publisher/me?token=invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPublisherPortalStats(t *testing.T) {
	router, db := setupTestRouter(t)

	result := registerPublisher(t, router, "TechBlog", "techblog.com")
	token := result["token"].(string)
	pubID := result["id"].(string)

	// Log auctions with publisher_id
	db.LogAuctionReturningIDWithPublisher("intent-a", "adv-1", 10.0, "USD", 5, pubID)
	db.LogAuctionReturningIDWithPublisher("intent-b", "adv-2", 5.0, "USD", 3, pubID)

	req := httptest.NewRequest("GET", "/portal/publisher/stats?token="+token, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	var stats map[string]interface{}
	json.NewDecoder(w.Body).Decode(&stats)
	if count, _ := stats["auction_count"].(float64); count != 2 {
		t.Errorf("auction_count = %v, want 2", count)
	}
	if rev, _ := stats["total_revenue"].(float64); rev <= 0 {
		t.Errorf("total_revenue = %v, want > 0", rev)
	}
}

func TestPublisherPortalRevenue(t *testing.T) {
	router, db := setupTestRouter(t)

	result := registerPublisher(t, router, "TechBlog", "techblog.com")
	token := result["token"].(string)
	pubID := result["id"].(string)

	db.LogAuctionReturningIDWithPublisher("intent-a", "adv-1", 10.0, "USD", 5, pubID)

	req := httptest.NewRequest("GET", "/portal/publisher/revenue?token="+token+"&group_by=day", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["group_by"] != "day" {
		t.Errorf("group_by = %v, want day", resp["group_by"])
	}
}

func TestPublisherPortalEvents(t *testing.T) {
	router, db := setupTestRouter(t)

	result := registerPublisher(t, router, "TechBlog", "techblog.com")
	token := result["token"].(string)
	pubID := result["id"].(string)

	db.LogEventWithPublisher(1, "adv-1", "impression", "u1", pubID)
	db.LogEventWithPublisher(1, "adv-1", "click", "u1", pubID)

	req := httptest.NewRequest("GET", "/portal/publisher/events?token="+token, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	var stats map[string]interface{}
	json.NewDecoder(w.Body).Decode(&stats)
	if imps, _ := stats["impressions"].(float64); imps != 1 {
		t.Errorf("impressions = %v, want 1", imps)
	}
	if clicks, _ := stats["clicks"].(float64); clicks != 1 {
		t.Errorf("clicks = %v, want 1", clicks)
	}
}

func TestPublisherPortalAuctions(t *testing.T) {
	router, db := setupTestRouter(t)

	result := registerPublisher(t, router, "TechBlog", "techblog.com")
	token := result["token"].(string)
	pubID := result["id"].(string)

	db.LogAuctionReturningIDWithPublisher("intent-a", "adv-1", 2.50, "USD", 5, pubID)

	req := httptest.NewRequest("GET", "/portal/publisher/auctions?token="+token+"&limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	total, _ := resp["total"].(float64)
	if total != 1 {
		t.Errorf("total = %v, want 1", total)
	}
}

func TestPublisherPortalTopAdvertisers(t *testing.T) {
	router, db := setupTestRouter(t)

	result := registerPublisher(t, router, "TechBlog", "techblog.com")
	token := result["token"].(string)
	pubID := result["id"].(string)

	db.LogAuctionReturningIDWithPublisher("intent-a", "adv-1", 5.00, "USD", 5, pubID)
	db.LogAuctionReturningIDWithPublisher("intent-b", "adv-2", 3.00, "USD", 3, pubID)

	req := httptest.NewRequest("GET", "/portal/publisher/top-advertisers?token="+token+"&limit=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	var resp []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 2 {
		t.Errorf("len = %d, want 2", len(resp))
	}
}

func TestAdRequestWithPublisherID(t *testing.T) {
	router, db, proxy := setupTestRouterWithProxy(t)

	result := registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)
	advID := result["id"].(string)
	registerPublisher(t, router, "TechBlog", "techblog.com")

	w := teeAdRequest(t, router, proxy,
		[]enclave.PositionSnapshot{
			{ID: advID, Name: "Adv1", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
		},
		[]enclave.BudgetSnapshot{
			{AdvertiserID: advID, Total: 1000, Spent: 0, Currency: "USD"},
		},
		"pub-1",
	)
	if w.Code != http.StatusOK {
		t.Fatalf("ad-request status = %d: %s", w.Code, w.Body.String())
	}

	// Verify the auction was logged with publisher_id
	auctions, total, err := db.GetAuctionsByPublisher("pub-1", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(auctions) != 1 {
		t.Errorf("len = %d, want 1", len(auctions))
	}
}

// --- Admin Auth Tests ---

func setupTestRouterWithPassword(t *testing.T, password string) (http.Handler, *platform.DB) {
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
		Registry:      registry,
		Budgets:       budgets,
		Engine:        engine,
		DB:            db,
		AdminPassword: password,
		TEEProxy:      proxy,
	})
	return router, db
}

func TestAdminAuthRejectsWithoutPassword(t *testing.T) {
	router, _ := setupTestRouterWithPassword(t, "secret")

	req := httptest.NewRequest("GET", "/admin/auctions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without password, got %d", w.Code)
	}
}

func TestAdminAuthRejectsWrongPassword(t *testing.T) {
	router, _ := setupTestRouterWithPassword(t, "secret")

	req := httptest.NewRequest("GET", "/admin/auctions", nil)
	req.Header.Set("X-Admin-Password", "wrong")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with wrong password, got %d", w.Code)
	}
}

func TestAdminAuthAcceptsCorrectPassword(t *testing.T) {
	router, _ := setupTestRouterWithPassword(t, "secret")

	registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)

	req := httptest.NewRequest("GET", "/admin/auctions", nil)
	req.Header.Set("X-Admin-Password", "secret")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with correct password, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminAuthPassthroughWhenEmpty(t *testing.T) {
	router, _ := setupTestRouter(t) // no password set

	registerAdvertiser(t, router, "Adv1", "intent one", 0.5, 2.0, 1000.0)

	req := httptest.NewRequest("GET", "/admin/auctions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 without password config, got %d", w.Code)
	}
}

func TestPublisherRegisterRequiresAdminPassword(t *testing.T) {
	router, _ := setupTestRouterWithPassword(t, "secret")

	body, _ := json.Marshal(map[string]interface{}{"name": "TechBlog", "domain": "techblog.com"})
	req := httptest.NewRequest("POST", "/publisher/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for publisher register without password, got %d", w.Code)
	}
}

// --- Create Publisher with Credentials Tests ---

func TestCreatePublisherWithCredentials(t *testing.T) {
	router, _ := setupTestRouterWithPassword(t, "secret")

	body, _ := json.Marshal(map[string]interface{}{
		"name": "TechBlog", "domain": "techblog.com",
		"email": "pub@test.com", "password": "pass123",
	})
	req := httptest.NewRequest("POST", "/admin/publishers", bytes.NewReader(body))
	req.Header.Set("X-Admin-Password", "secret")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if result["id"] == nil {
		t.Error("expected id in response")
	}
	if result["email"] != "pub@test.com" {
		t.Errorf("email = %v, want pub@test.com", result["email"])
	}
	if result["token"] == nil {
		t.Error("expected token in response")
	}
}

func TestCreatePublisherWithCredentialsMissingFields(t *testing.T) {
	router, _ := setupTestRouterWithPassword(t, "secret")

	body, _ := json.Marshal(map[string]interface{}{
		"name": "TechBlog", "domain": "techblog.com",
	})
	req := httptest.NewRequest("POST", "/admin/publishers", bytes.NewReader(body))
	req.Header.Set("X-Admin-Password", "secret")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing email/password, got %d", w.Code)
	}
}

// --- Publisher Login Tests ---

func TestPublisherLogin(t *testing.T) {
	router, _ := setupTestRouterWithPassword(t, "secret")

	// Create publisher with credentials
	body, _ := json.Marshal(map[string]interface{}{
		"name": "TechBlog", "domain": "techblog.com",
		"email": "pub@test.com", "password": "pass123",
	})
	req := httptest.NewRequest("POST", "/admin/publishers", bytes.NewReader(body))
	req.Header.Set("X-Admin-Password", "secret")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create publisher failed: %d: %s", w.Code, w.Body.String())
	}

	// Login
	body, _ = json.Marshal(map[string]interface{}{
		"email": "pub@test.com", "password": "pass123",
	})
	req = httptest.NewRequest("POST", "/publisher/login", bytes.NewReader(body))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if result["token"] == nil || result["token"] == "" {
		t.Error("expected token in login response")
	}
	if result["publisher_id"] == nil || result["publisher_id"] == "" {
		t.Error("expected publisher_id in login response")
	}
}

func TestPublisherLoginWrongPassword(t *testing.T) {
	router, _ := setupTestRouterWithPassword(t, "secret")

	body, _ := json.Marshal(map[string]interface{}{
		"name": "TechBlog", "domain": "techblog.com",
		"email": "pub@test.com", "password": "pass123",
	})
	req := httptest.NewRequest("POST", "/admin/publishers", bytes.NewReader(body))
	req.Header.Set("X-Admin-Password", "secret")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Login with wrong password
	body, _ = json.Marshal(map[string]interface{}{
		"email": "pub@test.com", "password": "wrong",
	})
	req = httptest.NewRequest("POST", "/publisher/login", bytes.NewReader(body))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password, got %d", w.Code)
	}
}

func TestPublisherLoginNonexistentEmail(t *testing.T) {
	router, _ := setupTestRouterWithPassword(t, "secret")

	body, _ := json.Marshal(map[string]interface{}{
		"email": "nonexistent@test.com", "password": "pass123",
	})
	req := httptest.NewRequest("POST", "/publisher/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for nonexistent email, got %d", w.Code)
	}
}
