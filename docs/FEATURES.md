# Vector Space Exchange

## Vision

An open, embedding-space ad auction protocol that replaces keyword advertising for AI chatbot conversations -- connecting people who need help to people who can provide it, verifiably and privately.

### Pillars

1. **Embedding-space scoring** -- `score = log(bid) - dist²/sigma²` produces power diagrams over the embedding space, giving each advertiser a continuous region of relevance rather than a discrete keyword list.

2. **Two-phase UX** -- A passive proximity indicator signals that relevant help exists; the auction only fires when the user opts in. No interruption, no surveillance. ("Ask First")

3. **TEE attestation** -- Auctions execute inside a Trusted Execution Environment. The exchange operator never sees the query embedding. Attestation proves model blindness to publishers and users.

4. **Relocation fees** -- Moving your position in embedding space costs money (proportional to distance moved). This prevents Hotelling drift, stabilizes the market, and rewards commitment.

5. **Open protocol** -- Keywords are the special case where sigma approaches zero. The protocol is backwards-compatible with keyword advertising but generalizes to the full embedding space.

6. **Privacy by construction** -- The query embedding never leaves publisher infrastructure (phase 1) or the TEE (phase 2). There is no behavioral profile, no cookie, no tracking pixel.

7. **Marketing-speak as protocol** -- Advertisers submit natural-language positioning statements ("I help people recover from knee surgery"), not keyword lists. The embedding model converts these to positions.

8. **Health vertical first** -- AI health chatbots have massive, unmonetized traffic because health ads trigger FTC/FDA compliance nightmares. Embedding-space ads are contextual, not behavioral, making them safe for regulated verticals.

---

## What's Missing

The 7 highest-impact gaps between the vision and the current implementation, ordered by deployment priority.

### Gap 1: ~~No Publisher-Facing Widget~~ → Partially addressed

`sdk-web/` (`@vectorspace/sdk`) is now the **reference SDK** — a UI-agnostic TypeScript package providing the full API surface for publishers: embedding sync, ad requests, TEE-encrypted auctions, intent extraction, event tracking, and IAB viewability observation. iOS, Android, and Python SDKs follow its API surface.

The two-phase UX (proximity dot -> tap -> auction) is **not** part of the SDK — it's a UI concern for publisher-specific widget implementations built on top of the SDK.

**Remaining:** Build example widget implementations (React component, vanilla JS snippet) that demonstrate the proximity dot + ad card pattern using the SDK.

Blog refs: *Ask First*, *Model Blindness*

### Gap 2: ~~No Relocation Fees~~ → Addressed

Position history tracking and relocation fee computation are implemented. `position_history` table records every position change with distance moved and fee charged. `RelocationFeeConfig` on `PositionRegistry` supports entry bonds and distance-proportional move costs (`fee = distanceFactor * sqrt(distance²)`). `ComputeRelocationFee` computes fees; `GetTotalRelocationFees` tracks revenue.

**Remaining:** UI for fee schedule configuration, fee revenue distribution to other advertisers (relocation fee dividend), commitment chain validation.

Blog refs: *It Costs Money to Move*, *Synthetic Friction*, *Stay or Pay*, *Relocation Fee Dividend*

### Gap 3: No Landing Page

`vectorspace.exchange` needs a public face that explains the product to both publishers (AI chatbot companies) and advertisers (specialist businesses). The landing page should communicate the value prop for each side of the marketplace.

**What's needed:** A static landing page at `landing/index.html` with publisher and advertiser CTAs, protocol explanation, and trust signals.

Blog refs: *The Easiest Sale*, *The Last Ad Layer*

### Gap 4: No Advertiser Trust Signals

No payment processor integration, no composite trust score, no semantic consistency checking. Open registration without trust verification is a spam/scam vector. The blog describes layered trust: payment history, reviews, web presence, booking platforms.

**What's needed:** Payment processor integration (Stripe), composite trust score computation, semantic consistency checking (does the ad match the landing page?), progressive trust unlocking.

Blog refs: *How to Trust Advertisers*, *Stay or Pay*

### Gap 5: ~~No Position History / Tenure System~~ → Addressed

`position_history` table tracks every position change with intent, embedding, sigma, bid_price, distance_moved, and relocation_fee. `GetPositionHistory`, `GetPositionCount`, `GetTenureDays` provide query access. Position changes are automatically recorded on `RegisterWithBudget` and `Update`.

