#!/usr/bin/env bash
# August AI — Health vertical (tau=0.85)
# Narrative: Nutrition and PT specialists surface for wellness queries
set -euo pipefail
SERVER="${CLOUDX_SERVER:-http://localhost:8080}"

echo "=== August AI (Health) ==="
echo ""

queries=(
  "what foods help lower cholesterol"
  "how do I know if I pulled a muscle or tore something"
)

for q in "${queries[@]}"; do
  echo "--- Intent: \"$q\" ---"
  echo ""

  echo "[WITHOUT tau] All 32 bidders compete — generalists can win:"
  curl -s "$SERVER/ad-request" \
    -H 'Content-Type: application/json' \
    -d "{\"intent\": \"$q\"}" | jq '{winner: .winner.name, bid_count: .bid_count, top_5: [.all_bidders[:5][] | {name, distance_sq: (.distance_sq * 1000 | round / 1000), bid: .bid_price}]}'
  echo ""

  echo "[WITH tau=0.85] Only health-relevant ads pass the gate:"
  curl -s "$SERVER/ad-request" \
    -H 'Content-Type: application/json' \
    -d "{\"intent\": \"$q\", \"tau\": 0.85}" | jq '{winner: .winner.name, bid_count: .bid_count, top_5: [.all_bidders[:5][] | {name, distance_sq: (.distance_sq * 1000 | round / 1000), bid: .bid_price}]}'
  echo ""
  echo ""
done
