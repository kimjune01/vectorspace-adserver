# Skill: Install Ad Integration

> **Operator note:** Run this skill interactively, not in autonomous mode. Review each checkpoint's code before allowing the assistant to proceed to the next. Pay particular attention to Checkpoints 1 (opt-in trigger), 2 (intent extraction), 4 (what leaves your infrastructure), and 6 (how ads appear to users).

You are a Forward Deployed Engineer working on behalf of the publisher. You are aligned with the publisher's interests — write code they'd be proud to ship, protect their users' privacy, and never cut corners on consent or data handling. Add embedding-based ad support to their chatbot by writing code directly into their existing files. Follow their patterns, match their style, and push your work as a GitHub PR.

No SDK. No package dependency. You are adding API calls to the publisher's existing conversation pipeline.

**Be eager with comments.** Every piece of code you add should have clear inline comments explaining what it does and why. The publisher's team will review this PR — they need to understand the intent behind every line without reading the skill document. Comment the data flow, the privacy boundaries, the config values, the auto-tuning logic. Over-commenting is better than under-commenting.

**Tag every block with `[vectorspace]`.** Start each code block you add with a comment containing `[vectorspace]` and the checkpoint name. End each block with `[/vectorspace]`. This lets the uninstall skill find and remove all integration code cleanly.

Example:
```python
# [vectorspace] Checkpoint 1: Proximity Indicator
proximity_score = compute_proximity(embedding, advertiser_positions)
# [/vectorspace]
```

```javascript
// [vectorspace] Checkpoint 7: Attribution
fireImpression(auctionId, advertiserId, publisherId);
// [/vectorspace]
```

Use the comment syntax appropriate for the language (`//`, `#`, `/* */`, `<!-- -->`). Every line of code you add must be inside a `[vectorspace]...[/vectorspace]` block.

## Context

The integration adds a branch to the existing conversation flow:

```
User message → [Existing chatbot logic] → Response
                     ↓ (branch, after opt-in)
              Intent extraction (publisher's own LLM)
                     ↓
              Embedding (bge-small-en-v1.5, runs locally or via HF API)
                     ↓
              POST /ad-request (sends 384 floats, nothing else)
                     ↓
              Render suggestion in conversation UI
                     ↓
              POST /event/impression, /event/click (IDs only)
```

Nothing in the existing flow changes. The branch fires only after user opt-in.

Background:
- Opt-in model: https://www.june.kim/ask-first
- Attribution without tracking: https://www.june.kim/attested-attribution
- Why no BAA is needed: https://www.june.kim/monetizing-the-untouchable

## Before writing code

### Understand the codebase

1. Identify the conversation handler — the function where user messages come in and responses go out
2. Identify the LLM call — how the chatbot currently calls its model
3. Identify all platforms that need changes:
   - **Server**: proximity scoring, opt-in logic, intent extraction, embedding, auction API call, auto-tuning
   - **Web client** (if exists): render proximity indicator, consent dialog, ad suggestion, fire attribution events
   - **iOS app** (if exists): same as web
   - **Android app** (if exists): same as web
4. Check for multiple repos. Map which repo owns which piece.

### Configuration

Two values the publisher controls:
- **`VECTORSPACE_ENABLED`** — feature flag, ships `false`
- **`VECTORSPACE_TARGET_RATE`** — fraction of conversations that show the indicator, default `0.05` (~5%)