**Remaining:** Commitment chain validation, tenure discounts for long-standing positions.

Blog refs: *Stay or Pay*, *The Last Signal*

### Gap 6: ~~Log Base Not Configurable~~ → Addressed

Log base is now configurable per-publisher via the `log_base` column in the `publishers` table (default 5.0). The `auction/` package exposes `RunAuctionWithBase`, `ComputeEmbeddingScoreWithBase`, and `ComputeVCGPaymentWithBase` for parameterized scoring. The TEE handler looks up the publisher's log base and passes it to the enclave. Publishers can set their log base via `SetPublisherLogBase` in `platform/db.go`.

Blog refs: *The Price of Relevance*, *Three Levers*

### Gap 7: No Health Vertical Seed Data

*The Easiest Sale* says start with health chatbots -- massive unmonetized traffic, FTC compliance fears keep traditional ads out, and embedding-space contextual ads sidestep the regulatory problem. No health-specific advertiser seed data exists, no publisher integration guide, no outreach materials.

**What's needed:** Health vertical seed advertisers (physical therapists, nutritionists, mental health providers), publisher integration guide for health chatbot companies, compliance documentation.

Blog refs: *The Easiest Sale*, *Monetizing the Untouchable*

---

## Roadmap

Ordered implementation plan to close the gaps above.

| Phase | Gap | Milestone | Depends on |
|-------|-----|-----------|------------|
| 1 | Gap 3 | Landing page live at `vectorspace.exchange` | -- |
| 2 | Gap 1 | Publisher widget SDK (`sdk-web/`) with proximity dot + ad card | -- |
| 3 | Gap 6 | Configurable log base per publisher | -- |
| 4 | Gap 5 | Position history table + tenure tracking | -- |
| 5 | Gap 2 | Relocation fees (entry bond + move cost) | Phase 4 |
| 6 | Gap 4 | Advertiser trust signals (Stripe + trust score) | -- |
| 7 | Gap 7 | Health vertical seed data + integration guide | Phases 1, 2 |

---

# Feature Reference

User stories and capabilities by stakeholder, with code pointers.

---

## Advertiser

### Registration & Profile

| Story | Endpoint | Code |
|-------|----------|------|
| Register with positioning statement (intent, sigma, bid, budget) | `POST /advertiser/register` | `handler/advertiser.go` HandleRegister |
| Get back an API token on registration | `POST /advertiser/register` | `platform/db.go` GenerateToken |
| Update profile (name, intent, sigma, bid price, URL) | `PUT /advertiser/{id}` | `handler/advertiser.go` handleUpdate |
| Delete my account | `DELETE /advertiser/{id}` | `handler/advertiser.go` handleDelete |
| View my profile via token | `GET /portal/me?token=` | `handler/portal.go` HandlePortalMe |
| Edit profile via portal | `PUT /portal/me?token=` | `handler/portal.go` handlePortalMeUpdate |

### Budget

| Story | Endpoint | Code |
|-------|----------|------|
| Check budget (total, spent, remaining) | `GET /budget/{id}` | `handler/advertiser.go` HandleBudget |
| Top up budget via portal | `PUT /portal/me?token=` | `handler/portal.go` handlePortalMeUpdate |
| Only charged on click (CPC billing) | `POST /event/click` | `handler/events.go` HandleClick |
| First click per auction charges VCG payment | `POST /event/click` | `handler/events.go` HandleClick, `platform/db.go` Charge |

### Creatives

| Story | Endpoint | Code |
|-------|----------|------|
| Create ad creative (title + subtitle) | `POST /portal/me/creatives?token=` | `handler/portal.go` HandlePortalCreatives |
| List my creatives | `GET /portal/me/creatives?token=` | `handler/portal.go` HandlePortalCreatives |
| Edit a creative | `PUT /portal/me/creatives/{id}?token=` | `handler/portal.go` HandlePortalCreative |
| Delete a creative | `DELETE /portal/me/creatives/{id}?token=` | `handler/portal.go` HandlePortalCreative |
| Winner's active creative shown in ad response | `POST /ad-request` | `platform/engine.go` RunAdRequestFull |
| Creative shown in simulated auctions | `POST /simulate` | `platform/engine.go` SimulateAuction |

