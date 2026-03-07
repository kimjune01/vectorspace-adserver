# CloudX Ad Server

Semantic ad auction system with embedding-based relevance scoring, VCG pricing, and multi-platform publisher SDKs.

## Architecture

```
cmd/server/main.go      — Server entry point (:8080)
platform/               — Core auction engine
  db.go                 — SQLite: advertisers, auctions, tokens, events, frequency_caps
  engine.go             — AuctionEngine: embed → filter → rank → VCG → charge
  registry.go           — PositionRegistry: in-memory + DB sync
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

## SDKs

| SDK | Path | Language | Event Tracking |
|-----|------|----------|----------------|
| TypeScript | `demo/src/cloudx-sdk.ts` | TS | reportImpression, reportClick, reportViewable, observeViewability |
| Python | `sdk/cloudx/client.py` | Python 3.10+ | report_impression, report_click, report_viewable |
| iOS | `sdk-ios/` | Swift 5.9+ | Swift Package, Accelerate framework |
| Android | `sdk-android/` | Kotlin | OkHttp, Coroutines, minSdk 21 |

## Frontend

| App | Path | Port | Purpose |
|-----|------|------|---------|
| Demo | `demo/` | :6969 | Publisher simulation — chat UI, proximity dot, ad card, advertiser sidebar, replay mode |
| Portal | `portal/` | :6970 | Advertiser dashboard + Admin dashboard |

### Demo (`demo/`)

Publisher-facing simulation that demonstrates the full ad experience. Built with Vite + React + TypeScript + Tailwind v4.

- **Chat panel** — Anthropic-powered chat where user conversations trigger semantic ad matching
- **Proximity dot** — Glows when nearby advertiser expertise is detected (no auction fired yet)
- **Ad card** — Shown when user taps the dot; fires auction, displays winner, records impression
- **Advertiser sidebar** — Live CRUD for advertisers (add/edit/delete, adjust bid/sigma/budget)
- **Replay mode** — `?replay=true` runs scripted demo sequences with phase banners (no-match, proximity, tap)
- **Prebuilt conversations** — Dropdown menu to load canned chat flows
- **Publisher themes** — Configurable theme context (colors, default tau)
- **TypeScript SDK** — `cloudx-sdk.ts` lives here, used by both demo and as reference for other SDKs

## Quick Start

```bash
# 1. Start embedding sidecar
cd sidecar && uv run main.py

# 2. Start server (seeded with 34 demo advertisers)
go run ./cmd/server/ -db-path=cloudx.db -seed -admin-password=secret -sidecar-url=http://localhost:8081

# 3. Start demo (separate terminal)
cd demo && pnpm dev

# 4. Start portal (separate terminal)
cd portal && pnpm dev

# 5. Create a publisher with credentials (admin-protected)
curl -X POST localhost:8080/admin/publishers \
  -H 'Content-Type: application/json' \
  -H 'X-Admin-Password: secret' \
  -d '{"name":"TechBlog","domain":"techblog.com","email":"pub@test.com","password":"pass123"}'

