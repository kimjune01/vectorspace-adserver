# Plan: Publisher-side trust filtering

When a publisher sends POST /ad-request, filter the bidder pool against the trust allowlist BEFORE passing bidders to the auction. The auction engine (auction/) must NOT be modified.

## 1. DB layer (platform/db.go)

New table in createTables:

```sql
CREATE TABLE IF NOT EXISTS publisher_trust_policies (
    publisher_id TEXT PRIMARY KEY,
    min_edges INTEGER NOT NULL DEFAULT 3,
    min_bilateral INTEGER NOT NULL DEFAULT 1,
    updated_at DATETIME NOT NULL
);
```

Add methods:

```go
func (db *DB) SetPublisherTrustPolicy(publisherID string, minEdges, minBilateral int) error
func (db *DB) GetPublisherTrustPolicy(publisherID string) (minEdges, minBilateral int, found bool, err error)
```

SetPublisherTrustPolicy uses INSERT OR REPLACE. GetPublisherTrustPolicy returns found=false when no row exists.

## 2. Enclave interface (enclave/types.go, enclave/process.go)

In enclave/types.go, add to AuctionRequest:

```go
AllowedAdvertiserIDs []string `json:"allowed_advertiser_ids,omitempty"`
```

In enclave/process.go ProcessPrivateAuction, after getting positions from the store, if len(req.AllowedAdvertiserIDs) > 0, filter the positions map to only include IDs in that list. Do this BEFORE the auction runs. Do NOT modify anything in auction/.

## 3. Request path (handler/tee_handler.go)

Add TrustLedger *trust.Ledger field to TEEHandler.

In the ad-request handler (HandleAdRequest or equivalent), after parsing the request and resolving publisher_id:

1. Call db.GetPublisherTrustPolicy(publisherID)
2. If !found, proceed as normal (all bidders enter — backwards compatible)
3. If found, call ledger.GetTrustedDomains(minEdges, minBilateral) to get trusted domain set
4. Get all advertisers from the registry, extract domain from each advertiser's URL (net/url.Parse, get host)
5. Build AllowedAdvertiserIDs: advertiser IDs whose URL domain is in the trusted set
6. Set req.AllowedAdvertiserIDs on the AuctionRequest before passing to the TEE proxy

## 4. Wire it up (handler/routes.go)

- Pass trust.Ledger to TEEHandler when constructing it
- Add route: PUT /publisher/trust-policy — authenticated by publisher token, body: {"min_edges": N, "min_bilateral": N}
- Handler calls db.SetPublisherTrustPolicy

## 5. Tests

### platform/db_test.go
- TestPublisherTrustPolicyRoundTrip: set policy, get it back, verify values
- TestPublisherTrustPolicyNotFound: get policy for unknown publisher, verify found=false

### handler/ tests
- Publisher with no trust policy: ad-request returns results from full bidder pool (backwards compatible)
- Publisher with trust policy: only advertisers whose URL domains meet the trust threshold enter the auction
- When policy excludes the would-be winner, a different advertiser wins

## Constraints

- auction/ directory must NOT be modified
- Backwards compatible: no policy = all bidders enter
- Run `go test ./...` and verify all tests pass
