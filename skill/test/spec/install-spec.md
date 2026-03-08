# Install Spec

A passing Install trial must:

1. **Create a feature branch** — not commit to main
2. **Add config guard** (Checkpoint 0) — `VECTORSPACE_ENABLED` boolean (default false) and `VECTORSPACE_TARGET_RATE` float (default 0.05), server auto-tunes tau internally via moving average
3. **Add proximity indicator** (Checkpoint 1) — fetch and cache advertiser positions from the exchange, server computes proximity score using local embedding, client renders ambient visual signal (dot, ring, or shimmer) that brightens with cosine similarity, hidden below auto-tuned tau, first-ever tap shows consent dialog, user tap triggers the auction

Note: In automated trials (no human operator), the agent should pick the best-fit indicator style based on the existing UI patterns.
4. **Add intent extraction** (Checkpoint 2) — use the publisher's existing LLM with the standard system prompt, triggered only after user taps indicator
5. **Add embedding call** (Checkpoint 3) — use `bge-small-en-v1.5` via HF API or local, reuse the same utility as the proximity indicator
6. **Add auction API call** (Checkpoint 4) — POST to `/ad-request` with embedding, publisher_id, auto-tuned tau
7. **Add render** (Checkpoint 6) — a dismissible suggestion in the conversation UI
8. **Add attribution events** (Checkpoint 7) — POST impression/click to `/event/*`
9. **Match existing code style** — language, formatting, patterns, error handling
10. **Not break existing functionality** — existing tests (if any) should still pass
11. **Push a PR** (or attempt to — will fail for repos where we don't have push access; that's fine)
