package platform

import (
	"math"
	"path/filepath"
	"strconv"
	"testing"
)

func newMemoryDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB(:memory:): %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func samplePosition(id string) *Position {
	return &Position{
		ID:        id,
		Name:      "Test Advertiser",
		Intent:    "test intent",
		Embedding: []float64{0.1, 0.2, 0.3},
		Sigma:     0.5,
		BidPrice:  2.50,
		Currency:  "USD",
	}
}

func TestCRUDLifecycle(t *testing.T) {
	db := newMemoryDB(t)
	pos := samplePosition("adv-1")

	// Insert
	if err := db.InsertAdvertiser(pos, 100.0); err != nil {
		t.Fatalf("InsertAdvertiser: %v", err)
	}

	// Get
	got, err := db.GetAdvertiser("adv-1")
	if err != nil {
		t.Fatalf("GetAdvertiser: %v", err)
	}
	if got == nil {
		t.Fatal("GetAdvertiser returned nil")
	}
	if got.Name != "Test Advertiser" {
		t.Errorf("Name = %q, want %q", got.Name, "Test Advertiser")
	}
	if got.Intent != "test intent" {
		t.Errorf("Intent = %q, want %q", got.Intent, "test intent")
	}
	if got.Sigma != 0.5 {
		t.Errorf("Sigma = %f, want 0.5", got.Sigma)
	}
	if got.BidPrice != 2.50 {
		t.Errorf("BidPrice = %f, want 2.50", got.BidPrice)
	}

	// GetAll
	all, err := db.GetAllAdvertisers()
	if err != nil {
		t.Fatalf("GetAllAdvertisers: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("GetAllAdvertisers len = %d, want 1", len(all))
	}

	// Update
	if err := db.UpdateAdvertiser("adv-1", "Updated Name", "new intent", []float64{0.4, 0.5, 0.6}, 0.8, 3.00); err != nil {
		t.Fatalf("UpdateAdvertiser: %v", err)
	}
	got, _ = db.GetAdvertiser("adv-1")
	if got.Name != "Updated Name" {
		t.Errorf("after update Name = %q, want %q", got.Name, "Updated Name")
	}
	if got.Intent != "new intent" {
		t.Errorf("after update Intent = %q, want %q", got.Intent, "new intent")
	}
	if got.Sigma != 0.8 {
		t.Errorf("after update Sigma = %f, want 0.8", got.Sigma)
	}
	if got.BidPrice != 3.00 {
		t.Errorf("after update BidPrice = %f, want 3.00", got.BidPrice)
	}

	// Delete
	if err := db.DeleteAdvertiser("adv-1"); err != nil {
		t.Fatalf("DeleteAdvertiser: %v", err)
	}
	got, err = db.GetAdvertiser("adv-1")
	if err != nil {
		t.Fatalf("GetAdvertiser after delete: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestEmbeddingJSONSerde(t *testing.T) {
	db := newMemoryDB(t)
	emb := []float64{0.123456789, -0.987654321, 1e-10, 3.14159265358979}
	pos := &Position{
		ID:        "adv-1",
		Name:      "Emb Test",
		Intent:    "embedding test",
		Embedding: emb,
		Sigma:     0.5,
		BidPrice:  1.0,
		Currency:  "USD",
	}
	if err := db.InsertAdvertiser(pos, 100.0); err != nil {
		t.Fatalf("InsertAdvertiser: %v", err)
	}

	got, err := db.GetAdvertiser("adv-1")
	if err != nil {
		t.Fatalf("GetAdvertiser: %v", err)
	}
	if len(got.Embedding) != len(emb) {
		t.Fatalf("embedding len = %d, want %d", len(got.Embedding), len(emb))
	}
	for i, v := range emb {
		if got.Embedding[i] != v {
			t.Errorf("embedding[%d] = %v, want %v", i, got.Embedding[i], v)
		}
	}
}

func TestChargeSufficientFunds(t *testing.T) {
	db := newMemoryDB(t)
	pos := samplePosition("adv-1")
	if err := db.InsertAdvertiser(pos, 10.0); err != nil {
		t.Fatal(err)
	}

	ok, err := db.Charge("adv-1", 3.0)
	if err != nil {
		t.Fatalf("Charge: %v", err)
	}
	if !ok {
		t.Fatal("Charge returned false for sufficient funds")
	}

	b, err := db.GetBudget("adv-1")
	if err != nil {
		t.Fatal(err)
	}
	if b.Spent != 3.0 {
		t.Errorf("Spent = %f, want 3.0", b.Spent)
	}
	if b.Total != 10.0 {
		t.Errorf("Total = %f, want 10.0", b.Total)
	}
}

func TestChargeInsufficientFunds(t *testing.T) {
	db := newMemoryDB(t)
	pos := samplePosition("adv-1")
	if err := db.InsertAdvertiser(pos, 5.0); err != nil {
		t.Fatal(err)
	}

	ok, err := db.Charge("adv-1", 6.0)
	if err != nil {
		t.Fatalf("Charge: %v", err)
	}
	if ok {
		t.Fatal("Charge returned true for insufficient funds")
	}

	b, _ := db.GetBudget("adv-1")
	if b.Spent != 0 {
		t.Errorf("Spent = %f, want 0 (should be unchanged)", b.Spent)
	}
}

func TestUpdateBudget(t *testing.T) {
	db := newMemoryDB(t)
	pos := samplePosition("adv-1")
	if err := db.InsertAdvertiser(pos, 100.0); err != nil {
		t.Fatal(err)
	}

	if err := db.UpdateBudget("adv-1", 200.0); err != nil {
		t.Fatalf("UpdateBudget: %v", err)
	}

	b, _ := db.GetBudget("adv-1")
	if b.Total != 200.0 {
		t.Errorf("Total = %f, want 200.0", b.Total)
	}
}

func TestUpdateBudgetNotFound(t *testing.T) {
	db := newMemoryDB(t)
	err := db.UpdateBudget("nonexistent", 100.0)
	if err == nil {
		t.Fatal("expected error for nonexistent advertiser")
	}
}

func TestAuctionLoggingAndStats(t *testing.T) {
	db := newMemoryDB(t)

	// Initial stats should be zero
	s, err := db.GetStats()
	if err != nil {
		t.Fatal(err)
	}
	if s.AuctionCount != 0 {
		t.Errorf("initial AuctionCount = %d, want 0", s.AuctionCount)
	}
	if s.TotalSpend != 0 {
		t.Errorf("initial TotalSpend = %f, want 0", s.TotalSpend)
	}

	// Log some auctions
	db.LogAuction("intent-a", "adv-1", 2.50, "USD", 5)
	db.LogAuction("intent-b", "adv-2", 1.75, "USD", 3)
	db.LogAuction("intent-c", "adv-1", 3.00, "USD", 4)

	s, err = db.GetStats()
	if err != nil {
		t.Fatal(err)
	}
	if s.AuctionCount != 3 {
		t.Errorf("AuctionCount = %d, want 3", s.AuctionCount)
	}
	expectedSpend := 2.50 + 1.75 + 3.00
	if math.Abs(s.TotalSpend-expectedSpend) > 0.001 {
		t.Errorf("TotalSpend = %f, want %f", s.TotalSpend, expectedSpend)
	}
	expectedExchange := expectedSpend * 0.15
	expectedPublisher := expectedSpend - expectedExchange
	if math.Abs(s.ExchangeRevenue-expectedExchange) > 0.001 {
		t.Errorf("ExchangeRevenue = %f, want %f", s.ExchangeRevenue, expectedExchange)
	}
	if math.Abs(s.PublisherRevenue-expectedPublisher) > 0.001 {
		t.Errorf("PublisherRevenue = %f, want %f", s.PublisherRevenue, expectedPublisher)
	}
}

func TestNextIDEmpty(t *testing.T) {
	db := newMemoryDB(t)
	id, err := db.NextID()
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Errorf("NextID on empty DB = %d, want 1", id)
	}
}

func TestNextIDAfterInserts(t *testing.T) {
	db := newMemoryDB(t)

	for i := 1; i <= 5; i++ {
		pos := samplePosition("adv-" + strconv.Itoa(i))
		pos.Name = "adv-" + strconv.Itoa(i)
		if err := db.InsertAdvertiser(pos, 100.0); err != nil {
			t.Fatal(err)
		}
	}

	id, err := db.NextID()
	if err != nil {
		t.Fatal(err)
	}
	if id != 6 {
		t.Errorf("NextID after 5 inserts = %d, want 6", id)
	}
}

func TestPersistenceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Open, insert, close
	db1, err := NewDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	pos := samplePosition("adv-1")
	if err := db1.InsertAdvertiser(pos, 50.0); err != nil {
		t.Fatal(err)
	}
	db1.Close()

	// Reopen and verify
	db2, err := NewDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	all, err := db2.GetAllAdvertisers()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("after reopen: len = %d, want 1", len(all))
	}
	if all[0].ID != "adv-1" {
		t.Errorf("after reopen: ID = %q, want %q", all[0].ID, "adv-1")
	}
	if all[0].Name != "Test Advertiser" {
		t.Errorf("after reopen: Name = %q, want %q", all[0].Name, "Test Advertiser")
	}

	b, _ := db2.GetBudget("adv-1")
	if b == nil {
		t.Fatal("budget nil after reopen")
	}
	if b.Total != 50.0 {
		t.Errorf("after reopen: budget Total = %f, want 50.0", b.Total)
	}
}

