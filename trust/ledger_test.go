package trust

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newTestLedger(t *testing.T) *Ledger {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	ledger, err := NewLedger(conn)
	if err != nil {
		t.Fatalf("new ledger: %v", err)
	}
	return ledger
}

func TestBilateralAttestationFlow(t *testing.T) {
	l := newTestLedger(t)

	// Step 1: Stripe sends attestation about merchant
	a := &Attestation{
		ID:             "stripe_merchant123_2026",
		Type:           TypePaymentProcessor,
		AttestorDomain: "stripe.com",
		SubjectEmail:   "merchant@example.com",
		Status:         StatusPending,
		EdgeKind:       EdgeBilateral,
		DKIMVerified:   true,
		Payload: map[string]any{
			"duration_years": float64(3),
			"status":         "good_standing",
		},
		PublishedFields: map[string]any{
			"duration_years": float64(3),
			"status":         "good_standing",
		},
		ReceivedAt: time.Now(),
	}

	if err := l.RecordAttestation(a); err != nil {
		t.Fatalf("record attestation: %v", err)
	}

	// Verify attestation is pending
	got, err := l.GetAttestation(a.ID)
	if err != nil {
		t.Fatalf("get attestation: %v", err)
	}
	if got.Status != StatusPending {
		t.Errorf("expected pending, got %s", got.Status)
	}

	// No edges yet (bilateral needs confirmation)
	edges, err := l.GetEdgesForDomain("stripe.com")
	if err != nil {
		t.Fatalf("get edges: %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges before confirmation, got %d", len(edges))
	}

	// Step 2: Merchant confirms
	if err := l.ConfirmAttestation(a.ID, "example.com"); err != nil {
		t.Fatalf("confirm attestation: %v", err)
	}

	// Verify attestation is confirmed
	got, _ = l.GetAttestation(a.ID)
	if got.Status != StatusConfirmed {
		t.Errorf("expected confirmed, got %s", got.Status)
	}
	if got.ConfirmedAt == nil {
		t.Error("expected confirmed_at to be set")
	}

	// Bilateral edges exist (both directions)
	edges, _ = l.GetEdgesForDomain("stripe.com")
	if len(edges) != 2 {
		t.Errorf("expected 2 bilateral edges, got %d", len(edges))
	}

	// Both directions
	directionFound := map[string]bool{}
	for _, e := range edges {
		directionFound[e.FromDomain+"→"+e.ToDomain] = true
		if e.Kind != EdgeBilateral {
			t.Errorf("expected bilateral edge, got %s", e.Kind)
		}
		if e.Weight != 3.0 { // duration_years = 3
			t.Errorf("expected weight 3.0, got %f", e.Weight)
		}
	}
	if !directionFound["stripe.com→example.com"] {
		t.Error("missing stripe.com→example.com edge")
	}
	if !directionFound["example.com→stripe.com"] {
		t.Error("missing example.com→stripe.com edge")
	}
}

func TestUnilateralAttestation(t *testing.T) {
	l := newTestLedger(t)

	// Google sends platform rating (unilateral — no confirmation needed)
	a := &Attestation{
		ID:             "google_restaurant456_2026",
		Type:           TypePlatformRating,
		AttestorDomain: "google.com",
		SubjectEmail:   "restaurant@example.com",
		Status:         StatusConfirmed, // unilateral = immediately confirmed
		EdgeKind:       EdgeUnilateral,
		DKIMVerified:   true,
		Payload: map[string]any{
			"rating":       4.5,
			"review_count": float64(247),
			"platform":     "Google Reviews",
		},
		PublishedFields: map[string]any{
			"rating":       4.5,
			"review_count": float64(247),
		},
		ReceivedAt: time.Now(),
	}

	if err := l.RecordAttestation(a); err != nil {
		t.Fatalf("record attestation: %v", err)
	}

	// Edge should exist immediately (unilateral)
	edges, _ := l.GetEdgesForDomain("google.com")
	if len(edges) != 1 {
		t.Fatalf("expected 1 unilateral edge, got %d", len(edges))
	}

	e := edges[0]
	if e.FromDomain != "google.com" || e.ToDomain != "example.com" {
		t.Errorf("unexpected edge direction: %s→%s", e.FromDomain, e.ToDomain)
	}
	if e.Kind != EdgeUnilateral {
		t.Errorf("expected unilateral, got %s", e.Kind)
	}
	// weight = review_count / 100.0 = 247/100 = 2.47
	if e.Weight < 2.46 || e.Weight > 2.48 {
		t.Errorf("expected weight ~2.47, got %f", e.Weight)
	}
}