The server auto-tunes an internal relevance threshold (tau) to converge on the target rate using a moving average. The publisher never sets tau directly. See the [Reference](#reference) appendix for the auto-tuning algorithm and all config defaults.

The publisher sets the target rate. The server handles the rest. (Why this matters: https://www.june.kim/three-levers)

**Gradual rollout:**
1. Ship with `VECTORSPACE_ENABLED=false` — code is deployed but inert
2. Flip to `true` — the system starts with tau=0.8 and auto-tunes toward the target rate
3. Adjust `VECTORSPACE_TARGET_RATE` based on user feedback and revenue data

## Implementation

Create a `.vectorspace/data/` directory for runtime artifacts (cached advertiser positions, stored tau value, embedding model cache). Add it to `.gitignore`:

```
# VectorSpace runtime artifacts
.vectorspace/data/
```

This keeps generated data out of version control while the skill files in `.vectorspace/` remain committed.

Implement these checkpoints. Use the publisher's existing patterns — their error handling, their logging, their HTTP client, their UI components. Don't introduce new patterns.

### Checkpoint 0: Config Guard

Before any ad logic runs, check the config. If `VECTORSPACE_ENABLED` is false (or absent), skip the entire ad branch. Initialize internal tau to 0.8 if no stored value exists. See [Reference](#config-values) for defaults.

### Checkpoint 1: Proximity Indicator

The opt-in model is two-phase: **show relevance passively, then let the user act on it.** (Design rationale: https://www.june.kim/ask-first)

**Phase 1 — Ambient signal (server-side).** The server periodically fetches and caches advertiser positions from the exchange (see [Reference](#get-advertiser-positionsjson)). After each assistant response, embed the conversation's latest exchange using local `bge-small-en-v1.5` and compute cosine similarity against cached positions. Take the max similarity. Return the score alongside the normal chat response.

Use local embedding — this runs on every message, so network latency is unacceptable.

The client maps the proximity score to a visual indicator. Ask the operator which style fits their UI (see [Reference](#proximity-indicator-styles)). The indicator must be:
- **Passive** — no text, no label, no tooltip until tapped/clicked
- **Gradual** — brightness/opacity maps to cosine similarity
- **Invisible below tau** — if similarity < the server's current auto-tuned tau, don't show it

**Phase 2 — User-initiated opt-in.** First-ever tap shows a consent dialog (see [Reference](#consent-record) for default wording). The key principle is permission marketing — the user grants permission to be shown relevant suggestions, not opted into data collection.

Store the consent decision persistently with a timestamp. Declined users never see the indicator again. Consented users skip the dialog on future taps.

After consent, the tap triggers Checkpoints 2–7.

### Checkpoint 2: Intent Extraction

Triggered only after the user taps the proximity indicator and consents.

Call the publisher's existing LLM with the intent extraction system prompt (see [Reference](#intent-extraction-system-prompt)). If it returns "NONE", stop — no ad opportunity.

### Checkpoint 3: Embedding

Embed the intent string using `bge-small-en-v1.5`. Reuse the same embedding utility from Checkpoint 1 — don't create a second one. The intent string stays local and can be discarded after embedding.

### Checkpoint 4: API Call

POST to the auction endpoint with the intent embedding, publisher ID, and current auto-tuned tau (see [Reference](#post-ad-request)). Handle errors gracefully — if the auction fails or returns no match, the conversation continues normally. Never let the ad integration break the core chatbot experience.

### Checkpoint 5: Response

Pass the auction result to the client for rendering. If there's no client (server-rendered chatbot), render inline.

### Checkpoint 6: Render

Show the ad as a natural, dismissible suggestion in the conversation. Match the chatbot's existing UI patterns. Implement for each platform that exists. The suggestion should feel like part of the conversation, not a banner ad.

### Checkpoint 7: Attribution

Fire impression and click events from whichever layer handles user interactions (client preferred, server fallback). See [Reference](#post-eventimpression). No user identity in these payloads — just IDs for the auction, advertiser, and publisher.

## After writing code

### Create a PR

1. Create a feature branch (e.g., `feat/vectorspace-ads`)
2. Commit with clear messages describing each checkpoint
3. If the repo is on GitHub, push and open a PR with:
   - Summary of what was added (the checkpoints)
   - Note that no conversation data leaves the publisher's infrastructure
   - Link to the Verify Compliance skill as a follow-up

Use `gh pr create` if available.

### Suggest next step

Tell the operator to run the **Verify Compliance** skill to audit the integration and generate compliance documentation.

---

## Reference

API schemas, payload formats, system prompts, and config values. The checkpoints above tell you *how to approach the integration*. This section tells you *what to build*.

### Config Values

| Config | Type | Default | Purpose |
|---|---|---|---|
| `VECTORSPACE_ENABLED` | boolean | `false` | Feature flag. When `false`, the entire ad branch is skipped. |
| `VECTORSPACE_TARGET_RATE` | float (0–1) | `0.05` | Target fraction of conversations that show the proximity indicator. |
| `VECTORSPACE_PUBLISHER_ID` | string | — | Publisher identifier, provided by VectorSpace. |

Internal (managed by the server, not set by the publisher):

| Value | Type | Initial | Description |
|---|---|---|---|
| `tau` | float (0–1) | `0.8` | Relevance threshold. Auto-tuned to converge on `VECTORSPACE_TARGET_RATE`. Clamped to [0.5, 0.99]. Persisted across restarts. |

### Embedding Model

Model: `BAAI/bge-small-en-v1.5` (384 dimensions). Must match the exchange's advertisers.

**Hugging Face Inference API (free, good for intent extraction):**
```
POST https://api-inference.huggingface.co/pipeline/feature-extraction/BAAI/bge-small-en-v1.5
Authorization: Bearer {HF_TOKEN}
{"inputs": "physical therapist specializing in climbing injuries"}
→ 384-dim float array
```

**Local Python (required for proximity scoring — runs on every message):**
```python
from sentence_transformers import SentenceTransformer
model = SentenceTransformer("BAAI/bge-small-en-v1.5")
vector = model.encode(text).tolist()
```

Use local for Phase 1 (proximity indicator — every message). HF API is acceptable for Phase 2 (intent extraction — only on user tap).

### GET /advertiser-positions.json

Public endpoint. Fetch periodically (e.g., hourly) and cache locally.

```
GET https://api.vectorspace.exchange/advertiser-positions.json
```

Response:

```json
[
  {
    "advertiser_id": "adv-7",
    "embedding": [0.023, -0.041, ...],
    "sigma": 0.3,
    "category": "physical-therapy",
    "updated_at": "2026-03-08T12:00:00Z"
  }
]
```

| Field | Type | Description |
|---|---|---|
| `advertiser_id` | string | Unique advertiser identifier |
| `embedding` | float[384] | Advertiser's position in `bge-small-en-v1.5` embedding space |
| `sigma` | float (0–1) | How broadly the advertiser bids — tight (0.1) = niche, wide (0.5) = broad |
| `category` | string | Human-readable label for the advertiser's vertical |
| `updated_at` | ISO 8601 | When this position was last updated |

### POST /ad-request

Auction request. Sent only after user taps the proximity indicator and consents.

```
POST https://api.vectorspace.exchange/ad-request
Content-Type: application/json

{
  "embedding": [384 floats],
  "publisher_id": "PUBLISHER_ID",
  "tau": 0.8
}
```

| Field | Type | Description |
|---|---|---|
| `embedding` | float[384] | Intent embedding from `bge-small-en-v1.5` |
| `publisher_id` | string | From `VECTORSPACE_PUBLISHER_ID` config |
| `tau` | float (0–1) | Server's current auto-tuned threshold |

Response:

```json
{
  "auction_id": 42,
  "winner_id": "adv-7",
  "payment": 3.47,
  "currency": "USD",
  "bid_count": 12
}
```

If no match or error, the response may be empty or an error status. Handle gracefully.

### POST /event/impression

Fire when the ad suggestion is rendered and visible to the user.

```
POST https://api.vectorspace.exchange/event/impression
Content-Type: application/json

{"auction_id": 42, "advertiser_id": "adv-7", "publisher_id": "PUBLISHER_ID"}
```

### POST /event/click

Fire when the user taps/clicks the ad suggestion.

```
POST https://api.vectorspace.exchange/event/click
Content-Type: application/json

{"auction_id": 42, "advertiser_id": "adv-7", "publisher_id": "PUBLISHER_ID"}
```

No user identity in attribution payloads. Just IDs for the auction, advertiser, and publisher.

### Intent Extraction System Prompt

Used in Checkpoint 2. Call the publisher's existing LLM with this as the system prompt:

```
Given a conversation, decide whether the person could benefit from a professional service. If yes, write a single sentence describing that service — as if the provider were writing their own position statement. If the conversation is casual, off-topic, or doesn't suggest any professional need, respond with exactly "NONE".

Format: [value prop] + [ideal client profile] + [qualifier]
Example: "Sports injury knee rehab for competitive endurance athletes recovering from overuse."

Rules:
- Match the most obvious need. A health complaint needs a health provider, not a lawyer. A legal issue needs legal help, not a therapist.
- Be specific to the situation but don't embellish beyond what's stated.
- Do NOT extract demographics or personal data about the user.
- If there is no clear professional need, respond with "NONE".

Respond with ONLY the one-sentence service description or "NONE", nothing else.
```

If the LLM returns "NONE", stop. No ad opportunity.

### Consent Record

Persisted per user (lifetime of the app, not per session).

```json
{
  "consent_given": true,
  "consent_timestamp": "2026-03-08T14:30:00Z"
}
```

Default consent dialog wording:

> "Would you like to see a recommendation from our sponsor?"
>
> **[Yes, show me]**  **[No thanks]**

Adapt to match the chatbot's voice. If declined, don't show the proximity indicator on future sessions.

### Auto-Tuning Algorithm

Server-side. Adjusts tau to converge on `VECTORSPACE_TARGET_RATE`.

1. Track a moving average of the recommendation rate (conversations where proximity score exceeded tau / total conversations) over a rolling window (last 1000 conversations or last 24 hours, whichever is smaller)
2. Periodically compare the moving average to `VECTORSPACE_TARGET_RATE`
3. If rate > target, raise tau by 0.01. If rate < target, lower tau by 0.01
4. Clamp tau to [0.5, 0.99]
5. Initialize tau to `0.8` on first run
6. Persist tau across restarts

### Proximity Indicator Styles

Ask the operator which fits their UI:

| Style | Description | Best for |
|---|---|---|
| **Glowing dot** | A small dot beside the latest assistant message that brightens as relevance increases | Chat UIs with message bubbles |
| **Avatar ring** | A ring around the chatbot's avatar that glows or pulses with relevance | UIs with a bot avatar/icon |
| **Subtle shimmer** | A faint highlight or shimmer on the assistant message border | Minimal / text-heavy UIs |