DB: `creatives` table (`platform/db.go`), CRUD methods: InsertCreative, UpdateCreative, DeleteCreative, GetCreativesByAdvertiser, GetActiveCreative.

### Analytics

| Story | Endpoint | Code |
|-------|----------|------|
| View auctions I won (paginated) | `GET /portal/me/auctions?token=` | `handler/portal.go` HandlePortalAuctions |
| Export my auctions as CSV | `GET /portal/me/auctions?token=&format=csv` | `handler/portal.go` HandlePortalAuctions |
| View event stats (impressions, clicks, viewable) | `GET /portal/me/events?token=` | `handler/portal.go` HandlePortalEvents |

### Dashboard UI

File: `portal/src/pages/advertiser/Dashboard.tsx`

- Budget progress bar (spent / total)
- Stat cards: impressions, clicks, viewable, CTR, CPC, auctions won
- Profile edit form (name, intent, sigma, bid price, budget top-up, URL)
- Creatives section: list, inline edit, delete, add form
- Auction history table with pagination and CSV export

---

## Publisher

### Registration & Auth

| Story | Endpoint | Code |
|-------|----------|------|
| Admin creates publisher with credentials | `POST /admin/publishers` | `handler/publisher.go` HandleCreatePublisherWithCredentials |
| Admin registers publisher (token only) | `POST /publisher/register` | `handler/publisher.go` HandleRegisterPublisher |
| Log in with email/password | `POST /publisher/login` | `handler/publisher.go` HandlePublisherLogin |

Auth: bcrypt password hashing, token returned on login. DB tables: `publishers`, `publisher_tokens`, `publisher_credentials`.

### Ad Serving

| Story | Endpoint | Code |
|-------|----------|------|
| Request an ad (TEE encrypted) | `POST /ad-request` | `handler/tee_handler.go` HandleAdRequestPrivate |
| Filter ads by relevance threshold (tau) | `POST /ad-request` body `{tau}` | `platform/engine.go` RunAdRequestFull |
| Embed arbitrary text | `POST /embed` | `handler/publisher.go` HandleEmbed |
| Simulate auction (admin-only, no billing/logging) | `POST /simulate` | `handler/publisher.go` HandleSimulate |

### Portal Analytics

| Story | Endpoint | Code |
|-------|----------|------|
| View my profile | `GET /portal/publisher/me?token=` | `handler/portal.go` HandlePublisherMe |
| View aggregate stats (auctions, revenue) | `GET /portal/publisher/stats?token=` | `handler/portal.go` HandlePublisherStats |
| Revenue by period (day/week/month) | `GET /portal/publisher/revenue?token=` | `handler/portal.go` HandlePublisherRevenue |
| Event stats (impressions, clicks, viewable) | `GET /portal/publisher/events?token=` | `handler/portal.go` HandlePublisherEvents |
| Auction history (paginated) | `GET /portal/publisher/auctions?token=` | `handler/portal.go` HandlePublisherAuctions |
| Top advertisers on my property | `GET /portal/publisher/top-advertisers?token=` | `handler/portal.go` HandlePublisherTopAdvertisers |

Revenue split: publishers receive 85%, exchange keeps 15%. Constant `exchangeCut = 0.15` in `platform/db.go`.

### Dashboard UI

File: `portal/src/pages/publisher/Dashboard.tsx`

- Profile card (name, domain, ID)
- Stats: auction count, total revenue, RPM, avg payment, clicks, viewable
- Revenue chart (line, groupable by day/week/month)
- Top advertisers bar chart
- Auction history table with pagination

---

## End User / SDK

### TypeScript SDK (Reference)

Package: `sdk-web/` (`@vectorspace/sdk`) — **source of truth** for all SDKs.