func TestGetAdvertiserNotFound(t *testing.T) {
	db := newMemoryDB(t)
	got, err := db.GetAdvertiser("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent advertiser")
	}
}

func TestDeleteAdvertiserNotFound(t *testing.T) {
	db := newMemoryDB(t)
	err := db.DeleteAdvertiser("nonexistent")
	if err == nil {
		t.Error("expected error for deleting nonexistent advertiser")
	}
}

func TestUpdateAdvertiserNotFound(t *testing.T) {
	db := newMemoryDB(t)
	err := db.UpdateAdvertiser("nonexistent", "name", "intent", []float64{0.1}, 0.5, 1.0)
	if err == nil {
		t.Error("expected error for updating nonexistent advertiser")
	}
}

func TestGetBudgetNotFound(t *testing.T) {
	db := newMemoryDB(t)
	b, err := db.GetBudget("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != nil {
		t.Error("expected nil for nonexistent budget")
	}
}


func TestGetAllAdvertisersEmpty(t *testing.T) {
	db := newMemoryDB(t)
	all, err := db.GetAllAdvertisers()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Errorf("expected empty slice, got %d", len(all))
	}
}

func TestChargeMultiple(t *testing.T) {
	db := newMemoryDB(t)
	pos := samplePosition("adv-1")
	if err := db.InsertAdvertiser(pos, 10.0); err != nil {
		t.Fatal(err)
	}

	// Charge 3 times: 3.0 + 3.0 + 3.0 = 9.0 (should succeed)
	for i := 0; i < 3; i++ {
		ok, err := db.Charge("adv-1", 3.0)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("charge %d failed unexpectedly", i+1)
		}
	}

	// 4th charge of 3.0 would need 12.0 total but budget is 10.0
	ok, err := db.Charge("adv-1", 3.0)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("4th charge should have failed (insufficient funds)")
	}

	b, _ := db.GetBudget("adv-1")
	if b.Spent != 9.0 {
		t.Errorf("Spent = %f, want 9.0", b.Spent)
	}
}

func TestChargeNonexistent(t *testing.T) {
	db := newMemoryDB(t)
	_, err := db.Charge("nonexistent", 1.0)
	if err == nil {
		t.Error("expected error for charging nonexistent advertiser")
	}
}
