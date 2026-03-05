#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

DB_PATH="${1:-demo.db}"
SIDECAR_PORT=8081
SERVER_PORT=8080

cleanup() {
  echo "Cleaning up..."
  [ -n "${SERVER_PID:-}" ] && kill "$SERVER_PID" 2>/dev/null || true
  [ -n "${SIDECAR_PID:-}" ] && kill "$SIDECAR_PID" 2>/dev/null || true
  wait 2>/dev/null || true
}
trap cleanup EXIT

# Check if sidecar is already running
if curl -sf "http://localhost:$SIDECAR_PORT/health" >/dev/null 2>&1; then
  echo "Sidecar already running on :$SIDECAR_PORT"
  SIDECAR_PID=""
else
  echo "Starting embedding sidecar on :$SIDECAR_PORT..."
  cd sidecar
  uv run uvicorn main:app --port "$SIDECAR_PORT" --log-level warning &
  SIDECAR_PID=$!
  cd ..

  # Wait for sidecar to be ready
  echo -n "Waiting for sidecar"
  for i in $(seq 1 30); do
    if curl -sf "http://localhost:$SIDECAR_PORT/health" >/dev/null 2>&1; then
      echo " ready."
      break
    fi
    echo -n "."
    sleep 1
  done

  if ! curl -sf "http://localhost:$SIDECAR_PORT/health" >/dev/null 2>&1; then
    echo " FAILED (sidecar didn't start in 30s)"
    exit 1
  fi
fi

# Run the server with --seed, then shut it down after seeding
echo "Seeding database: $DB_PATH"
go run ./cmd/server/ \
  --sidecar-url "http://localhost:$SIDECAR_PORT" \
  --db-path "$DB_PATH" \
  --seed &
SERVER_PID=$!

# Wait for server to be ready, then verify positions were seeded
echo -n "Waiting for server"
for i in $(seq 1 30); do
  if curl -sf "http://localhost:$SERVER_PORT/health" >/dev/null 2>&1; then
    echo " ready."
    break
  fi
  echo -n "."
  sleep 1
done

if ! curl -sf "http://localhost:$SERVER_PORT/health" >/dev/null 2>&1; then
  echo " FAILED (server didn't start in 30s)"
  exit 1
fi

# Verify
COUNT=$(curl -sf "http://localhost:$SERVER_PORT/positions" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))")
echo "Seeded $COUNT advertisers into $DB_PATH"

# Kill server (sidecar killed by trap)
kill "$SERVER_PID" 2>/dev/null || true
SERVER_PID=""

echo "Done. Start the server with:"
echo "  go run ./cmd/server/ --sidecar-url http://localhost:$SIDECAR_PORT --db-path $DB_PATH"
