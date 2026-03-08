#!/usr/bin/env bash
# Trial runner: clone repos, run skills, capture output + diff
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SKILLS_REPO="https://github.com/kimjune01/vectorspace-skills.git"
CACHE_DIR="${HOME}/.cache/vectorspace-skill-test"
SKILLS_CACHE="$CACHE_DIR/_skills"
RESULTS_DIR="$SCRIPT_DIR/results"
REPOS_FILE="$SCRIPT_DIR/repos.json"
SPEC_DIR="$SCRIPT_DIR/spec"
MAX_TURNS=30
GRADE=false

# Parse args
SINGLE_REPO=""
SINGLE_SKILL=""
while [[ $# -gt 0 ]]; do
  case $1 in
    --repo) SINGLE_REPO="$2"; shift 2 ;;
    --skill) SINGLE_SKILL="$2"; shift 2 ;;
    --grade) GRADE=true; shift ;;
    *) echo "Usage: $0 [--repo owner/name] [--skill evaluate|install|verify] [--grade]"; exit 1 ;;
  esac
done

mkdir -p "$CACHE_DIR" "$RESULTS_DIR"

# Fetch latest skills from public repo
if [ -d "$SKILLS_CACHE/.git" ]; then
  git -C "$SKILLS_CACHE" pull --ff-only -q
else
  git clone --depth=1 "$SKILLS_REPO" "$SKILLS_CACHE"
fi

# Read repos
if [ -n "$SINGLE_REPO" ]; then
  REPOS=$(jq -r --arg r "$SINGLE_REPO" '.[] | select(.name == $r) | .name' "$REPOS_FILE")
else
  REPOS=$(jq -r '.[].name' "$REPOS_FILE")
fi

for REPO in $REPOS; do
  SLUG=$(echo "$REPO" | tr '/' '-')
  CLONE_DIR="$CACHE_DIR/$SLUG"
  RESULT_DIR="$RESULTS_DIR/$SLUG"
  mkdir -p "$RESULT_DIR"

  # Clone or reset
  if [ -d "$CLONE_DIR/.git" ]; then
    echo "=== Resetting cached clone: $REPO ==="
    git -C "$CLONE_DIR" checkout . && git -C "$CLONE_DIR" clean -fd
  else
    echo "=== Cloning: $REPO ==="
    gh repo clone "$REPO" "$CLONE_DIR" -- --depth=1
  fi

  # Copy skills from public repo into the clone
  mkdir -p "$CLONE_DIR/.vectorspace"
  cp "$SKILLS_CACHE/evaluate.md" "$CLONE_DIR/.vectorspace/"
  cp "$SKILLS_CACHE/install.md" "$CLONE_DIR/.vectorspace/"
  cp "$SKILLS_CACHE/verify.md" "$CLONE_DIR/.vectorspace/"

  SKILLS="${SINGLE_SKILL:-evaluate install verify}"
  PREV_SKILL=""
  for SKILL in $SKILLS; do
    echo "--- Trial: $SKILL on $REPO ---"

    # Reset logic:
    # - evaluate: always reset (read-only, clean slate)
    # - install: always reset (starts from clean repo)
    # - verify: skip reset if install just ran (verify audits installed code)
    if [ "$SKILL" = "evaluate" ]; then
      : # no reset needed, evaluate is read-only
    elif [ "$SKILL" = "verify" ] && [ "$PREV_SKILL" = "install" ]; then
      echo "    (chained: verify runs on install output)"
    else
      git -C "$CLONE_DIR" checkout . && git -C "$CLONE_DIR" clean -fd -e .vectorspace
    fi

    # Run skill via claude headless mode
    (cd "$CLONE_DIR" && claude -p \
      "Read .vectorspace/$SKILL.md and follow the instructions. The publisher_id is 'test-$SLUG'." \
      --dangerously-skip-permissions \
      --max-turns "$MAX_TURNS" \
      --output-format json \
    ) > "$RESULT_DIR/${SKILL}-output.json" 2>&1 || true

    # Capture diff
    git -C "$CLONE_DIR" diff > "$RESULT_DIR/${SKILL}-diff.patch" 2>/dev/null || true
    git -C "$CLONE_DIR" diff --stat >> "$RESULT_DIR/${SKILL}-diff.patch" 2>/dev/null || true
    # Also capture untracked files
    git -C "$CLONE_DIR" ls-files --others --exclude-standard > "$RESULT_DIR/${SKILL}-untracked.txt" 2>/dev/null || true

    echo "    Output: $RESULT_DIR/${SKILL}-output.json"
    echo "    Diff:   $RESULT_DIR/${SKILL}-diff.patch"

    # Grade against spec
    if [ "$GRADE" = true ] && [ -f "$SPEC_DIR/${SKILL}-spec.md" ]; then
      echo "    Grading $SKILL against spec..."
      GRADE_PROMPT="You are grading a skill trial. Read the spec and the trial output, then produce a JSON object with:
- \"skill\": the skill name
- \"repo\": the repo name
- \"criteria\": an array of objects, each with \"name\" (string), \"pass\" (boolean), \"evidence\" (string quoting the relevant output)
- \"summary\": one sentence overall assessment
- \"pass\": boolean, true only if ALL criteria pass

SPEC:
$(cat "$SPEC_DIR/${SKILL}-spec.md")

TRIAL OUTPUT (agent result):
$(jq -r '.result // .message // "NO RESULT FIELD"' "$RESULT_DIR/${SKILL}-output.json" 2>/dev/null || cat "$RESULT_DIR/${SKILL}-output.json")

DIFF STAT:
$(tail -20 "$RESULT_DIR/${SKILL}-diff.patch" 2>/dev/null || echo "no diff")

UNTRACKED FILES:
$(cat "$RESULT_DIR/${SKILL}-untracked.txt" 2>/dev/null || echo "none")

Respond with ONLY the JSON object, no markdown fences."

      claude -p "$GRADE_PROMPT" \
        --max-turns 1 \
        --output-format json \
      > "$RESULT_DIR/${SKILL}-grade.json" 2>&1 || true

      echo "    Grade:  $RESULT_DIR/${SKILL}-grade.json"
    fi

    PREV_SKILL="$SKILL"
  done

  echo ""
done

echo "=== All trials complete. Results in $RESULTS_DIR ==="
