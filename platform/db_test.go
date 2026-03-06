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
	if err := db.UpdateAdvertiser("adv-1", "Updated Name", "new intent", []float64{0.4, 0.5, 0.6}, 0.8, 3.00, ""); err != nil {
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
	err := db.UpdateAdvertiser("nonexistent", "name", "intent", []float64{0.1}, 0.5, 1.0, "")
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

// --- Token Tests ---

func TestGenerateAndLookupToken(t *testing.T) {
	db := newMemoryDB(t)
	pos := samplePosition("adv-1")
	if err := db.InsertAdvertiser(pos, 100.0); err != nil {
		t.Fatal(err)
	}

	token, err := db.GenerateToken("adv-1")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if len(token) != 32 {
		t.Errorf("token length = %d, want 32", len(token))
	}

	advID, err := db.LookupToken(token)
	if err != nil {
		t.Fatalf("LookupToken: %v", err)
	}
	if advID != "adv-1" {
		t.Errorf("LookupToken = %q, want %q", advID, "adv-1")
	}
}

func TestLookupTokenNotFound(t *testing.T) {
	db := newMemoryDB(t)
	advID, err := db.LookupToken("nonexistent-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if advID != "" {
		t.Errorf("expected empty string for nonexistent token, got %q", advID)
	}
}

// --- Event Tests ---

func TestLogAndGetEventStats(t *testing.T) {
	db := newMemoryDB(t)

	// Log events
	if err := db.LogEvent(1, "adv-1", "impression", "u1"); err != nil {
		t.Fatal(err)
	}
	if err := db.LogEvent(1, "adv-1", "impression", "u2"); err != nil {
		t.Fatal(err)
	}
	if err := db.LogEvent(1, "adv-1", "click", "u1"); err != nil {
		t.Fatal(err)
	}
	if err := db.LogEvent(2, "adv-1", "viewable", "u1"); err != nil {
		t.Fatal(err)
	}
	if err := db.LogEvent(3, "adv-2", "impression", "u1"); err != nil {
		t.Fatal(err)
	}

	// Stats for adv-1
	stats, err := db.GetEventStats("adv-1")
	if err != nil {
		t.Fatal(err)
	}
	if stats.Impressions != 2 {
		t.Errorf("Impressions = %d, want 2", stats.Impressions)
	}
	if stats.Clicks != 1 {
		t.Errorf("Clicks = %d, want 1", stats.Clicks)
	}
	if stats.Viewable != 1 {
		t.Errorf("Viewable = %d, want 1", stats.Viewable)
	}

	// Stats for all
	allStats, err := db.GetEventStats("")
	if err != nil {
		t.Fatal(err)
	}
	if allStats.Impressions != 3 {
		t.Errorf("All Impressions = %d, want 3", allStats.Impressions)
	}
}

// --- Frequency Cap Tests ---

func TestFrequencyCapUnderLimit(t *testing.T) {
	db := newMemoryDB(t)

	ok, err := db.CheckFrequencyCap("adv-1", "u1", 3, 60)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected under cap for new user")
	}

	// Increment twice
	for i := 0; i < 2; i++ {
		if err := db.IncrementFrequencyCap("adv-1", "u1", 60); err != nil {
			t.Fatal(err)
		}
	}

	ok, err = db.CheckFrequencyCap("adv-1", "u1", 3, 60)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected under cap at 2/3")
	}
}

func TestFrequencyCapAtLimit(t *testing.T) {
	db := newMemoryDB(t)

	// Increment 3 times (hit the cap)
	for i := 0; i < 3; i++ {
		if err := db.IncrementFrequencyCap("adv-1", "u1", 60); err != nil {
			t.Fatal(err)
		}
	}

	ok, err := db.CheckFrequencyCap("adv-1", "u1", 3, 60)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected over cap at 3/3")
	}
}

// --- Auction Query Tests ---

func TestGetAuctionsByAdvertiser(t *testing.T) {
	db := newMemoryDB(t)

	db.LogAuction("intent-a", "adv-1", 2.50, "USD", 5)
	db.LogAuction("intent-b", "adv-2", 1.75, "USD", 3)
	db.LogAuction("intent-c", "adv-1", 3.00, "USD", 4)

	auctions, total, err := db.GetAuctionsByAdvertiser("adv-1", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(auctions) != 2 {
		t.Errorf("len = %d, want 2", len(auctions))
	}
}

func TestGetAllAuctionsWithFilters(t *testing.T) {
	db := newMemoryDB(t)

	db.LogAuction("therapy session", "adv-1", 2.50, "USD", 5)
	db.LogAuction("sleep help", "adv-2", 1.75, "USD", 3)
	db.LogAuction("therapy advice", "adv-1", 3.00, "USD", 4)

	// Filter by winner
	auctions, total, err := db.GetAllAuctions(10, 0, "adv-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("winner filter: total = %d, want 2", total)
	}

	// Filter by intent
	auctions, total, err = db.GetAllAuctions(10, 0, "", "therapy")
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("intent filter: total = %d, want 2", total)
	}
	_ = auctions
}

