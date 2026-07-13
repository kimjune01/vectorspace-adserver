package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vectorspace/platform"
	"vectorspace/tee"
)

// textSidecar returns a fake embedding sidecar whose output depends on the
// input text, so identical text yields identical vectors and different text
// yields different vectors. This is the property the σ = 0 keyword limit
// relies on (deterministic embedder), which the seed-by-batch-index fake
// in handler_test.go does not provide for single-text requests.
func textSidecar(embDim int) *httptest.Server {
	makeEmb := func(text string) []float64 {
		seed := 0
		for _, c := range text {
			seed = (seed*31 + int(c)) % 9973
		}
		emb := make([]float64, embDim)
		for d := range emb {
			emb[d] = float64(seed%97+1) * 0.01 * float64(d+1)
		}
		return emb
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Text  string   `json:"text"`
			Texts []string `json:"texts"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if req.Texts != nil {
			embeddings := make([][]float64, len(req.Texts))
			for i, t := range req.Texts {
				embeddings[i] = makeEmb(t)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"embeddings": embeddings, "dim": embDim})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"embedding": makeEmb(req.Text), "dim": embDim})
		}
	}))
}

func setupKeywordTestRouter(t *testing.T) (http.Handler, *platform.DB) {
	t.Helper()
	sidecar := textSidecar(3)
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
	return router, db
}

func postJSON(t *testing.T, router http.Handler, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", path, bytes.NewReader(b))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestRegisterKeywordGroup(t *testing.T) {
	router, _ := setupKeywordTestRouter(t)

	w := postJSON(t, router, "/advertiser/register", map[string]interface{}{
		"name":      "Knee Clinic",
		"keywords":  []string{"knee brace", "acl surgery", "meniscus tear"},
		"bid_price": 2.0,
		"budget":    100.0,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		BudgetID  string `json:"budget_id"`
		Token     string `json:"token"`
		Positions []struct {
			ID       string  `json:"id"`
			Keyword  string  `json:"keyword"`
			Sigma    float64 `json:"sigma"`
			BudgetID string  `json:"budget_id"`
		} `json:"positions"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Positions) != 3 {
		t.Fatalf("positions = %d, want 3", len(resp.Positions))
	}
	for _, p := range resp.Positions {
		if p.Sigma != 0 {
			t.Errorf("position %s sigma = %v, want 0 (keyword default)", p.ID, p.Sigma)
		}
		if p.BudgetID != resp.BudgetID {
			t.Errorf("position %s budget_id = %q, want group head %q", p.ID, p.BudgetID, resp.BudgetID)
		}
	}

	// One budget for the whole group, held by the head.
	req := httptest.NewRequest("GET", "/budget/"+resp.BudgetID, nil)
	bw := httptest.NewRecorder()
	router.ServeHTTP(bw, req)
	if bw.Code != http.StatusOK {
		t.Fatalf("budget lookup: status %d", bw.Code)
	}
	var budget struct {
		Total float64 `json:"total"`
	}
	json.NewDecoder(bw.Body).Decode(&budget)
	if budget.Total != 100.0 {
		t.Errorf("budget total = %v, want 100", budget.Total)
	}
}

