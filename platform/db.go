package platform

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

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

	CREATE TABLE IF NOT EXISTS tokens (
		token TEXT PRIMARY KEY,
		advertiser_id TEXT NOT NULL REFERENCES advertisers(id) ON DELETE CASCADE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		auction_id INTEGER NOT NULL,
		advertiser_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		user_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS frequency_caps (
		advertiser_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		impression_count INTEGER NOT NULL DEFAULT 0,
		window_start DATETIME NOT NULL,
		PRIMARY KEY (advertiser_id, user_id)
	);

	CREATE TABLE IF NOT EXISTS publishers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		domain TEXT NOT NULL DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS publisher_tokens (
		token TEXT PRIMARY KEY,
		publisher_id TEXT NOT NULL REFERENCES publishers(id) ON DELETE CASCADE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS publisher_credentials (
		publisher_id TEXT PRIMARY KEY REFERENCES publishers(id) ON DELETE CASCADE,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS creatives (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		advertiser_id TEXT NOT NULL REFERENCES advertisers(id) ON DELETE CASCADE,
		title TEXT NOT NULL,
		subtitle TEXT NOT NULL DEFAULT '',
		active INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS intake_submissions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		email TEXT NOT NULL,
		company TEXT NOT NULL DEFAULT '',
		detail TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS position_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		advertiser_id TEXT NOT NULL REFERENCES advertisers(id) ON DELETE CASCADE,
		intent TEXT NOT NULL,
		embedding TEXT NOT NULL,
		sigma REAL NOT NULL,
		bid_price REAL NOT NULL,
		distance_moved REAL NOT NULL DEFAULT 0,
		relocation_fee REAL NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	// Idempotent migration: add publisher_id to auctions and events
	if err := db.migratePublisherID(); err != nil {
		return fmt.Errorf("migrate publisher_id: %w", err)
	}

	// Idempotent migration: add url to advertisers
	if err := db.migrateAdvertiserURL(); err != nil {
		return fmt.Errorf("migrate advertiser url: %w", err)
	}

	return nil
}

func (db *DB) migratePublisherID() error {
	for _, table := range []string{"auctions", "events"} {
		var hasColumn bool
		rows, err := db.conn.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
		if err != nil {
			return err
		}
		for rows.Next() {
			var cid int
			var name, typ string
			var notnull int
			var dflt sql.NullString
			var pk int
			if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
				rows.Close()
				return err
			}
			if name == "publisher_id" {
				hasColumn = true
			}
		}
		rows.Close()

		if !hasColumn {
			_, err := db.conn.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN publisher_id TEXT DEFAULT ''", table))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (db *DB) migrateAdvertiserURL() error {
	var hasColumn bool
	rows, err := db.conn.Query("PRAGMA table_info(advertisers)")
	if err != nil {
		return err
	}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "url" {
			hasColumn = true
		}
	}
	rows.Close()

	if !hasColumn {
		_, err := db.conn.Exec("ALTER TABLE advertisers ADD COLUMN url TEXT NOT NULL DEFAULT ''")
		if err != nil {
			return err
		}
	}
	return nil
}

// Conn returns the underlying sql.DB connection for sharing with other packages.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Exec runs a raw SQL statement. Used for seed data with custom timestamps.
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.conn.Exec(query, args...)
}

// InsertAdvertiser stores a position and its budget in the advertisers table.
func (db *DB) InsertAdvertiser(pos *Position, budgetTotal float64) error {
	embJSON, err := json.Marshal(pos.Embedding)
	if err != nil {
		return fmt.Errorf("marshal embedding: %w", err)
	}

	_, err = db.conn.Exec(
		`INSERT INTO advertisers (id, name, intent, embedding, sigma, bid_price, budget_total, currency, url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pos.ID, pos.Name, pos.Intent, string(embJSON),
		pos.Sigma, pos.BidPrice, budgetTotal, pos.Currency, pos.URL,
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
		`SELECT id, name, intent, embedding, sigma, bid_price, currency, url FROM advertisers WHERE id = ?`, id,
	).Scan(&pos.ID, &pos.Name, &pos.Intent, &embJSON, &pos.Sigma, &pos.BidPrice, &pos.Currency, &pos.URL)
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
		`SELECT id, name, intent, embedding, sigma, bid_price, currency, url FROM advertisers`,
	)
	if err != nil {
		return nil, fmt.Errorf("query advertisers: %w", err)
	}
	defer rows.Close()

	var positions []*Position
	for rows.Next() {
		var pos Position
		var embJSON string
		if err := rows.Scan(&pos.ID, &pos.Name, &pos.Intent, &embJSON, &pos.Sigma, &pos.BidPrice, &pos.Currency, &pos.URL); err != nil {
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
func (db *DB) UpdateAdvertiser(id string, name, intent string, embedding []float64, sigma, bidPrice float64, url string) error {
	embJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("marshal embedding: %w", err)
	}

	result, err := db.conn.Exec(
		`UPDATE advertisers SET name = ?, intent = ?, embedding = ?, sigma = ?, bid_price = ?, url = ? WHERE id = ?`,
		name, intent, string(embJSON), sigma, bidPrice, url, id,
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

// --- Position History ---

// PositionHistoryEntry represents a snapshot of an advertiser's position at a point in time.
type PositionHistoryEntry struct {
	ID             int64   `json:"id"`
	AdvertiserID   string  `json:"advertiser_id"`
	Intent         string  `json:"intent"`
	Sigma          float64 `json:"sigma"`
	BidPrice       float64 `json:"bid_price"`
	DistanceMoved  float64 `json:"distance_moved"`
	RelocationFee  float64 `json:"relocation_fee"`
	CreatedAt      string  `json:"created_at"`
}

// RecordPositionChange inserts a position history entry.
// distanceMoved is the squared Euclidean distance from the previous embedding.
// relocationFee is the fee charged for this move (0 for initial registration).
func (db *DB) RecordPositionChange(advertiserID, intent string, embedding []float64, sigma, bidPrice, distanceMoved, relocationFee float64) error {
	embJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("marshal embedding: %w", err)
	}
	_, err = db.conn.Exec(
		`INSERT INTO position_history (advertiser_id, intent, embedding, sigma, bid_price, distance_moved, relocation_fee) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		advertiserID, intent, string(embJSON), sigma, bidPrice, distanceMoved, relocationFee,
	)
	if err != nil {
		return fmt.Errorf("record position change: %w", err)
	}
	return nil
}

// GetPositionHistory returns the position history for an advertiser, newest first.
func (db *DB) GetPositionHistory(advertiserID string, limit int) ([]PositionHistoryEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.conn.Query(
		`SELECT id, advertiser_id, intent, sigma, bid_price, distance_moved, relocation_fee, created_at
		 FROM position_history WHERE advertiser_id = ? ORDER BY id DESC LIMIT ?`,
		advertiserID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get position history: %w", err)
	}
	defer rows.Close()

	var entries []PositionHistoryEntry
	for rows.Next() {
		var e PositionHistoryEntry
		if err := rows.Scan(&e.ID, &e.AdvertiserID, &e.Intent, &e.Sigma, &e.BidPrice, &e.DistanceMoved, &e.RelocationFee, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan position history: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// GetPositionCount returns how many position changes an advertiser has made.
func (db *DB) GetPositionCount(advertiserID string) (int, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM position_history WHERE advertiser_id = ?`, advertiserID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetTenureDays returns the number of days since the advertiser's last position change.
// Returns 0 if no history exists.
func (db *DB) GetTenureDays(advertiserID string) (float64, error) {
	var days float64
	err := db.conn.QueryRow(
		`SELECT JULIANDAY('now') - JULIANDAY(MAX(created_at)) FROM position_history WHERE advertiser_id = ?`,
		advertiserID,
	).Scan(&days)
	if err != nil {
		return 0, nil
	}
	return days, nil
}

// GetTotalRelocationFees returns the sum of all relocation fees collected.
func (db *DB) GetTotalRelocationFees() (float64, error) {
	var total sql.NullFloat64
	err := db.conn.QueryRow(
		`SELECT SUM(relocation_fee) FROM position_history WHERE relocation_fee > 0`,
	).Scan(&total)
	if err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Float64, nil
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
	AdvertiserCount  int     `json:"advertiser_count"`
	PublisherCount   int     `json:"publisher_count"`
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

	db.conn.QueryRow(`SELECT COUNT(*) FROM advertisers`).Scan(&s.AdvertiserCount)
	db.conn.QueryRow(`SELECT COUNT(*) FROM publishers`).Scan(&s.PublisherCount)

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

// --- Token Management ---

// GenerateToken creates a crypto/rand 32-char hex token for an advertiser.
func (db *DB) GenerateToken(advertiserID string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	_, err := db.conn.Exec(
		`INSERT INTO tokens (token, advertiser_id) VALUES (?, ?)`,
		token, advertiserID,
	)
	if err != nil {
		return "", fmt.Errorf("insert token: %w", err)
	}
	return token, nil
}

// LookupToken returns the advertiser_id for a token, or "" if not found.
func (db *DB) LookupToken(token string) (string, error) {
	var advertiserID string
	err := db.conn.QueryRow(
		`SELECT advertiser_id FROM tokens WHERE token = ?`, token,
	).Scan(&advertiserID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("lookup token: %w", err)
	}
	return advertiserID, nil
}

// GetAuctionPayment returns the payment and winner_id for an auction.
func (db *DB) GetAuctionPayment(auctionID int64) (string, float64, error) {
	var winnerID string
	var payment float64
	err := db.conn.QueryRow(
		`SELECT winner_id, payment FROM auctions WHERE id = ?`, auctionID,
	).Scan(&winnerID, &payment)
	if err != nil {
		return "", 0, fmt.Errorf("get auction payment: %w", err)
	}
	return winnerID, payment, nil
}

// HasClickEvent returns true if a click event already exists for this auction.
func (db *DB) HasClickEvent(auctionID int64) (bool, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM events WHERE auction_id = ? AND event_type = 'click'`, auctionID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check click event: %w", err)
	}
	return count > 0, nil
}

// --- Event Tracking ---

// LogEvent records an impression/click/viewable event.
func (db *DB) LogEvent(auctionID int64, advertiserID, eventType, userID string) error {
	_, err := db.conn.Exec(
		`INSERT INTO events (auction_id, advertiser_id, event_type, user_id) VALUES (?, ?, ?, ?)`,
		auctionID, advertiserID, eventType, userID,
	)
	if err != nil {
		return fmt.Errorf("log event: %w", err)
	}
	return nil
}

// CheckFrequencyCap returns true if the user is under the frequency cap for the advertiser.
func (db *DB) CheckFrequencyCap(advertiserID, userID string, maxPerWindow int, windowMinutes int) (bool, error) {
	cutoff := time.Now().Add(-time.Duration(windowMinutes) * time.Minute)

	var count int
	var windowStart time.Time
	err := db.conn.QueryRow(
		`SELECT impression_count, window_start FROM frequency_caps WHERE advertiser_id = ? AND user_id = ?`,
		advertiserID, userID,
	).Scan(&count, &windowStart)

	if err == sql.ErrNoRows {
		return true, nil // no record = under cap
	}
	if err != nil {
		return false, fmt.Errorf("check frequency cap: %w", err)
	}

	// If window has expired, reset
	if windowStart.Before(cutoff) {
		return true, nil
	}

	return count < maxPerWindow, nil
}

// IncrementFrequencyCap increments the impression count for a user/advertiser pair.
func (db *DB) IncrementFrequencyCap(advertiserID, userID string, windowMinutes int) error {
	cutoff := time.Now().Add(-time.Duration(windowMinutes) * time.Minute)
	now := time.Now()

	// Check if existing record is within window
	var windowStart time.Time
	err := db.conn.QueryRow(
		`SELECT window_start FROM frequency_caps WHERE advertiser_id = ? AND user_id = ?`,
		advertiserID, userID,
	).Scan(&windowStart)

	if err == sql.ErrNoRows || windowStart.Before(cutoff) {
		// Insert or reset
		_, err = db.conn.Exec(
			`INSERT OR REPLACE INTO frequency_caps (advertiser_id, user_id, impression_count, window_start) VALUES (?, ?, 1, ?)`,
			advertiserID, userID, now,
		)
	} else if err == nil {
		// Increment within window
		_, err = db.conn.Exec(
			`UPDATE frequency_caps SET impression_count = impression_count + 1 WHERE advertiser_id = ? AND user_id = ?`,
			advertiserID, userID,
		)
	}

	if err != nil {
		return fmt.Errorf("increment frequency cap: %w", err)
	}
	return nil
}

// --- Auction Queries ---

// AuctionRow represents a single auction record.
type AuctionRow struct {
	ID         int64   `json:"id"`
	Intent     string  `json:"intent"`
	WinnerID   string  `json:"winner_id"`
	WinnerName string  `json:"winner_name"`
	Payment    float64 `json:"payment"`
	Currency   string  `json:"currency"`
	BidCount   int     `json:"bid_count"`
	CreatedAt  string  `json:"created_at"`
}

// GetAuctionsByAdvertiser returns paginated auctions won by a specific advertiser.
func (db *DB) GetAuctionsByAdvertiser(advertiserID string, limit, offset int) ([]AuctionRow, int, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM auctions WHERE winner_id = ?`, advertiserID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count auctions: %w", err)
	}

	rows, err := db.conn.Query(
		`SELECT a.id, a.intent, a.winner_id, COALESCE(adv.name, ''), a.payment, a.currency, a.bid_count, a.created_at
		 FROM auctions a LEFT JOIN advertisers adv ON a.winner_id = adv.id
		 WHERE a.winner_id = ? ORDER BY a.id DESC LIMIT ? OFFSET ?`,
		advertiserID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query auctions: %w", err)
	}
	defer rows.Close()

	var auctions []AuctionRow
	for rows.Next() {
		var a AuctionRow
		if err := rows.Scan(&a.ID, &a.Intent, &a.WinnerID, &a.WinnerName, &a.Payment, &a.Currency, &a.BidCount, &a.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan auction: %w", err)
		}
		auctions = append(auctions, a)
	}
	return auctions, total, rows.Err()
}

// GetAllAuctions returns paginated auctions with optional filters.
func (db *DB) GetAllAuctions(limit, offset int, winnerFilter, intentFilter string) ([]AuctionRow, int, error) {
	if limit <= 0 {
		limit = 20
	}

	where := "1=1"
	args := []interface{}{}
	if winnerFilter != "" {
		where += " AND a.winner_id = ?"
		args = append(args, winnerFilter)
	}
	if intentFilter != "" {
		where += " AND a.intent LIKE ?"
		args = append(args, "%"+intentFilter+"%")
	}

	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	err := db.conn.QueryRow(
		fmt.Sprintf(`SELECT COUNT(*) FROM auctions a WHERE %s`, where), countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count auctions: %w", err)
	}

	queryArgs := append(args, limit, offset)
	rows, err := db.conn.Query(
		fmt.Sprintf(`SELECT a.id, a.intent, a.winner_id, COALESCE(adv.name, ''), a.payment, a.currency, a.bid_count, a.created_at
		 FROM auctions a LEFT JOIN advertisers adv ON a.winner_id = adv.id
		 WHERE %s ORDER BY a.id DESC LIMIT ? OFFSET ?`, where),
		queryArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query auctions: %w", err)
	}
	defer rows.Close()

	var auctions []AuctionRow
	for rows.Next() {
		var a AuctionRow
		if err := rows.Scan(&a.ID, &a.Intent, &a.WinnerID, &a.WinnerName, &a.Payment, &a.Currency, &a.BidCount, &a.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan auction: %w", err)
		}
		auctions = append(auctions, a)
	}
	return auctions, total, rows.Err()
}

// --- Revenue Analytics ---

// RevenuePeriod represents aggregated revenue for a time period.
type RevenuePeriod struct {
	Period           string  `json:"period"`
	AuctionCount     int     `json:"auction_count"`
	TotalSpend       float64 `json:"total_spend"`
	PublisherRevenue float64 `json:"publisher_revenue"`
	ExchangeRevenue  float64 `json:"exchange_revenue"`
}

// GetRevenueByPeriod returns revenue aggregated by day, week, or month.
func (db *DB) GetRevenueByPeriod(groupBy string) ([]RevenuePeriod, error) {
	var dateExpr string
	switch groupBy {
	case "week":
		dateExpr = "strftime('%Y-W%W', created_at)"
	case "month":
		dateExpr = "strftime('%Y-%m', created_at)"
	default:
		dateExpr = "date(created_at)"
	}

	rows, err := db.conn.Query(fmt.Sprintf(
		`SELECT %s as period, COUNT(*) as cnt, COALESCE(SUM(payment), 0) as total
		 FROM auctions GROUP BY period ORDER BY period DESC`, dateExpr,
	))
	if err != nil {
		return nil, fmt.Errorf("revenue by period: %w", err)
	}
	defer rows.Close()

	var result []RevenuePeriod
	for rows.Next() {
		var rp RevenuePeriod
		if err := rows.Scan(&rp.Period, &rp.AuctionCount, &rp.TotalSpend); err != nil {
			return nil, fmt.Errorf("scan revenue: %w", err)
		}
		rp.ExchangeRevenue = rp.TotalSpend * exchangeCut
		rp.PublisherRevenue = rp.TotalSpend - rp.ExchangeRevenue
		result = append(result, rp)
	}
	return result, rows.Err()
}

// --- Top Advertisers ---

// AdvertiserSpend represents an advertiser's total spend.
type AdvertiserSpend struct {
	AdvertiserID string  `json:"advertiser_id"`
	Name         string  `json:"name"`
	TotalSpend   float64 `json:"total_spend"`
	AuctionCount int     `json:"auction_count"`
}

// GetTopAdvertisersBySpend returns the top N advertisers by total auction spend.
func (db *DB) GetTopAdvertisersBySpend(limit int) ([]AdvertiserSpend, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := db.conn.Query(
		`SELECT a.winner_id, COALESCE(adv.name, a.winner_id), SUM(a.payment), COUNT(*)
		 FROM auctions a
		 LEFT JOIN advertisers adv ON a.winner_id = adv.id
		 WHERE a.winner_id IS NOT NULL
		 GROUP BY a.winner_id
		 ORDER BY SUM(a.payment) DESC
		 LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("top advertisers: %w", err)
	}
	defer rows.Close()

	var result []AdvertiserSpend
	for rows.Next() {
		var as AdvertiserSpend
		if err := rows.Scan(&as.AdvertiserID, &as.Name, &as.TotalSpend, &as.AuctionCount); err != nil {
			return nil, fmt.Errorf("scan advertiser spend: %w", err)
		}
		result = append(result, as)
	}
	return result, rows.Err()
}

// --- Advertiser Budget Overview ---

// AdvertiserWithBudget represents an advertiser with their budget data.
type AdvertiserWithBudget struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Intent       string  `json:"intent"`
	Sigma        float64 `json:"sigma"`
	BidPrice     float64 `json:"bid_price"`
	BudgetTotal  float64 `json:"budget_total"`
	BudgetSpent  float64 `json:"budget_spent"`
	Currency     string  `json:"currency"`
	URL          string  `json:"url"`
}

// GetAllAdvertisersWithBudget returns all advertisers with their budget data.
func (db *DB) GetAllAdvertisersWithBudget() ([]AdvertiserWithBudget, error) {
	rows, err := db.conn.Query(
		`SELECT id, name, intent, sigma, bid_price, budget_total, budget_spent, currency, url FROM advertisers ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("query advertisers with budget: %w", err)
	}
	defer rows.Close()

	var result []AdvertiserWithBudget
	for rows.Next() {
		var a AdvertiserWithBudget
		if err := rows.Scan(&a.ID, &a.Name, &a.Intent, &a.Sigma, &a.BidPrice, &a.BudgetTotal, &a.BudgetSpent, &a.Currency, &a.URL); err != nil {
			return nil, fmt.Errorf("scan advertiser: %w", err)
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// --- Event Stats ---

// EventStats holds impression/click/viewable counts.
type EventStats struct {
	Impressions int `json:"impressions"`
	Clicks      int `json:"clicks"`
	Viewable    int `json:"viewable"`
}

// GetEventStats returns event counts for an advertiser (or all if advertiserID is empty).
func (db *DB) GetEventStats(advertiserID string) (*EventStats, error) {
	var stats EventStats
	var query string
	var args []interface{}

	if advertiserID != "" {
		query = `SELECT
			COALESCE(SUM(CASE WHEN event_type = 'impression' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN event_type = 'click' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN event_type = 'viewable' THEN 1 ELSE 0 END), 0)
		FROM events WHERE advertiser_id = ?`
		args = []interface{}{advertiserID}
	} else {
		query = `SELECT
			COALESCE(SUM(CASE WHEN event_type = 'impression' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN event_type = 'click' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN event_type = 'viewable' THEN 1 ELSE 0 END), 0)
		FROM events`
	}

	err := db.conn.QueryRow(query, args...).Scan(&stats.Impressions, &stats.Clicks, &stats.Viewable)
	if err != nil {
		return nil, fmt.Errorf("get event stats: %w", err)
	}
	return &stats, nil
}

// LogAuctionReturningID records a completed auction and returns the auto-increment ID.
func (db *DB) LogAuctionReturningID(intent, winnerID string, payment float64, currency string, bidCount int) (int64, error) {
	result, err := db.conn.Exec(
		`INSERT INTO auctions (intent, winner_id, payment, currency, bid_count) VALUES (?, ?, ?, ?, ?)`,
		intent, winnerID, payment, currency, bidCount,
	)
	if err != nil {
		return 0, fmt.Errorf("log auction: %w", err)
	}
	return result.LastInsertId()
}

// LogAuctionReturningIDWithPublisher records an auction with publisher_id and returns the ID.
func (db *DB) LogAuctionReturningIDWithPublisher(intent, winnerID string, payment float64, currency string, bidCount int, publisherID string) (int64, error) {
	result, err := db.conn.Exec(
		`INSERT INTO auctions (intent, winner_id, payment, currency, bid_count, publisher_id) VALUES (?, ?, ?, ?, ?, ?)`,
		intent, winnerID, payment, currency, bidCount, publisherID,
	)
	if err != nil {
		return 0, fmt.Errorf("log auction: %w", err)
	}
	return result.LastInsertId()
}

// LogEventWithPublisher records an event with publisher_id.
func (db *DB) LogEventWithPublisher(auctionID int64, advertiserID, eventType, userID, publisherID string) error {
	_, err := db.conn.Exec(
		`INSERT INTO events (auction_id, advertiser_id, event_type, user_id, publisher_id) VALUES (?, ?, ?, ?, ?)`,
		auctionID, advertiserID, eventType, userID, publisherID,
	)
	if err != nil {
		return fmt.Errorf("log event: %w", err)
	}
	return nil
}

// --- Publisher Management ---

// Publisher represents a publisher in the system.
type Publisher struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	CreatedAt string `json:"created_at"`
}

// GetAllPublishers returns all publishers ordered by ID.
func (db *DB) GetAllPublishers() ([]Publisher, error) {
	rows, err := db.conn.Query(
		`SELECT id, name, domain, created_at FROM publishers ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("get all publishers: %w", err)
	}
	defer rows.Close()

	var publishers []Publisher
	for rows.Next() {
		var p Publisher
		if err := rows.Scan(&p.ID, &p.Name, &p.Domain, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan publisher: %w", err)
		}
		publishers = append(publishers, p)
	}
	return publishers, rows.Err()
}

// InsertPublisher stores a new publisher.
func (db *DB) InsertPublisher(id, name, domain string) error {
	_, err := db.conn.Exec(
		`INSERT INTO publishers (id, name, domain) VALUES (?, ?, ?)`,
		id, name, domain,
	)
	if err != nil {
		return fmt.Errorf("insert publisher: %w", err)
	}
	return nil
}

// GetPublisher loads a publisher by ID.
func (db *DB) GetPublisher(id string) (*Publisher, error) {
	var p Publisher
	err := db.conn.QueryRow(
		`SELECT id, name, domain, created_at FROM publishers WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Domain, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get publisher: %w", err)
	}
	return &p, nil
}

// NextPublisherID returns the next publisher ID based on current max.
func (db *DB) NextPublisherID() (int64, error) {
	var maxID sql.NullInt64
	err := db.conn.QueryRow(
		`SELECT MAX(CAST(SUBSTR(id, 5) AS INTEGER)) FROM publishers WHERE id LIKE 'pub-%'`,
	).Scan(&maxID)
	if err != nil {
		return 1, nil
	}
	if !maxID.Valid {
		return 1, nil
	}
	return maxID.Int64 + 1, nil
}

// GeneratePublisherToken creates a token for a publisher.
func (db *DB) GeneratePublisherToken(publisherID string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	_, err := db.conn.Exec(
		`INSERT INTO publisher_tokens (token, publisher_id) VALUES (?, ?)`,
		token, publisherID,
	)
	if err != nil {
		return "", fmt.Errorf("insert publisher token: %w", err)
	}
	return token, nil
}

// LookupPublisherToken returns the publisher_id for a token, or "" if not found.
func (db *DB) LookupPublisherToken(token string) (string, error) {
	var publisherID string
	err := db.conn.QueryRow(
		`SELECT publisher_id FROM publisher_tokens WHERE token = ?`, token,
	).Scan(&publisherID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("lookup publisher token: %w", err)
	}
	return publisherID, nil
}

// --- Publisher Analytics ---

// GetPublisherRevenue returns total revenue (85% of payments) for a publisher.
func (db *DB) GetPublisherRevenue(publisherID string) (float64, error) {
	var total float64
	err := db.conn.QueryRow(
		`SELECT COALESCE(SUM(payment), 0) FROM auctions WHERE publisher_id = ?`, publisherID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get publisher revenue: %w", err)
	}
	return total * (1 - exchangeCut), nil
}

// GetPublisherRevenueByPeriod returns revenue aggregated by day, week, or month for a publisher.
func (db *DB) GetPublisherRevenueByPeriod(publisherID, groupBy string) ([]RevenuePeriod, error) {
	var dateExpr string
	switch groupBy {
	case "week":
		dateExpr = "strftime('%Y-W%W', created_at)"
	case "month":
		dateExpr = "strftime('%Y-%m', created_at)"
	default:
		dateExpr = "date(created_at)"
	}

	rows, err := db.conn.Query(fmt.Sprintf(
		`SELECT %s as period, COUNT(*) as cnt, COALESCE(SUM(payment), 0) as total
		 FROM auctions WHERE publisher_id = ? GROUP BY period ORDER BY period DESC`, dateExpr,
	), publisherID)
	if err != nil {
		return nil, fmt.Errorf("publisher revenue by period: %w", err)
	}
	defer rows.Close()

	var result []RevenuePeriod
	for rows.Next() {
		var rp RevenuePeriod
		if err := rows.Scan(&rp.Period, &rp.AuctionCount, &rp.TotalSpend); err != nil {
			return nil, fmt.Errorf("scan revenue: %w", err)
		}
		rp.ExchangeRevenue = rp.TotalSpend * exchangeCut
		rp.PublisherRevenue = rp.TotalSpend - rp.ExchangeRevenue
		result = append(result, rp)
	}
	return result, rows.Err()
}

// GetPublisherEventStats returns event counts for a publisher.
func (db *DB) GetPublisherEventStats(publisherID string) (*EventStats, error) {
	var stats EventStats
	err := db.conn.QueryRow(
		`SELECT
			COALESCE(SUM(CASE WHEN event_type = 'impression' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN event_type = 'click' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN event_type = 'viewable' THEN 1 ELSE 0 END), 0)
		FROM events WHERE publisher_id = ?`, publisherID,
	).Scan(&stats.Impressions, &stats.Clicks, &stats.Viewable)
	if err != nil {
		return nil, fmt.Errorf("get publisher event stats: %w", err)
	}
	return &stats, nil
}

// GetAuctionsByPublisher returns paginated auctions for a publisher.
func (db *DB) GetAuctionsByPublisher(publisherID string, limit, offset int) ([]AuctionRow, int, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM auctions WHERE publisher_id = ?`, publisherID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count auctions: %w", err)
	}

	rows, err := db.conn.Query(
		`SELECT a.id, a.intent, a.winner_id, COALESCE(adv.name, ''), a.payment, a.currency, a.bid_count, a.created_at
		 FROM auctions a LEFT JOIN advertisers adv ON a.winner_id = adv.id
		 WHERE a.publisher_id = ? ORDER BY a.id DESC LIMIT ? OFFSET ?`,
		publisherID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query auctions: %w", err)
	}
	defer rows.Close()

	var auctions []AuctionRow
	for rows.Next() {
		var a AuctionRow
		if err := rows.Scan(&a.ID, &a.Intent, &a.WinnerID, &a.WinnerName, &a.Payment, &a.Currency, &a.BidCount, &a.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan auction: %w", err)
		}
		auctions = append(auctions, a)
	}
	return auctions, total, rows.Err()
}

// GetPublisherTopAdvertisers returns top advertisers by spend on a publisher's property.
func (db *DB) GetPublisherTopAdvertisers(publisherID string, limit int) ([]AdvertiserSpend, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := db.conn.Query(
		`SELECT a.winner_id, COALESCE(adv.name, a.winner_id), SUM(a.payment), COUNT(*)
		 FROM auctions a
		 LEFT JOIN advertisers adv ON a.winner_id = adv.id
		 WHERE a.publisher_id = ? AND a.winner_id IS NOT NULL
		 GROUP BY a.winner_id
		 ORDER BY SUM(a.payment) DESC
		 LIMIT ?`, publisherID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("publisher top advertisers: %w", err)
	}
	defer rows.Close()

	var result []AdvertiserSpend
	for rows.Next() {
		var as AdvertiserSpend
		if err := rows.Scan(&as.AdvertiserID, &as.Name, &as.TotalSpend, &as.AuctionCount); err != nil {
			return nil, fmt.Errorf("scan advertiser spend: %w", err)
		}
		result = append(result, as)
	}
	return result, rows.Err()
}

// PublisherStats holds aggregate stats for a publisher.
type PublisherStats struct {
	AuctionCount int     `json:"auction_count"`
	TotalRevenue float64 `json:"total_revenue"`
	Currency     string  `json:"currency"`
}

// GetPublisherStats returns aggregate stats for a publisher.
func (db *DB) GetPublisherStats(publisherID string) (*PublisherStats, error) {
	var s PublisherStats
	var totalPayment float64
	err := db.conn.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(payment), 0) FROM auctions WHERE publisher_id = ?`, publisherID,
	).Scan(&s.AuctionCount, &totalPayment)
	if err != nil {
		return nil, fmt.Errorf("get publisher stats: %w", err)
	}
	s.TotalRevenue = totalPayment * (1 - exchangeCut)
	s.Currency = "USD"
	return &s, nil
}

// --- Publisher Credentials ---

// InsertPublisherCredentials stores email/password credentials for a publisher.
func (db *DB) InsertPublisherCredentials(publisherID, email, passwordHash string) error {
	_, err := db.conn.Exec(
		`INSERT INTO publisher_credentials (publisher_id, email, password_hash) VALUES (?, ?, ?)`,
		publisherID, email, passwordHash,
	)
	if err != nil {
		return fmt.Errorf("insert publisher credentials: %w", err)
	}
	return nil
}

// LookupPublisherByEmail returns the publisherID and passwordHash for an email, or ("", "", nil) if not found.
func (db *DB) LookupPublisherByEmail(email string) (string, string, error) {
	var publisherID, passwordHash string
	err := db.conn.QueryRow(
		`SELECT publisher_id, password_hash FROM publisher_credentials WHERE email = ?`, email,
	).Scan(&publisherID, &passwordHash)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("lookup publisher by email: %w", err)
	}
	return publisherID, passwordHash, nil
}

// GetPublisherToken returns the first existing token for a publisher, or ("", nil) if none.
func (db *DB) GetPublisherToken(publisherID string) (string, error) {
	var token string
	err := db.conn.QueryRow(
		`SELECT token FROM publisher_tokens WHERE publisher_id = ? LIMIT 1`, publisherID,
	).Scan(&token)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get publisher token: %w", err)
	}
	return token, nil
}

// --- Creative Management ---

// Creative represents an ad creative (title + subtitle).
type Creative struct {
	ID           int64  `json:"id"`
	AdvertiserID string `json:"advertiser_id"`
	Title        string `json:"title"`
	Subtitle     string `json:"subtitle"`
	Active       bool   `json:"active"`
	CreatedAt    string `json:"created_at"`
}

// InsertCreative creates a new creative for an advertiser.
func (db *DB) InsertCreative(advertiserID, title, subtitle string) (int64, error) {
	result, err := db.conn.Exec(
		`INSERT INTO creatives (advertiser_id, title, subtitle) VALUES (?, ?, ?)`,
		advertiserID, title, subtitle,
	)
	if err != nil {
		return 0, fmt.Errorf("insert creative: %w", err)
	}
	return result.LastInsertId()
}

// UpdateCreative updates the title and subtitle of a creative.
func (db *DB) UpdateCreative(id int64, title, subtitle string) error {
	result, err := db.conn.Exec(
		`UPDATE creatives SET title = ?, subtitle = ? WHERE id = ?`,
		title, subtitle, id,
	)
	if err != nil {
		return fmt.Errorf("update creative: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("creative %d not found", id)
	}
	return nil
}

// DeleteCreative removes a creative by ID.
func (db *DB) DeleteCreative(id int64) error {
	result, err := db.conn.Exec(`DELETE FROM creatives WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete creative: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("creative %d not found", id)
	}
	return nil
}

