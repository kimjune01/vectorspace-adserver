# Roadmap

## Publisher-side trust policy enforcement

The trust exchange builds the graph. The auction engine stays pure. The missing piece is the publisher-side policy layer that sits between them — composing allowlists from curators and filtering advertisers before they enter the auction.

### Prior art: email policy daemons

Email infrastructure already solved federated policy composition:

- **rspamd / SpamAssassin** — composable rule engines checking multiple blocklists before accepting mail
- **postscreen (Postfix)** — pre-filter checking DNSBL blocklists before mail enters the queue
- **PolicyD / Cluebringer** — policy daemons sitting in front of MTAs, enforcing rules per sender/domain

All follow the same pattern: the MTA (SMTP protocol) stays pure, the policy daemon sits at the edge and decides who gets in. Publishers compose from multiple lists. The lists compete.

The gap: all existing implementations are **negative** filters (blocklists — who to reject). Nobody's built the **positive** version (allowlists — who to trust based on attested relationships). The infrastructure for composing policies from federated sources is mature. The trust graph that feeds it is what doesn't exist yet.

### What to build

A policy daemon for the ad server. The publisher defines a trust policy:

```yaml
trust_policy:
  require_any:
    - curator: health-trust-network
    - curator: general-commerce-verified
    - curator: local-biz-portland
  deny:
    - denylist: known-scam-advertisers
    - denylist: low-quality-supplements
```

Union the allowlists. Subtract the denylists. What remains is the set of advertisers eligible to enter the auction. The auction never knows the difference.

Same architecture as email: the protocol stays pure, the policy is composed at the edges.

---

## Natural Framework audit

Map the ad server to the six roles: Perceive → Cache → Filter → Attend → Remember → Consolidate.

| Role | What exists | Status |
|------|-------------|--------|
| **Perceive** | Ad request arrives with intent. System reads advertiser embeddings, trust graph, budget state. | ✓ |
| **Cache** | PositionRegistry (in-memory + DB sync, ETag versioning). BudgetTracker. Embedding sidecar. | ✓ |
| **Filter** | Bid floors, budget exhaustion, frequency caps. Trust policy filtering (roadmap above). | Partial |
| **Attend** | Auction ranking: `Score = log₅(price) - dist²/σ²`. VCG pricing. | ✓ |
| **Remember** | SQLite: auctions, events, attestations, trust edges, ledger log. All writes are lossless CRUD. | ✓ |
| **Consolidate** | — | Missing |

### What's missing

**Filter is partial.** The static gates exist (bid floor, budget, frequency cap), but the trust policy layer — composing allowlists from federated curators — is the gap described above. Without it, the system can't distinguish Bark & Bond from Sunny Paws. Filter without trust policy is a bouncer who checks IDs but not the guest list.

**Consolidate doesn't exist.** Nothing reads from Remember to update how the system processes next time. The system records impressions, clicks, and viewability events, but no backward pass learns from them. Specific gaps:

- **Sigma doesn't adapt.** Advertisers set σ (relevance spread) manually at registration. A pet food brand that consistently wins auctions for "best treats for senior dogs" but loses for "puppy toys" has the signal to narrow its σ. No process reads click/impression history to suggest or auto-adjust it.
- **Embeddings don't drift.** Advertiser embeddings are set once from intent text. A brand that evolves its positioning over months still occupies its original point in embedding space. Consolidate would re-embed based on what the advertiser actually wins, not what it claimed at registration.
- **Trust edge weights are static.** A 3-year payment relationship weighs 3.0 forever. Consolidate would decay stale edges, boost edges correlated with low fraud, and surface trust patterns across the graph (e.g., clusters of mutual attestation that predict quality).
- **Curators don't learn.** Allowlist thresholds (`min_edges=3, min_bilateral=1`) are set by hand. A curator that tracks which allowlisted advertisers later get flagged for fraud could tighten or loosen thresholds automatically. The curator's cron job.
- **Publishers don't learn.** A publisher's trust policy is static YAML. A publisher whose health-trust-network curator lets through a bad advertiser has no feedback mechanism to downweight that curator. Consolidate would adjust curator weights based on outcomes.

### Parts bin prescriptions

Each gap mapped to the [embedding pipe](https://june.kim/embedding-pipe) and [parts bin](https://june.kim/the-parts-bin). The broken contract names the problem. The algorithm names the fix.

- **Sigma adaptation.** Sigma is frozen at registration. Problem: a pet food brand that wins "senior dog treats" but loses "puppy toys" has the signal to narrow its spread, but nothing reads the signal. Prescription: Consolidate × embedding_space from the [parts bin](https://june.kim/the-parts-bin). Read win/loss history from Remember, fit an online Bayesian posterior update over sigma per advertiser. Algorithm: Bayesian posterior update (parts bin Consolidate catalog). The loss function is auction outcome regret.

- **Embedding drift.** Advertiser embeddings are set once from registration intent. Problem: a brand that evolves its positioning still occupies its original point. Prescription: Consolidate re-embeds from actual auction wins, not claimed intent. Algorithm: online k-means or Growing Neural Gas from the [Consolidate catalog](https://june.kim/the-parts-bin#catalog). Cluster winning queries per advertiser, update the embedding centroid toward the cluster mean.

- **Trust edge decay.** A 3-year payment relationship weighs 3.0 forever. Problem: stale trust edges never expire, fraud signals never propagate. Prescription: Consolidate reads from Remember (trust_edges, ledger_log) and writes updated weights. Algorithm: exponential decay with a fraud-signal correction term. This is the same backward pass the [handshake](https://june.kim/the-handshake) describes: persisted → policy′. The trust graph is the policy store. Consolidate reshapes it.

- **Curator threshold tuning.** Allowlist thresholds (`min_edges=3, min_bilateral=1`) are hand-set. Problem: a curator can't tell whether its threshold is too loose (lets bad advertisers through) or too tight (blocks good ones). Prescription: Consolidate tracks which allowlisted advertisers later get flagged, adjusts thresholds. Algorithm: online gradient descent on a threshold-vs-fraud-rate objective from the [Consolidate catalog](https://june.kim/the-parts-bin#catalog).

- **Publisher policy learning.** Publisher trust policies are static YAML. Problem: a publisher whose curator lets through a bad advertiser has no feedback mechanism. Prescription: Consolidate adjusts curator weights per publisher based on impression outcomes. This is the [embedding pipe](https://june.kim/embedding-pipe) Consolidate step: update the retrieval policy from outcomes.

### The arc

Perceive through Remember is the forward pass — the system serves ads. Consolidate is the backward pass — the system learns to serve better ads. Right now, every parameter (σ, embeddings, trust weights, curator thresholds, publisher policies) is set by humans and stays fixed. The system has the job but no crontab.

Filter (trust policy) is the immediate priority — it gates what enters the auction. Consolidate is next — it's what makes the gates adaptive. The [parts bin](https://june.kim/the-parts-bin) names the algorithms. The [embedding pipe](https://june.kim/embedding-pipe) maps the stages.
