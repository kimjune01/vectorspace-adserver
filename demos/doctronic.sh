#!/usr/bin/env bash
# Doctronic — Health vertical
set -euo pipefail
source "$(dirname "$0")/common.sh"

echo "=== Doctronic (Health) ==="
echo ""

run_query "I think I have a sinus infection" 0.85
run_query "my child has a fever and sore throat" 0.85
run_query "what time zone is tokyo in" 0.85
