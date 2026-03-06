#!/usr/bin/env bash
# CodyMD — Health vertical
set -euo pipefail
source "$(dirname "$0")/common.sh"

echo "=== CodyMD (Health) ==="
echo ""

run_query "my lower back has been hurting for two weeks" 0.8
run_query "feeling anxious and can't sleep at night" 0.8
run_query "I have a rash on my arm that won't go away" 0.8
run_query "what's the best movie you've seen lately" 0.8
