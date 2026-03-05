#!/usr/bin/env bash
# FlyFin — Freelancer Finance vertical (tau=0.7)
# Narrative: Freelancer bookkeeping and tax services surface, not generalist financial advisors
set -euo pipefail
SERVER="${CLOUDX_SERVER:-http://localhost:8080}"

echo "=== FlyFin (Freelancer Finance) ==="
echo ""

queries=(
  "what expenses can I deduct as a freelance designer"
  "do I need to pay quarterly estimated taxes on 1099 income"
  "should I set up an LLC or stay sole proprietor"
)

for q in "${queries[@]}"; do
  echo "--- Intent: \"$q\" ---"
  echo ""

  echo "[WITHOUT tau] All 32 bidders compete — generalists can win:"
  curl -s "$SERVER/ad-request" \
    -H 'Content-Type: application/json' \
    -d "{\"intent\": \"$q\"}" | jq '{winner: .winner.name, bid_count: .bid_count, top_5: [.all_bidders[:5][] | {name, distance_sq: (.distance_sq * 1000 | round / 1000), bid: .bid_price}]}'
  echo ""

  echo "[WITH tau=0.7] Only freelancer finance ads pass the gate:"
  curl -s "$SERVER/ad-request" \
    -H 'Content-Type: application/json' \
    -d "{\"intent\": \"$q\", \"tau\": 0.7}" | jq '{winner: .winner.name, bid_count: .bid_count, top_5: [.all_bidders[:5][] | {name, distance_sq: (.distance_sq * 1000 | round / 1000), bid: .bid_price}]}'
  echo ""
  echo ""
done