| Capability | Method | Notes |
|------------|--------|-------|
| Sync embeddings | `syncEmbeddings()` | `GET /embeddings` with ETag/304 caching |
| Embed text to vector | `embed(text)` | `POST /embed` sidecar proxy |
| Local proximity search | `proximity(query)` | Squared Euclidean distance, sorted |
| Request ad (server auction) | `requestAd(options)` | `POST /ad-request` with intent + tau |
| Extract intent from chat | `extractIntent(messages)` | `POST /chat` (Claude API) |
| Chat-to-ad | `requestAdFromChat(messages, tau)` | extractIntent + requestAd |
| TEE encrypted auction | `requestAdTEE(options)` | Hybrid RSA-OAEP + AES-256-GCM |
| Fetch enclave attestation | `fetchAttestation()` | Caches RSA public key |
| Report impression | `reportImpression(...)` | Returns false on 429 frequency cap |
| Report click | `reportClick(...)` | Triggers CPC billing |
| Report viewable | `reportViewable(...)` | IAB standard |
| Auto viewability | `observeViewability(el, ...)` | Separate `./viewability` entry point, IntersectionObserver 50% for 1s |

Other SDKs follow this API surface: `sdk-ios/` (Swift), `sdk-android/` (Kotlin), `sdk/` (Python).

### Auction Scoring

```
score = log5(bid_price) - (distance^2 / sigma^2)
```

VCG payment = runner_up.bid * 5^((winner_dist^2/sigma_w^2 - runner_dist^2/sigma_r^2)), capped at winner's bid.

Code: `auction/auction.go` (RunAuction), `auction/vcg.go` (ComputeVCGPayment).

---

## Admin / Exchange

### Endpoints (all require `X-Admin-Password` header)

| Story | Endpoint | Code |
|-------|----------|------|
| View all auctions (filter by winner/intent) | `GET /admin/auctions` | `handler/portal.go` HandleAdminAuctions |
| Export auctions CSV | `GET /admin/auctions?format=csv` | `handler/portal.go` HandleAdminAuctions |
| Revenue by period | `GET /admin/revenue?group_by=` | `handler/portal.go` HandleAdminRevenue |
| Top advertisers by spend | `GET /admin/top-advertisers` | `handler/portal.go` HandleAdminTopAdvertisers |
| List all advertisers with budgets | `GET /admin/advertisers` | `handler/portal.go` HandleAdminAdvertisers |
| List all publishers | `GET /admin/publishers` | `handler/portal.go` HandleAdminPublishers |
| Create publisher with credentials | `POST /admin/publishers` | `handler/publisher.go` HandleCreatePublisherWithCredentials |
| Global event stats | `GET /admin/events` | `handler/portal.go` HandleAdminEvents |
| Simulate auction (no billing) | `POST /simulate` | `handler/publisher.go` HandleSimulate |
| Platform stats (auctions, revenue, counts) | `GET /stats` | `handler/routes.go` |
| Reset auction logs | `DELETE /stats` | `handler/routes.go` |

Auth middleware: `handler/routes.go` adminAuthMiddleware. If AdminPassword is empty, all requests pass (dev mode).

### Dashboard UI

File: `portal/src/pages/admin/Overview.tsx`

- Stat cards: auctions, revenue, exchange cut, impressions, clicks, advertiser count, publisher count
- Revenue chart (line, day/week/month)
- Top advertisers bar chart

Other admin pages:
- `portal/src/pages/admin/AuctionLog.tsx` -- auction log with filters + CSV export
- `portal/src/pages/admin/Advertisers.tsx` -- advertiser list with budget usage
- `portal/src/pages/admin/Publishers.tsx` -- publisher list

---

## Event Tracking

| Event | Endpoint | Billing? | Code |
|-------|----------|----------|------|
| Impression | `POST /event/impression` | No | `handler/events.go` HandleImpression |
| Click | `POST /event/click` | Yes (first click charges VCG payment) | `handler/events.go` HandleClick |
| Viewable | `POST /event/viewable` | No | `handler/events.go` HandleViewable |

All events: body `{ auction_id, advertiser_id, user_id?, publisher_id? }`.

Server-side frequency cap: default 3 impressions per 60 minutes per (advertiser, user) pair. Configurable via `FREQ_CAP_MAX`, `FREQ_CAP_WINDOW` env vars. Code: `platform/db.go` CheckFrequencyCap, IncrementFrequencyCap.

---

## TEE (Trusted Execution Environment)

All auctions run through the TEE enclave. The exchange operator never sees the query embedding.

| Story | Endpoint | Code |
|-------|----------|------|
| Get enclave attestation + public key | `GET /tee/attestation` | `handler/tee_handler.go` HandleAttestation |
| Run encrypted auction (intent never seen by server) | `POST /ad-request` | `handler/tee_handler.go` HandleAdRequestPrivate |