func TestRegisterRejectsIntentWithZeroSigma(t *testing.T) {
	router, _ := setupKeywordTestRouter(t)

	w := postJSON(t, router, "/advertiser/register", map[string]interface{}{
		"name":      "Vague Adv",
		"intent":    "help with knees",
		"sigma":     0,
		"bid_price": 2.0,
		"budget":    100.0,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400 (σ=0 intent position is a footgun)", w.Code)
	}
}

func TestRegisterRejectsIntentAndKeywordsTogether(t *testing.T) {
	router, _ := setupKeywordTestRouter(t)

	w := postJSON(t, router, "/advertiser/register", map[string]interface{}{
		"name":      "Confused Adv",
		"intent":    "help with knees",
		"keywords":  []string{"knee brace"},
		"bid_price": 2.0,
		"budget":    100.0,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", w.Code)
	}
}

// TestOpenRTBKeywordRouting is the end-to-end claim of the essay: a keyword
// campaign imported as σ = 0 positions wins at its exact keyword and matches
// nothing else, through a standard OpenRTB 2.5 bid request.
func TestOpenRTBKeywordRouting(t *testing.T) {
	router, _ := setupKeywordTestRouter(t)

	// Import a keyword campaign.
	w := postJSON(t, router, "/advertiser/register", map[string]interface{}{
		"name":      "Knee Clinic",
		"keywords":  []string{"knee brace"},
		"bid_price": 2.0,
		"budget":    100.0,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register: status %d: %s", w.Code, w.Body.String())
	}
	var reg struct {
		BudgetID  string `json:"budget_id"`
		Positions []struct {
			ID string `json:"id"`
		} `json:"positions"`
	}
	json.NewDecoder(w.Body).Decode(&reg)

	// A broad vector advertiser for competition.
	w = postJSON(t, router, "/advertiser/register", map[string]interface{}{
		"name":      "General Ortho",
		"intent":    "orthopedic care for joint pain",
		"sigma":     5.0,
		"bid_price": 1.0,
		"budget":    100.0,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register vector adv: status %d: %s", w.Code, w.Body.String())
	}

	// Exact keyword in the ORTB content.keywords field: keyword bid must win.
	w = postJSON(t, router, "/openrtb2/auction", map[string]interface{}{
		"id":   "req-1",
		"imp":  []map[string]interface{}{{"id": "1"}},
		"site": map[string]interface{}{"content": map[string]interface{}{"keywords": "knee brace"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("ortb auction: status %d: %s", w.Code, w.Body.String())
	}
	var ortb struct {
		ID      string `json:"id"`
		SeatBid []struct {
			Seat string `json:"seat"`
			Bid  []struct {
				ID    string  `json:"id"`
				ImpID string  `json:"impid"`
				Price float64 `json:"price"`
			} `json:"bid"`
		} `json:"seatbid"`
	}
	json.NewDecoder(w.Body).Decode(&ortb)
	if ortb.ID != "req-1" {
		t.Errorf("response id = %q, want req-1", ortb.ID)
	}
	if len(ortb.SeatBid) != 1 || len(ortb.SeatBid[0].Bid) != 1 {
		t.Fatalf("seatbid shape: %+v", ortb.SeatBid)
	}
	winner := ortb.SeatBid[0].Bid[0]
	if winner.ID != reg.Positions[0].ID {
		t.Errorf("winner = %q, want keyword position %q", winner.ID, reg.Positions[0].ID)
	}
	if winner.Price <= 0 {
		t.Errorf("price = %v, want > 0", winner.Price)
	}

	// Unrelated query: the σ = 0 keyword scores -Inf, the broad vector
	// advertiser picks it up instead. The keyword never leaks off its point.
	w = postJSON(t, router, "/openrtb2/auction", map[string]interface{}{
		"id":   "req-2",
		"imp":  []map[string]interface{}{{"id": "1"}},
		"site": map[string]interface{}{"content": map[string]interface{}{"keywords": "yoga retreat bali"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("ortb auction 2: status %d: %s", w.Code, w.Body.String())
	}
	var ortb2 struct {
		SeatBid []struct {
			Bid []struct {
				ID string `json:"id"`
			} `json:"bid"`
		} `json:"seatbid"`
	}
	json.NewDecoder(w.Body).Decode(&ortb2)
	if len(ortb2.SeatBid) != 1 || ortb2.SeatBid[0].Bid[0].ID == reg.Positions[0].ID {
		t.Errorf("keyword position won off its point: %+v", ortb2.SeatBid)
	}
}

// TestClickChargesGroupHeadBudget verifies a click on a keyword-group member
// draws down the shared budget held by the group head.
func TestClickChargesGroupHeadBudget(t *testing.T) {
	router, _ := setupKeywordTestRouter(t)

	w := postJSON(t, router, "/advertiser/register", map[string]interface{}{
		"name":      "Knee Clinic",
		"keywords":  []string{"knee brace", "acl surgery"},
		"bid_price": 2.0,
		"budget":    100.0,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register: status %d: %s", w.Code, w.Body.String())
	}
	var reg struct {
		BudgetID  string `json:"budget_id"`
		Positions []struct {
			ID string `json:"id"`
		} `json:"positions"`
	}
	json.NewDecoder(w.Body).Decode(&reg)
	memberID := reg.Positions[1].ID // not the head

	// Competing advertiser so VCG payment is positive.
	w = postJSON(t, router, "/advertiser/register", map[string]interface{}{
		"name":      "Rival Clinic",
		"keywords":  []string{"acl surgery"},
		"bid_price": 1.5,
		"budget":    100.0,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register rival: status %d: %s", w.Code, w.Body.String())
	}

	// Win an auction at the member's keyword.
	w = postJSON(t, router, "/openrtb2/auction", map[string]interface{}{
		"id":   "req-1",
		"imp":  []map[string]interface{}{{"id": "1"}},
		"site": map[string]interface{}{"content": map[string]interface{}{"keywords": "acl surgery"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("ortb auction: status %d: %s", w.Code, w.Body.String())
	}
	var ortb struct {
		SeatBid []struct {
			Bid []struct {
				ID  string `json:"id"`
				Ext struct {
					Vectorspace struct {
						AuctionID int64 `json:"auction_id"`
					} `json:"vectorspace"`
				} `json:"ext"`
			} `json:"bid"`
		} `json:"seatbid"`
	}
	json.NewDecoder(w.Body).Decode(&ortb)
	winner := ortb.SeatBid[0].Bid[0]
	if winner.ID != memberID {
		t.Fatalf("winner = %q, want member %q", winner.ID, memberID)
	}
	if winner.Ext.Vectorspace.AuctionID == 0 {
		t.Fatal("auction_id missing from bid ext")
	}

	// Click the member's placement.
	w = postJSON(t, router, "/event/click", map[string]interface{}{
		"auction_id":    winner.Ext.Vectorspace.AuctionID,
		"advertiser_id": memberID,
		"user_id":       "user-1",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("click: status %d: %s", w.Code, w.Body.String())
	}

	// The head's budget was charged; the member has no budget of its own.
	req := httptest.NewRequest("GET", "/budget/"+reg.BudgetID, nil)
	bw := httptest.NewRecorder()
	router.ServeHTTP(bw, req)
	var budget struct {
		Total float64 `json:"total"`
		Spent float64 `json:"spent"`
	}
	json.NewDecoder(bw.Body).Decode(&budget)
	if budget.Spent <= 0 {
		t.Errorf("head budget spent = %v, want > 0 after member click", budget.Spent)
	}
}

// TestUpdateSigmaSemantics: omitted sigma keeps the current value; an
// explicit 0 sets the keyword limit.
func TestUpdateSigmaSemantics(t *testing.T) {
	router, _ := setupKeywordTestRouter(t)

	reg := registerAdvertiserOn(t, router, "Adv", "knee rehab", 0.5, 2.0, 100.0)
	id := reg["id"].(string)

	// Omitted sigma: unchanged.
	b, _ := json.Marshal(map[string]interface{}{"bid_price": 3.0})
	req := httptest.NewRequest("PUT", "/advertiser/"+id, bytes.NewReader(b))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update: status %d: %s", w.Code, w.Body.String())
	}
	var pos struct {
		Sigma float64 `json:"sigma"`
	}
	json.NewDecoder(w.Body).Decode(&pos)
	if pos.Sigma != 0.5 {
		t.Errorf("sigma after omitted update = %v, want 0.5", pos.Sigma)
	}

	// Explicit zero: set to the keyword limit.
	b, _ = json.Marshal(map[string]interface{}{"sigma": 0.0})
	req = httptest.NewRequest("PUT", "/advertiser/"+id, bytes.NewReader(b))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update: status %d: %s", w.Code, w.Body.String())
	}
	json.NewDecoder(w.Body).Decode(&pos)
	if pos.Sigma != 0 {
		t.Errorf("sigma after explicit-zero update = %v, want 0", pos.Sigma)
	}
}

func registerAdvertiserOn(t *testing.T, router http.Handler, name, intent string, sigma, bidPrice, budget float64) map[string]interface{} {
	t.Helper()
	w := postJSON(t, router, "/advertiser/register", map[string]interface{}{
		"name":      name,
		"intent":    intent,
		"sigma":     sigma,
		"bid_price": bidPrice,
		"budget":    budget,
		"currency":  "USD",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register failed: status %d, body: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	return result
}

// TestOpenRTBCommaSeparatedKeywords: ORTB keywords fields are comma-separated
// lists; a σ = 0 import must match when ANY listed keyword is its exact text.
func TestOpenRTBCommaSeparatedKeywords(t *testing.T) {
	router, _ := setupKeywordTestRouter(t)

	w := postJSON(t, router, "/advertiser/register", map[string]interface{}{
		"name":      "Knee Clinic",
		"keywords":  []string{"knee brace"},
		"bid_price": 2.0,
		"budget":    100.0,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register: status %d: %s", w.Code, w.Body.String())
	}
	var reg struct {
		Positions []struct {
			ID string `json:"id"`
		} `json:"positions"`
	}
	json.NewDecoder(w.Body).Decode(&reg)

	w = postJSON(t, router, "/openrtb2/auction", map[string]interface{}{
		"id":   "req-1",
		"imp":  []map[string]interface{}{{"id": "1"}},
		"site": map[string]interface{}{"content": map[string]interface{}{"keywords": "orthopedics, knee brace , recovery"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("ortb auction: status %d: %s", w.Code, w.Body.String())
	}
	var ortb struct {
		SeatBid []struct {
			Bid []struct {
				ID  string `json:"id"`
				AdM string `json:"adm"`
			} `json:"bid"`
		} `json:"seatbid"`
	}
	json.NewDecoder(w.Body).Decode(&ortb)
	if len(ortb.SeatBid) != 1 || ortb.SeatBid[0].Bid[0].ID != reg.Positions[0].ID {
		t.Fatalf("winner: %+v, want keyword position (matched via list element)", ortb.SeatBid)
	}
	if ortb.SeatBid[0].Bid[0].AdM == "" {
		t.Error("adm missing: a standard renderer has nothing to display")
	}
}

// TestOpenRTBTestModeNotBillable: test=1 requests are never logged, so they
// can never be settled.
func TestOpenRTBTestModeNotBillable(t *testing.T) {
	router, db := setupKeywordTestRouter(t)

	w := postJSON(t, router, "/advertiser/register", map[string]interface{}{
		"name":      "Knee Clinic",
		"keywords":  []string{"knee brace"},
		"bid_price": 2.0,
		"budget":    100.0,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register: status %d: %s", w.Code, w.Body.String())
	}

	w = postJSON(t, router, "/openrtb2/auction", map[string]interface{}{
		"id":   "req-1",
		"test": 1,
		"imp":  []map[string]interface{}{{"id": "1"}},
		"site": map[string]interface{}{"content": map[string]interface{}{"keywords": "knee brace"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("ortb auction: status %d: %s", w.Code, w.Body.String())
	}
	var ortb struct {
		SeatBid []struct {
			Bid []struct {
				Ext struct {
					Vectorspace struct {
						AuctionID int64 `json:"auction_id"`
					} `json:"vectorspace"`
				} `json:"ext"`
			} `json:"bid"`
		} `json:"seatbid"`
	}
	json.NewDecoder(w.Body).Decode(&ortb)
	if ortb.SeatBid[0].Bid[0].Ext.Vectorspace.AuctionID != 0 {
		t.Error("test request produced a logged (billable) auction")
	}
	stats, err := db.GetStats()
	if err == nil && stats.AuctionCount != 0 {
		t.Errorf("auction_count = %d after test request, want 0", stats.AuctionCount)
	}
}
