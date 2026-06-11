#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Check dependencies
if ! command -v ghz &> /dev/null; then
    echo "Error: ghz is not installed. Install it: https://github.com/bojand/ghz"
    exit 1
fi

# JWT generation
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
    echo "Error: JWT_SECRET environment variable is required for authenticated gRPC benchmarks."
    echo "Set it in .env or export JWT_SECRET=your-secret"
    exit 1
fi

TOKEN=$(cd "$PROJECT_ROOT" && go run "$SCRIPT_DIR/gen_jwt.go" -secret "$JWT_SECRET")
METADATA="{\"authorization\":[\"Bearer $TOKEN\"]}"

echo "=== gRPC Benchmark Suite ==="

# Helper to check port
check_port() {
    local host=$1
    local port=$2
    if ! nc -z "$host" "$port" 2>/dev/null; then
        echo "Error: Service on $host:$port is not reachable. Start the service first."
        return 1
    fi
}

# Order Service (localhost:50055)
echo ""
echo "--- OrderService::CreateOrder (localhost:50055) ---"
if check_port localhost 50055; then
    ghz --proto api/proto/order/v1/order.proto \
        --call order.v1.OrderService/CreateOrder \
        -d @"$SCRIPT_DIR/create_order.json" \
        --metadata "$METADATA" \
        -n 10000 -c 100 \
        localhost:50055 || echo "Benchmark failed"
fi

# Catalog Service (localhost:50052)
echo ""
echo "--- CatalogService::SearchProducts (localhost:50052) ---"
if check_port localhost 50052; then
    ghz --proto api/proto/catalog/v1/catalog.proto \
        --call catalog.v1.CatalogService/SearchProducts \
        -d @"$SCRIPT_DIR/search_products.json" \
        --metadata "$METADATA" \
        -n 10000 -c 100 \
        localhost:50052 || echo "Benchmark failed"
fi

echo ""
echo "--- CatalogService::ListProducts (localhost:50052) ---"
if check_port localhost 50052; then
    ghz --proto api/proto/catalog/v1/catalog.proto \
        --call catalog.v1.CatalogService/ListProducts \
        -d @"$SCRIPT_DIR/list_products.json" \
        --metadata "$METADATA" \
        -n 10000 -c 100 \
        localhost:50052 || echo "Benchmark failed"
fi

# User Service (localhost:50051)
echo ""
echo "--- UserService::GetUser (localhost:50051) ---"
if check_port localhost 50051; then
    ghz --proto api/proto/user/v1/user.proto \
        --call user.v1.UserService/GetUser \
        -d @"$SCRIPT_DIR/get_user.json" \
        --metadata "$METADATA" \
        -n 10000 -c 100 \
        localhost:50051 || echo "Benchmark failed"
fi

echo ""
echo "=== gRPC Benchmark Suite Complete ==="
