# Publisher-Side Embedding Cache

Phase 1 (the proximity indicator) should work without contacting the exchange.
Advertiser embeddings are public — they're positioning claims, not secrets. The
exchange publishes the full set, publishers cache a copy, and proximity is
computed locally. The exchange doesn't know a user exists until they tap.

## Server

New endpoint: `GET /embeddings`

Returns `{id, embedding}` pairs. Nothing else — no names, bids, or sigma.
Those are phase 2 concerns.

```json
{
  "version": "a3f8…",
  "embeddings": [
    {"id": "adv-1", "embedding": [0.012, -0.034, ...]},
    {"id": "adv-2", "embedding": [0.008, 0.041, ...]}
  ]
}
```

`version` is a hash of the data, also sent as `ETag`. SDK sends `If-None-Match`
on subsequent requests; server returns 304 if nothing changed.

Files: `handler/publisher.go`, `handler/routes.go`, `platform/registry.go`

## SDK

New methods on `CloudXClient`:

```python
client.sync_embeddings()          # pull catalog, respect ETag
client.proximity(query_embedding) # local cosine distance, sorted
# → [{"id": "adv-3", "distance": 0.042}, {"id": "adv-1", "distance": 0.187}, …]
```

- `sync_embeddings()` — GET /embeddings, store in memory, save ETag
- `proximity(embedding)` — squared euclidean distance against cached embeddings,
  sorted ascending. Pure math, no network call.
- Auto-syncs on first `proximity()` call if cache is empty
- No background thread. Publisher controls the sync schedule.

Files: `sdk/cloudx/client.py`, `sdk/tests/test_client.py`

## Verification

- Mock `/embeddings`, assert `proximity()` returns correct sorted distances
- Assert second `sync_embeddings()` sends `If-None-Match` header
- Integration: seed advertisers, sync, verify proximity ordering matches
  auction distance ranking
