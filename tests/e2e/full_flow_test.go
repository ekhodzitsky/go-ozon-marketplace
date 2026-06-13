//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
	"google.golang.org/grpc"
)

func graphqlRequestWithAuth(t *testing.T, url, query, token string) map[string]interface{} {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"query": query})
	req, err := http.NewRequest(http.MethodPost, url+"/query", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("graphql request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode graphql response: %v", err)
	}
	return result
}

func TestFullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}
	ctx := context.Background()

	adminDSN := tests.StartPostgres(ctx, t)

	esURL, cleanupES := tests.StartElasticsearch(ctx, t)
	defer cleanupES()

	redisAddr := tests.StartRedis(ctx, t)
	kafkaAddr := tests.StartKafka(ctx, t)

	userDSN := tests.CreateDatabase(ctx, t, adminDSN, "user_service")
	catalogDSN := tests.CreateDatabase(ctx, t, adminDSN, "catalog_service")
	orderDSN := tests.CreateDatabase(ctx, t, adminDSN, "order_service")

	tests.RunMigrations(ctx, t, userDSN, "../../services/user-service/migrations")
	tests.RunMigrations(ctx, t, catalogDSN, "../../services/catalog-service/migrations")
	tests.RunMigrations(ctx, t, orderDSN, "../../services/order-service/migrations")

	invAddr := startMockGRPCServer(t, func(s *grpc.Server) {
		inventoryv1.RegisterInventoryServiceServer(s, &mockInventoryServer{})
	})
	payAddr := startMockGRPCServer(t, func(s *grpc.Server) {
		paymentv1.RegisterPaymentServiceServer(s, &mockPaymentServer{})
	})

	userPort := tests.GetFreePort(t)
	userMetricsPort := tests.GetFreePort(t)
	tests.StartService(t, "../../services/user-service", []string{
		"POSTGRES_DSN=" + userDSN,
		fmt.Sprintf("GRPC_PORT=%d", userPort),
		fmt.Sprintf("METRICS_PORT=%d", userMetricsPort),
		"JWT_SECRET=" + e2eJWTSecret,
	})
	userAddr := fmt.Sprintf("127.0.0.1:%d", userPort)
	tests.WaitForGRPC(t, userAddr)

	catalogPort := tests.GetFreePort(t)
	catalogMetricsPort := tests.GetFreePort(t)
	tests.StartService(t, "../../services/catalog-service", []string{
		"POSTGRES_DSN=" + catalogDSN,
		fmt.Sprintf("GRPC_PORT=%d", catalogPort),
		fmt.Sprintf("METRICS_PORT=%d", catalogMetricsPort),
		"ES_URL=" + esURL,
		"JWT_SECRET=" + e2eJWTSecret,
	})
	catalogAddr := fmt.Sprintf("127.0.0.1:%d", catalogPort)
	tests.WaitForGRPC(t, catalogAddr)

	orderPort := tests.GetFreePort(t)
	orderMetricsPort := tests.GetFreePort(t)
	tests.StartService(t, "../../services/order-service", []string{
		"POSTGRES_DSN=" + orderDSN,
		fmt.Sprintf("GRPC_PORT=%d", orderPort),
		fmt.Sprintf("METRICS_PORT=%d", orderMetricsPort),
		"INVENTORY_ADDR=" + invAddr,
		"PAYMENT_ADDR=" + payAddr,
		"CATALOG_ADDR=" + catalogAddr,
		"REDIS_ADDR=" + redisAddr,
		"KAFKA_BROKERS=" + kafkaAddr,
		"JWT_SECRET=" + e2eJWTSecret,
	})
	orderAddr := fmt.Sprintf("127.0.0.1:%d", orderPort)
	tests.WaitForGRPC(t, orderAddr)

	gatewayPort := tests.GetFreePort(t)
	gatewayMetricsPort := tests.GetFreePort(t)
	tests.StartService(t, "../../services/api-gateway", []string{
		fmt.Sprintf("USER_SERVICE_ADDR=%s", userAddr),
		fmt.Sprintf("CATALOG_SERVICE_ADDR=%s", catalogAddr),
		fmt.Sprintf("ORDER_SERVICE_ADDR=%s", orderAddr),
		fmt.Sprintf("INVENTORY_SERVICE_ADDR=%s", invAddr),
		fmt.Sprintf("PAYMENT_SERVICE_ADDR=%s", payAddr),
		fmt.Sprintf("PORT=%d", gatewayPort),
		fmt.Sprintf("METRICS_PORT=%d", gatewayMetricsPort),
		"JWT_SECRET=" + e2eJWTSecret,
		"INSECURE_SKIP_TLS=true",
		"REDIS_ADDR=" + redisAddr,
	})
	gatewayURL := fmt.Sprintf("http://127.0.0.1:%d", gatewayPort)
	tests.WaitForHTTP(t, gatewayURL+"/query")

	// 1. Register
	regResult := graphqlRequestWithAuth(t, gatewayURL, `mutation { register(email: "fullflow@example.com", password: "password123", name: "Full Flow") }`, "")
	data, ok := regResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("register returned no data: %v", regResult)
	}
	userID, ok := data["register"].(string)
	if !ok || userID == "" {
		t.Fatalf("expected user id, got: %v", data["register"])
	}

	// 2. Login
	loginResult := graphqlRequestWithAuth(t, gatewayURL, `mutation { login(email: "fullflow@example.com", password: "password123") }`, "")
	data, ok = loginResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("login returned no data: %v", loginResult)
	}
	token, ok := data["login"].(string)
	if !ok || token == "" {
		t.Fatalf("expected token, got: %v", data["login"])
	}

	// 3. CreateProduct (requires admin role)
	prodResult := graphqlRequestWithAuth(t, gatewayURL, `mutation { createProduct(name: "Flow Product", description: "A flow product", price: 49.99, categories: ["test"]) }`, adminToken(userID, e2eJWTSecret))
	data, ok = prodResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("createProduct returned no data: %v", prodResult)
	}
	productID, ok := data["createProduct"].(string)
	if !ok || productID == "" {
		t.Fatalf("expected product id, got: %v", data["createProduct"])
	}

	// 4. CreateOrder
	createOrderQuery := fmt.Sprintf(`mutation { createOrder(items: [{productId: "%s", quantity: 1, price: 49.99}]) }`, productID)
	orderResult := graphqlRequestWithAuth(t, gatewayURL, createOrderQuery, token)
	data, ok = orderResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("createOrder returned no data: %v", orderResult)
	}
	orderID, ok := data["createOrder"].(string)
	if !ok || orderID == "" {
		t.Fatalf("expected order id, got: %v", data["createOrder"])
	}

	// 5. GetOrder (status may be pending or paid depending on saga speed)
	getOrderQuery := fmt.Sprintf(`query { order(id: "%s") { id status } }`, orderID)
	getResult := graphqlRequestWithAuth(t, gatewayURL, getOrderQuery, token)
	data, ok = getResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("getOrder returned no data: %v", getResult)
	}
	orderData, ok := data["order"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected order data, got: %v", data["order"])
	}
	status, ok := orderData["status"].(string)
	if !ok || (status != "pending" && status != "paid") {
		t.Fatalf("expected order status pending or paid, got: %v", status)
	}

	// 6. CancelOrder
	cancelQuery := fmt.Sprintf(`mutation { cancelOrder(orderId: "%s") }`, orderID)
	cancelResult := graphqlRequestWithAuth(t, gatewayURL, cancelQuery, token)
	data, ok = cancelResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("cancelOrder returned no data: %v", cancelResult)
	}
	cancelled, ok := data["cancelOrder"].(bool)
	if !ok || !cancelled {
		t.Fatalf("expected cancelOrder to return true, got: %v", data["cancelOrder"])
	}

	// Verify cancelled status
	getResult = graphqlRequestWithAuth(t, gatewayURL, getOrderQuery, token)
	data, ok = getResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("getOrder after cancel returned no data: %v", getResult)
	}
	orderData, ok = data["order"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected order data after cancel, got: %v", data["order"])
	}
	status, ok = orderData["status"].(string)
	if !ok || status != "cancelled" {
		t.Fatalf("expected order status cancelled, got: %v", status)
	}
}
