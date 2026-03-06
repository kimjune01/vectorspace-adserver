#!/usr/bin/env bash
# Counsel Health — Health vertical
set -euo pipefail
source "$(dirname "$0")/common.sh"

echo "=== Counsel Health (Health) ==="
echo ""

run_query "I need to talk to someone about my depression" 0.76
run_query "should I see a doctor about this knee pain" 0.76
run_query "can you explain how bitcoin mining works" 0.76
