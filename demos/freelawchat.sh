#!/usr/bin/env bash
# FreeLawChat — Legal vertical (tau=0.7)
# Narrative: Tenant rights, family law, injury attorneys surface — not financial advisors
set -euo pipefail
SERVER="${CLOUDX_SERVER:-http://localhost:8080}"

echo "=== FreeLawChat (Legal) ==="
echo ""

queries=(
  "my landlord is trying to evict me without notice"
  "going through a divorce and need custody advice"
  "I was rear-ended and need to file an injury claim"
)
taus=(0.7 0.7 0.95)

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

  echo "[WITH tau=$tau] Only legal-relevant ads pass the gate:"
  curl -s "$SERVER/ad-request" \
    -H 'Content-Type: application/json' \
    -d "{\"intent\": \"$q\", \"tau\": $tau}" | jq '{winner: .winner.name, bid_count: .bid_count, top_5: [.all_bidders[:5][] | {name, distance_sq: (.distance_sq * 1000 | round / 1000), bid: .bid_price}]}'
  echo ""
  echo ""
done
