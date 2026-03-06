#!/usr/bin/env bash
# Piere — Personal Finance vertical
set -euo pipefail
source "$(dirname "$0")/common.sh"

echo "=== Piere (Finance) ==="
echo ""

run_query "I want to start saving for retirement but don't know where to begin" 0.8
run_query "how do I pay off credit card debt faster" 0.8
run_query "what's the weather going to be like this weekend" 0.8
