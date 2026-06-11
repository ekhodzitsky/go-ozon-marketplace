//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"

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

	dsn := tests.StartPostgres(ctx, t)
	esURL, cleanupES := tests.StartElasticsearch(ctx, t)
	defer cleanupES()

	tests.RunMigrations(ctx, t, dsn,
		"../../services/user-service/migrations",
		"../../services/catalog-service/migrations",
		"../../services/order-service/migrations",
	)

	invAddr := startMockGRPCServer(t, func(s *grpc.Server) {
		inventoryv1.RegisterInventoryServiceServer(s, &mockInventoryServer{})
	})
	payAddr := startMockGRPCServer(t, func(s *grpc.Server) {
		paymentv1.RegisterPaymentServiceServer(s, &mockPaymentServer{})
	})

	jwtSecret := "test-secret"

	userPort := tests.GetFreePort(t)
	tests.StartService(t, "../../services/user-service", []string{
		"POSTGRES_DSN=" + dsn,
		fmt.Sprintf("GRPC_PORT=%d", userPort),
		"JWT_SECRET=" + jwtSecret,
	})
	userAddr := fmt.Sprintf("127.0.0.1:%d", userPort)
	tests.WaitForGRPC(t, userAddr)

	catalogPort := tests.GetFreePort(t)
	tests.StartService(t, "../../services/catalog-service", []string{
		"POSTGRES_DSN=" + dsn,
		fmt.Sprintf("GRPC_PORT=%d", catalogPort),
		"ES_URL=" + esURL,
		"JWT_SECRET=" + jwtSecret,
	})
	catalogAddr := fmt.Sprintf("127.0.0.1:%d", catalogPort)
	tests.WaitForGRPC(t, catalogAddr)

	orderPort := tests.GetFreePort(t)
	tests.StartService(t, "../../services/order-service", []string{
		"POSTGRES_DSN=" + dsn,
		fmt.Sprintf("GRPC_PORT=%d", orderPort),
		"INVENTORY_ADDR=" + invAddr,
		"PAYMENT_ADDR=" + payAddr,
		"JWT_SECRET=" + jwtSecret,
	})
	orderAddr := fmt.Sprintf("127.0.0.1:%d", orderPort)
	tests.WaitForGRPC(t, orderAddr)

	gatewayPort := tests.GetFreePort(t)
	tests.StartService(t, "../../services/api-gateway", []string{
		fmt.Sprintf("USER_SERVICE_ADDR=%s", userAddr),
		fmt.Sprintf("CATALOG_SERVICE_ADDR=%s", catalogAddr),
		fmt.Sprintf("PORT=%d", gatewayPort),
		"JWT_SECRET=" + jwtSecret,
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

	prodResult := graphqlRequest(t, gatewayURL+"/query", `mutation { createProduct(name: "Expensive", description: "Expensive item", price: 100.00, categories: ["test"]) }`)
	data, ok = prodResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("createProduct returned no data: %v", prodResult)
	}
	productID, ok := data["createProduct"].(string)
	if !ok || productID == "" {
		t.Fatalf("expected product id, got: %v", data["createProduct"])
	}

	orderConn, err := grpc.NewClient(orderAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to order-service: %v", err)
	}
	defer orderConn.Close()

	orderClient := orderv1.NewOrderServiceClient(orderConn)
	createCtx := authContext(ctx, userID, jwtSecret)
	createResp, err := orderClient.CreateOrder(createCtx, &orderv1.CreateOrderRequest{
		UserId: userID,
		Items: []*orderv1.OrderItem{
			{ProductId: productID, Quantity: 1, Price: 0.01},
		},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	getResp, err := orderClient.GetOrder(authContext(ctx, userID, jwtSecret), &orderv1.GetOrderRequest{
		OrderId: createResp.OrderId,
	})
	if err != nil {
		t.Fatalf("get order failed: %v", err)
	}

	if getResp.Order.TotalAmount != 0.01 {
		t.Fatalf("expected tampered total 0.01, got %v", getResp.Order.TotalAmount)
	}
	if len(getResp.Order.Items) != 1 || getResp.Order.Items[0].Price != 0.01 {
		t.Fatalf("expected item price 0.01, got %v", getResp.Order.Items)
	}
}

func TestPaymentFailThenRefund(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}
	ctx := context.Background()

	dsn := tests.StartPostgres(ctx, t)
	tests.RunMigrations(ctx, t, dsn, "../../services/payment-service/migrations")

	jwtSecret := "test-secret"
	payPort := tests.GetFreePort(t)
	tests.StartService(t, "../../services/payment-service", []string{
		"POSTGRES_DSN=" + dsn,
		fmt.Sprintf("GRPC_PORT=%d", payPort),
		"JWT_SECRET=" + jwtSecret,
	})
	payAddr := fmt.Sprintf("127.0.0.1:%d", payPort)
	tests.WaitForGRPC(t, payAddr)

	conn, err := grpc.NewClient(payAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to payment-service: %v", err)
	}
	defer conn.Close()

	client := paymentv1.NewPaymentServiceClient(conn)
	userID := uuid.New().String()

	var failedPaymentID string
	for i := 0; i < 50; i++ {
		orderID := uuid.New().String()
		resp, err := client.ProcessPayment(authContext(ctx, userID, jwtSecret), &paymentv1.ProcessPaymentRequest{
			OrderId: orderID,
			UserId:  userID,
			Amount:  99.99,
		})
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

	_, err = client.Refund(authContext(ctx, userID, jwtSecret), &paymentv1.RefundRequest{
		PaymentId: failedPaymentID,
	})
	if err == nil {
		t.Fatal("expected refund of failed payment to return error")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}
}
