package trust

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/emersion/go-smtp"
)

// ExchangeServer is the SMTP server that receives attestation emails.
// It verifies DKIM signatures and indexes attestations into the ledger.
type ExchangeServer struct {
	server *smtp.Server
	ledger *Ledger
	domain string // e.g., "exchange.com"
}

// NewExchangeServer creates an SMTP server listening for attestation emails.
// domain is the exchange's mail domain (e.g., "exchange.com").
// addr is the listen address (e.g., ":2525" for dev, ":25" for production).
func NewExchangeServer(ledger *Ledger, domain, addr string) *ExchangeServer {
	es := &ExchangeServer{
		ledger: ledger,
		domain: domain,
	}

	backend := &exchangeBackend{exchange: es}
	s := smtp.NewServer(backend)
	s.Addr = addr
	s.Domain = domain
	s.ReadTimeout = 30 * time.Second
	s.WriteTimeout = 30 * time.Second
	s.MaxMessageBytes = 1024 * 1024 // 1 MB — attestations are small
	s.MaxRecipients = 1             // attestations are addressed to the exchange only
	s.AllowInsecureAuth = true      // no auth required for attestation submission

	es.server = s
	return es
}

// ListenAndServe starts the SMTP server. Blocks until shutdown.
func (es *ExchangeServer) ListenAndServe() error {
	log.Printf("trust exchange SMTP server listening on %s (domain: %s)", es.server.Addr, es.domain)
	return es.server.ListenAndServe()
}

// Close shuts down the SMTP server.
func (es *ExchangeServer) Close() error {
	return es.server.Close()
}

// processMessage handles a received email: verifies DKIM, parses JSON body, routes to ledger.
func (es *ExchangeServer) processMessage(from string, to string, rawMessage []byte) error {
	// Step 1: Verify DKIM signature
	verifications, err := dkim.Verify(bytes.NewReader(rawMessage))
	if err != nil {
		return fmt.Errorf("dkim verify: %w", err)
	}

	dkimValid := false
	var verifiedDomain string
	for _, v := range verifications {
		if v.Err == nil {
			dkimValid = true
			verifiedDomain = v.Domain
			break
		}
	}

	if !dkimValid {
		// Fall back to sender domain from MAIL FROM
		verifiedDomain = domainFromEmail(from)
		log.Printf("trust: DKIM verification failed for %s (proceeding with unverified domain)", from)
	}

	// Step 2: Parse the email to extract JSON body
	msg, err := mail.ReadMessage(bytes.NewReader(rawMessage))
	if err != nil {
		return fmt.Errorf("parse email: %w", err)
	}

	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	// Step 3: Parse JSON payload
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("parse JSON body: %w", err)
	}

	// Step 4: Route by action
	action, _ := payload["action"].(string)

	switch action {
	case "confirm":
		return es.handleConfirm(payload, verifiedDomain, dkimValid, rawMessage)
	case "revoke":
		return es.handleRevoke(payload, verifiedDomain, dkimValid, rawMessage)
	default:
		return es.handleAttestation(payload, verifiedDomain, dkimValid, rawMessage)
	}
}

func (es *ExchangeServer) handleAttestation(payload map[string]any, senderDomain string, dkimValid bool, raw []byte) error {
	attestationID, _ := payload["attestation_id"].(string)
	if attestationID == "" {
		return fmt.Errorf("missing attestation_id")
	}

	attestationType, _ := payload["attestation_type"].(string)
	if attestationType == "" {
		return fmt.Errorf("missing attestation_type")
	}

	subject, _ := payload["subject"].(string)
	if subject == "" {
		return fmt.Errorf("missing subject")
	}

	// Determine edge kind: platform_rating is unilateral, everything else starts bilateral
	edgeKind := EdgeBilateral
	status := StatusPending
	if attestationType == TypePlatformRating {
		edgeKind = EdgeUnilateral
		status = StatusConfirmed
	}

	a := &Attestation{
		ID:             attestationID,
		Type:           attestationType,
		AttestorDomain: senderDomain,
		SubjectEmail:   subject,
		Status:         status,
		EdgeKind:       edgeKind,
		DKIMVerified:   dkimValid,
		Payload:        payload,
		PublishedFields: payload, // initially publish everything; subject can redact later
		ReceivedAt:     time.Now(),
	}

	if err := es.ledger.RecordAttestation(a); err != nil {
		return fmt.Errorf("record attestation: %w", err)
	}

	log.Printf("trust: recorded %s attestation %s from %s about %s (dkim=%v, kind=%s)",
		attestationType, attestationID, senderDomain, subject, dkimValid, edgeKind)
	return nil
}

func (es *ExchangeServer) handleConfirm(payload map[string]any, senderDomain string, dkimValid bool, raw []byte) error {
	attestationID, _ := payload["attestation_id"].(string)
	if attestationID == "" {
		return fmt.Errorf("confirm: missing attestation_id")
	}

	if err := es.ledger.ConfirmAttestation(attestationID, senderDomain); err != nil {
		return fmt.Errorf("confirm attestation: %w", err)
	}

	log.Printf("trust: confirmed attestation %s by %s (dkim=%v)", attestationID, senderDomain, dkimValid)
	return nil
}

func (es *ExchangeServer) handleRevoke(payload map[string]any, senderDomain string, dkimValid bool, raw []byte) error {
	attestationID, _ := payload["attestation_id"].(string)
	if attestationID == "" {
		return fmt.Errorf("revoke: missing attestation_id")
	}

	reason, _ := payload["reason"].(string)

	if err := es.ledger.RevokeAttestation(attestationID, senderDomain, reason); err != nil {
		return fmt.Errorf("revoke attestation: %w", err)
	}

	log.Printf("trust: revoked attestation %s by %s reason=%s (dkim=%v)", attestationID, senderDomain, reason, dkimValid)
	return nil
}

// --- go-smtp backend implementation ---

type exchangeBackend struct {
	exchange *ExchangeServer
}

func (b *exchangeBackend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &exchangeSession{exchange: b.exchange}, nil
}

type exchangeSession struct {
	exchange *ExchangeServer
	from     string
	to       string
}

func (s *exchangeSession) Reset() {}

func (s *exchangeSession) Logout() error { return nil }

func (s *exchangeSession) AuthPlain(username, password string) error {
	// No authentication required — anyone can send attestations
	return nil
}

func (s *exchangeSession) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *exchangeSession) Rcpt(to string, opts *smtp.RcptOptions) error {
	// Only accept mail to attestations@<domain>
	expectedAddr := "attestations@" + s.exchange.domain
	if !strings.EqualFold(to, expectedAddr) {
		return fmt.Errorf("recipient must be %s", expectedAddr)
	}
	s.to = to
	return nil
}

func (s *exchangeSession) Data(r io.Reader) error {
	rawMessage, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read message: %w", err)
	}

	if err := s.exchange.processMessage(s.from, s.to, rawMessage); err != nil {
		log.Printf("trust: failed to process message from %s: %v", s.from, err)
		return err
	}

	return nil
}
