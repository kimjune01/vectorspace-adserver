# VectorSpace Ad Server

Semantic ad auction with embedding-based relevance scoring, VCG pricing, and multi-platform SDKs.

## Architecture

```
cmd/server/main.go      — Server entry point (:8080)
platform/               — Core auction engine
  db.go                 — SQLite: advertisers, auctions, tokens, events, frequency_caps
  engine.go             — AuctionEngine: embed → filter → rank → VCG → charge
  registry.go           — PositionRegistry: in-memory + DB sync, ETag versioning
  billing.go            — BudgetTracker: per-advertiser spend tracking
  embedder.go           — HTTP client to embedding sidecar
auction/                — Pure auction algorithm
  auction.go            — RunAuction orchestrator
  ranking.go            — Score = log₅(price) - dist²/σ²
  vcg.go                — VCG payment with embedding adjustment
  embedding.go          — SquaredEuclideanDistance, ComputeEmbeddingScore
  floor.go              — Bid floor enforcement
handler/                — HTTP handlers
  routes.go             — Route wiring, CORS, logging middleware
  advertiser.go         — CRUD /advertiser/*, /positions, /budget/*
  publisher.go          — POST /ad-request, POST /ad-claim, GET /embeddings, POST /embed
  portal.go             — Advertiser portal (/portal/me/*) + Admin (/admin/*)
  events.go             — Impression tracking (/event/impression|click|viewable)
  chat.go               — Anthropic chat proxy
sidecar/main.py         — Embedding sidecar (BGE-small-en-v1.5, :8081)
```

The TEE enclave lives in a [standalone repo](https://github.com/kimjune01/vectorspace-enclave) for attestability. `auction_crosscheck_test.go` asserts bit-identical results between the adserver's `auction/` and the enclave's vendored copy.

## SDKs

| SDK | Path | Language |
|-----|------|----------|
| TypeScript | `demo/src/vectorspace-sdk.ts` | TS |
| Python | `sdk/vectorspace/client.py` | Python 3.10+ |
| iOS | `sdk-ios/` | Swift 5.9+ |
| Android | `sdk-android/` | Kotlin |

## Quick Start

```bash
# 1. Start embedding sidecar
cd sidecar && uv run main.py

# 2. Start server (seeded with 34 demo advertisers)
go run ./cmd/server/ -db-path=vectorspace.db -seed -admin-password=secret -sidecar-url=http://localhost:8081

# 3. Start demo (separate terminal)
cd demo && pnpm dev

# 4. Start portal (separate terminal)
cd portal && pnpm dev
```

## API

### Publisher
- `POST /ad-request` — Run auction. Body: `{intent, tau?, publisher_id?}`
- `POST /ad-claim` — Record SDK-side auction result. Body: `{winner_id, payment, publisher_id}`
- `GET /embeddings` — Advertiser embeddings (ETag caching)
- `POST /embed` — Embed text via sidecar

### Advertiser
- `POST /advertiser/register` — Register (returns token). Body: `{name, intent, sigma, bid_price, budget}`
- `PUT /advertiser/{id}` — Update
- `DELETE /advertiser/{id}` — Delete
- `GET /positions` — List all
- `GET /budget/{id}` — Budget info

### Events (called by SDKs)
- `POST /event/impression` — Log impression (frequency-capped)
- `POST /event/click` — Log click (triggers CPC charge on first click)
- `POST /event/viewable` — Log viewability

### Portal (token-authenticated)
- `GET /portal/me?token=` — Profile + budget
- `GET /portal/me/auctions?token=` — Auction history (CSV export)
- `GET /portal/me/events?token=` — Event stats

### Publisher Portal (token-authenticated)
- `GET /portal/publisher/stats?token=` — Aggregate stats
- `GET /portal/publisher/auctions?token=` — Auction history
- `GET /portal/publisher/revenue?token=&group_by=` — Revenue over time

### Admin (`X-Admin-Password` header)
- `POST /admin/publishers` — Create publisher
- `GET /admin/auctions` — Audit trail (CSV export)
- `GET /admin/revenue?group_by=` — Revenue analytics

## Deploy

Infrastructure managed with Pulumi in `infra/`. EC2 + Docker Compose (Go server + Python sidecar + Caddy) + S3/CloudFront + Route 53 + ACM.

```bash
cp infra/.env.example infra/.env  # fill in credentials
make deploy
```

## Testing

```bash
go test ./... -count=1        # All Go tests
cd portal && npx tsc --noEmit # Portal type-check
cd sdk-ios && swift test       # iOS SDK tests
```

## Privacy

SDK extracts intent and embeds it locally — no chat text leaves the device. The embedding is encrypted with the [enclave](https://github.com/kimjune01/vectorspace-enclave)'s attested public key and sent to the server as ciphertext. The server passes it to the TEE, which decrypts, runs the auction with live budgets, returns `{winner_id, payment}`, and zeros the embedding. The exchange never sees the query.
