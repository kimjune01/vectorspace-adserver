#!/usr/bin/env bash
# Origin Financial — Finance vertical
set -euo pipefail
source "$(dirname "$0")/common.sh"

echo "=== Origin Financial (Finance) ==="
echo ""

run_query "how should I invest my first ten thousand dollars" 0.85
run_query "what's the difference between a Roth IRA and traditional IRA" 0.85
run_query "what's a good beginner programming language" 0.85