// --- Revenue Tests ---

func TestGetRevenueByPeriod(t *testing.T) {
	db := newMemoryDB(t)

	db.LogAuction("intent-a", "adv-1", 2.50, "USD", 5)
	db.LogAuction("intent-b", "adv-2", 1.75, "USD", 3)

	periods, err := db.GetRevenueByPeriod("day")
	if err != nil {
		t.Fatal(err)
	}
	if len(periods) == 0 {
		t.Fatal("expected at least 1 period")
	}
	if periods[0].AuctionCount != 2 {
		t.Errorf("AuctionCount = %d, want 2", periods[0].AuctionCount)
	}
}

// --- Top Advertisers Tests ---

func TestGetTopAdvertisersBySpend(t *testing.T) {
	db := newMemoryDB(t)

	db.LogAuction("intent-a", "adv-1", 2.50, "USD", 5)
	db.LogAuction("intent-b", "adv-1", 3.00, "USD", 3)
	db.LogAuction("intent-c", "adv-2", 1.75, "USD", 4)

	top, err := db.GetTopAdvertisersBySpend(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 2 {
		t.Fatalf("len = %d, want 2", len(top))
	}
	if top[0].AdvertiserID != "adv-1" {
		t.Errorf("top spender = %q, want adv-1", top[0].AdvertiserID)
	}
	if top[0].TotalSpend != 5.50 {
		t.Errorf("top spend = %f, want 5.50", top[0].TotalSpend)
	}
}

// --- Advertisers With Budget Tests ---

func TestGetAllAdvertisersWithBudget(t *testing.T) {
	db := newMemoryDB(t)
	pos := samplePosition("adv-1")
	if err := db.InsertAdvertiser(pos, 100.0); err != nil {
		t.Fatal(err)
	}

	advs, err := db.GetAllAdvertisersWithBudget()
	if err != nil {
		t.Fatal(err)
	}
	if len(advs) != 1 {
		t.Fatalf("len = %d, want 1", len(advs))
	}
	if advs[0].BudgetTotal != 100.0 {
		t.Errorf("BudgetTotal = %f, want 100.0", advs[0].BudgetTotal)
	}
}

// --- LogAuctionReturningID Tests ---

func TestLogAuctionReturningID(t *testing.T) {
	db := newMemoryDB(t)

	id1, err := db.LogAuctionReturningID("intent-a", "adv-1", 2.50, "USD", 5)
	if err != nil {
		t.Fatal(err)
	}
	if id1 <= 0 {
		t.Errorf("id1 = %d, want > 0", id1)
	}

	id2, err := db.LogAuctionReturningID("intent-b", "adv-2", 1.75, "USD", 3)
	if err != nil {
		t.Fatal(err)
	}
	if id2 <= id1 {
		t.Errorf("id2 = %d, want > %d", id2, id1)
	}
}

// --- Publisher Tests ---

func TestPublisherCRUD(t *testing.T) {
	db := newMemoryDB(t)

	// Insert
	if err := db.InsertPublisher("pub-1", "TechBlog", "techblog.com"); err != nil {
		t.Fatalf("InsertPublisher: %v", err)
	}

	// Get
	pub, err := db.GetPublisher("pub-1")
	if err != nil {
		t.Fatalf("GetPublisher: %v", err)
	}
	if pub == nil {
		t.Fatal("GetPublisher returned nil")
	}
	if pub.Name != "TechBlog" {
		t.Errorf("Name = %q, want %q", pub.Name, "TechBlog")
	}
	if pub.Domain != "techblog.com" {
		t.Errorf("Domain = %q, want %q", pub.Domain, "techblog.com")
	}

	// Not found
	pub, err = db.GetPublisher("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pub != nil {
		t.Error("expected nil for nonexistent publisher")
	}
}

func TestNextPublisherID(t *testing.T) {
	db := newMemoryDB(t)

	// Empty
	id, err := db.NextPublisherID()
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Errorf("NextPublisherID on empty = %d, want 1", id)
	}

	// After inserts
	for i := 1; i <= 3; i++ {
		if err := db.InsertPublisher("pub-"+strconv.Itoa(i), "Pub"+strconv.Itoa(i), ""); err != nil {
			t.Fatal(err)
		}
	}
	id, err = db.NextPublisherID()
	if err != nil {
		t.Fatal(err)
	}
	if id != 4 {
		t.Errorf("NextPublisherID after 3 = %d, want 4", id)
	}
}

