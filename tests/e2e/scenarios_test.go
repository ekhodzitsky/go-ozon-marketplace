//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"

	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestPriceTamper(t *testing.T) {
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

	regResult := graphqlRequest(t, gatewayURL+"/query", `mutation { register(email: "tamper@example.com", password: "password123", name: "Tamper") }`)
	data, ok := regResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("register returned no data: %v", regResult)
	}
	userID, ok := data["register"].(string)
	if !ok || userID == "" {
		t.Fatalf("expected user id, got: %v", data["register"])
	}

	prodResult := graphqlRequestWithAuth(t, gatewayURL, `mutation { createProduct(name: "Expensive", description: "Expensive item", price: 100.00, categories: ["test"]) }`, adminToken(userID, e2eJWTSecret))
	data, ok = prodResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("createProduct returned no data: %v", prodResult)
	}
	productID, ok := data["createProduct"].(string)
	if !ok || productID == "" {
		t.Fatalf("expected product id, got: %v", data["createProduct"])
	}

	// Fetch the real product price from catalog-service.
	catalogConn, err := grpc.NewClient(catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to catalog-service: %v", err)
	}
	defer func() { _ = catalogConn.Close() }()
	catalogClient := catalogv1.NewCatalogServiceClient(catalogConn)
	prodResp, err := catalogClient.GetProduct(ctx, tests.NewGetProductRequestBuilder().WithProductID(productID).Build())
	if err != nil {
		t.Fatalf("failed to get product: %v", err)
	}
	realPriceCents := prodResp.Product.PriceCents
	if realPriceCents <= 0 {
		t.Fatalf("expected positive product price, got %d", realPriceCents)
	}

	orderConn, err := grpc.NewClient(orderAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to order-service: %v", err)
	}
	defer func() { _ = orderConn.Close() }()

	orderClient := orderv1.NewOrderServiceClient(orderConn)
	createCtx := authContext(ctx, userID, e2eJWTSecret)

	// Attempt to create an order with a tampered price (1 cent instead of the real price).
	_, err = orderClient.CreateOrder(createCtx, tests.NewCreateOrderRequestBuilder().
		AddItem(productID, 1, 1).
		Build())
	if err == nil {
		t.Fatal("expected price tamper to be rejected, but order was created")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument && st.Code() != codes.FailedPrecondition {
		t.Fatalf("expected InvalidArgument or FailedPrecondition for tampered price, got %v: %s", st.Code(), st.Message())
	}
}

func TestPaymentFailThenRefund(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}
	ctx := context.Background()

	dsn := tests.StartPostgres(ctx, t)
	tests.RunMigrations(ctx, t, dsn, "../../services/payment-service/migrations")

	payPort := tests.GetFreePort(t)
	payMetricsPort := tests.GetFreePort(t)
	tests.StartService(t, "../../services/payment-service", []string{
		"POSTGRES_DSN=" + dsn,
		fmt.Sprintf("GRPC_PORT=%d", payPort),
		fmt.Sprintf("METRICS_PORT=%d", payMetricsPort),
		"JWT_SECRET=" + e2eJWTSecret,
	})
	payAddr := fmt.Sprintf("127.0.0.1:%d", payPort)
	tests.WaitForGRPC(t, payAddr)

	conn, err := grpc.NewClient(payAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to payment-service: %v", err)
	}
	defer func() { _ = conn.Close() }()

	client := paymentv1.NewPaymentServiceClient(conn)
	userID := uuid.New().String()
	authCtx := authContext(ctx, userID, e2eJWTSecret)

	var failedPaymentID string
	for i := 0; i < 50; i++ {
		orderID := uuid.New().String()
		resp, err := client.ProcessPayment(authCtx, tests.NewProcessPaymentRequestBuilder().
			WithOrderID(orderID).
			WithAmountCents(9999).
			Build())
		if err != nil {
			t.Fatalf("process payment failed unexpectedly: %v", err)
		}
		if resp.Status == paymentv1.PaymentStatus_PAYMENT_STATUS_FAILED {
			failedPaymentID = resp.PaymentId
			break
		}
	}
	if failedPaymentID == "" {
		t.Fatal("could not get a failed payment after 50 attempts")
	}

	_, err = client.Refund(authCtx, tests.NewRefundRequestBuilder().
		WithPaymentID(failedPaymentID).
		Build())
	if err == nil {
		t.Fatal("expected refund of failed payment to return error")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}
}
