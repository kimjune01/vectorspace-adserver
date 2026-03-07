package enclave

import "sync"

// PositionStore holds advertiser positions with thread-safe atomic snapshot replace.
type PositionStore struct {
	mu        sync.RWMutex
	positions map[string]*PositionSnapshot
}

// NewPositionStore creates an empty PositionStore.
func NewPositionStore() *PositionStore {
	return &PositionStore{
		positions: make(map[string]*PositionSnapshot),
	}
}

// ReplaceAll atomically replaces all positions with a new snapshot.
func (s *PositionStore) ReplaceAll(positions []PositionSnapshot) {
	m := make(map[string]*PositionSnapshot, len(positions))
	for i := range positions {
		m[positions[i].ID] = &positions[i]
	}
	s.mu.Lock()
	s.positions = m
	s.mu.Unlock()
}

// GetAll returns a snapshot of all positions.
func (s *PositionStore) GetAll() []PositionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]PositionSnapshot, 0, len(s.positions))
	for _, p := range s.positions {
		result = append(result, *p)
	}
	return result
}

// BudgetStore holds advertiser budgets with thread-safe atomic snapshot replace.
type BudgetStore struct {
	mu      sync.RWMutex
	budgets map[string]*BudgetSnapshot
}

// NewBudgetStore creates an empty BudgetStore.
func NewBudgetStore() *BudgetStore {
	return &BudgetStore{
		budgets: make(map[string]*BudgetSnapshot),
	}
}

// ReplaceAll atomically replaces all budgets with a new snapshot.
func (s *BudgetStore) ReplaceAll(budgets []BudgetSnapshot) {
	m := make(map[string]*BudgetSnapshot, len(budgets))
	for i := range budgets {
		m[budgets[i].AdvertiserID] = &budgets[i]
	}
	s.mu.Lock()
	s.budgets = m
	s.mu.Unlock()
}

// CanAfford returns true if the advertiser has enough remaining budget.
func (s *BudgetStore) CanAfford(advertiserID string, amount float64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.budgets[advertiserID]
	if !ok {
		return false
	}
	return (b.Total - b.Spent) >= amount
}

// GetAll returns a snapshot of all budgets.
func (s *BudgetStore) GetAll() []BudgetSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]BudgetSnapshot, 0, len(s.budgets))
	for _, b := range s.budgets {
		result = append(result, *b)
	}
	return result
}
