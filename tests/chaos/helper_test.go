//go:build chaos

package chaos

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"testing"
	"time"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const (
	jwtSecret    = "chaos-test-secret"
	postgresDSN  = "postgres://ozon:ozonpass@localhost:5432/marketplace?sslmode=disable"
	composeBase  = "../../infra/docker/docker-compose.yml"
	composeChaos = "../../infra/docker/docker-compose.chaos.yml"
)

func dockerComposeUp(t *testing.T) {
	t.Helper()
	cmd := exec.Command("docker", "compose", "-f", composeBase, "-f", composeChaos, "up", "--build", "-d")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose up failed: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		dockerComposeDown(t)
	})
	// Allow containers to start and become healthy
	time.Sleep(8 * time.Second)
}

func dockerComposeDown(t *testing.T) {
	t.Helper()
	cmd := exec.Command("docker", "compose", "-f", composeBase, "-f", composeChaos, "down", "-v")
	_ = cmd.Run()
}

func dockerKill(t *testing.T, container string) {
	t.Helper()
	cmd := exec.Command("docker", "kill", container)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker kill %s failed: %v\n%s", container, err, out)
	}
}

func dockerStop(t *testing.T, container string) {
	t.Helper()
	cmd := exec.Command("docker", "stop", container)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker stop %s failed: %v\n%s", container, err, out)
	}
}

func dockerStart(t *testing.T, container string) {
	t.Helper()
	cmd := exec.Command("docker", "start", container)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker start %s failed: %v\n%s", container, err, out)
	}
}

func runMigrations(t *testing.T) {
	t.Helper()
	tests.RunMigrations(context.Background(), t, postgresDSN,
		"../../services/order-service/migrations",
		"../../services/inventory-service/migrations",
		"../../services/payment-service/migrations",
		"../../services/user-service/migrations",
		"../../services/catalog-service/migrations",
	)
}

func authContext(ctx context.Context, userID string) context.Context {
	claims := middleware.CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Role: string(middleware.RoleUser),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(jwtSecret))
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tokenStr)
}

func serviceAuthContext(ctx context.Context) context.Context {
	claims := middleware.CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "order-service",
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Role: string(middleware.RoleService),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(jwtSecret))
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tokenStr)
}

func newOrderClient(t *testing.T) orderv1.OrderServiceClient {
	t.Helper()
	addr := "localhost:50051"
	tests.WaitForGRPC(t, addr)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to order-service: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return orderv1.NewOrderServiceClient(conn)
}

func newInventoryClient(t *testing.T) inventoryv1.InventoryServiceClient {
	t.Helper()
	addr := "localhost:50052"
	tests.WaitForGRPC(t, addr)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to inventory-service: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return inventoryv1.NewInventoryServiceClient(conn)
}

func newPaymentClient(t *testing.T) paymentv1.PaymentServiceClient {
	t.Helper()
	addr := "localhost:50053"
	tests.WaitForGRPC(t, addr)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to payment-service: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return paymentv1.NewPaymentServiceClient(conn)
}

func ensureStock(t *testing.T, productID string, available int) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), postgresDSN)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	defer pool.Close()
	_, err = pool.Exec(context.Background(), `
		INSERT INTO inventory (product_id, available, reserved) VALUES ($1, $2, 0)
		ON CONFLICT (product_id) DO UPDATE SET available = $2
	`, productID, available)
	if err != nil {
		t.Fatalf("failed to ensure stock: %v", err)
	}
}

func getOrderStatus(t *testing.T, orderID string) string {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), postgresDSN)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	defer pool.Close()
	var status string
	err = pool.QueryRow(context.Background(), `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to get order status: %v", err)
	}
	return status
}

func getOutboxUnprocessedCount(t *testing.T) int {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), postgresDSN)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	defer pool.Close()
	var count int
	err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM outbox WHERE processed_at IS NULL`).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count outbox: %v", err)
	}
	return count
}

func graphqlRequest(t *testing.T, url, query string) map[string]interface{} {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"query": query})
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("graphql request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode graphql response: %v", err)
	}
	result["_http_status"] = resp.StatusCode
	return result
}
