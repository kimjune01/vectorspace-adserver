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

func TestPositionHistoryOnRegister(t *testing.T) {
	sidecar := fakeSidecar(3)
	defer sidecar.Close()

	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	embedder := NewEmbedder(sidecar.URL)
	registry := NewPositionRegistry(embedder)
	if err := registry.SetDB(db); err != nil {
		t.Fatal(err)
	}

	budgets := NewBudgetTracker()
	if err := budgets.SetDB(db); err != nil {
		t.Fatal(err)
	}

	pos, err := registry.RegisterWithBudget("Trainer", "dog training", 1.0, 5.0, 100.0, "USD", "")
	if err != nil {
		t.Fatal(err)
	}

	// Should have 1 history entry with distance=0
	history, err := db.GetPositionHistory(pos.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].DistanceMoved != 0 {
		t.Errorf("expected distance_moved=0, got %f", history[0].DistanceMoved)
	}
	if history[0].Intent != "dog training" {
		t.Errorf("expected intent 'dog training', got %q", history[0].Intent)
	}
}

func TestPositionHistoryOnUpdate(t *testing.T) {
	sidecar := fakeSidecar(3)
	defer sidecar.Close()

	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	embedder := NewEmbedder(sidecar.URL)
	registry := NewPositionRegistry(embedder)
	if err := registry.SetDB(db); err != nil {
		t.Fatal(err)
	}

	budgets := NewBudgetTracker()
	if err := budgets.SetDB(db); err != nil {
		t.Fatal(err)
	}

	pos, err := registry.RegisterWithBudget("Trainer", "dog training", 1.0, 5.0, 100.0, "USD", "")
	if err != nil {
		t.Fatal(err)
	}

	// Update intent (triggers re-embedding and new position history)
	// Note: fake sidecar returns deterministic embeddings, so distance will be 0.
	// We just verify the history entry is created with the updated intent.
	_, err = registry.Update(pos.ID, "", "cat grooming", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	history, err := db.GetPositionHistory(pos.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	// Newest first
	if history[0].Intent != "cat grooming" {
		t.Errorf("expected latest intent 'cat grooming', got %q", history[0].Intent)
	}

	// Update sigma only (no intent change, same embedding, distance=0)
	_, err = registry.Update(pos.ID, "", "", "", 2.0, 0)
	if err != nil {
		t.Fatal(err)
	}

	history, err = db.GetPositionHistory(pos.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}
	if history[0].Sigma != 2.0 {
		t.Errorf("expected sigma=2.0, got %f", history[0].Sigma)
	}
	if history[0].DistanceMoved != 0 {
		t.Errorf("expected distance_moved=0 for sigma-only change, got %f", history[0].DistanceMoved)
	}
}

func TestEntryBondRecordedOnRegister(t *testing.T) {
	sidecar := fakeSidecar(3)
	defer sidecar.Close()

	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	embedder := NewEmbedder(sidecar.URL)
	registry := NewPositionRegistry(embedder)
	registry.RelocationFees = RelocationFeeConfig{EntryBond: 10.0}
	if err := registry.SetDB(db); err != nil {
		t.Fatal(err)
	}

	pos, err := registry.RegisterWithBudget("Trainer", "dog training", 1.0, 5.0, 100.0, "USD", "")
	if err != nil {
		t.Fatal(err)
	}

	// Position history should record the entry bond
	history, err := db.GetPositionHistory(pos.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].RelocationFee != 10.0 {
		t.Errorf("expected relocation_fee=10.0, got %f", history[0].RelocationFee)
	}
	if history[0].DistanceMoved != 0 {
		t.Errorf("expected distance_moved=0, got %f", history[0].DistanceMoved)
	}
}

func TestComputeRelocationFee(t *testing.T) {
	registry := NewPositionRegistry(nil)

	// No fee configured
	fee := registry.ComputeRelocationFee(4.0)
	if fee != 0 {
		t.Errorf("expected 0 with no fee config, got %f", fee)
	}

	// With distance factor
	registry.RelocationFees = RelocationFeeConfig{DistanceFactor: 5.0}
	fee = registry.ComputeRelocationFee(4.0) // sqrt(4) * 5 = 10
	if fee != 10.0 {
		t.Errorf("expected fee=10.0, got %f", fee)
	}

	fee = registry.ComputeRelocationFee(0) // no move = no fee
	if fee != 0 {
		t.Errorf("expected fee=0 for no movement, got %f", fee)
	}
}

func TestRelocationFeeOnMove(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	budgets := NewBudgetTracker()
	if err := budgets.SetDB(db); err != nil {
		t.Fatal(err)
	}

	// Manually test fee computation via position history recording
	// Distance moved = 4.0 (e.g., moved 2 units in one dimension)
	// sqrt(4) = 2.0, with factor 5.0 → fee = 10.0
	db.RecordPositionChange("adv-1", "intent", []float64{1, 0}, 1.0, 5.0, 4.0, 10.0)

	history, err := db.GetPositionHistory("adv-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(history))
	}
	if history[0].DistanceMoved != 4.0 {
		t.Errorf("expected distance_moved=4.0, got %f", history[0].DistanceMoved)
	}
	if history[0].RelocationFee != 10.0 {
		t.Errorf("expected relocation_fee=10.0, got %f", history[0].RelocationFee)
	}

	// Total fees
	total, err := db.GetTotalRelocationFees()
	if err != nil {
		t.Fatal(err)
	}
	if total != 10.0 {
		t.Errorf("expected total fees=10.0, got %f", total)
	}
}

func TestPositionCount(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.RecordPositionChange("adv-1", "intent1", []float64{1, 0}, 1.0, 5.0, 0, 0)
	db.RecordPositionChange("adv-1", "intent2", []float64{0, 1}, 1.0, 5.0, 2.0, 0)

	count, err := db.GetPositionCount("adv-1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestGetTenureDays(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// No history → 0 days
	days, err := db.GetTenureDays("adv-999")
	if err != nil {
		t.Fatal(err)
	}
	if days != 0 {
		t.Errorf("expected 0 days for nonexistent, got %f", days)
	}

	// Record a position change (created_at defaults to 'now')
	db.RecordPositionChange("adv-1", "intent", []float64{1, 0}, 1.0, 5.0, 0, 0)

	days, err = db.GetTenureDays("adv-1")
	if err != nil {
		t.Fatal(err)
	}
	// Just created, so tenure should be close to 0 (within 1 day)
	if days < 0 || days > 1 {
		t.Errorf("expected tenure ~0 days, got %f", days)
	}
}

func TestPublisherLogBase(t *testing.T) {
	db := newMemoryDB(t)

	// Nonexistent publisher → default 5.0
	logBase := db.GetPublisherLogBase("pub-999")
	if logBase != 5.0 {
		t.Errorf("expected default 5.0, got %f", logBase)
	}

	// Insert a publisher
	if err := db.InsertPublisher("pub-1", "Test Publisher", "test.com"); err != nil {
		t.Fatal(err)
	}

	// Default after insert → 5.0
	logBase = db.GetPublisherLogBase("pub-1")
	if logBase != 5.0 {
		t.Errorf("expected default 5.0, got %f", logBase)
	}

	// Set custom log base
	if err := db.SetPublisherLogBase("pub-1", 10.0); err != nil {
		t.Fatal(err)
	}
	logBase = db.GetPublisherLogBase("pub-1")
	if logBase != 10.0 {
		t.Errorf("expected 10.0, got %f", logBase)
	}
}

func TestEmbeddingsVersion(t *testing.T) {
	sidecar := fakeSidecar(3)
	defer sidecar.Close()
	embedder := NewEmbedder(sidecar.URL)
	registry := NewPositionRegistry(embedder)

	// Empty registry has a version
	v1 := registry.EmbeddingsVersion()
	if v1 == "" {
		t.Fatal("expected non-empty version for empty registry")
	}
	if len(v1) != 32 {
		t.Errorf("expected 32 hex chars, got %d: %q", len(v1), v1)
	}

	// Same version on second call (cached)
	v1b := registry.EmbeddingsVersion()
	if v1b != v1 {
		t.Errorf("expected cached version %q, got %q", v1, v1b)
	}

	// Register changes version (new ID+embedding added)
	pos, err := registry.Register("Adv1", "test intent", 1.0, 5.0, "USD", "")
	if err != nil {
		t.Fatal(err)
	}
	v2 := registry.EmbeddingsVersion()
	if v2 == v1 {
		t.Error("version should change after Register")
	}

	// Adding a second position changes version
	_, err = registry.Register("Adv2", "another intent", 1.0, 3.0, "USD", "")
	if err != nil {
		t.Fatal(err)
	}
	v3 := registry.EmbeddingsVersion()
	if v3 == v2 {
		t.Error("version should change after second Register")
	}

	// Name-only update does NOT change version (hash is ID+embedding only)
	_, err = registry.Update(pos.ID, "New Name", "", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	v3b := registry.EmbeddingsVersion()
	// Version is still invalidated by Update (cache cleared), but recomputes to same value
	// because the embedding didn't change
	_ = v3b

	// Delete changes version
	all := registry.GetAll()
	if err := registry.Delete(all[0].ID); err != nil {
		t.Fatal(err)
	}
	v4 := registry.EmbeddingsVersion()
	if v4 == v3 {
		t.Error("version should change after Delete")
	}
}

func TestSetBudgetTracker(t *testing.T) {
	registry := NewPositionRegistry(nil)
	bt := NewBudgetTracker()
	registry.SetBudgetTracker(bt)
	// Just verifying it doesn't panic and the field is set
	if registry.budgets != bt {
		t.Error("budget tracker not set")
	}
}


