#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PPROF_PORT="${PPROF_PORT:-6060}"
PROFILE_DIR="${PROFILE_DIR:-$PROJECT_ROOT/profiles}"
SERVICE_BIN="${SERVICE_BIN:-$PROJECT_ROOT/services/order-service/main}"
SERVICE_CMD="${SERVICE_CMD:-}"

mkdir -p "$PROFILE_DIR"

# Check go
if ! command -v go &> /dev/null; then
    echo "Error: go is not installed"
    exit 1
fi

# Determine service command
if [ -n "$SERVICE_CMD" ]; then
    CMD="$SERVICE_CMD"
elif [ -x "$SERVICE_BIN" ]; then
    CMD="$SERVICE_BIN"
else
    echo "Error: No service binary found at $SERVICE_BIN"
    echo "Build the service first: make build-order"
    echo "Or set SERVICE_CMD / SERVICE_BIN env variable"
    exit 1
fi

echo "=== Go Profiling ==="
echo "Service: $CMD"
echo "pprof port: $PPROF_PORT"
echo "Profiles dir: $PROFILE_DIR"

# Check if pprof port is already occupied
if nc -z localhost "$PPROF_PORT" 2>/dev/null; then
    echo "Note: pprof endpoint already available on localhost:$PPROF_PORT"
else
    echo "Starting service with GODEBUG=gctrace=1 ..."
    GODEBUG=gctrace=1 "$CMD" &
    PID=$!
    echo "Service PID: $PID"

    # Wait for pprof
    for i in {1..30}; do
        if nc -z localhost "$PPROF_PORT" 2>/dev/null; then
            break
        fi
        if ! kill -0 $PID 2>/dev/null; then
            echo "Error: service process (PID: $PID) exited before pprof endpoint was available."
            echo "Check service logs and environment variables (e.g., POSTGRES_DSN, JWT_SECRET)."
            exit 1
        fi
        sleep 1
    done

    if ! nc -z localhost "$PPROF_PORT" 2>/dev/null; then
        echo "Error: pprof endpoint did not start on localhost:$PPROF_PORT"
        echo "Make sure the service imports _ \"net/http/pprof\" and starts debug server"
        kill $PID 2>/dev/null || true
        exit 1
    fi
fi

echo ""
echo "Collecting CPU profile (30s) ..."
go tool pprof -svg "http://localhost:${PPROF_PORT}/debug/pprof/profile?seconds=30" > "$PROFILE_DIR/cpu.svg" || {
    echo "Warning: CPU profile collection failed. Ensure the service has pprof enabled."
}

echo "Collecting heap profile ..."
go tool pprof -svg "http://localhost:${PPROF_PORT}/debug/pprof/heap" > "$PROFILE_DIR/heap.svg" || {
    echo "Warning: heap profile collection failed. Ensure the service has pprof enabled."
}

echo "Collecting goroutine profile ..."
go tool pprof -svg "http://localhost:${PPROF_PORT}/debug/pprof/goroutine" > "$PROFILE_DIR/goroutine.svg" || {
    echo "Warning: goroutine profile collection failed."
}

echo ""
echo "Profiles saved to $PROFILE_DIR:"
ls -la "$PROFILE_DIR"/*.svg 2>/dev/null || true

if [ -n "${PID:-}" ]; then
    echo ""
    echo "Stopping service (PID: $PID) ..."
    kill $PID 2>/dev/null || true
fi

echo "=== Profiling Complete ==="
