#!/usr/bin/env bash
# FlyFin — Freelancer Finance vertical
set -euo pipefail
source "$(dirname "$0")/common.sh"

echo "=== FlyFin (Freelancer Finance) ==="
echo ""

run_query "what expenses can I deduct as a freelance designer" 0.7
run_query "do I need to pay quarterly estimated taxes on 1099 income" 0.7
run_query "should I set up an LLC or stay sole proprietor" 0.7
run_query "who won the world series last year" 0.7
