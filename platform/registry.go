package platform

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
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
	URL       string    `json:"url"`
	// BudgetID points at the position whose budget this position spends from.
	// Empty means the position holds its own budget. Keyword-group members
	// all point at the group head, so one imported campaign spends one budget.
	BudgetID string `json:"budget_id,omitempty"`
}

// BudgetKey returns the ID whose budget this position draws down.
func (p *Position) BudgetKey() string {
	if p.BudgetID != "" {
		return p.BudgetID
	}
	return p.ID
}

// RelocationFeeConfig controls the fee schedule for position changes.
type RelocationFeeConfig struct {
	// EntryBond is the flat fee charged on initial registration (default 0).
	EntryBond float64
	// DistanceFactor multiplied by sqrt(distance²) gives the move cost.
	// Fee = DistanceFactor * sqrt(distanceMoved). Default 0 (no move fees).
	DistanceFactor float64
}

// PositionRegistry stores advertiser positions with thread-safe access.
// When a DB is set, it persists changes and uses in-memory cache for reads.
type PositionRegistry struct {
	mu        sync.RWMutex
	positions map[string]*Position
	nextID    atomic.Int64
	Embedder  *Embedder
	db        *DB
	budgets   *BudgetTracker

	// Relocation fee config
	RelocationFees RelocationFeeConfig

	// Cached embedding version hash; invalidated on mutation.
	embeddingsVersion string
}

func NewPositionRegistry(embedder *Embedder) *PositionRegistry {
	return &PositionRegistry{
		positions: make(map[string]*Position),
		Embedder:  embedder,
	}
}

// SetBudgetTracker attaches a budget tracker for charging relocation fees.
func (r *PositionRegistry) SetBudgetTracker(bt *BudgetTracker) {
	r.budgets = bt
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
	r.invalidateVersion()

	// Set nextID to max existing ID + 1
	nextID, err := db.NextID()
	if err == nil {
		r.nextID.Store(nextID - 1) // Add(1) will make it nextID
	}

	return nil
}

// Register adds a new advertiser position. It calls the sidecar to embed the intent text.
func (r *PositionRegistry) Register(name, intent string, sigma, bidPrice float64, currency, url string) (*Position, error) {
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
		URL:       url,
	}

	r.mu.Lock()
	r.positions[id] = pos
	r.invalidateVersion()
	r.mu.Unlock()

	return pos, nil
}

// RegisterWithBudget registers and persists to DB with budget.
// Records the entry bond in position history. The caller (handler) is responsible
// for charging the entry bond from the budget after calling Budgets.Set.
func (r *PositionRegistry) RegisterWithBudget(name, intent string, sigma, bidPrice, budget float64, currency, url string) (*Position, error) {
	pos, err := r.Register(name, intent, sigma, bidPrice, currency, url)
	if err != nil {
		return nil, err
	}

	if r.db != nil {
		if err := r.db.InsertAdvertiser(pos, budget); err != nil {
			return nil, fmt.Errorf("persist advertiser: %w", err)
		}
		// Record initial position with entry bond amount
		r.db.RecordPositionChange(pos.ID, pos.Intent, pos.Embedding, pos.Sigma, pos.BidPrice, 0, r.RelocationFees.EntryBond)
	}

	return pos, nil
}

// RegisterKeywordGroupWithBudget registers one position per keyword, all
// sharing a single budget held by the first position (the group head).
// This is the import path for keyword campaigns: each keyword becomes a
// σ-circle at its own embedding (σ = 0 by default, the exact-match limit),
// and the group spends one budget, matching how a keyword campaign behaves
// on the platform it was imported from.
// See: june.kim/keywords-are-tiny-circles
func (r *PositionRegistry) RegisterKeywordGroupWithBudget(name string, keywords []string, sigma, bidPrice, budget float64, currency, url string) ([]*Position, error) {
	// Hygiene: trim, drop empties, deduplicate.
	cleaned := make([]string, 0, len(keywords))
	seen := make(map[string]bool)
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" || seen[kw] {
			continue
		}
		seen[kw] = true
		cleaned = append(cleaned, kw)
	}
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("keywords must not be empty")
	}

	// The group is all-or-nothing: any failure rolls back every position
	// registered so far, in memory and in the DB.
	positions := make([]*Position, 0, len(cleaned))
	rollback := func() {
		for _, p := range positions {
			r.mu.Lock()
			delete(r.positions, p.ID)
			r.invalidateVersion()
			r.mu.Unlock()
			if r.db != nil {
				r.db.DeleteAdvertiser(p.ID)
			}
		}
	}

	var headID string
	for i, kw := range cleaned {
		pos, err := r.Register(fmt.Sprintf("%s [%s]", name, kw), kw, sigma, bidPrice, currency, url)
		if err != nil {
			rollback()
			return nil, fmt.Errorf("embed keyword %q: %w", kw, err)
		}

		posBudget := 0.0
		if i == 0 {
			headID = pos.ID
			posBudget = budget
		} else {
			pos.BudgetID = headID
			r.mu.Lock()
			r.positions[pos.ID] = pos
			r.mu.Unlock()
		}

		if r.db != nil {
			if err := r.db.InsertAdvertiser(pos, posBudget); err != nil {
				r.mu.Lock()
				delete(r.positions, pos.ID)
				r.invalidateVersion()
				r.mu.Unlock()
				rollback()
				return nil, fmt.Errorf("persist keyword position %q: %w", kw, err)
			}
			if err := r.db.RecordPositionChange(pos.ID, pos.Intent, pos.Embedding, pos.Sigma, pos.BidPrice, 0, 0); err != nil {
				r.mu.Lock()
				delete(r.positions, pos.ID)
				r.invalidateVersion()
				r.mu.Unlock()
				r.db.DeleteAdvertiser(pos.ID)
				rollback()
				return nil, fmt.Errorf("record position history %q: %w", kw, err)
			}
		}

		positions = append(positions, pos)
	}

	return positions, nil
}

