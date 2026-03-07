# Portal Stakeholder Expectations

What each stakeholder should be able to do in the portal, grounded in the
"Marketing-Speak Is the Protocol" model: advertisers submit positioning
statements, the system extracts user intent in the same language, and embedding
distance decides the match.

---

## Advertiser

"I wrote my positioning statement. Now what?"

### Currently works
- View budget utilization (total, spent, remaining)
- Edit profile: name, intent, sigma, bid price, top-up budget
- View event stats: impressions, clicks, viewable, CTR
- View auction history with pagination (auctions I won)

### Missing — basic expectations
1. **See what user intents I matched.** My auction history shows payment and
   timestamps but not the user intent that triggered the auction. I can't tell
   if I'm matching "knee pain for marathon runner" or "dog bit my ankle." The
   `intent` field is on `AuctionRow` but the advertiser portal doesn't show it.

2. **See auctions I lost.** I only see auctions I won. I need to see auctions
   where I was eligible but lost — and ideally why (outbid? lower relevance
   score?). Without this I can't tune sigma or bid price.

3. **Preview reach before committing.** I set sigma to 0.4 but have no idea
   what that covers. A "test my positioning" tool: type an intent, see which
   example queries would match at my sigma and which wouldn't.

4. **Derived metrics.** Cost per click (CPC), cost per mille (CPM), average
   payment, budget burn rate. The raw numbers are there but the portal doesn't
   compute them.

5. **Feedback on intent edits.** If I change my intent, I get no feedback on
   how my reach shifted. Even a "your positioning is most similar to these
   other advertisers" would help me differentiate.

---

## Publisher

"I have a chatbot. I want to monetize without ruining the experience."

### Currently works
- View revenue earned, total auctions, impressions
- Revenue chart by day/week/month
- Top advertisers on my property (bar chart)
- Auction history with pagination
- Login / logout

### Missing — basic expectations
1. **See user intent alongside winning ad.** I see auctions on my property but
   can't judge match quality. I need intent and winner positioning side by side
   so I can audit whether tau is set correctly.

2. **Fill rate.** What percentage of ad requests returned a winner vs. came
   back empty? If fill rate is low, I need to know — maybe tau is too strict or
   there aren't enough advertisers in my niche.

3. **Tau tuning feedback.** I don't know what tau=0.6 means in practice. A
   histogram of match distances for my recent auctions would let me see where
   to draw the line. "At tau=0.4, 30% of your auctions would not have filled."

4. **Revenue by intent category.** Which user conversations monetize well?
   Health questions vs. legal vs. cooking. This informs content strategy.

5. **Integration test mode.** After SDK install, send a sample conversation and
   see what intent gets extracted, which advertisers match, and what ad would
   be shown — before going live.

---

## User (end user of publisher's chatbot)

Users don't log into the portal. Their experience is the ad match quality.

### Basic expectations
1. **Relevant ads.** If I'm describing lateral knee pain with a race in 8
   weeks, I see a sports PT who works with runners — not a generic clinic. This
   is the core promise of positioning-as-protocol.

2. **Not seeing the same ad repeatedly.** Frequency capping exists (3/60min)
   but there's no dismiss/feedback mechanism.

3. **Transparency.** "Shown because you discussed marathon training" — a
   one-line explanation connecting the ad to the conversation.

---

## Admin / Exchange

"I run the marketplace. Both sides need to trust it."

### Currently works
- Stat cards: total auctions, revenue, exchange cut, impressions
- Revenue chart by day/week/month
- Top advertisers by spend (bar chart)
- Auction log with filters (winner, intent) + CSV export
- Advertisers table (name, bid, sigma, budget, spent)
- Admin login / logout

### Missing — basic expectations
1. **Supply/demand health.** How many active advertisers? How many publishers
   sending requests? Match rate? Where are there gaps — intents with no
   advertisers, advertisers with no matches?

2. **Revenue by publisher.** Which publishers drive volume? Current dashboard
   shows aggregate revenue only.

3. **Advertiser churn signals.** Who has burned their budget? Who has budget
   left but isn't winning? These are at risk of leaving.

4. **Publisher management UI.** Publishers are created via API only. No admin
   page to list, view, or manage publishers.

5. **Auction detail for disputes.** If an advertiser says "I got charged for
   irrelevant impressions," I need the intent, embedding distance, score, and
   payment calculation. The auction log has payment and intent but not distance
   or score.

---

## Cross-cutting gap

**The intent match is invisible.** The blog post's argument is that
positioning-speak bridges supply and demand. But nobody in the portal can see
that bridge working:

- Advertisers can't see what user intents they matched
- Publishers can't see whether tau is filtering well
- Admin can't see where supply and demand are misaligned

Surfacing the intent-to-positioning match quality is the single
highest-leverage improvement across all three stakeholders.

---

## Fixed defects

1. ~~Advertiser dashboard shows no derived metrics.~~ Added Avg Payment, CPC,
   CPM, Auctions Won stat cards.

2. ~~Publisher dashboard shows no derived metrics.~~ Added RPM, Avg Payment,
   Clicks, Viewable stat cards.

3. ~~Admin has no publisher management page.~~ Added GET /admin/publishers
   endpoint + Publishers page with nav link from Overview.

4. ~~CSV export for advertiser's own auctions.~~ Added format=csv support to
   /portal/me/auctions + Export CSV button on advertiser dashboard.

5. ~~Auction log shows winner_id instead of name.~~ All three auction views
   (admin, publisher, advertiser) now show winner name via LEFT JOIN. CSV
   exports also include winner_name.

6. ~~Admin dashboard doesn't show marketplace size.~~ Added advertiser_count
   and publisher_count to /stats endpoint + stat cards on Overview.

7. ~~Billing model: charge on impression.~~ Moved to CPC (cost-per-click).
   Charge removed from RunAdRequestFull, added to HandleClick with
   deduplication (only first click per auction charges). Auction still
   calculates VCG payment and logs it; budget check at auction time still
   filters broke advertisers. Portal metrics updated: advertiser sees CPC as
   primary metric, publisher sees Rev/Click.

## Remaining — requires product decisions or new infrastructure

1. **Publisher fill rate.** Needs tracking ad requests that returned no winner.
   Requires deciding: track in DB? in-memory counter? What constitutes a
   "request" (intent extraction vs. ad-request)?

2. **Revenue by publisher for admin.** Data exists in auctions table but no
   aggregation endpoint. Needs deciding: separate page? chart on Overview?

3. **Advertiser churn signals.** Conceptually clear but needs defining: what
   threshold = "at risk"? Budget <10%? No wins in 7 days?

4. **Auction detail for disputes.** Would require storing embedding distance
   and score at auction time (currently computed and discarded).

5. **Advertiser preview/test positioning.** Requires a new endpoint that runs
   a mock auction without charging.

6. **Publisher tau tuning feedback.** Requires storing match distances per
   auction (currently not persisted).
