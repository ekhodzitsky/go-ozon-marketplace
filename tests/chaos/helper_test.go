//go:build chaos

package chaos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const (
	jwtSecret   = "chaos-test-secret"
	composeBase = "../../infra/docker/docker-compose.yml"
)

func postgresDSN() string {
	if dsn := os.Getenv("POSTGRES_DSN"); dsn != "" {
		return dsn
	}
	return "postgres://ozon:ozonpass@localhost:5432/marketplace?sslmode=disable"
}

func composeProjectName() string {
	if name := os.Getenv("COMPOSE_PROJECT_NAME"); name != "" {
		return name
	}
	return "go-ozon-marketplace"
}

func containerName(service string) string {
	return fmt.Sprintf("%s-%s-1", composeProjectName(), service)
}

func dockerComposeFiles() []string {
	files := []string{"-f", composeBase}
	if chaos := os.Getenv("COMPOSE_CHAOS_FILE"); chaos != "" {
		files = append(files, "-f", chaos)
	} else {
		files = append(files, "-f", "../../infra/docker/docker-compose.chaos.yml")
	}
	return files
}

func dockerComposeUp(t *testing.T) {
	t.Helper()
	args := append(dockerComposeFiles(), "up", "--build", "-d")
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose up failed: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		dockerComposeDown(t)
	})
	// Wait for the gateway to be ready instead of sleeping a fixed amount.
	gatewayURL := "http://localhost:8080/query"
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(gatewayURL)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Logf("gateway did not become ready within 30s, continuing anyway")
}

func dockerComposeDown(t *testing.T) {
	t.Helper()
	args := append(dockerComposeFiles(), "down", "-v")
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
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

func dockerHealthCheck(t *testing.T, container string) bool {
	t.Helper()
	cmd := exec.Command("docker", "inspect", "--format={{.State.Status}}", container)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "running"
}

func waitForContainer(t *testing.T, container string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if dockerHealthCheck(t, container) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("container %s did not become running within %v", container, timeout)
}

func runMigrations(t *testing.T) {
	t.Helper()
	tests.RunMigrations(context.Background(), t, postgresDSN(),
		"../../services/order-service/migrations",
		"../../services/inventory-service/migrations",
		"../../services/payment-service/migrations",
		"../../services/user-service/migrations",
		"../../services/catalog-service/migrations",
	)
}

func authContext(ctx context.Context, userID string) context.Context {
	claims := auth.CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        uuid.New().String(),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			NotBefore: jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
		},
		Role: string(auth.RoleUser),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(jwtSecret))
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tokenStr)
}

func serviceAuthContext(ctx context.Context) context.Context {
	claims := auth.CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "order-service",
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
		},
		Role: string(auth.RoleService),
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
	t.Cleanup(func() { _ = conn.Close() })
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
	t.Cleanup(func() { _ = conn.Close() })
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
	t.Cleanup(func() { _ = conn.Close() })
	return paymentv1.NewPaymentServiceClient(conn)
}

func ensureStock(t *testing.T, productID string, available int) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), postgresDSN())
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
	pool, err := pgxpool.New(context.Background(), postgresDSN())
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
	pool, err := pgxpool.New(context.Background(), postgresDSN())
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
	defer func() { _ = resp.Body.Close() }()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode graphql response: %v", err)
	}
	result["_http_status"] = resp.StatusCode
	return result
}