func TestPublisherTokens(t *testing.T) {
	db := newMemoryDB(t)

	if err := db.InsertPublisher("pub-1", "TechBlog", "techblog.com"); err != nil {
		t.Fatal(err)
	}

	token, err := db.GeneratePublisherToken("pub-1")
	if err != nil {
		t.Fatalf("GeneratePublisherToken: %v", err)
	}
	if len(token) != 32 {
		t.Errorf("token length = %d, want 32", len(token))
	}

	pubID, err := db.LookupPublisherToken(token)
	if err != nil {
		t.Fatalf("LookupPublisherToken: %v", err)
	}
	if pubID != "pub-1" {
		t.Errorf("LookupPublisherToken = %q, want %q", pubID, "pub-1")
	}

	// Not found
	pubID, err = db.LookupPublisherToken("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pubID != "" {
		t.Errorf("expected empty for nonexistent token, got %q", pubID)
	}
}

func TestLogAuctionReturningIDWithPublisher(t *testing.T) {
	db := newMemoryDB(t)

	id, err := db.LogAuctionReturningIDWithPublisher("intent-a", "adv-1", 2.50, "USD", 5, "pub-1")
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Errorf("id = %d, want > 0", id)
	}

	// Verify publisher_id via GetAuctionsByPublisher
	auctions, total, err := db.GetAuctionsByPublisher("pub-1", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(auctions) != 1 {
		t.Fatalf("len = %d, want 1", len(auctions))
	}
}

func TestLogEventWithPublisher(t *testing.T) {
	db := newMemoryDB(t)

	if err := db.LogEventWithPublisher(1, "adv-1", "impression", "u1", "pub-1"); err != nil {
		t.Fatal(err)
	}

	stats, err := db.GetPublisherEventStats("pub-1")
	if err != nil {
		t.Fatal(err)
	}
	if stats.Impressions != 1 {
		t.Errorf("Impressions = %d, want 1", stats.Impressions)
	}
}

func TestGetPublisherRevenue(t *testing.T) {
	db := newMemoryDB(t)

	db.LogAuctionReturningIDWithPublisher("intent-a", "adv-1", 10.0, "USD", 5, "pub-1")
	db.LogAuctionReturningIDWithPublisher("intent-b", "adv-2", 5.0, "USD", 3, "pub-1")
	db.LogAuctionReturningIDWithPublisher("intent-c", "adv-1", 8.0, "USD", 4, "pub-2")

	rev, err := db.GetPublisherRevenue("pub-1")
	if err != nil {
		t.Fatal(err)
	}
	expected := 15.0 * 0.85
	if math.Abs(rev-expected) > 0.001 {
		t.Errorf("revenue = %f, want %f", rev, expected)
	}
}

func TestGetPublisherRevenueByPeriod(t *testing.T) {
	db := newMemoryDB(t)

	db.LogAuctionReturningIDWithPublisher("intent-a", "adv-1", 2.50, "USD", 5, "pub-1")
	db.LogAuctionReturningIDWithPublisher("intent-b", "adv-2", 1.75, "USD", 3, "pub-1")

	periods, err := db.GetPublisherRevenueByPeriod("pub-1", "day")
	if err != nil {
		t.Fatal(err)
	}
	if len(periods) == 0 {
		t.Fatal("expected at least 1 period")
	}
	if periods[0].AuctionCount != 2 {
		t.Errorf("AuctionCount = %d, want 2", periods[0].AuctionCount)
	}
}

func TestGetPublisherEventStats(t *testing.T) {
	db := newMemoryDB(t)

	db.LogEventWithPublisher(1, "adv-1", "impression", "u1", "pub-1")
	db.LogEventWithPublisher(1, "adv-1", "impression", "u2", "pub-1")
	db.LogEventWithPublisher(1, "adv-1", "click", "u1", "pub-1")
	db.LogEventWithPublisher(2, "adv-1", "viewable", "u1", "pub-1")
	db.LogEventWithPublisher(3, "adv-2", "impression", "u1", "pub-2")

	stats, err := db.GetPublisherEventStats("pub-1")
	if err != nil {
		t.Fatal(err)
	}
	if stats.Impressions != 2 {
		t.Errorf("Impressions = %d, want 2", stats.Impressions)
	}
	if stats.Clicks != 1 {
		t.Errorf("Clicks = %d, want 1", stats.Clicks)
	}
	if stats.Viewable != 1 {
		t.Errorf("Viewable = %d, want 1", stats.Viewable)
	}
}

