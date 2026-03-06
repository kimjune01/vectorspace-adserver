#!/usr/bin/env bash
# FreeLawChat — Legal vertical
set -euo pipefail
source "$(dirname "$0")/common.sh"

echo "=== FreeLawChat (Legal) ==="
echo ""

run_query "my landlord is trying to evict me without notice" 0.7
run_query "going through a divorce and need custody advice" 0.7
run_query "I was rear-ended and need to file an injury claim" 0.95
run_query "how do I make sourdough starter" 0.7
