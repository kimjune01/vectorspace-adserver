package platform

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Position represents a registered advertiser's public position in embedding space.
type Position struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Intent    string    `json:"intent"`
	Embedding []float64 `json:"-"`
	Sigma     float64   `json:"sigma"`
	BidPrice  float64   `json:"bid_price"`
	Currency  string    `json:"currency"`
}

// PositionRegistry stores advertiser positions with thread-safe access.
// When a DB is set, it persists changes and uses in-memory cache for reads.
type PositionRegistry struct {
	mu        sync.RWMutex
	positions map[string]*Position
	nextID    atomic.Int64
	Embedder  *Embedder
	db        *DB
}

func NewPositionRegistry(embedder *Embedder) *PositionRegistry {
	return &PositionRegistry{
		positions: make(map[string]*Position),
		Embedder:  embedder,
	}
}

// SetDB attaches a database and loads existing advertisers into the in-memory cache.
func (r *PositionRegistry) SetDB(db *DB) error {
	r.db = db

	positions, err := db.GetAllAdvertisers()
	if err != nil {
		return fmt.Errorf("load advertisers from db: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range positions {
		r.positions[p.ID] = p
	}

	// Set nextID to max existing ID + 1
	nextID, err := db.NextID()
	if err == nil {
		r.nextID.Store(nextID - 1) // Add(1) will make it nextID
	}

	return nil
}

// Register adds a new advertiser position. It calls the sidecar to embed the intent text.
func (r *PositionRegistry) Register(name, intent string, sigma, bidPrice float64, currency string) (*Position, error) {
	embedding, err := r.Embedder.Embed(intent)
	if err != nil {
		return nil, fmt.Errorf("embed intent: %w", err)
	}

	id := fmt.Sprintf("adv-%d", r.nextID.Add(1))

	pos := &Position{
		ID:        id,
		Name:      name,
		Intent:    intent,
		Embedding: embedding,
		Sigma:     sigma,
		BidPrice:  bidPrice,
		Currency:  currency,
	}

	r.mu.Lock()
	r.positions[id] = pos
	r.mu.Unlock()

	return pos, nil
}

// RegisterWithBudget registers and persists to DB with budget.
func (r *PositionRegistry) RegisterWithBudget(name, intent string, sigma, bidPrice, budget float64, currency string) (*Position, error) {
	pos, err := r.Register(name, intent, sigma, bidPrice, currency)
	if err != nil {
		return nil, err
	}

	if r.db != nil {
		if err := r.db.InsertAdvertiser(pos, budget); err != nil {
			return nil, fmt.Errorf("persist advertiser: %w", err)
		}
	}

	return pos, nil
}

// Update modifies an existing advertiser. Re-embeds if intent changed.
func (r *PositionRegistry) Update(id, name, intent string, sigma, bidPrice float64) (*Position, error) {
	r.mu.RLock()
	existing := r.positions[id]
	r.mu.RUnlock()

	if existing == nil {
		return nil, fmt.Errorf("advertiser %s not found", id)
	}

	embedding := existing.Embedding
	if intent != "" && intent != existing.Intent {
		var err error
		embedding, err = r.Embedder.Embed(intent)
		if err != nil {
			return nil, fmt.Errorf("embed intent: %w", err)
		}
	}

	if name == "" {
		name = existing.Name
	}
	if intent == "" {
		intent = existing.Intent
	}
	if sigma == 0 {
		sigma = existing.Sigma
	}
	if bidPrice == 0 {
		bidPrice = existing.BidPrice
	}

	updated := &Position{
		ID:        id,
		Name:      name,
		Intent:    intent,
		Embedding: embedding,
		Sigma:     sigma,
		BidPrice:  bidPrice,
		Currency:  existing.Currency,
	}

	r.mu.Lock()
	r.positions[id] = updated
	r.mu.Unlock()

	if r.db != nil {
		if err := r.db.UpdateAdvertiser(id, name, intent, embedding, sigma, bidPrice); err != nil {
			return nil, fmt.Errorf("persist update: %w", err)
		}
	}

	return updated, nil
}

// Delete removes an advertiser.
func (r *PositionRegistry) Delete(id string) error {
	r.mu.Lock()
	_, exists := r.positions[id]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("advertiser %s not found", id)
	}
	delete(r.positions, id)
	r.mu.Unlock()

	if r.db != nil {
		if err := r.db.DeleteAdvertiser(id); err != nil {
			return err
		}
	}

	return nil
}

// Get returns a single position by ID, or nil if not found.
func (r *PositionRegistry) Get(id string) *Position {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.positions[id]
}

// GetAll returns a snapshot of all registered positions.
func (r *PositionRegistry) GetAll() []*Position {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Position, 0, len(r.positions))
	for _, p := range r.positions {
		result = append(result, p)
	}
	return result
}