# 6. Open publisher login at http://localhost:6970/publisher/login
```

## API Endpoints

### Publisher
- `POST /ad-request` — Run auction (server-side). Body: `{intent, tau?, publisher_id?}`. Returns `{auction_id, winner, payment, ...}`
- `POST /ad-claim` — Record a publisher-reported auction result (private flow). Body: `{winner_id, payment, publisher_id}`. Returns `{auction_id, status}`
- `POST /publisher/register` — Register publisher (admin-protected). Body: `{name, domain?}`. Returns `{id, name, domain, token}`
- `POST /publisher/login` — Publisher login. Body: `{email, password}`. Returns `{token, publisher_id}`
- `GET /embeddings` — All advertiser embeddings + bid data (ETag caching)
- `POST /embed` — Embed arbitrary text

### Advertiser Management
- `POST /advertiser/register` — Register (returns token). Body: `{name, intent, sigma, bid_price, budget}`
- `PUT /advertiser/{id}` — Update advertiser
- `DELETE /advertiser/{id}` — Delete advertiser
- `GET /positions` — List all advertisers
- `GET /budget/{id}` — Budget info

### Event Tracking (called by SDKs)
- `POST /event/impression` — Log impression (frequency-capped). Body: `{auction_id, advertiser_id, user_id?, publisher_id?}`
- `POST /event/click` — Log click
- `POST /event/viewable` — Log viewability

### Portal (token-authenticated)
- `GET /portal/me?token=` — Own profile + budget
- `PUT /portal/me?token=` — Update profile
- `GET /portal/me/auctions?token=` — Own auction history
- `GET /portal/me/events?token=` — Own event stats

### Publisher Portal (token-authenticated)
- `GET /portal/publisher/me?token=` — Publisher profile
- `GET /portal/publisher/stats?token=` — Aggregate stats (auctions, revenue)
- `GET /portal/publisher/revenue?token=&group_by=` — Revenue over time
- `GET /portal/publisher/events?token=` — Impression/click/viewable stats
- `GET /portal/publisher/auctions?token=&limit=&offset=` — Auction history
- `GET /portal/publisher/top-advertisers?token=&limit=` — Top advertisers on property

### Admin (protected by `X-Admin-Password` header)
- `POST /admin/publishers` — Create publisher with credentials. Body: `{name, domain, email, password}`. Returns `{id, name, domain, email, token}`
- `GET /admin/auctions?limit=&offset=&winner=&intent=&format=csv` — Audit trail
- `GET /admin/revenue?group_by=day|week|month` — Revenue analytics
- `GET /admin/top-advertisers?limit=` — Top spenders
- `GET /admin/advertisers` — All advertisers with budget data
- `GET /admin/events` — Global event analytics

## Configuration

### Server Flags
| Flag | Default | Description |
|------|---------|-------------|
| `-sidecar-url` | `http://localhost:8081` | Embedding sidecar URL |
| `-db-path` | (empty) | SQLite path (empty = in-memory) |
| `-seed` | false | Seed 34 demo advertisers |
| `-anthropic-key` | `$ANTHROPIC_API_KEY` | For /chat proxy |
| `-freq-cap-max` | 3 | Max impressions per user per window |
| `-freq-cap-window` | 60 | Frequency cap window (minutes) |
| `-admin-password` | `$ADMIN_PASSWORD` | Password for admin endpoints (empty = no auth) |

### Portal Environment
| File | `VITE_API_URL` |
|------|----------------|
| `.env.development` | `http://localhost:8080` |
| `.env.staging` | `https://staging-api.cloudx.dev` |
| `.env.production` | `https://api.cloudx.dev` |

## Build & Deploy

```bash
make test          # Go tests
make test-portal   # Portal type-check + build
make build         # Production build (server binary + portal)
make staging       # Staging build
make docker-build  # Docker image
```

## Testing

```bash
go test ./... -count=1        # All Go tests (platform + handler)
cd portal && npx tsc --noEmit # Portal type-check
cd sdk-ios && swift test       # iOS SDK tests (14 tests)
```

## Privacy

See [PRIVACY.md](PRIVACY.md) for the full architecture.

- **Phase 1 (implemented):** SDK runs the auction locally. Exchange only learns `{winner_id, payment}` via `POST /ad-claim`. No intent, user ID, or chat messages ever leave the browser.
- **Phase 2 (planned):** Embedding encrypted with TEE's attested public key. Auction runs inside enclave with live bids/budgets. Embedding destroyed after execution. HBNR safe harbor applies.

## Key Design Decisions

- **Score** = log₅(price) - distance²/σ² — balances bid price vs. semantic relevance
- **VCG pricing** — winner pays runner-up's competitive price, adjusted for embedding distance
- **Exchange cut** = 15% of payment (publisher gets 85%)
- **ETag caching** — SDKs cache embeddings locally, sync efficiently via If-None-Match
- **Frequency caps** — per-advertiser per-user, configurable window (default 3/60min)
- **Tokens** — 32-char hex via crypto/rand, returned on registration