// ComputeRelocationFee calculates the fee for moving a position by the given distance².
func (r *PositionRegistry) ComputeRelocationFee(distanceSquared float64) float64 {
	if distanceSquared <= 0 || r.RelocationFees.DistanceFactor <= 0 {
		return 0
	}
	return r.RelocationFees.DistanceFactor * math.Sqrt(distanceSquared)
}

// Update modifies an existing advertiser. Re-embeds if intent changed.
// A negative sigma means "keep the existing sigma" — zero is a real value
// (the keyword limit), so it cannot double as the omitted sentinel.
func (r *PositionRegistry) Update(id, name, intent, url string, sigma, bidPrice float64) (*Position, error) {
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
	if sigma < 0 {
		sigma = existing.Sigma
	}
	if bidPrice == 0 {
		bidPrice = existing.BidPrice
	}
	if url == "" {
		url = existing.URL
	}

	updated := &Position{
		ID:        id,
		Name:      name,
		Intent:    intent,
		Embedding: embedding,
		Sigma:     sigma,
		BidPrice:  bidPrice,
		Currency:  existing.Currency,
		URL:       url,
		BudgetID:  existing.BudgetID,
	}

	r.mu.Lock()
	r.positions[id] = updated
	r.invalidateVersion()
	r.mu.Unlock()

	if r.db != nil {
		if err := r.db.UpdateAdvertiser(id, name, intent, embedding, sigma, bidPrice, url); err != nil {
			return nil, fmt.Errorf("persist update: %w", err)
		}
		// Compute distance moved and relocation fee
		distMoved := squaredEuclidean(existing.Embedding, embedding)
		var relocationFee float64
		if distMoved > 0 && r.RelocationFees.DistanceFactor > 0 {
			relocationFee = r.RelocationFees.DistanceFactor * math.Sqrt(distMoved)
			// Charge from budget (keyword-group members spend the head's budget)
			if r.budgets != nil {
				r.budgets.Charge(existing.BudgetKey(), relocationFee)
			}
		}
		r.db.RecordPositionChange(id, intent, embedding, sigma, bidPrice, distMoved, relocationFee)
	}

	return updated, nil
}

// squaredEuclidean computes ||a - b||² between two vectors.
func squaredEuclidean(a, b []float64) float64 {
	sum := 0.0
	for i := range a {
		if i >= len(b) {
			break
		}
		d := a[i] - b[i]
		sum += d * d
	}
	return sum
}

// Delete removes an advertiser. Deleting a keyword-group head deletes the
// whole group: members without the head would point at a budget that no
// longer exists.
func (r *PositionRegistry) Delete(id string) error {
	r.mu.Lock()
	_, exists := r.positions[id]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("advertiser %s not found", id)
	}
	toDelete := []string{id}
	for memberID, p := range r.positions {
		if p.BudgetID == id {
			toDelete = append(toDelete, memberID)
		}
	}
	for _, did := range toDelete {
		delete(r.positions, did)
	}
	r.invalidateVersion()
	r.mu.Unlock()

	if r.db != nil {
		for _, did := range toDelete {
			if err := r.db.DeleteAdvertiser(did); err != nil {
				return err
			}
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

// EmbeddingsVersion returns a short hex hash of all position IDs + embeddings.
// The value is cached and invalidated whenever positions are mutated.
func (r *PositionRegistry) EmbeddingsVersion() string {
	r.mu.RLock()
	if v := r.embeddingsVersion; v != "" {
		r.mu.RUnlock()
		return v
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock.
	if r.embeddingsVersion != "" {
		return r.embeddingsVersion
	}

	r.embeddingsVersion = r.computeVersion()
	return r.embeddingsVersion
}

// invalidateVersion must be called with mu held (write lock).
func (r *PositionRegistry) invalidateVersion() {
	r.embeddingsVersion = ""
}

func (r *PositionRegistry) computeVersion() string {
	// Sort IDs for deterministic output.
	ids := make([]string, 0, len(r.positions))
	for id := range r.positions {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	h := sha256.New()
	buf := make([]byte, 8)
	for _, id := range ids {
		h.Write([]byte(id))
		for _, v := range r.positions[id].Embedding {
			binary.LittleEndian.PutUint64(buf, math.Float64bits(v))
			h.Write(buf)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)[:16]) // 32 hex chars
}
