package platform

import (
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

func TestRegistrySetDB(t *testing.T) {
	db := newMemoryDB(t)

	// Insert rows directly in DB
	pos1 := &Position{ID: "adv-1", Name: "First", Intent: "intent one", Embedding: []float64{0.1, 0.2}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"}
	pos2 := &Position{ID: "adv-2", Name: "Second", Intent: "intent two", Embedding: []float64{0.3, 0.4}, Sigma: 0.6, BidPrice: 3.0, Currency: "USD"}
	if err := db.InsertAdvertiser(pos1, 100.0); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertAdvertiser(pos2, 200.0); err != nil {
		t.Fatal(err)
	}

	sidecar := fakeSidecar(2)
	defer sidecar.Close()
	embedder := NewEmbedder(sidecar.URL)
	registry := NewPositionRegistry(embedder)

	if err := registry.SetDB(db); err != nil {
		t.Fatalf("SetDB: %v", err)
	}

	all := registry.GetAll()
	if len(all) != 2 {
		t.Fatalf("GetAll len = %d, want 2", len(all))
	}

	got := registry.Get("adv-1")
	if got == nil {
		t.Fatal("Get(adv-1) returned nil")
	}
	if got.Name != "First" {
		t.Errorf("Name = %q, want %q", got.Name, "First")
	}
}

func TestRegisterWithBudgetPersists(t *testing.T) {
	db := newMemoryDB(t)
	sidecar := fakeSidecar(3)
	defer sidecar.Close()
	embedder := NewEmbedder(sidecar.URL)
	registry := NewPositionRegistry(embedder)
	if err := registry.SetDB(db); err != nil {
		t.Fatal(err)
	}

	pos, err := registry.RegisterWithBudget("Test Adv", "test intent", 0.5, 2.0, 500.0, "USD", "")
	if err != nil {
		t.Fatalf("RegisterWithBudget: %v", err)
	}

	// Verify persisted in DB
	got, err := db.GetAdvertiser(pos.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("advertiser not found in DB after RegisterWithBudget")
	}
	if got.Name != "Test Adv" {
		t.Errorf("DB Name = %q, want %q", got.Name, "Test Adv")
	}
	if got.Intent != "test intent" {
		t.Errorf("DB Intent = %q, want %q", got.Intent, "test intent")
	}

	// Verify budget persisted
	b, _ := db.GetBudget(pos.ID)
	if b == nil {
		t.Fatal("budget not found in DB")
	}
	if b.Total != 500.0 {
		t.Errorf("budget Total = %f, want 500.0", b.Total)
	}
}

func TestRegistryUpdateNameOnly(t *testing.T) {
	db := newMemoryDB(t)
	sidecar := fakeSidecar(3)
	defer sidecar.Close()
	embedder := NewEmbedder(sidecar.URL)
	registry := NewPositionRegistry(embedder)
	if err := registry.SetDB(db); err != nil {
		t.Fatal(err)
	}

	pos, err := registry.RegisterWithBudget("Original", "test intent", 0.5, 2.0, 100.0, "USD", "")
	if err != nil {
		t.Fatalf("RegisterWithBudget: %v", err)
	}
	origEmb := make([]float64, len(pos.Embedding))
	copy(origEmb, pos.Embedding)

	// Update name only (empty intent = keep existing)
	updated, err := registry.Update(pos.ID, "New Name", "", "", 0, 0)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("Name = %q, want %q", updated.Name, "New Name")
	}
	if updated.Intent != "test intent" {
		t.Errorf("Intent should be unchanged, got %q", updated.Intent)
	}
	// Embedding should be unchanged (no re-embed)
	for i, v := range origEmb {
		if updated.Embedding[i] != v {
			t.Errorf("embedding[%d] changed after name-only update", i)
		}
	}

	// Verify DB updated
	got, err := db.GetAdvertiser(pos.ID)
	if err != nil {
		t.Fatalf("GetAdvertiser: %v", err)
	}
	if got == nil {
		t.Fatal("GetAdvertiser returned nil")
	}
	if got.Name != "New Name" {
		t.Errorf("DB Name = %q, want %q", got.Name, "New Name")
	}
}

func TestRegistryUpdateIntentReembeds(t *testing.T) {
	db := newMemoryDB(t)
	sidecar := fakeSidecar(3)
	defer sidecar.Close()
	embedder := NewEmbedder(sidecar.URL)
	registry := NewPositionRegistry(embedder)
	if err := registry.SetDB(db); err != nil {
		t.Fatal(err)
	}

	pos, err := registry.RegisterWithBudget("Adv", "original intent", 0.5, 2.0, 100.0, "USD", "")
	if err != nil {
		t.Fatalf("RegisterWithBudget: %v", err)
	}

	// Update with new intent — should trigger re-embed
	updated, err := registry.Update(pos.ID, "", "new intent", "", 0, 0)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Intent != "new intent" {
		t.Errorf("Intent = %q, want %q", updated.Intent, "new intent")
	}
	// Embedding should have been re-generated (fakeSidecar always returns seed=0 embedding)
	if len(updated.Embedding) != 3 {
		t.Errorf("embedding len = %d, want 3", len(updated.Embedding))
	}
}

func TestRegistryDelete(t *testing.T) {
	db := newMemoryDB(t)
	sidecar := fakeSidecar(3)
	defer sidecar.Close()
	embedder := NewEmbedder(sidecar.URL)
	registry := NewPositionRegistry(embedder)
	if err := registry.SetDB(db); err != nil {
		t.Fatal(err)
	}

	pos, err := registry.RegisterWithBudget("ToDelete", "delete me", 0.5, 2.0, 100.0, "USD", "")
	if err != nil {
		t.Fatalf("RegisterWithBudget: %v", err)
	}

	if err := registry.Delete(pos.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify removed from cache
	if registry.Get(pos.ID) != nil {
		t.Error("position still in cache after delete")
	}

	// Verify removed from DB
	got, err := db.GetAdvertiser(pos.ID)
	if got != nil {
		t.Error("position still in DB after delete")
	}
}

func TestRegistryDeleteNotFound(t *testing.T) {
	sidecar := fakeSidecar(3)
	defer sidecar.Close()
	embedder := NewEmbedder(sidecar.URL)
	registry := NewPositionRegistry(embedder)

	err := registry.Delete("nonexistent")
	if err == nil {
		t.Error("expected error for deleting nonexistent advertiser")
	}
}

func TestBudgetTrackerSetDB(t *testing.T) {
	db := newMemoryDB(t)

	// Insert some advertisers with budgets directly in DB
	pos1 := &Position{ID: "adv-1", Name: "A", Intent: "i", Embedding: []float64{0.1}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"}
	pos2 := &Position{ID: "adv-2", Name: "B", Intent: "j", Embedding: []float64{0.2}, Sigma: 0.5, BidPrice: 3.0, Currency: "EUR"}
	db.InsertAdvertiser(pos1, 100.0)
	db.InsertAdvertiser(pos2, 200.0)

	// Charge adv-1 in DB to create non-zero spent
	db.Charge("adv-1", 25.0)

	bt := NewBudgetTracker()
	if err := bt.SetDB(db); err != nil {
		t.Fatalf("SetDB: %v", err)
	}

	info1 := bt.GetInfo("adv-1")
	if info1 == nil {
		t.Fatal("adv-1 budget not loaded")
	}
	if info1.Total != 100.0 {
		t.Errorf("adv-1 Total = %f, want 100.0", info1.Total)
	}
	if info1.Spent != 25.0 {
		t.Errorf("adv-1 Spent = %f, want 25.0", info1.Spent)
	}
	if info1.Remaining != 75.0 {
		t.Errorf("adv-1 Remaining = %f, want 75.0", info1.Remaining)
	}
	if info1.Currency != "USD" {
		t.Errorf("adv-1 Currency = %q, want USD", info1.Currency)
	}

	info2 := bt.GetInfo("adv-2")
	if info2 == nil {
		t.Fatal("adv-2 budget not loaded")
	}
	if info2.Total != 200.0 {
		t.Errorf("adv-2 Total = %f, want 200.0", info2.Total)
	}
	if info2.Currency != "EUR" {
		t.Errorf("adv-2 Currency = %q, want EUR", info2.Currency)
	}
}

func TestBudgetTrackerChargeWithDB(t *testing.T) {
	db := newMemoryDB(t)
	pos := &Position{ID: "adv-1", Name: "A", Intent: "i", Embedding: []float64{0.1}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"}
	db.InsertAdvertiser(pos, 100.0)

	bt := NewBudgetTracker()
	bt.SetDB(db)
	bt.Set("adv-1", 100.0, "USD")

	// Charge in-memory + DB
	ok := bt.Charge("adv-1", 30.0)
	if !ok {
		t.Fatal("Charge returned false")
	}

	// Verify in-memory state
	info := bt.GetInfo("adv-1")
	if info.Spent != 30.0 {
		t.Errorf("in-memory Spent = %f, want 30.0", info.Spent)
	}

	// Verify DB state
	b, _ := db.GetBudget("adv-1")
	if b.Spent != 30.0 {
		t.Errorf("DB Spent = %f, want 30.0", b.Spent)
	}
}

func TestBudgetTrackerChargeInsufficientFunds(t *testing.T) {
	bt := NewBudgetTracker()
	bt.Set("adv-1", 10.0, "USD")

	ok := bt.Charge("adv-1", 15.0)
	if ok {
		t.Fatal("Charge should return false for insufficient funds")
	}

	info := bt.GetInfo("adv-1")
	if info.Spent != 0 {
		t.Errorf("Spent = %f, want 0 (no mutation)", info.Spent)
	}
}

func TestBudgetTrackerCanAfford(t *testing.T) {
	bt := NewBudgetTracker()
	bt.Set("adv-1", 10.0, "USD")

	if !bt.CanAfford("adv-1", 10.0) {
		t.Error("should be able to afford exactly 10.0")
	}
	if bt.CanAfford("adv-1", 10.01) {
		t.Error("should not afford 10.01")
	}
	if bt.CanAfford("nonexistent", 1.0) {
		t.Error("should not afford for nonexistent advertiser")
	}
}

func TestBudgetTrackerDelete(t *testing.T) {
	bt := NewBudgetTracker()
	bt.Set("adv-1", 100.0, "USD")
	bt.Delete("adv-1")

	info := bt.GetInfo("adv-1")
	if info != nil {
		t.Error("expected nil after delete")
	}
}

// --- Tau (relevance gate) tests ---

// setupEngineWithPositions creates an AuctionEngine with manually-placed positions.
// The fakeSidecar always returns [0.01, 0.02, 0.03] for query embeddings (seed=0, dim=3).
func setupEngineWithPositions(t *testing.T, positions []*Position, budgetPerAdv float64) *AuctionEngine {
	t.Helper()
	sidecar := fakeSidecar(3)
	t.Cleanup(sidecar.Close)

	embedder := NewEmbedder(sidecar.URL)
	registry := NewPositionRegistry(embedder)
	budgets := NewBudgetTracker()

	for _, pos := range positions {
		registry.mu.Lock()
		registry.positions[pos.ID] = pos
		registry.mu.Unlock()
		budgets.Set(pos.ID, budgetPerAdv, pos.Currency)
	}

	return NewAuctionEngine(registry, budgets, embedder)
}

func TestRunAdRequestWithTauFilters(t *testing.T) {
	// Query embedding from fakeSidecar(3) is always [0.01, 0.02, 0.03].
	// Close advertiser: embedding = [0.01, 0.02, 0.03] → dist² = 0
	// Far advertiser:   embedding = [1.0, 1.0, 1.0]   → dist² = (0.99² + 0.98² + 0.97²) ≈ 2.8814
	close := &Position{ID: "close", Name: "Close Ad", Intent: "close", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"}
	far := &Position{ID: "far", Name: "Far Ad", Intent: "far", Embedding: []float64{1.0, 1.0, 1.0}, Sigma: 0.5, BidPrice: 5.0, Currency: "USD"}

	engine := setupEngineWithPositions(t, []*Position{close, far}, 1000.0)

	// tau=1.0 should only admit the close ad (dist²=0 < 1.0) but not the far ad (dist²≈2.88 > 1.0)
	resp, err := engine.RunAdRequestWithTau("test query", 1.0)
	if err != nil {
		t.Fatalf("RunAdRequestWithTau: %v", err)
	}
	if resp.BidCount != 1 {
		t.Errorf("bid_count = %d, want 1 (only close ad should pass tau)", resp.BidCount)
	}
	if resp.Winner.ID != "close" {
		t.Errorf("winner = %q, want %q", resp.Winner.ID, "close")
	}
}

func TestRunAdRequestWithTauPassesAll(t *testing.T) {
	close := &Position{ID: "close", Name: "Close Ad", Intent: "close", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"}
	far := &Position{ID: "far", Name: "Far Ad", Intent: "far", Embedding: []float64{1.0, 1.0, 1.0}, Sigma: 0.5, BidPrice: 5.0, Currency: "USD"}

	engine := setupEngineWithPositions(t, []*Position{close, far}, 1000.0)

	// tau=0 means no filtering — both ads pass
	resp, err := engine.RunAdRequestWithTau("test query", 0)
	if err != nil {
		t.Fatalf("RunAdRequestWithTau: %v", err)
	}
	if resp.BidCount != 2 {
		t.Errorf("bid_count = %d, want 2 (tau=0 should pass all)", resp.BidCount)
	}
}

func TestRunAdRequestWithTauFiltersAll(t *testing.T) {
	// Both ads are far from the query
	far1 := &Position{ID: "far1", Name: "Far 1", Intent: "far", Embedding: []float64{1.0, 1.0, 1.0}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"}
	far2 := &Position{ID: "far2", Name: "Far 2", Intent: "far", Embedding: []float64{2.0, 2.0, 2.0}, Sigma: 0.5, BidPrice: 3.0, Currency: "USD"}

	engine := setupEngineWithPositions(t, []*Position{far1, far2}, 1000.0)

	// tau=0.01 — nothing passes
	_, err := engine.RunAdRequestWithTau("test query", 0.01)
	if err == nil {
		t.Fatal("expected error when tau filters all bidders")
	}
}

func TestRunAdRequestWithTauRanksByBid(t *testing.T) {
	// Two ads both within tau, but different bids.
	// The higher bidder should win (ranking by log(b) among survivors).
	cheap := &Position{ID: "cheap", Name: "Cheap", Intent: "close", Embedding: []float64{0.01, 0.02, 0.03}, Sigma: 0.5, BidPrice: 1.0, Currency: "USD"}
	expensive := &Position{ID: "expensive", Name: "Expensive", Intent: "close", Embedding: []float64{0.02, 0.03, 0.04}, Sigma: 0.5, BidPrice: 10.0, Currency: "USD"}

	engine := setupEngineWithPositions(t, []*Position{cheap, expensive}, 1000.0)

	// tau=1.0 — both pass (both very close to query)
	resp, err := engine.RunAdRequestWithTau("test query", 1.0)
	if err != nil {
		t.Fatalf("RunAdRequestWithTau: %v", err)
	}
	if resp.BidCount != 2 {
		t.Errorf("bid_count = %d, want 2", resp.BidCount)
	}
	if resp.Winner.ID != "expensive" {
		t.Errorf("winner = %q, want %q (higher bid should win)", resp.Winner.ID, "expensive")
	}
}

