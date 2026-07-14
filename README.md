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
  publisher.go          — GET /embeddings, POST /embed, POST /simulate, /openrtb2/auction
  tee_handler.go        — POST /ad-request (encrypted), GET /tee/attestation
  portal.go             — Advertiser portal (/portal/me/*) + Admin (/admin/*)
  events.go             — Impression tracking (/event/impression|click|viewable)
  chat.go               — Anthropic chat proxy
sidecar/main.py         — Embedding sidecar (BGE-small-en-v1.5, :8081)
```

The TEE enclave lives in a [standalone repo](https://github.com/kimjune01/vectorspace-enclave) for attestability. `auction_crosscheck_test.go` cross-validates the adserver's `auction/` against the enclave's vendored copy on a fixed set of query vectors — a differential spot-check between two independent implementations, not a proof of equivalence. Known gaps: the copies still differ on floor-rounding (`auction/floor.go` exact vs `enclave/auction/floor.go` 4-decimal) outside the tested vectors, and CI checks the vendored copy rather than the standalone repo actually built into the attested binary. Closing both is on the [roadmap](ROADMAP.md).

## SDKs

| SDK | Path | Language |
|-----|------|----------|
| TypeScript | `demo/src/vectorspace-sdk.ts` | TS |
| Python | `sdk/vectorspace/client.py` | Python 3.10+ |
| iOS | `sdk-ios/` | Swift 5.9+ |
| Android | `sdk-android/` | Kotlin |

## Quick Start

```bash
# 1. Start embedding sidecar (first run downloads BAAI/bge-small-en-v1.5, ~130 MB;
#    needs network once, then runs offline. Wait for it to log "ready" before step 2 —
#    the Go server's embed calls fail until the sidecar is up.)
cd sidecar && uv run main.py

# 2. Start server (seeded with 34 demo advertisers)
go run ./cmd/server/ -db-path=vectorspace.db -seed -admin-password=secret -sidecar-url=http://localhost:8081

# 3. Start demo (separate terminal)
cd demo && pnpm dev

# 4. Start portal (separate terminal)
cd portal && pnpm dev
```

## API

Full machine-readable surface: [`apidocs/openapi.yaml`](apidocs/openapi.yaml). The running server serves it at `GET /openapi.json` and `GET /openapi.yaml`, with a rendered reference at `GET /docs` (Redoc, vendored — no CDN). Point an agent at `/openapi.json` to generate a client. The list below is a guide; the spec is authoritative.

### Publisher
- `POST /ad-request` — Private auction path. Body is an **encrypted envelope**: `{encrypted_embedding: {aes_key_encrypted, encrypted_payload, nonce, hash_algorithm?}, publisher_id?, tau?}` (`hash_algorithm` defaults to SHA-256; all three of key, payload, and nonce are required or decryption fails). Plaintext embeddings are rejected with `400`. Encrypt against the key from `GET /tee/attestation`. (For a plaintext auction in local dev, use `POST /simulate` or `POST /openrtb2/auction` — see below.)
- `GET /tee/attestation` — Enclave public key + attestation document. Note: in the default (mock) deployment this returns a placeholder document, not a real hardware attestation — see [Privacy](#privacy).
- `POST /simulate` (admin) — Plaintext auction for local dev and testing. Body: `{intent, tau?}`.
- `POST /openrtb2/auction` — OpenRTB 2.5 wire format: standard BidRequest in (query from `ext.vectorspace.{embedding,intent}` or comma-separated `content.keywords`/`user.keywords`, each keyword matched separately), standard BidResponse out. Two declared deviations from stock 2.5 semantics: settlement is per-click VCG (`bid.price` = amount charged on click, `bid.ext.vectorspace.settlement = "cpc-vcg"`), and `adm` is an HTML snippet whose click-through routes via `GET /click` — rendering it as-is settles correctly. `test=1` is honored (never logged, never billable). Plaintext path; the private path is `/ad-request`
- `GET /click?auction_id=` — settlement redirect: charges the winner's budget on first click, then 302s to the advertiser URL
- `GET /embeddings` — Advertiser embeddings (ETag caching)
- `POST /embed` — Embed text via sidecar

### Advertiser
- `POST /advertiser/register` — Register (returns token). Body: `{name, intent, sigma, bid_price, budget}` or `{name, keywords: [...], bid_price, budget}` — the keyword-import path: one position per keyword at σ = 0 (the exact-match limit), all spending one shared budget held by the first position
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

Two phases (full detail in [PRIVACY.md](PRIVACY.md)).

**Phase 1 — implemented.** The SDK extracts intent and embeds it locally, runs the auction on-device, and posts only `{winner_id, payment, publisher_id}`. No chat text, user ID, or embedding vector leaves the device; the exchange is a billing ledger.

**Phase 2 — implemented behind the `nitro` build tag; not yet hardware-validated.** An embedding encrypted against the [enclave](https://github.com/kimjune01/vectorspace-enclave)'s attested key is decrypted and auctioned inside a TEE, so the exchange operator cannot read it. The pieces now exist: the enclave binary built with `-tags nitro` requests a real AWS NSM attestation document (`cmd/enclave/attestation_nitro.go`) and listens over real vsock (`enclave/listen_nitro.go`), and a reference verifier (`vectorspace/verify`) checks the COSE_Sign1 signature, the certificate chain to a **caller-pinned** root, PCRs against a **caller-supplied** allowlist, and freshness, then hands back **only the attested key** to encrypt to (`verify.AttestedRSAKey`). What remains: (1) the pieces are **not yet integrated end-to-end** — the web SDK (`sdk-web/src/tee.ts`) still encrypts to the unverified `public_key` from `/tee/attestation` instead of verifying and using the attested key, and no production AWS Nitro root or PCR allowlist is wired into this repo; (2) the **default build** (`--tee` off, no `nitro` tag) still runs a `MockTEEProxy` that decrypts **in the exchange's own process** with a stub attestation — so the default deployment sees the embedding, by design, for local dev; (3) none of the `nitro` path has been validated against a **real captured attestation on EC2 Nitro hardware** (the verifier is exercised only against self-minted documents); (4) freshness is **timestamp-based** (`MaxAge`), not challenge-response — for key attestation, replaying an old document only encrypts to a stale enclave key (a liveness concern), not a disclosure. Until these close, the Phase 1 on-device path is the guarantee that holds in production.

## Known gaps (deferred, tracked)

Flagged here so scrutiny finds them stated rather than hidden. In rough priority:

- **`/stats` is unauthenticated** — `GET` leaks stats, `DELETE` wipes the auction log. TODO in `handler/routes.go`; gate behind `adminAuthMiddleware` before any non-dev deploy.
- **Missing `nonce` → 500, not 400** — `/ad-request` validates key and payload but not nonce; an absent nonce fails at decryption. TODO in `handler/tee_handler.go`.
- **TEE attestation: components implemented behind `nitro`, not integrated or hardware-validated** — real NSM attestation, vsock, and a reference verifier now exist (see Privacy), but they are not wired into an end-to-end flow: the web SDK still encrypts to the unverified `public_key` rather than the attested key, no production AWS root/PCR policy is pinned, and nothing has run against a real captured attestation on EC2 Nitro. The default build still decrypts in-process. The `nitro` target adds `hf/nsm` + `mdlayher/vsock` deps (the default build and the standalone enclave package stay stdlib-only). The standalone enclave repo also needs its listener/attestation synced with the adserver's vendored copy so the crosscheck guards the attested binary.
- **No mechanized code↔Lean correspondence** — the auction is matched to the verified model by inspection, not extraction (see Formal verification).
- **Crosscheck is a fixed-vector spot-check** — make it property-based/fuzzed, reconcile the floor-rounding divergence, and CI-guard the standalone enclave repo (not just the vendored copy).

## Formal verification & contact

An abstract single-shot model of the auction is machine-checked in Lean 4 (zero `sorry`): with Gaussian relevance, `log₅(bid) − dist²/σ²` scoring is a monotone transform of value, so the power-diagram allocation is welfare-optimal and the Clarke-pivot VCG payment is DSIC (`vcg_dsic`). What that does and does not cover, for a reader who checks the code against the proof:

- The proof establishes the **lump-sum Clarke pivot**; the shipped code (`auction/vcg.go`) charges a **per-click critical value**, `runner_up_price · B^(termW − termR)`. The two coincide only at a keyword / common-center query point (`vcgPayment_common_center_second_price`); off that point they differ by a factor of `B^termW`.
- The Lean model does **not** include the shipped reserve/floor clamp, the `payment ≤ bid` IR clamp, budget filtering, the σ = 0 branch, or the randomized tie-break (the code breaks ties with `crypto/rand`; the model breaks them deterministically).
- There is **no mechanized correspondence** between the Go implementation and the Lean model. The link is by inspection, not extraction; the `auction_crosscheck` tests check Go-against-Go, not Go-against-Lean.
- The proof states scoring in natural log; the code uses log₅. The base is **not** free: it rescales only the bid term, so `log₅(bid) − dist²/σ²` and `ln(bid) − dist²/σ²` are the same mechanism only after rescaling σ by √ln 5 (equivalently, the proof-model σ is `σ_code / √ln 5`). With σ held fixed, a different base can change the winner. The paper and code correspond under that reparameterization, not term-by-term.

So: a clean single-shot VCG model is verified welfare-optimal and DSIC, and the deployed per-click, reserve-clamped, budget-filtered auction is that model's engineering extension. Even at the keyword / common-center point, it is only the *unclamped payment formulas* that coincide; the deployed mechanism as a whole — floor, the `payment ≤ bid` clamp, budget filtering, σ=0 handling, randomized ties — is not mechanically matched to the Lean model anywhere. Paper: **[Formally Verified VCG Mechanisms for Advertising in Embedding Spaces](https://june.kim/formally-verified-vcg-mechanisms)**. Proofs: **[kimjune01/auction-proof](https://github.com/kimjune01/auction-proof)**.

Maintained by June Kim, <june@june.kim>.
