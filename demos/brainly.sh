#!/usr/bin/env bash
# Brainly — Education vertical
set -euo pipefail
source "$(dirname "$0")/common.sh"

echo "=== Brainly (Education) ==="
echo ""

run_query "my child needs a tutor for math class" 0.8
run_query "how do I study for the SAT effectively" 0.8
run_query "my child has ADHD and is falling behind in school" 0.8
run_query "what's a good recipe for banana bread" 0.8