// GetCreativesByAdvertiser returns all creatives for an advertiser.
func (db *DB) GetCreativesByAdvertiser(advertiserID string) ([]Creative, error) {
	rows, err := db.conn.Query(
		`SELECT id, advertiser_id, title, subtitle, active, created_at FROM creatives WHERE advertiser_id = ? ORDER BY id DESC`,
		advertiserID,
	)
	if err != nil {
		return nil, fmt.Errorf("get creatives: %w", err)
	}
	defer rows.Close()

	var creatives []Creative
	for rows.Next() {
		var c Creative
		var active int
		if err := rows.Scan(&c.ID, &c.AdvertiserID, &c.Title, &c.Subtitle, &active, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan creative: %w", err)
		}
		c.Active = active == 1
		creatives = append(creatives, c)
	}
	return creatives, rows.Err()
}

// GetActiveCreative returns the most recent active creative for an advertiser, or nil if none.
func (db *DB) GetActiveCreative(advertiserID string) (*Creative, error) {
	var c Creative
	var active int
	err := db.conn.QueryRow(
		`SELECT id, advertiser_id, title, subtitle, active, created_at FROM creatives WHERE advertiser_id = ? AND active = 1 ORDER BY id DESC LIMIT 1`,
		advertiserID,
	).Scan(&c.ID, &c.AdvertiserID, &c.Title, &c.Subtitle, &active, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active creative: %w", err)
	}
	c.Active = active == 1
	return &c, nil
}