func TestRevocation(t *testing.T) {
	l := newTestLedger(t)

	// Create and confirm a bilateral attestation
	a := &Attestation{
		ID:             "stripe_merchant_rev",
		Type:           TypePaymentProcessor,
		AttestorDomain: "stripe.com",
		SubjectEmail:   "merchant@example.com",
		Status:         StatusPending,
		EdgeKind:       EdgeBilateral,
		DKIMVerified:   true,
		Payload:        map[string]any{"duration_years": float64(2)},
		PublishedFields: map[string]any{},
		ReceivedAt:     time.Now(),
	}
	l.RecordAttestation(a)
	l.ConfirmAttestation(a.ID, "example.com")

	// Verify edges exist
	edges, _ := l.GetEdgesForDomain("stripe.com")
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges after confirm, got %d", len(edges))
	}

	// Stripe revokes (attestor can revoke)
	if err := l.RevokeAttestation(a.ID, "stripe.com", "account_closed"); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// Edges removed
	edges, _ = l.GetEdgesForDomain("stripe.com")
	if len(edges) != 0 {
		t.Errorf("expected 0 edges after revocation, got %d", len(edges))
	}

	// Attestation marked revoked
	got, _ := l.GetAttestation(a.ID)
	if got.Status != StatusRevoked {
		t.Errorf("expected revoked, got %s", got.Status)
	}
	if got.RevokedAt == nil {
		t.Error("expected revoked_at to be set")
	}

	// Can't revoke twice
	if err := l.RevokeAttestation(a.ID, "stripe.com", "duplicate"); err == nil {
		t.Error("expected error on double revocation")
	}
}

func TestSubjectCanRevoke(t *testing.T) {
	l := newTestLedger(t)

	a := &Attestation{
		ID:             "stripe_subj_rev",
		Type:           TypePaymentProcessor,
		AttestorDomain: "stripe.com",
		SubjectEmail:   "merchant@example.com",
		Status:         StatusPending,
		EdgeKind:       EdgeBilateral,
		DKIMVerified:   true,
		Payload:        map[string]any{},
		PublishedFields: map[string]any{},
		ReceivedAt:     time.Now(),
	}
	l.RecordAttestation(a)
	l.ConfirmAttestation(a.ID, "example.com")

	// Subject (merchant) revokes
	if err := l.RevokeAttestation(a.ID, "example.com", "ended_relationship"); err != nil {
		t.Fatalf("subject revoke: %v", err)
	}

	got, _ := l.GetAttestation(a.ID)
	if got.Status != StatusRevoked {
		t.Errorf("expected revoked, got %s", got.Status)
	}
}

func TestUnauthorizedRevocationFails(t *testing.T) {
	l := newTestLedger(t)

	a := &Attestation{
		ID:             "stripe_unauth_rev",
		Type:           TypePaymentProcessor,
		AttestorDomain: "stripe.com",
		SubjectEmail:   "merchant@example.com",
		Status:         StatusPending,
		EdgeKind:       EdgeBilateral,
		DKIMVerified:   true,
		Payload:        map[string]any{},
		PublishedFields: map[string]any{},
		ReceivedAt:     time.Now(),
	}
	l.RecordAttestation(a)
	l.ConfirmAttestation(a.ID, "example.com")

	// Third party can't revoke
	if err := l.RevokeAttestation(a.ID, "attacker.com", "lol"); err == nil {
		t.Error("expected error from unauthorized revocation")
	}
}

func TestWrongDomainConfirmationFails(t *testing.T) {
	l := newTestLedger(t)

	a := &Attestation{
		ID:             "stripe_wrong_confirm",
		Type:           TypePaymentProcessor,
		AttestorDomain: "stripe.com",
		SubjectEmail:   "merchant@example.com",
		Status:         StatusPending,
		EdgeKind:       EdgeBilateral,
		DKIMVerified:   true,
		Payload:        map[string]any{},
		PublishedFields: map[string]any{},
		ReceivedAt:     time.Now(),
	}
	l.RecordAttestation(a)

	// Wrong domain tries to confirm
	if err := l.ConfirmAttestation(a.ID, "attacker.com"); err == nil {
		t.Error("expected error from wrong-domain confirmation")
	}
}