func TestGetAuctionsByPublisher(t *testing.T) {
	db := newMemoryDB(t)

	db.LogAuctionReturningIDWithPublisher("intent-a", "adv-1", 2.50, "USD", 5, "pub-1")
	db.LogAuctionReturningIDWithPublisher("intent-b", "adv-2", 1.75, "USD", 3, "pub-2")
	db.LogAuctionReturningIDWithPublisher("intent-c", "adv-1", 3.00, "USD", 4, "pub-1")

	auctions, total, err := db.GetAuctionsByPublisher("pub-1", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(auctions) != 2 {
		t.Errorf("len = %d, want 2", len(auctions))
	}
}

func TestGetPublisherTopAdvertisers(t *testing.T) {
	db := newMemoryDB(t)

	db.LogAuctionReturningIDWithPublisher("intent-a", "adv-1", 5.00, "USD", 5, "pub-1")
	db.LogAuctionReturningIDWithPublisher("intent-b", "adv-1", 3.00, "USD", 3, "pub-1")
	db.LogAuctionReturningIDWithPublisher("intent-c", "adv-2", 1.75, "USD", 4, "pub-1")
	db.LogAuctionReturningIDWithPublisher("intent-d", "adv-3", 10.00, "USD", 2, "pub-2") // different publisher

	top, err := db.GetPublisherTopAdvertisers("pub-1", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 2 {
		t.Fatalf("len = %d, want 2", len(top))
	}
	if top[0].AdvertiserID != "adv-1" {
		t.Errorf("top = %q, want adv-1", top[0].AdvertiserID)
	}
	if top[0].TotalSpend != 8.00 {
		t.Errorf("top spend = %f, want 8.00", top[0].TotalSpend)
	}
}

func TestGetPublisherStats(t *testing.T) {
	db := newMemoryDB(t)

	db.LogAuctionReturningIDWithPublisher("intent-a", "adv-1", 10.0, "USD", 5, "pub-1")
	db.LogAuctionReturningIDWithPublisher("intent-b", "adv-2", 5.0, "USD", 3, "pub-1")

	s, err := db.GetPublisherStats("pub-1")
	if err != nil {
		t.Fatal(err)
	}
	if s.AuctionCount != 2 {
		t.Errorf("AuctionCount = %d, want 2", s.AuctionCount)
	}
	expected := 15.0 * 0.85
	if math.Abs(s.TotalRevenue-expected) > 0.001 {
		t.Errorf("TotalRevenue = %f, want %f", s.TotalRevenue, expected)
	}
	if s.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", s.Currency)
	}
}

// --- Publisher Credentials Tests ---

func TestInsertAndLookupPublisherCredentials(t *testing.T) {
	db := newMemoryDB(t)

	if err := db.InsertPublisher("pub-1", "TechBlog", "techblog.com"); err != nil {
		t.Fatal(err)
	}

	if err := db.InsertPublisherCredentials("pub-1", "pub@test.com", "$2a$10$fakehash"); err != nil {
		t.Fatalf("InsertPublisherCredentials: %v", err)
	}

	pubID, hash, err := db.LookupPublisherByEmail("pub@test.com")
	if err != nil {
		t.Fatalf("LookupPublisherByEmail: %v", err)
	}
	if pubID != "pub-1" {
		t.Errorf("publisherID = %q, want pub-1", pubID)
	}
	if hash != "$2a$10$fakehash" {
		t.Errorf("passwordHash = %q, want $2a$10$fakehash", hash)
	}
}

func TestLookupPublisherByEmailNotFound(t *testing.T) {
	db := newMemoryDB(t)

	pubID, hash, err := db.LookupPublisherByEmail("nonexistent@test.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pubID != "" || hash != "" {
		t.Errorf("expected empty strings, got pubID=%q hash=%q", pubID, hash)
	}
}

func TestInsertPublisherCredentialsDuplicateEmail(t *testing.T) {
	db := newMemoryDB(t)

	db.InsertPublisher("pub-1", "TechBlog", "techblog.com")
	db.InsertPublisher("pub-2", "OtherBlog", "other.com")

	if err := db.InsertPublisherCredentials("pub-1", "dup@test.com", "hash1"); err != nil {
		t.Fatal(err)
	}

	err := db.InsertPublisherCredentials("pub-2", "dup@test.com", "hash2")
	if err == nil {
		t.Error("expected error for duplicate email")
	}
}

func TestGetPublisherToken(t *testing.T) {
	db := newMemoryDB(t)

	db.InsertPublisher("pub-1", "TechBlog", "techblog.com")
	token, err := db.GeneratePublisherToken("pub-1")
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.GetPublisherToken("pub-1")
	if err != nil {
		t.Fatalf("GetPublisherToken: %v", err)
	}
	if got != token {
		t.Errorf("GetPublisherToken = %q, want %q", got, token)
	}
}

func TestGetPublisherTokenNotFound(t *testing.T) {
	db := newMemoryDB(t)

	got, err := db.GetPublisherToken("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty for nonexistent, got %q", got)
	}
}
