package trust

import (
	"database/sql"
	"fmt"
	"net"
	"net/smtp"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSMTPAttestationFlow(t *testing.T) {
	// Set up ledger
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer conn.Close()

	ledger, err := NewLedger(conn)
	if err != nil {
		t.Fatalf("new ledger: %v", err)
	}

	// Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Start SMTP exchange server
	es := NewExchangeServer(ledger, "exchange.test", addr)
	go es.ListenAndServe()
	defer es.Close()

	// Wait for server to be ready
	for i := 0; i < 20; i++ {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Send an attestation email via SMTP
	// The body is JSON (no DKIM — DKIM verification will fail gracefully)
	attestationJSON := `{
		"attestation_type": "payment_processor",
		"attestation_id": "smtp_stripe_test_2026",
		"subject": "merchant@example.com",
		"duration_years": 3,
		"status": "good_standing"
	}`

	msg := fmt.Sprintf("From: attestations@stripe.com\r\nTo: attestations@exchange.test\r\nSubject: Payment Processing Attestation\r\n\r\n%s", attestationJSON)

	err = smtp.SendMail(addr, nil, "attestations@stripe.com", []string{"attestations@exchange.test"}, []byte(msg))
	if err != nil {
		t.Fatalf("send mail: %v", err)
	}

	// Give the server a moment to process
	time.Sleep(100 * time.Millisecond)

	// Verify attestation was recorded
	a, err := ledger.GetAttestation("smtp_stripe_test_2026")
	if err != nil {
		t.Fatalf("get attestation: %v", err)
	}
	if a == nil {
		t.Fatal("attestation not found after SMTP delivery")
	}
	if a.Status != StatusPending {
		t.Errorf("expected pending (bilateral), got %s", a.Status)
	}
	if a.Type != TypePaymentProcessor {
		t.Errorf("expected payment_processor, got %s", a.Type)
	}
	if a.SubjectEmail != "merchant@example.com" {
		t.Errorf("expected merchant@example.com, got %s", a.SubjectEmail)
	}
	// DKIM will be false since we're sending without real DKIM signing
	if a.DKIMVerified {
		t.Error("expected DKIM to be unverified for test message")
	}

	// Now send a confirmation email
	confirmJSON := `{
		"action": "confirm",
		"attestation_id": "smtp_stripe_test_2026"
	}`
	confirmMsg := fmt.Sprintf("From: merchant@example.com\r\nTo: attestations@exchange.test\r\nSubject: Attestation Confirmation\r\n\r\n%s", confirmJSON)

	err = smtp.SendMail(addr, nil, "merchant@example.com", []string{"attestations@exchange.test"}, []byte(confirmMsg))
	if err != nil {
		t.Fatalf("send confirmation: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify bilateral edges were created
	a, _ = ledger.GetAttestation("smtp_stripe_test_2026")
	if a.Status != StatusConfirmed {
		t.Errorf("expected confirmed after bilateral confirmation, got %s", a.Status)
	}

	edges, err := ledger.GetEdgesForAddr("attestations@stripe.com")
	if err != nil {
		t.Fatalf("get edges: %v", err)
	}
	if len(edges) != 2 {
		t.Errorf("expected 2 bilateral edges, got %d", len(edges))
	}

	// Send a revocation
	revokeJSON := `{
		"action": "revoke",
		"attestation_id": "smtp_stripe_test_2026",
		"reason": "account_closed"
	}`
	revokeMsg := fmt.Sprintf("From: attestations@stripe.com\r\nTo: attestations@exchange.test\r\nSubject: Attestation Revocation\r\n\r\n%s", revokeJSON)

	err = smtp.SendMail(addr, nil, "attestations@stripe.com", []string{"attestations@exchange.test"}, []byte(revokeMsg))
	if err != nil {
		t.Fatalf("send revocation: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify revocation
	a, _ = ledger.GetAttestation("smtp_stripe_test_2026")
	if a.Status != StatusRevoked {
		t.Errorf("expected revoked, got %s", a.Status)
	}

	edges, _ = ledger.GetEdgesForAddr("attestations@stripe.com")
	if len(edges) != 0 {
		t.Errorf("expected 0 edges after revocation, got %d", len(edges))
	}

	// Check the append-only log has all 3 actions
	entries, err := ledger.GetLedgerLog(10)
	if err != nil {
		t.Fatalf("get log: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 log entries, got %d", len(entries))
	}
}

func TestSMTPRejectsWrongRecipient(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer conn.Close()

	ledger, err := NewLedger(conn)
	if err != nil {
		t.Fatalf("new ledger: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	es := NewExchangeServer(ledger, "exchange.test", addr)
	go es.ListenAndServe()
	defer es.Close()

	for i := 0; i < 20; i++ {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			c.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	msg := "From: test@test.com\r\nTo: wrong@exchange.test\r\nSubject: Test\r\n\r\n{}"
	err = smtp.SendMail(addr, nil, "test@test.com", []string{"wrong@exchange.test"}, []byte(msg))
	if err == nil {
		t.Error("expected error for wrong recipient")
	}
}

func TestSMTPUnilateralPlatformRating(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer conn.Close()

	ledger, err := NewLedger(conn)
	if err != nil {
		t.Fatalf("new ledger: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	es := NewExchangeServer(ledger, "exchange.test", addr)
	go es.ListenAndServe()
	defer es.Close()

	for i := 0; i < 20; i++ {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			c.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Google sends a platform_rating (unilateral — no confirmation needed)
	ratingJSON := `{
		"attestation_type": "platform_rating",
		"attestation_id": "google_rest456",
		"subject": "restaurant@example.com",
		"rating": 4.5,
		"review_count": 247
	}`
	msg := fmt.Sprintf("From: attestations@google.com\r\nTo: attestations@exchange.test\r\nSubject: Platform Rating\r\n\r\n%s", ratingJSON)

	err = smtp.SendMail(addr, nil, "attestations@google.com", []string{"attestations@exchange.test"}, []byte(msg))
	if err != nil {
		t.Fatalf("send mail: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	a, _ := ledger.GetAttestation("google_rest456")
	if a == nil {
		t.Fatal("attestation not found")
	}
	if a.Status != StatusConfirmed {
		t.Errorf("expected confirmed (unilateral), got %s", a.Status)
	}

	// Edge should exist immediately
	edges, _ := ledger.GetEdgesForAddr("attestations@google.com")
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Kind != EdgeUnilateral {
		t.Errorf("expected unilateral, got %s", edges[0].Kind)
	}
}
