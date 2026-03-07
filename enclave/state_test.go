package enclave

import (
	"sync"
	"testing"
)

func TestPositionStoreReplaceAll(t *testing.T) {
	store := NewPositionStore()

	positions := []PositionSnapshot{
		{ID: "adv-1", Name: "A", Embedding: []float64{0.1, 0.2}, Sigma: 0.5, BidPrice: 2.0, Currency: "USD"},
		{ID: "adv-2", Name: "B", Embedding: []float64{0.3, 0.4}, Sigma: 0.6, BidPrice: 3.0, Currency: "USD"},
	}
	store.ReplaceAll(positions)

	all := store.GetAll()
	if len(all) != 2 {
		t.Fatalf("GetAll len = %d, want 2", len(all))
	}

	// Replace with a different set
	store.ReplaceAll([]PositionSnapshot{
		{ID: "adv-3", Name: "C", Embedding: []float64{0.5}, Sigma: 0.7, BidPrice: 4.0, Currency: "EUR"},
	})

	all = store.GetAll()
	if len(all) != 1 {
		t.Fatalf("after replace, GetAll len = %d, want 1", len(all))
	}
	if all[0].ID != "adv-3" {
		t.Errorf("ID = %q, want %q", all[0].ID, "adv-3")
	}
}

func TestBudgetStoreCanAfford(t *testing.T) {
	store := NewBudgetStore()
	store.ReplaceAll([]BudgetSnapshot{
		{AdvertiserID: "adv-1", Total: 100, Spent: 30, Currency: "USD"},
		{AdvertiserID: "adv-2", Total: 50, Spent: 50, Currency: "USD"},
	})

	if !store.CanAfford("adv-1", 70) {
		t.Error("adv-1 should afford 70 (remaining=70)")
	}
	if store.CanAfford("adv-1", 71) {
		t.Error("adv-1 should not afford 71")
	}
	if store.CanAfford("adv-2", 1) {
		t.Error("adv-2 should not afford anything (spent=total)")
	}
	if store.CanAfford("nonexistent", 1) {
		t.Error("nonexistent should not afford anything")
	}
}

func TestPositionStoreConcurrent(t *testing.T) {
	store := NewPositionStore()
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			store.ReplaceAll([]PositionSnapshot{
				{ID: "adv-1", Name: "A", BidPrice: float64(i)},
			})
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = store.GetAll()
		}()
	}

	wg.Wait()
}

func TestBudgetStoreConcurrent(t *testing.T) {
	store := NewBudgetStore()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			store.ReplaceAll([]BudgetSnapshot{
				{AdvertiserID: "adv-1", Total: 100, Spent: float64(i)},
			})
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = store.CanAfford("adv-1", 50)
		}()
	}

	wg.Wait()
}
