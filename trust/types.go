package trust

import "time"

// AttestationType defines the canonical set of attestation types.
// Attestors extend with URI-prefixed types (e.g., "https://stripe.com/attestation/payment_processing").
const (
	TypePaymentProcessor    = "payment_processor"
	TypePlatformRating      = "platform_rating"
	TypeCustomerEndorsement = "customer_endorsement"
	TypeVendorRelationship  = "vendor_relationship"
	TypeLicense             = "license"
)

// EdgeKind distinguishes bilateral (mutual) from unilateral (observation) edges.
type EdgeKind string

const (
	EdgeBilateral  EdgeKind = "bilateral"
	EdgeUnilateral EdgeKind = "unilateral"
)

// AttestationStatus tracks the lifecycle of an attestation.
type AttestationStatus string

const (
	StatusPending   AttestationStatus = "pending"   // awaiting bilateral confirmation
	StatusConfirmed AttestationStatus = "confirmed" // both parties confirmed (bilateral) or accepted (unilateral)
	StatusRevoked   AttestationStatus = "revoked"
)

// Attestation is a claim received via DKIM-signed email.
// Stored in the ledger exactly as received, with verification metadata.
type Attestation struct {
	ID              string            `json:"attestation_id"`
	Type            string            `json:"attestation_type"`
	AttestorDomain  string            `json:"attestor_domain"`  // DKIM-verified sending domain
	SubjectEmail    string            `json:"subject"`          // who the attestation is about
	Status          AttestationStatus `json:"status"`
	EdgeKind        EdgeKind          `json:"edge_kind"`
	DKIMVerified    bool              `json:"dkim_verified"`
	Payload         map[string]any    `json:"payload"`          // full JSON from email body
	PublishedFields map[string]any    `json:"published_fields"` // opted-in fields only
	ReceivedAt      time.Time         `json:"received_at"`
	ConfirmedAt     *time.Time        `json:"confirmed_at,omitempty"`
	RevokedAt       *time.Time        `json:"revoked_at,omitempty"`
}

// Confirmation is sent by the subject to confirm a bilateral attestation.
type Confirmation struct {
	AttestationID string `json:"attestation_id"`
	SenderDomain  string `json:"sender_domain"` // DKIM-verified
}

// Revocation removes an edge. Either party can send it.
type Revocation struct {
	AttestationID string `json:"attestation_id"`
	SenderDomain  string `json:"sender_domain"` // DKIM-verified
	Reason        string `json:"reason"`
}

// TrustEdge is a directed edge in the trust graph.
// Bilateral attestations create two edges (A→B and B→A).
// Unilateral attestations create one edge (attestor→subject).
type TrustEdge struct {
	ID             int64    `json:"id"`
	AttestationID  string   `json:"attestation_id"`
	FromDomain     string   `json:"from_domain"`
	ToDomain       string   `json:"to_domain"`
	Kind           EdgeKind `json:"kind"`
	AttestationType string  `json:"attestation_type"`
	Weight         float64  `json:"weight"` // signal strength (duration, volume, etc.)
	CreatedAt      time.Time `json:"created_at"`
}

// TrustNode is a node in the trust graph with aggregated edge info.
type TrustNode struct {
	Domain         string  `json:"domain"`
	EdgeCount      int     `json:"edge_count"`
	BilateralCount int     `json:"bilateral_count"`
	UnilateralCount int    `json:"unilateral_count"`
	OldestEdge     string  `json:"oldest_edge"`
	NewestEdge     string  `json:"newest_edge"`
}

// PublishPreference controls which optional fields an advertiser publishes.
type PublishPreference struct {
	SubjectEmail string   `json:"subject_email"`
	Publish      []string `json:"publish"`
	Redact       []string `json:"redact"`
}

// AttestationEmail is the structured JSON payload expected in attestation email bodies.
type AttestationEmail struct {
	Action        string         `json:"action,omitempty"`         // "confirm" or "revoke" (empty = new attestation)
	AttestationID string         `json:"attestation_id,omitempty"` // required for confirm/revoke
	Type          string         `json:"attestation_type,omitempty"`
	Subject       string         `json:"subject,omitempty"`
	Reason        string         `json:"reason,omitempty"` // for revocations
	Fields        map[string]any `json:"-"`                // all other fields captured dynamically
}
