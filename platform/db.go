package platform

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite connection for advertiser and auction persistence.
type DB struct {
	conn *sql.DB
}

// NewDB opens a SQLite database at the given path and creates tables if needed.
func NewDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Enable WAL mode for better concurrent read performance
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.createTables(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

func (db *DB) createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS advertisers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		intent TEXT NOT NULL,
		embedding TEXT NOT NULL,
		sigma REAL NOT NULL,
		bid_price REAL NOT NULL,
		budget_total REAL NOT NULL,
		budget_spent REAL NOT NULL DEFAULT 0,
		currency TEXT NOT NULL DEFAULT 'USD',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS auctions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		intent TEXT NOT NULL,
		winner_id TEXT,
		payment REAL,
		currency TEXT,
		bid_count INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	return nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// InsertAdvertiser stores a position and its budget in the advertisers table.
func (db *DB) InsertAdvertiser(pos *Position, budgetTotal float64) error {
	embJSON, err := json.Marshal(pos.Embedding)
	if err != nil {
		return fmt.Errorf("marshal embedding: %w", err)
	}

	_, err = db.conn.Exec(
		`INSERT INTO advertisers (id, name, intent, embedding, sigma, bid_price, budget_total, currency)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		pos.ID, pos.Name, pos.Intent, string(embJSON),
		pos.Sigma, pos.BidPrice, budgetTotal, pos.Currency,
	)
	if err != nil {
		return fmt.Errorf("insert advertiser: %w", err)
	}
	return nil
}

// GetAdvertiser loads a single advertiser by ID.
func (db *DB) GetAdvertiser(id string) (*Position, error) {
	var pos Position
	var embJSON string
	err := db.conn.QueryRow(
		`SELECT id, name, intent, embedding, sigma, bid_price, currency FROM advertisers WHERE id = ?`, id,
	).Scan(&pos.ID, &pos.Name, &pos.Intent, &embJSON, &pos.Sigma, &pos.BidPrice, &pos.Currency)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get advertiser: %w", err)
	}

	if err := json.Unmarshal([]byte(embJSON), &pos.Embedding); err != nil {
		return nil, fmt.Errorf("unmarshal embedding: %w", err)
	}
	return &pos, nil
}

// GetAllAdvertisers loads all advertisers from the database.
func (db *DB) GetAllAdvertisers() ([]*Position, error) {
	rows, err := db.conn.Query(
		`SELECT id, name, intent, embedding, sigma, bid_price, currency FROM advertisers`,
	)
	if err != nil {
		return nil, fmt.Errorf("query advertisers: %w", err)
	}
	defer rows.Close()

	var positions []*Position
	for rows.Next() {
		var pos Position
		var embJSON string
		if err := rows.Scan(&pos.ID, &pos.Name, &pos.Intent, &embJSON, &pos.Sigma, &pos.BidPrice, &pos.Currency); err != nil {
			return nil, fmt.Errorf("scan advertiser: %w", err)
		}
		if err := json.Unmarshal([]byte(embJSON), &pos.Embedding); err != nil {
			return nil, fmt.Errorf("unmarshal embedding: %w", err)
		}
		positions = append(positions, &pos)
	}
	return positions, rows.Err()
}

// UpdateAdvertiser updates mutable fields. Embedding should be updated separately if intent changed.
func (db *DB) UpdateAdvertiser(id string, name, intent string, embedding []float64, sigma, bidPrice float64) error {
	embJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("marshal embedding: %w", err)
	}

	result, err := db.conn.Exec(
		`UPDATE advertisers SET name = ?, intent = ?, embedding = ?, sigma = ?, bid_price = ? WHERE id = ?`,
		name, intent, string(embJSON), sigma, bidPrice, id,
	)
	if err != nil {
		return fmt.Errorf("update advertiser: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("advertiser %s not found", id)
	}
	return nil
}

// UpdateBudget updates the budget total for an advertiser.
func (db *DB) UpdateBudget(id string, budgetTotal float64) error {
	result, err := db.conn.Exec(`UPDATE advertisers SET budget_total = ? WHERE id = ?`, budgetTotal, id)
	if err != nil {
		return fmt.Errorf("update budget: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("advertiser %s not found", id)
	}
	return nil
}

// DeleteAdvertiser removes an advertiser from the database.
func (db *DB) DeleteAdvertiser(id string) error {
	result, err := db.conn.Exec(`DELETE FROM advertisers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete advertiser: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("advertiser %s not found", id)
	}
	return nil
}

// BudgetRow holds budget info from the DB.
type BudgetRow struct {
	Total    float64
	Spent    float64
	Currency string
}

// GetBudget returns budget info for an advertiser.
func (db *DB) GetBudget(id string) (*BudgetRow, error) {
	var b BudgetRow
	err := db.conn.QueryRow(
		`SELECT budget_total, budget_spent, currency FROM advertisers WHERE id = ?`, id,
	).Scan(&b.Total, &b.Spent, &b.Currency)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get budget: %w", err)
	}
	return &b, nil
}

// Charge deducts amount from an advertiser's budget. Returns false if insufficient funds.
func (db *DB) Charge(id string, amount float64) (bool, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var total, spent float64
	err = tx.QueryRow(
		`SELECT budget_total, budget_spent FROM advertisers WHERE id = ?`, id,
	).Scan(&total, &spent)
	if err != nil {
		return false, err
	}

	if total-spent < amount {
		return false, nil
	}

	_, err = tx.Exec(`UPDATE advertisers SET budget_spent = budget_spent + ? WHERE id = ?`, amount, id)
	if err != nil {
		return false, err
	}

	return true, tx.Commit()
}

// LogAuction records a completed auction.
func (db *DB) LogAuction(intent, winnerID string, payment float64, currency string, bidCount int) {
	_, err := db.conn.Exec(
		`INSERT INTO auctions (intent, winner_id, payment, currency, bid_count) VALUES (?, ?, ?, ?, ?)`,
		intent, winnerID, payment, currency, bidCount,
	)
	if err != nil {
		log.Printf("WARN: failed to log auction: %v", err)
	}
}

// Stats holds aggregate auction statistics.
type Stats struct {
	AuctionCount     int     `json:"auction_count"`
	TotalSpend       float64 `json:"total_spend"`
	PublisherRevenue float64 `json:"publisher_revenue"`
	ExchangeRevenue  float64 `json:"exchange_revenue"`
}

const exchangeCut = 0.15

// GetStats returns aggregate auction statistics.
func (db *DB) GetStats() (*Stats, error) {
	var s Stats
	err := db.conn.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(payment), 0) FROM auctions`,
	).Scan(&s.AuctionCount, &s.TotalSpend)
	if err != nil {
		return nil, fmt.Errorf("get stats: %w", err)
	}
	s.ExchangeRevenue = s.TotalSpend * exchangeCut
	s.PublisherRevenue = s.TotalSpend - s.ExchangeRevenue
	return &s, nil
}

// ResetStats deletes all auction log entries.
func (db *DB) ResetStats() error {
	_, err := db.conn.Exec(`DELETE FROM auctions`)
	return err
}

// NextID returns the next advertiser ID based on current max.
func (db *DB) NextID() (int64, error) {
	var maxID sql.NullInt64
	err := db.conn.QueryRow(
		`SELECT MAX(CAST(SUBSTR(id, 5) AS INTEGER)) FROM advertisers WHERE id LIKE 'adv-%'`,
	).Scan(&maxID)
	if err != nil {
		return 1, nil
	}
	if !maxID.Valid {
		return 1, nil
	}
	return maxID.Int64 + 1, nil
}
