# Privacy Architecture: SDK-Local & TEE Auction

## Problem

The standard ad-request flow leaks user data to the exchange:

| Leak | Where | Impact |
|------|-------|--------|
| Intent text ("I need therapy") | `POST /ad-request` body, stored in `auctions.intent`, encoded in `utm_term` | Exchange + advertiser see browsing context |
| User ID | `POST /event/*` body, stored in `events` + `frequency_caps` | Exchange tracks individual users |
| Chat messages | `POST /chat` for intent extraction | Exchange sees full conversation |

For health-adjacent verticals, this creates HIPAA exposure: intent text constitutes individually identifiable health information (IIHI) if combined with an IP address or user ID.

## Two-Phase Solution

### Phase 1: SDK-Local Auction (implemented)

The publisher SDK runs the entire auction on the user's device. The exchange becomes a billing ledger.

```
Browser                          Exchange
  |                                 |
  |  GET /embeddings                |
  |  (cached, ETag 304)            |
  |<--- embeddings + bid data ----->|
  |                                 |
  |  [user types in chat]           |
  |  [intent extracted locally]     |
  |  [embedding computed locally]   |
  |  [auction scored locally]       |
  |  [VCG payment computed]         |
  |                                 |
  |  POST /ad-claim                 |
  |  {winner_id, payment,           |
  |   publisher_id}                 |
  |----- no intent, no user_id ---->|
  |                                 |
  |<--- {auction_id, status} -------|
```

**What the exchange learns:** Which advertiser won, how much to charge, which publisher.
**What the exchange never sees:** Intent text, user ID, chat messages, embedding vector.

#### Components

- **`GET /embeddings`** returns `{id, name, embedding, bid_price, sigma, currency}` per advertiser. Cached via ETag.
- **`CloudX.runLocalAuction()`** replicates server scoring: `score = log_5(price) - dist^2/sigma^2`. VCG payment: `runner_up_price * 5^(distW^2/sigmaW^2 - distR^2/sigmaR^2)`, capped at winner's bid.
- **`POST /ad-claim`** records the claim for billing. Validates `winner_id` exists and `payment <= bid_price`. Logs with `intent = "[private]"`.
- **`FrequencyCapLocal`** tracks impressions in-memory per advertiser. No user ID ever leaves the browser.
- **`buildClickURL()`** omits `utm_term` when intent is `"[private]"`.

#### Limitations

- Budget enforcement is stale: SDK sees bid prices at sync time, not real-time remaining budget.
- Pacing not possible: exchange can't throttle delivery if it doesn't see requests.
- Fraud surface: publisher self-reports winner and payment. Mitigated by `payment <= bid_price` check.

### Phase 2: TEE-Encrypted Auction (planned)

Adds real-time budget/pacing without sacrificing privacy. The embedding is encrypted with the TEE's attested public key — the exchange operator cannot read it.

```
Browser                        TEE Enclave              Exchange DB
  |                               |                        |
  |  [compute embedding locally]  |                        |
  |                               |                        |
  |  [encrypt embedding with      |                        |
  |   TEE's attested public key]  |                        |
  |                               |                        |
  |  POST /ad-request-private     |                        |
  |  {ciphertext, publisher_id}   |                        |
  |------------------------------>|                        |
  |                               |  [decrypt embedding]   |
  |                               |  [load live bids,      |
  |                               |   budgets, pacing]     |
  |                               |<--- DB read ---------->|
  |                               |                        |
  |                               |  [compute distances]   |
  |                               |  [run full auction]    |
  |                               |  [destroy embedding]   |
  |                               |                        |
  |<-- {winner_id, payment} ------|  [log auction] ------->|
  |                               |                        |
```

**Privacy argument:**
1. Conversation never leaves the publisher.
2. Embedding encrypted in transit — exchange operator cannot read the ciphertext.
3. Inside TEE: embedding decrypted, distances computed, embedding destroyed after execution.
4. Exchange DB only stores `{winner_id, payment, publisher_id}` — never the embedding or intent.
5. HIPAA/HBNR "unsecured" safe harbor applies: no party acquires readable health information.

**Advantages over Phase 1:**
- Real-time budget enforcement (no stale bids)
- Pacing and frequency capping with live data
- Reduced fraud surface (auction runs in attested enclave, not publisher-controlled browser)
- Same privacy guarantees as Phase 1

#### TEE Attestation Flow

1. TEE enclave starts, generates keypair, produces attestation report.
2. SDK fetches attestation + public key, verifies against platform root of trust.
3. SDK encrypts embedding with attested public key before sending.
4. Enclave decrypts, runs auction, returns result. Embedding never persisted.
