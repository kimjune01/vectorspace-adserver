#!/usr/bin/env bash
# Doctronic — Health vertical (tau=0.85)
# Narrative: Telehealth surfaces for primary care, not tutoring or dog training
set -euo pipefail
SERVER="${CLOUDX_SERVER:-http://localhost:8080}"

echo "=== Doctronic (Health) ==="
echo ""

queries=(
  "I think I have a sinus infection"
  "my child has a fever and sore throat"
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