// --- Intake Submissions ---

// IntakeSubmission represents a publisher or advertiser intake form submission.
type IntakeSubmission struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Company     string `json:"company"`
	Detail      string `json:"detail"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

// InsertIntakeSubmission stores a new intake form submission.
func (db *DB) InsertIntakeSubmission(submissionType, name, email, company, detail, description string) (int64, error) {
	result, err := db.conn.Exec(
		`INSERT INTO intake_submissions (type, name, email, company, detail, description) VALUES (?, ?, ?, ?, ?, ?)`,
		submissionType, name, email, company, detail, description,
	)
	if err != nil {
		return 0, fmt.Errorf("insert intake submission: %w", err)
	}
	return result.LastInsertId()
}

// GetIntakeSubmissions returns all intake submissions, newest first.
func (db *DB) GetIntakeSubmissions() ([]IntakeSubmission, error) {
	rows, err := db.conn.Query(
		`SELECT id, type, name, email, company, detail, description, created_at FROM intake_submissions ORDER BY id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("get intake submissions: %w", err)
	}
	defer rows.Close()

	var subs []IntakeSubmission
	for rows.Next() {
		var s IntakeSubmission
		if err := rows.Scan(&s.ID, &s.Type, &s.Name, &s.Email, &s.Company, &s.Detail, &s.Description, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan intake submission: %w", err)
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}
