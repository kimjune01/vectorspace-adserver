# Enclave: Open-Source TEE Auction

This package runs inside an [AWS Nitro Enclave](https://aws.amazon.com/ec2/nitro/nitro-enclaves/) and is designed to be **open-sourced independently** so that anyone — publishers, regulators, security researchers — can audit exactly what code touches user data.

## Why This Exists

Ad auctions need user intent signals (embeddings) to rank ads by relevance. But embeddings derived from chat messages are sensitive — especially in health-adjacent verticals where intent text like "I need therapy" constitutes individually identifiable health information under HIPAA.

The enclave solves this: the embedding is encrypted on the user's device with the enclave's attested public key, decrypted only inside the TEE, used for one auction, then destroyed. The exchange operator never sees it.

**This only works if people can verify the claim.** If the enclave code imports private modules, verification requires trusting the operator — defeating the purpose. That's why this package is self-contained.

## Architecture

```
Publisher SDK                    Enclave (this package)           Exchange
     |                                |                              |
     | 1. Compute embedding locally   |                              |
     | 2. Encrypt with TEE public key |                              |
     |                                |                              |
     | ---- ciphertext + tau -------> |                              |
     |                                | 3. Decrypt embedding         |
     |                                | 4. Load positions & budgets  |
     |                                |    (pushed from exchange)     |
     |                                | 5. Filter by budget & tau    |
     |                                | 6. Score: log_5(price) -     |
     |                                |    dist²/sigma²              |
     |                                | 7. Rank + VCG payment        |
     |                                | 8. Zero embedding            |
     |                                |                              |
     | <-- {winner_id, payment} ----- |                              |
     |                                |                              |
     |                                |   (parent logs to DB) -----> |
```

### Trust Boundary

The enclave is a **trust boundary between the user and the exchange operator**.

| Component | Who controls it | What it sees |
|-----------|----------------|--------------|
| Publisher SDK | Publisher | Raw chat, embedding (plaintext) |
| Enclave | No one (attested code) | Embedding (decrypted inside TEE, then destroyed) |
| Exchange parent | Exchange operator | `{winner_id, payment, publisher_id}` only |
| Exchange DB | Exchange operator | Never sees embedding or intent |

The exchange parent pushes positions and budgets into the enclave via `sync_positions` / `sync_budgets` messages. The enclave uses these for live budget enforcement and pacing. The parent never sends — and the enclave never returns — the embedding.

### What Enters and Exits the Enclave

**In:**
- Encrypted embedding (RSA-OAEP + AES-256-GCM ciphertext)
- Tau threshold (optional relevance filter)
- Advertiser positions (bid price, embedding, sigma) — pushed by parent
- Budget snapshots (total, spent) — pushed by parent

**Out:**
- Winner ID
- VCG payment amount
- Currency
- Bid count

**Never exits:**
- The query embedding
- Individual bid scores or distances
- Which advertisers were filtered by tau or budget

## Critical Design Decisions

### 1. Vendored auction logic (no private imports)

The `auction/` subdirectory is a **verbatim copy** of the auction math from the private adserver repo. This is intentional duplication, not an abstraction failure.

**Why copy instead of import?** If this package imported `vectorspace-adserver/auction`, anyone auditing the enclave would need access to the private repo to verify what code runs inside the TEE. The whole point is that they don't need to trust us.

**Why not a shared Go module?** A separate `github.com/org/auction` module would work, but adds release coordination overhead for ~280 lines of pure math. Copy is simpler and the drift risk is managed by a cross-validation test in the parent repo.

**Drift risk:** The parent repo has a test that runs identical inputs through both `auction.RunAuction` (original) and `enclave/auction.RunAuction` (vendored) and asserts bit-identical results. Any divergence fails CI.

### 2. Stdlib-only dependencies

The `enclave/auction/` subpackage imports only Go standard library (`math`, `crypto/rand`, `sort`, `fmt`, `math/big`). The enclave package itself adds only `crypto/*`, `encoding/*`, `net`, and `sync` — all stdlib.

This matters because every dependency is attack surface. A TEE that imports an HTTP framework or ORM has a vastly larger surface to audit than one that uses only the Go standard library.

### 3. Embedding destruction

After the auction completes, the query embedding is zeroed in memory (`defer` in `ProcessPrivateAuction`). This is a defense-in-depth measure — the enclave's memory is already isolated by the Nitro hypervisor — but it ensures the embedding doesn't survive in a heap dump if something goes wrong.

### 4. VCG pricing (not first-price)

The enclave uses VCG (Vickrey-Clarke-Groves) payment, not first-price. The winner pays only enough to beat the runner-up in embedding-adjusted score space:

```
payment = runner_up_price * 5^(dist_winner²/sigma_winner² - dist_runner²/sigma_runner²)
```

Capped at the winner's bid (individual rationality). This is critical for incentive compatibility: advertisers should bid their true value, not shade bids. Since the auction runs in an attested enclave, advertisers can verify that the payment rule is actually VCG, not a first-price auction disguised as VCG.

### 5. Vsock with TCP fallback

The enclave listens on vsock (AF_VSOCK) in production, which is the only network interface available inside a Nitro Enclave — there is no IP networking. For local development and testing, it falls back to TCP on localhost. The vsock integration (via `github.com/mdlayher/vsock`) is gated behind a build tag for Phase 2b.

### 6. JSON message protocol

The parent communicates with the enclave via a simple JSON envelope (`{type, payload}`). This is intentionally not gRPC or any framework — it keeps the dependency surface minimal and the protocol trivially auditable. Message types:

- `ping` / `pong` — health check
- `key_request` / `key_response` — attestation + public key
- `sync_positions` — push advertiser positions
- `sync_budgets` — push budget snapshots
- `auction_request` / `auction_response` — run an auction

## Package Structure

```
enclave/
  auction/         # Vendored auction math (stdlib-only, independently auditable)
    types.go       #   CoreBid, ScoredBid, AuctionResult
    embedding.go   #   ComputeEmbeddingScore, SquaredEuclideanDistance
    vcg.go         #   ComputeVCGPayment
    auction.go     #   RunAuction (orchestrator)
    floor.go       #   EnforceBidFloor, BidMeetsFloor
    ranking.go     #   RankScoredBids, RankCoreBids (crypto-random tie-breaking)
  types.go         # Message types (AuctionRequest, AuctionResponse, snapshots)
  crypto.go        # RSA-OAEP + AES-256-GCM hybrid encryption/decryption
  keymanager.go    # RSA-2048 keypair generation and PEM export
  state.go         # Thread-safe stores for positions and budgets
  process.go       # ProcessPrivateAuction (decrypt -> filter -> auction -> payment)
  listen.go        # Vsock listener with TCP fallback
```

## Verification

To verify this package is self-contained:

```bash
# All enclave/auction/ imports are stdlib
go list -f '{{join .Imports "\n"}}' ./enclave/auction/

# No imports from the private auction package
grep -r '"vectorspace-adserver/auction"' enclave/
# (should return nothing)

# Build and test in isolation
go test -count=1 ./enclave/...
```