func TestTrustNodeAggregation(t *testing.T) {
	l := newTestLedger(t)

	// Create multiple attestations for example.com
	attestations := []struct {
		id      string
		typ     string
		from    string
		subject string
		kind    EdgeKind
	}{
		{"stripe_1", TypePaymentProcessor, "stripe.com", "merchant@example.com", EdgeBilateral},
		{"google_1", TypePlatformRating, "google.com", "merchant@example.com", EdgeUnilateral},
		{"yelp_1", TypePlatformRating, "yelp.com", "merchant@example.com", EdgeUnilateral},
	}

	for _, att := range attestations {
		status := StatusPending
		if att.kind == EdgeUnilateral {
			status = StatusConfirmed
		}
		a := &Attestation{
			ID: att.id, Type: att.typ, AttestorDomain: att.from,
			SubjectEmail: att.subject, Status: status, EdgeKind: att.kind,
			DKIMVerified: true, Payload: map[string]any{}, PublishedFields: map[string]any{},
			ReceivedAt: time.Now(),
		}
		l.RecordAttestation(a)
	}
	// Confirm the bilateral one
	l.ConfirmAttestation("stripe_1", "example.com")

	node, err := l.GetNode("example.com")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if node.EdgeCount != 4 { // 2 bilateral + 2 unilateral
		t.Errorf("expected 4 edges, got %d", node.EdgeCount)
	}
	if node.BilateralCount != 2 {
		t.Errorf("expected 2 bilateral, got %d", node.BilateralCount)
	}
	if node.UnilateralCount != 2 {
		t.Errorf("expected 2 unilateral, got %d", node.UnilateralCount)
	}
}

func TestGetTrustedDomains(t *testing.T) {
	l := newTestLedger(t)

	// Rich topology: merchant with 3 bilateral + 2 unilateral = 8 edges total (each bilateral=2)
	for i, att := range []struct {
		id, typ, from, subject string
		kind                   EdgeKind
	}{
		{"a1", TypePaymentProcessor, "stripe.com", "rich@example.com", EdgeBilateral},
		{"a2", TypeVendorRelationship, "supplier.com", "rich@example.com", EdgeBilateral},
		{"a3", TypeCustomerEndorsement, "customer.com", "rich@example.com", EdgeBilateral},
		{"a4", TypePlatformRating, "google.com", "rich@example.com", EdgeUnilateral},
		{"a5", TypePlatformRating, "yelp.com", "rich@example.com", EdgeUnilateral},
	} {
		status := StatusPending
		if att.kind == EdgeUnilateral {
			status = StatusConfirmed
		}
		a := &Attestation{
			ID: att.id, Type: att.typ, AttestorDomain: att.from,
			SubjectEmail: att.subject, Status: status, EdgeKind: att.kind,
			DKIMVerified: true, Payload: map[string]any{}, PublishedFields: map[string]any{},
			ReceivedAt: time.Now(),
		}
		l.RecordAttestation(a)
		if att.kind == EdgeBilateral {
			l.ConfirmAttestation(att.id, "example.com")
		}
		_ = i
	}

	// Thin topology: new merchant with just 1 unilateral
	l.RecordAttestation(&Attestation{
		ID: "thin_1", Type: TypePlatformRating, AttestorDomain: "google.com",
		SubjectEmail: "new@thin.com", Status: StatusConfirmed, EdgeKind: EdgeUnilateral,
		DKIMVerified: true, Payload: map[string]any{}, PublishedFields: map[string]any{},
		ReceivedAt: time.Now(),
	})

	// Query: min 3 edges, min 1 bilateral
	nodes, err := l.GetTrustedDomains(3, 1)
	if err != nil {
		t.Fatalf("get trusted domains: %v", err)
	}

	// Only example.com should qualify
	found := false
	for _, n := range nodes {
		if n.Domain == "example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected example.com in trusted domains")
	}

	// thin.com should NOT be there
	for _, n := range nodes {
		if n.Domain == "thin.com" {
			t.Error("thin.com should not meet threshold")
		}
	}
}

func TestLedgerLog(t *testing.T) {
	l := newTestLedger(t)

	a := &Attestation{
		ID: "log_test", Type: TypePaymentProcessor, AttestorDomain: "stripe.com",
		SubjectEmail: "m@example.com", Status: StatusPending, EdgeKind: EdgeBilateral,
		DKIMVerified: true, Payload: map[string]any{"x": "y"}, PublishedFields: map[string]any{},
		ReceivedAt: time.Now(),
	}
	l.RecordAttestation(a)
	l.ConfirmAttestation(a.ID, "example.com")
	l.RevokeAttestation(a.ID, "stripe.com", "test")

	entries, err := l.GetLedgerLog(10)
	if err != nil {
		t.Fatalf("get log: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 log entries, got %d", len(entries))
	}

	// Newest first
	actions := make([]string, len(entries))
	for i, e := range entries {
		actions[i] = e["action"].(string)
	}
	if actions[0] != "revoke" || actions[1] != "confirm" || actions[2] != "attestation" {
		t.Errorf("unexpected log order: %v", actions)
	}
}

func TestDomainFromEmail(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"merchant@example.com", "example.com"},
		{"user@sub.domain.com", "sub.domain.com"},
		{"nodomain", "nodomain"},
		{"@empty", "empty"},
	}
	for _, tc := range cases {
		got := domainFromEmail(tc.input)
		if got != tc.want {
			t.Errorf("domainFromEmail(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
