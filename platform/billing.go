package platform

import (
	"log"
	"sync"
)

// BudgetInfo represents an advertiser's budget state.
type BudgetInfo struct {
	AdvertiserID string  `json:"advertiser_id"`
	Total        float64 `json:"total"`
	Spent        float64 `json:"spent"`
	Remaining    float64 `json:"remaining"`
	Currency     string  `json:"currency"`
}

// BudgetTracker manages per-advertiser budgets with thread-safe access.
// When a DB is set, it persists changes and uses in-memory cache for reads.
type BudgetTracker struct {
	mu       sync.Mutex
	budgets  map[string]float64 // total budget
	spent    map[string]float64 // amount spent
	currency map[string]string  // currency per advertiser
	db       *DB
}

func NewBudgetTracker() *BudgetTracker {
	return &BudgetTracker{
		budgets:  make(map[string]float64),
		spent:    make(map[string]float64),
		currency: make(map[string]string),
	}
}

// SetDB attaches a database and loads existing budgets into cache.
func (b *BudgetTracker) SetDB(db *DB) error {
	b.db = db

	positions, err := db.GetAllAdvertisers()
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	for _, pos := range positions {
		budgetRow, err := db.GetBudget(pos.ID)
		if err != nil || budgetRow == nil {
			continue
		}
		b.budgets[pos.ID] = budgetRow.Total
		b.spent[pos.ID] = budgetRow.Spent
		b.currency[pos.ID] = budgetRow.Currency
	}

	return nil
}

// Set initializes budget for an advertiser.
func (b *BudgetTracker) Set(advertiserID string, total float64, currency string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.budgets[advertiserID] = total
	b.spent[advertiserID] = 0
	b.currency[advertiserID] = currency
}

// CanAfford returns true if the advertiser has enough remaining budget for the given amount.
func (b *BudgetTracker) CanAfford(advertiserID string, amount float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	total, ok := b.budgets[advertiserID]
	if !ok {
		return false
	}
	return (total - b.spent[advertiserID]) >= amount
}

// Charge deducts the payment from the advertiser's budget. Returns false if insufficient funds.
func (b *BudgetTracker) Charge(advertiserID string, amount float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	total, ok := b.budgets[advertiserID]
	if !ok {
		return false
	}
	remaining := total - b.spent[advertiserID]
	if remaining < amount {
		return false
	}
	b.spent[advertiserID] += amount

	// Persist to DB
	if b.db != nil {
		if _, err := b.db.Charge(advertiserID, amount); err != nil {
			log.Printf("WARN: failed to persist charge for %s: %v", advertiserID, err)
		}
	}

	return true
}

// GetInfo returns budget information for an advertiser.
func (b *BudgetTracker) GetInfo(advertiserID string) *BudgetInfo {
	b.mu.Lock()
	defer b.mu.Unlock()
	total, ok := b.budgets[advertiserID]
	if !ok {
		return nil
	}
	spent := b.spent[advertiserID]
	return &BudgetInfo{
		AdvertiserID: advertiserID,
		Total:        total,
		Spent:        spent,
		Remaining:    total - spent,
		Currency:     b.currency[advertiserID],
	}
}

// Delete removes an advertiser's budget tracking.
func (b *BudgetTracker) Delete(advertiserID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.budgets, advertiserID)
	delete(b.spent, advertiserID)
	delete(b.currency, advertiserID)
}
