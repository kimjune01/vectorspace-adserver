#!/usr/bin/env bash
# AskLegal.bot — Legal vertical (tau=0.75)
# Narrative: Employment and injury attorneys surface for legal queries
set -euo pipefail
SERVER="${CLOUDX_SERVER:-http://localhost:8080}"

echo "=== AskLegal.bot (Legal) ==="
echo ""

queries=(
  "can my employer fire me for taking medical leave"
  "someone hit my parked car and drove away"
)

for q in "${queries[@]}"; do
  echo "--- Intent: \"$q\" ---"
  echo ""

  echo "[WITHOUT tau] All 32 bidders compete — generalists can win:"
  curl -s "$SERVER/ad-request" \
    -H 'Content-Type: application/json' \
    -d "{\"intent\": \"$q\"}" | jq '{winner: .winner.name, bid_count: .bid_count, top_5: [.all_bidders[:5][] | {name, distance_sq: (.distance_sq * 1000 | round / 1000), bid: .bid_price}]}'
  echo ""

  echo "[WITH tau=0.75] Only legal-relevant ads pass the gate:"
  curl -s "$SERVER/ad-request" \
    -H 'Content-Type: application/json' \
    -d "{\"intent\": \"$q\", \"tau\": 0.75}" | jq '{winner: .winner.name, bid_count: .bid_count, top_5: [.all_bidders[:5][] | {name, distance_sq: (.distance_sq * 1000 | round / 1000), bid: .bid_price}]}'
  echo ""
  echo ""
done
