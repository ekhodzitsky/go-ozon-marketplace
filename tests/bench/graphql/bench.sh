#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Check dependencies
if ! command -v k6 &> /dev/null; then
    echo "Error: k6 is not installed. Install it: https://k6.io/docs/get-started/installation/"
    exit 1
fi

# Auth token generation
JWT_SECRET="${JWT_SECRET:-}"
if [ -z "$JWT_SECRET" ]; then
    echo "Warning: JWT_SECRET not set. Trying to load from .env"
    if [ -f "$PROJECT_ROOT/.env" ]; then
        set -a
        # shellcheck source=/dev/null
        source "$PROJECT_ROOT/.env"
        set +a
    fi
fi

if [ -z "$JWT_SECRET" ]; then
    echo "Error: JWT_SECRET environment variable is required for authenticated GraphQL benchmarks."
    echo "Set it in .env or export JWT_SECRET=your-secret"
    exit 1
fi

TOKEN=$(cd "$PROJECT_ROOT" && go run "$PROJECT_ROOT/tests/bench/grpc/gen_jwt.go" -secret "$JWT_SECRET")
export AUTH_TOKEN="$TOKEN"
export GRAPHQL_URL="${GRAPHQL_URL:-http://localhost:8080/query}"

echo "=== GraphQL Benchmark Suite ==="
echo "Target: $GRAPHQL_URL"

# Check gateway
if ! nc -z localhost 8080 2>/dev/null; then
    echo "Error: API Gateway on localhost:8080 is not reachable. Start the gateway first."
    exit 1
fi

echo ""
echo "--- searchProducts ---"
k6 run --summary-trend-stats="avg,min,med,max,p(75),p(90),p(95),p(99)" \
    "$SCRIPT_DIR/search_products.js" || echo "Benchmark failed"

echo ""
echo "--- createProduct ---"
k6 run --summary-trend-stats="avg,min,med,max,p(75),p(90),p(95),p(99)" \
    "$SCRIPT_DIR/create_product.js" || echo "Benchmark failed"

echo ""
echo "=== GraphQL Benchmark Suite Complete ==="
