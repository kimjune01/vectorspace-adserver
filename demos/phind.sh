#!/usr/bin/env bash
# Phind — Developer vertical (tau=0.6–0.9)
# Narrative: CI, observability, and cloud deploy tools surface — feels like recommendations
set -euo pipefail
SERVER="${CLOUDX_SERVER:-http://localhost:8080}"

echo "=== Phind (Developer) ==="
echo ""

queries=(
  "how do I set up a CI pipeline for my monorepo"
  "how do I monitor my API for errors and latency"
  "set up kubernetes deployment pipeline"
)
taus=(0.6 0.9 0.9)

for i in "${!queries[@]}"; do
  q="${queries[$i]}"
  tau="${taus[$i]}"
  echo "--- Intent: \"$q\" ---"
  echo ""

  echo "[WITHOUT tau] All 32 bidders compete — generalists can win:"
  curl -s "$SERVER/ad-request" \
    -H 'Content-Type: application/json' \
    -d "{\"intent\": \"$q\"}" | jq '{winner: .winner.name, bid_count: .bid_count, top_5: [.all_bidders[:5][] | {name, distance_sq: (.distance_sq * 1000 | round / 1000), bid: .bid_price}]}'
  echo ""

  echo "[WITH tau=$tau] Only dev-relevant ads pass the gate:"
  curl -s "$SERVER/ad-request" \
    -H 'Content-Type: application/json' \
    -d "{\"intent\": \"$q\", \"tau\": $tau}" | jq '{winner: .winner.name, bid_count: .bid_count, top_5: [.all_bidders[:5][] | {name, distance_sq: (.distance_sq * 1000 | round / 1000), bid: .bid_price}]}'
  echo ""
  echo ""
done
