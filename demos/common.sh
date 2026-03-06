#!/usr/bin/env bash
# Shared helpers for demo scripts
SERVER="${CLOUDX_SERVER:-http://localhost:8080}"

JQ_FMT='{winner: .winner.name, bid_count: .bid_count, top_5: [.all_bidders[:5][] | {name, distance_sq: (.distance_sq * 1000 | round / 1000), bid: .bid_price}]}'

run_query() {
  local intent="$1"
  local tau="${2:-}"

  echo "--- Intent: \"$intent\" ---"
  echo ""

  echo "[WITHOUT tau] All bidders compete:"
  curl -s "$SERVER/ad-request" \
    -H 'Content-Type: application/json' \
    -d "{\"intent\": \"$intent\"}" | jq "$JQ_FMT"
  echo ""

  if [ -n "$tau" ]; then
    echo "[WITH tau=$tau] Only relevant ads pass the gate:"
    curl -s "$SERVER/ad-request" \
      -H 'Content-Type: application/json' \
      -d "{\"intent\": \"$intent\", \"tau\": $tau}" | jq "$JQ_FMT"
    echo ""
  fi

  echo ""
}
