package trust

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Ledger is the append-only trust graph backed by SQLite.
// It stores attestations and the edges they produce.
type Ledger struct {
	conn *sql.DB
}

// NewLedger opens a SQLite database and creates trust tables.
func NewLedger(conn *sql.DB) (*Ledger, error) {
	l := &Ledger{conn: conn}
	if err := l.createTables(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *Ledger) createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS attestations (
		id TEXT PRIMARY KEY,
		attestation_type TEXT NOT NULL,
		attestor_domain TEXT NOT NULL,
		subject_email TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		edge_kind TEXT NOT NULL DEFAULT 'bilateral',
		dkim_verified INTEGER NOT NULL DEFAULT 0,
		payload TEXT NOT NULL DEFAULT '{}',
		published_fields TEXT NOT NULL DEFAULT '{}',
		received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		confirmed_at DATETIME,
		revoked_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_attestations_subject ON attestations(subject_email);
	CREATE INDEX IF NOT EXISTS idx_attestations_attestor ON attestations(attestor_domain);
	CREATE INDEX IF NOT EXISTS idx_attestations_status ON attestations(status);

	CREATE TABLE IF NOT EXISTS trust_edges (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		attestation_id TEXT NOT NULL REFERENCES attestations(id),
		from_domain TEXT NOT NULL,
		to_domain TEXT NOT NULL,
		kind TEXT NOT NULL,
		attestation_type TEXT NOT NULL,
		weight REAL NOT NULL DEFAULT 1.0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_edges_from ON trust_edges(from_domain);
	CREATE INDEX IF NOT EXISTS idx_edges_to ON trust_edges(to_domain);
	CREATE INDEX IF NOT EXISTS idx_edges_attestation ON trust_edges(attestation_id);

	CREATE TABLE IF NOT EXISTS publish_preferences (
		subject_email TEXT PRIMARY KEY,
		publish_fields TEXT NOT NULL DEFAULT '[]',
		redact_fields TEXT NOT NULL DEFAULT '[]',
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS ledger_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		action TEXT NOT NULL,
		attestation_id TEXT NOT NULL,
		sender_domain TEXT NOT NULL,
		raw_payload TEXT NOT NULL,
		dkim_verified INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := l.conn.Exec(schema); err != nil {
		return fmt.Errorf("create trust tables: %w", err)
	}
	return nil
}

// RecordAttestation stores a new attestation from a DKIM-verified email.
// For bilateral attestations, status starts as "pending" until the subject confirms.
// For unilateral attestations (platform_rating, etc.), status is immediately "confirmed"
// and edges are created.
func (l *Ledger) RecordAttestation(a *Attestation) error {
	payloadJSON, err := json.Marshal(a.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	publishedJSON, err := json.Marshal(a.PublishedFields)
	if err != nil {
		return fmt.Errorf("marshal published fields: %w", err)
	}

	dkimInt := 0
	if a.DKIMVerified {
		dkimInt = 1
	}

	tx, err := l.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO attestations (id, attestation_type, attestor_domain, subject_email, status, edge_kind, dkim_verified, payload, published_fields, received_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Type, a.AttestorDomain, a.SubjectEmail, a.Status, a.EdgeKind, dkimInt,
		string(payloadJSON), string(publishedJSON), a.ReceivedAt,
	)
	if err != nil {
		return fmt.Errorf("insert attestation: %w", err)
	}

	// Log to append-only ledger
	_, err = tx.Exec(
		`INSERT INTO ledger_log (action, attestation_id, sender_domain, raw_payload, dkim_verified) VALUES (?, ?, ?, ?, ?)`,
		"attestation", a.ID, a.AttestorDomain, string(payloadJSON), dkimInt,
	)
	if err != nil {
		return fmt.Errorf("log attestation: %w", err)
	}

	// Unilateral attestations get edges immediately
	if a.EdgeKind == EdgeUnilateral && a.Status == StatusConfirmed {
		weight := computeWeight(a.Payload)
		_, err = tx.Exec(
			`INSERT INTO trust_edges (attestation_id, from_domain, to_domain, kind, attestation_type, weight) VALUES (?, ?, ?, ?, ?, ?)`,
			a.ID, a.AttestorDomain, domainFromEmail(a.SubjectEmail), EdgeUnilateral, a.Type, weight,
		)
		if err != nil {
			return fmt.Errorf("insert unilateral edge: %w", err)
		}
	}

	return tx.Commit()
}

// ConfirmAttestation records bilateral confirmation and creates mutual edges.
func (l *Ledger) ConfirmAttestation(attestationID, confirmerDomain string) error {
	tx, err := l.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Load the attestation
	var a Attestation
	var payloadJSON, publishedJSON string
	var dkimInt int
	var confirmedAt, revokedAt sql.NullTime
	err = tx.QueryRow(
		`SELECT id, attestation_type, attestor_domain, subject_email, status, edge_kind, dkim_verified, payload, published_fields, received_at, confirmed_at, revoked_at
		 FROM attestations WHERE id = ?`, attestationID,
	).Scan(&a.ID, &a.Type, &a.AttestorDomain, &a.SubjectEmail, &a.Status, &a.EdgeKind, &dkimInt,
		&payloadJSON, &publishedJSON, &a.ReceivedAt, &confirmedAt, &revokedAt)
	if err == sql.ErrNoRows {
		return fmt.Errorf("attestation %s not found", attestationID)
	}
	if err != nil {
		return fmt.Errorf("load attestation: %w", err)
	}
	json.Unmarshal([]byte(payloadJSON), &a.Payload)

	if a.Status != StatusPending {
		return fmt.Errorf("attestation %s is %s, not pending", attestationID, a.Status)
	}

	// Verify the confirmer is the subject (or their domain matches)
	subjectDomain := domainFromEmail(a.SubjectEmail)
	if confirmerDomain != subjectDomain {
		return fmt.Errorf("confirmer domain %s does not match subject domain %s", confirmerDomain, subjectDomain)
	}

	now := time.Now()
	_, err = tx.Exec(
		`UPDATE attestations SET status = ?, confirmed_at = ? WHERE id = ?`,
		StatusConfirmed, now, attestationID,
	)
	if err != nil {
		return fmt.Errorf("update attestation status: %w", err)
	}

	// Log confirmation
	_, err = tx.Exec(
		`INSERT INTO ledger_log (action, attestation_id, sender_domain, raw_payload, dkim_verified) VALUES (?, ?, ?, ?, ?)`,
		"confirm", attestationID, confirmerDomain, fmt.Sprintf(`{"attestation_id":"%s"}`, attestationID), 1,
	)
	if err != nil {
		return fmt.Errorf("log confirmation: %w", err)
	}

	// Create bilateral edges (both directions)
	weight := computeWeight(a.Payload)
	for _, edge := range []struct{ from, to string }{
		{a.AttestorDomain, subjectDomain},
		{subjectDomain, a.AttestorDomain},
	} {
		_, err = tx.Exec(
			`INSERT INTO trust_edges (attestation_id, from_domain, to_domain, kind, attestation_type, weight) VALUES (?, ?, ?, ?, ?, ?)`,
			attestationID, edge.from, edge.to, EdgeBilateral, a.Type, weight,
		)
		if err != nil {
			return fmt.Errorf("insert bilateral edge: %w", err)
		}
	}

	return tx.Commit()
}

// RevokeAttestation removes edges and marks the attestation as revoked.
// Either party can revoke.
func (l *Ledger) RevokeAttestation(attestationID, senderDomain, reason string) error {
	tx, err := l.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var attestorDomain, subjectEmail string
	var status AttestationStatus
	err = tx.QueryRow(
		`SELECT attestor_domain, subject_email, status FROM attestations WHERE id = ?`, attestationID,
	).Scan(&attestorDomain, &subjectEmail, &status)
	if err == sql.ErrNoRows {
		return fmt.Errorf("attestation %s not found", attestationID)
	}
	if err != nil {
		return fmt.Errorf("load attestation: %w", err)
	}

	if status == StatusRevoked {
		return fmt.Errorf("attestation %s already revoked", attestationID)
	}

	// Verify sender is either the attestor or the subject
	subjectDomain := domainFromEmail(subjectEmail)
	if senderDomain != attestorDomain && senderDomain != subjectDomain {
		return fmt.Errorf("sender %s is neither attestor (%s) nor subject (%s)", senderDomain, attestorDomain, subjectDomain)
	}

	now := time.Now()
	_, err = tx.Exec(
		`UPDATE attestations SET status = ?, revoked_at = ? WHERE id = ?`,
		StatusRevoked, now, attestationID,
	)
	if err != nil {
		return fmt.Errorf("revoke attestation: %w", err)
	}

	// Remove edges
	_, err = tx.Exec(`DELETE FROM trust_edges WHERE attestation_id = ?`, attestationID)
	if err != nil {
		return fmt.Errorf("delete edges: %w", err)
	}

	// Log revocation
	logPayload, _ := json.Marshal(map[string]string{
		"attestation_id": attestationID,
		"reason":         reason,
	})
	_, err = tx.Exec(
		`INSERT INTO ledger_log (action, attestation_id, sender_domain, raw_payload, dkim_verified) VALUES (?, ?, ?, ?, ?)`,
		"revoke", attestationID, senderDomain, string(logPayload), 1,
	)
	if err != nil {
		return fmt.Errorf("log revocation: %w", err)
	}

	return tx.Commit()
}

// GetAttestation returns a single attestation by ID.
func (l *Ledger) GetAttestation(id string) (*Attestation, error) {
	var a Attestation
	var payloadJSON, publishedJSON string
	var dkimInt int
	var confirmedAt, revokedAt sql.NullTime
	err := l.conn.QueryRow(
		`SELECT id, attestation_type, attestor_domain, subject_email, status, edge_kind, dkim_verified, payload, published_fields, received_at, confirmed_at, revoked_at
		 FROM attestations WHERE id = ?`, id,
	).Scan(&a.ID, &a.Type, &a.AttestorDomain, &a.SubjectEmail, &a.Status, &a.EdgeKind, &dkimInt,
		&payloadJSON, &publishedJSON, &a.ReceivedAt, &confirmedAt, &revokedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get attestation: %w", err)
	}
	a.DKIMVerified = dkimInt == 1
	json.Unmarshal([]byte(payloadJSON), &a.Payload)
	json.Unmarshal([]byte(publishedJSON), &a.PublishedFields)
	if confirmedAt.Valid {
		a.ConfirmedAt = &confirmedAt.Time
	}
	if revokedAt.Valid {
		a.RevokedAt = &revokedAt.Time
	}
	return &a, nil
}

// GetEdgesForDomain returns all trust edges involving a domain (as source or target).
func (l *Ledger) GetEdgesForDomain(domain string) ([]TrustEdge, error) {
	rows, err := l.conn.Query(
		`SELECT id, attestation_id, from_domain, to_domain, kind, attestation_type, weight, created_at
		 FROM trust_edges WHERE from_domain = ? OR to_domain = ? ORDER BY created_at DESC`,
		domain, domain,
	)
	if err != nil {
		return nil, fmt.Errorf("get edges: %w", err)
	}
	defer rows.Close()

	var edges []TrustEdge
	for rows.Next() {
		var e TrustEdge
		if err := rows.Scan(&e.ID, &e.AttestationID, &e.FromDomain, &e.ToDomain, &e.Kind, &e.AttestationType, &e.Weight, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// GetNode returns aggregated trust info for a domain.
func (l *Ledger) GetNode(domain string) (*TrustNode, error) {
	var node TrustNode
	node.Domain = domain

	err := l.conn.QueryRow(
		`SELECT COUNT(*),
			COALESCE(SUM(CASE WHEN kind = 'bilateral' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN kind = 'unilateral' THEN 1 ELSE 0 END), 0),
			COALESCE(MIN(created_at), ''),
			COALESCE(MAX(created_at), '')
		 FROM trust_edges WHERE from_domain = ? OR to_domain = ?`,
		domain, domain,
	).Scan(&node.EdgeCount, &node.BilateralCount, &node.UnilateralCount, &node.OldestEdge, &node.NewestEdge)
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}
	return &node, nil
}

// GetGraph returns all edges in the trust graph.
func (l *Ledger) GetGraph() ([]TrustEdge, error) {
	rows, err := l.conn.Query(
		`SELECT id, attestation_id, from_domain, to_domain, kind, attestation_type, weight, created_at
		 FROM trust_edges ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("get graph: %w", err)
	}
	defer rows.Close()

	var edges []TrustEdge
	for rows.Next() {
		var e TrustEdge
		if err := rows.Scan(&e.ID, &e.AttestationID, &e.FromDomain, &e.ToDomain, &e.Kind, &e.AttestationType, &e.Weight, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// GetLedgerLog returns the append-only log entries, newest first.
func (l *Ledger) GetLedgerLog(limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := l.conn.Query(
		`SELECT id, action, attestation_id, sender_domain, raw_payload, dkim_verified, created_at
		 FROM ledger_log ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get ledger log: %w", err)
	}
	defer rows.Close()

	var entries []map[string]any
	for rows.Next() {
		var id int64
		var action, attestationID, senderDomain, rawPayload string
		var dkimVerified int
		var createdAt string
		if err := rows.Scan(&id, &action, &attestationID, &senderDomain, &rawPayload, &dkimVerified, &createdAt); err != nil {
			return nil, fmt.Errorf("scan log: %w", err)
		}
		entries = append(entries, map[string]any{
			"id":              id,
			"action":          action,
			"attestation_id":  attestationID,
			"sender_domain":   senderDomain,
			"raw_payload":     rawPayload,
			"dkim_verified":   dkimVerified == 1,
			"created_at":      createdAt,
		})
	}
	return entries, rows.Err()
}

// GetAttestationsForSubject returns all attestations about a given email/domain.
func (l *Ledger) GetAttestationsForSubject(subjectEmail string) ([]Attestation, error) {
	rows, err := l.conn.Query(
		`SELECT id, attestation_type, attestor_domain, subject_email, status, edge_kind, dkim_verified, payload, published_fields, received_at, confirmed_at, revoked_at
		 FROM attestations WHERE subject_email = ? AND status != 'revoked' ORDER BY received_at DESC`,
		subjectEmail,
	)
	if err != nil {
		return nil, fmt.Errorf("get attestations for subject: %w", err)
	}
	defer rows.Close()

	var attestations []Attestation
	for rows.Next() {
		var a Attestation
		var payloadJSON, publishedJSON string
		var dkimInt int
		var confirmedAt, revokedAt sql.NullTime
		if err := rows.Scan(&a.ID, &a.Type, &a.AttestorDomain, &a.SubjectEmail, &a.Status, &a.EdgeKind, &dkimInt,
			&payloadJSON, &publishedJSON, &a.ReceivedAt, &confirmedAt, &revokedAt); err != nil {
			return nil, fmt.Errorf("scan attestation: %w", err)
		}
		a.DKIMVerified = dkimInt == 1
		json.Unmarshal([]byte(payloadJSON), &a.Payload)
		json.Unmarshal([]byte(publishedJSON), &a.PublishedFields)
		if confirmedAt.Valid {
			a.ConfirmedAt = &confirmedAt.Time
		}
		if revokedAt.Valid {
			a.RevokedAt = &revokedAt.Time
		}
		attestations = append(attestations, a)
	}
	return attestations, rows.Err()
}

// SetPublishPreference stores which fields a subject opts to publish.
func (l *Ledger) SetPublishPreference(pref *PublishPreference) error {
	publishJSON, _ := json.Marshal(pref.Publish)
	redactJSON, _ := json.Marshal(pref.Redact)
	_, err := l.conn.Exec(
		`INSERT OR REPLACE INTO publish_preferences (subject_email, publish_fields, redact_fields, updated_at) VALUES (?, ?, ?, ?)`,
		pref.SubjectEmail, string(publishJSON), string(redactJSON), time.Now(),
	)
	return err
}

// GetTrustedDomains returns domains that meet minimum edge thresholds.
// This is the primitive curators use to build allowlists.
func (l *Ledger) GetTrustedDomains(minEdges int, minBilateral int) ([]TrustNode, error) {
	rows, err := l.conn.Query(
		`SELECT domain, edge_count, bilateral_count, unilateral_count, oldest_edge, newest_edge FROM (
			SELECT
				domain,
				COUNT(*) as edge_count,
				SUM(CASE WHEN kind = 'bilateral' THEN 1 ELSE 0 END) as bilateral_count,
				SUM(CASE WHEN kind = 'unilateral' THEN 1 ELSE 0 END) as unilateral_count,
				MIN(created_at) as oldest_edge,
				MAX(created_at) as newest_edge
			FROM (
				SELECT from_domain as domain, kind, created_at FROM trust_edges
				UNION ALL
				SELECT to_domain as domain, kind, created_at FROM trust_edges
			)
			GROUP BY domain
		) WHERE edge_count >= ? AND bilateral_count >= ?
		ORDER BY edge_count DESC`,
		minEdges, minBilateral,
	)
	if err != nil {
		return nil, fmt.Errorf("get trusted domains: %w", err)
	}
	defer rows.Close()

	var nodes []TrustNode
	for rows.Next() {
		var n TrustNode
		if err := rows.Scan(&n.Domain, &n.EdgeCount, &n.BilateralCount, &n.UnilateralCount, &n.OldestEdge, &n.NewestEdge); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// computeWeight derives an edge weight from attestation payload.
// Duration-based attestations get higher weight for longer relationships.
func computeWeight(payload map[string]any) float64 {
	weight := 1.0
	if years, ok := payload["duration_years"]; ok {
		if y, ok := years.(float64); ok && y > 0 {
			weight = y
		}
	}
	if count, ok := payload["review_count"]; ok {
		if c, ok := count.(float64); ok && c > 0 {
			weight = c / 100.0 // normalize: 100 reviews = weight 1.0
			if weight < 0.1 {
				weight = 0.1
			}
		}
	}
	return weight
}

// domainFromEmail extracts the domain part from an email address.
func domainFromEmail(email string) string {
	for i := len(email) - 1; i >= 0; i-- {
		if email[i] == '@' {
			return email[i+1:]
		}
	}
	return email // if no @, treat the whole thing as a domain
}