Enclave code: `enclave/` (self-contained, vendored auction logic, stdlib-only). Proxy interface: `tee/interface.go`.

SDK encryption: hybrid RSA-OAEP + AES-256-GCM (`demo/src/cloudx-sdk.ts` encryptEmbedding).

---

## Global

| Story | Endpoint | Code |
|-------|----------|------|
| Health check | `GET /health` | `handler/routes.go` |
| Chat proxy (Claude API) | `POST /chat` | `handler/chat.go` HandleChat |

---

## Demo App (archived — deleted from repo)

The demo app was a React + TypeScript + Vite application that showcased the full publisher integration experience. Key features it demonstrated:

### Chat-to-Ad Flow
- Multi-turn chat interface with LLM-powered intent extraction
- Proximity dot: a glowing indicator that brightened as conversation intent approached available ads
- Clicking the dot triggered a TEE-encrypted auction and displayed the winning ad in a modal card
- Revenue breakdown (publisher 85% / exchange 15%) shown in auction summary

### Replay Mode (`?replay=true`)
- Automated walkthrough of the ad lifecycle: off-topic message (no dot) -> deepening conversation (dot brightens) -> user tap (auction fires, revenue earned)
- Scripted multi-step demo with phase banners explaining each stage

### Publisher Theming
- 10 publisher themes (Chai, Amp, Luzia, Kindroid, Galen AI, Autonomous, Sonia, YouLearn, Alice)
- URL-driven theme switching (`?publisher=<id>`)
- Each theme had distinct colors, greeting messages, and default tau thresholds

### Advertiser Sidebar
- Live CRUD for advertiser positions (name, intent, sigma, bid price, budget)
- Immediate effect on subsequent auctions

### Prebuilt Conversations
- 12 canned conversations across verticals: physical therapy, tutoring, finance, dog training, health, therapy, education
- 3 "no-ad" conversations (casual chat, recipe, travel) demonstrating clean UX when no intent is detected

### Probe Tool (`/probe`)
- Standalone intent input with tau slider
- Direct auction results: winner, scores, distances, full bidder ranking table
- Useful for testing embedding space and auction mechanics without conversational context

### Running Totals
- Live polling of `/stats` every 3 seconds
- Displayed auction count, publisher revenue, exchange revenue
- Reset button for demo sessions

---

## Database Tables

| Table | Purpose | Key columns |
|-------|---------|-------------|
| `advertisers` | Advertiser profiles + budgets | id, name, intent, embedding, sigma, bid_price, budget_total, budget_spent, url |
| `auctions` | Auction results log | id, intent, winner_id, payment, currency, bid_count, publisher_id |
| `tokens` | Advertiser API tokens | token, advertiser_id |
| `creatives` | Ad creatives (title + subtitle) | id, advertiser_id, title, subtitle, active |
| `events` | Impression/click/viewable tracking | id, auction_id, advertiser_id, event_type, user_id, publisher_id |
| `frequency_caps` | Per-user impression throttling | advertiser_id, user_id, impression_count, window_start |
| `publishers` | Publisher registry | id, name, domain |
| `publisher_tokens` | Publisher API tokens | token, publisher_id |
| `publisher_credentials` | Publisher login credentials | publisher_id, email, password_hash |

Schema: `platform/db.go` createTables.

---

## Frontend Routes

File: `portal/src/router.tsx`

| Path | Page | File |
|------|------|------|
| `/` | Redirects to `/admin` | -- |
| `/advertiser` | Advertiser dashboard | `portal/src/pages/advertiser/Dashboard.tsx` |
| `/publisher` | Publisher dashboard | `portal/src/pages/publisher/Dashboard.tsx` |
| `/publisher/login` | Publisher login | `portal/src/pages/publisher/Login.tsx` |
| `/admin/login` | Admin login | `portal/src/pages/admin/Login.tsx` |
| `/admin` | Admin overview (guarded) | `portal/src/pages/admin/Overview.tsx` |
| `/admin/auctions` | Admin auction log (guarded) | `portal/src/pages/admin/AuctionLog.tsx` |
| `/admin/advertisers` | Admin advertiser list (guarded) | `portal/src/pages/admin/Advertisers.tsx` |
| `/admin/publishers` | Admin publisher list (guarded) | `portal/src/pages/admin/Publishers.tsx` |
