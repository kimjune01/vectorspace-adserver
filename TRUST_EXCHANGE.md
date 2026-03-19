# Trust Exchange Protocol

The VectorSpace ad server includes a **trust exchange** — an SMTP server that receives DKIM-signed attestation emails, verifies signatures, and indexes them into a public trust graph. Publishers use the graph to filter which advertisers are eligible to appear on their pages.

The protocol is described in [Proof of Trust](https://june.kim/proof-of-trust).

## Architecture

```
Attestor (e.g., Stripe)         Subject (e.g., merchant)
       │                                │
       │ DKIM-signed email              │ DKIM-signed confirmation
       ▼                                ▼
  ┌─────────────────────────────────────────┐
  │         Exchange SMTP Server            │
  │         attestations@exchange.…         │
  │                                         │
  │  1. Verify DKIM signature               │
  │  2. Parse JSON body                     │
  │  3. Route: attest / confirm / revoke    │
  │  4. Write to append-only ledger         │
  └─────────────┬───────────────────────────┘
                │
                ▼
  ┌─────────────────────────────────────────┐
  │           Trust Graph (SQLite)          │
  │                                         │
  │  attestations: claims received          │
  │  trust_edges: verified relationships    │
  │  ledger_log: append-only audit trail    │
  └─────────────┬───────────────────────────┘
                │
                ▼
  ┌─────────────────────────────────────────┐
  │           Public HTTP API               │
  │                                         │
  │  GET /trust/graph      full edge list   │
  │  GET /trust/log        append-only log  │
  │  GET /trust/allowlist  curator query    │
  │  GET /trust/node/:d    single domain    │
  └─────────────────────────────────────────┘
                │
                ▼
          Curators / Publishers
```

## Email Protocol

Attestation emails are sent directly to `attestations@exchange.<domain>`. The exchange is discoverable via standard MX records.

### New Attestation

Sent by the attestor (e.g., Stripe) about a subject (e.g., a merchant).

```
From: attestations@stripe.com
To: attestations@exchange.vectorspace.exchange
DKIM-Signature: [cryptographic signature]
Subject: Payment Processing Attestation

{
  "attestation_type": "payment_processor",
  "attestation_id": "stripe_merchant123_2026",
  "subject": "merchant@example.com",
  "duration_years": 3,
  "status": "good_standing",
  "timestamp": "2026-03-18T15:00:00Z"
}
```

### Bilateral Confirmation

Sent by the subject to confirm a relationship. Both parties must send DKIM-signed emails for bilateral edges to be created.

```
From: merchant@example.com
To: attestations@exchange.vectorspace.exchange
DKIM-Signature: [cryptographic signature]
Subject: Attestation Confirmation

{
  "action": "confirm",
  "attestation_id": "stripe_merchant123_2026"
}
```

### Revocation

Either party can revoke at any time. No permission needed from the other party.

```
From: attestations@stripe.com
To: attestations@exchange.vectorspace.exchange
DKIM-Signature: [cryptographic signature]
Subject: Attestation Revocation

{
  "action": "revoke",
  "attestation_id": "stripe_merchant123_2026",
  "reason": "account_closed"
}
```

## Attestation Types

| Type | Edge Kind | Description |
|------|-----------|-------------|
| `payment_processor` | bilateral | Payment processing relationship (Stripe, Square, etc.) |
| `platform_rating` | unilateral | Platform's observation of public data (Google Reviews, Yelp) |
| `customer_endorsement` | bilateral | Customer confirms business relationship |
| `vendor_relationship` | bilateral | Supplier/vendor confirms trade relationship |
| `license` | unilateral | Licensing authority confirms credential |

**Bilateral** attestations require both parties to send emails. The exchange creates mutual edges (A ↔ B) only after both arrive.

**Unilateral** attestations create one-directional edges immediately. The subject doesn't confirm; the attestor stakes their reputation on accuracy.

Attestors extend with URI-prefixed types: `https://stripe.com/attestation/payment_processing`. Curators normalize across synonyms.

## Edge Weights

Weights are derived from the attestation payload:

- `duration_years`: weight = years (3 years → weight 3.0)
- `review_count`: weight = count / 100 (247 reviews → weight 2.47)
- Default: 1.0

## HTTP API

### Public Ledger

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/trust/graph` | GET | Full trust graph (all edges). Curators sync this. |
| `/trust/node/{domain}` | GET | Node info + edges for a single domain. |
| `/trust/attestation/{id}` | GET | Single attestation by ID. |
| `/trust/log?limit=N` | GET | Append-only ledger log (default 100 entries). |
| `/trust/allowlist?min_edges=N&min_bilateral=N` | GET | Domains meeting trust thresholds. |
| `/trust/publish` | PUT | Set field-level publish preferences. |

### HTTP Attestation Submission (Dev)

For development and API integration, attestations can also be submitted via HTTP POST. Production attestations arrive via DKIM-signed email.

```
POST /trust/attest

# New attestation
{
  "sender_domain": "stripe.com",
  "attestation_id": "stripe_merchant123_2026",
  "attestation_type": "payment_processor",
  "subject": "merchant@example.com",
  "duration_years": 3
}

# Confirm
{
  "sender_domain": "example.com",
  "action": "confirm",
  "attestation_id": "stripe_merchant123_2026"
}

# Revoke
{
  "sender_domain": "stripe.com",
  "action": "revoke",
  "attestation_id": "stripe_merchant123_2026",
  "reason": "account_closed"
}
```

## Trust Graph Queries

### Allowlist (Curator Primitive)

```
GET /trust/allowlist?min_edges=3&min_bilateral=1
```

Returns domains that meet the specified thresholds:

```json
{
  "domains": [
    {
      "domain": "example.com",
      "edge_count": 8,
      "bilateral_count": 6,
      "unilateral_count": 2,
      "oldest_edge": "2023-06-15T10:00:00Z",
      "newest_edge": "2026-03-18T15:00:00Z"
    }
  ],
  "count": 1,
  "criteria": {
    "min_edges": 3,
    "min_bilateral": 1
  }
}
```

### Node Detail

```
GET /trust/node/example.com
```

Returns aggregated trust info and all edges involving the domain:

```json
{
  "node": {
    "domain": "example.com",
    "edge_count": 8,
    "bilateral_count": 6,
    "unilateral_count": 2
  },
  "edges": [
    {
      "attestation_id": "stripe_merchant123_2026",
      "from_domain": "stripe.com",
      "to_domain": "example.com",
      "kind": "bilateral",
      "attestation_type": "payment_processor",
      "weight": 3.0
    }
  ]
}
```

## DKIM Verification

The exchange verifies DKIM signatures on all incoming emails using [go-msgauth](https://github.com/emersion/go-msgauth). When DKIM verification fails, the exchange:

1. Logs a warning
2. Falls back to the SMTP envelope sender domain
3. Marks `dkim_verified: false` on the attestation

Curators can use the `dkim_verified` field to filter unverified attestations.

## Infrastructure

The exchange runs as part of the VectorSpace ad server:

- **SMTP**: Port 25 (production) or 2525 (dev). Configurable via `--smtp-addr`.
- **Exchange domain**: Configurable via `--exchange-domain`. Default: `exchange.localhost`.
- **DNS**: MX record points to `exchange.<domain>`. A record resolves to the server IP.
- **Storage**: Shares the SQLite database with the ad server. WAL mode for concurrent access.

### Server Flags

```
--smtp-addr=:25               SMTP listen address
--exchange-domain=exchange.x   Mail domain for the trust exchange
```

### DNS Records (managed by Pulumi)

```
exchange.vectorspace.exchange.  A     <server-ip>
exchange.vectorspace.exchange.  MX    10 exchange.vectorspace.exchange.
```

## Security

| Concern | Mitigation |
|---------|------------|
| Forged attestations | DKIM signature verification. Faking requires compromising the attestor's mail server. |
| Unauthorized confirmation | Confirmer domain must match the subject's email domain. |
| Unauthorized revocation | Only the attestor or subject can revoke. Third parties are rejected. |
| Spam/DDoS | Standard SMTP rate limiting. 1 MB message size limit. Single recipient only. |
| Privacy | Extensible schemas with opt-in field publishing. Edges are public; field values are optional. |

## Database Schema

```sql
-- Claims received via DKIM-signed email
CREATE TABLE attestations (
    id TEXT PRIMARY KEY,
    attestation_type TEXT NOT NULL,
    attestor_domain TEXT NOT NULL,
    subject_email TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',  -- pending | confirmed | revoked
    edge_kind TEXT NOT NULL DEFAULT 'bilateral',  -- bilateral | unilateral
    dkim_verified INTEGER NOT NULL DEFAULT 0,
    payload TEXT NOT NULL DEFAULT '{}',
    published_fields TEXT NOT NULL DEFAULT '{}',
    received_at DATETIME NOT NULL,
    confirmed_at DATETIME,
    revoked_at DATETIME
);

-- Verified relationships in the trust graph
CREATE TABLE trust_edges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    attestation_id TEXT NOT NULL REFERENCES attestations(id),
    from_domain TEXT NOT NULL,
    to_domain TEXT NOT NULL,
    kind TEXT NOT NULL,  -- bilateral | unilateral
    attestation_type TEXT NOT NULL,
    weight REAL NOT NULL DEFAULT 1.0,
    created_at DATETIME NOT NULL
);

-- Append-only audit trail
CREATE TABLE ledger_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    action TEXT NOT NULL,  -- attestation | confirm | revoke
    attestation_id TEXT NOT NULL,
    sender_domain TEXT NOT NULL,
    raw_payload TEXT NOT NULL,
    dkim_verified INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL
);

-- Field-level disclosure preferences
CREATE TABLE publish_preferences (
    subject_email TEXT PRIMARY KEY,
    publish_fields TEXT NOT NULL DEFAULT '[]',
    redact_fields TEXT NOT NULL DEFAULT '[]',
    updated_at DATETIME NOT NULL
);
```
