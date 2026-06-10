package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type mockInventoryServer struct {
	inventoryv1.UnimplementedInventoryServiceServer
}

func (s *mockInventoryServer) Reserve(_ context.Context, _ *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	return &inventoryv1.ReserveResponse{Success: true}, nil
}

func (s *mockInventoryServer) Release(_ context.Context, _ *inventoryv1.ReleaseRequest) (*inventoryv1.ReleaseResponse, error) {
	return &inventoryv1.ReleaseResponse{Success: true}, nil
}

func (s *mockInventoryServer) GetStock(_ context.Context, _ *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	return &inventoryv1.GetStockResponse{Available: 100, Reserved: 0}, nil
}

type mockPaymentServer struct {
	paymentv1.UnimplementedPaymentServiceServer
}

func (s *mockPaymentServer) ProcessPayment(_ context.Context, _ *paymentv1.ProcessPaymentRequest) (*paymentv1.ProcessPaymentResponse, error) {
	return &paymentv1.ProcessPaymentResponse{PaymentId: uuid.New().String(), Status: "completed"}, nil
}

func (s *mockPaymentServer) Refund(_ context.Context, _ *paymentv1.RefundRequest) (*paymentv1.RefundResponse, error) {
	return &paymentv1.RefundResponse{Status: "refunded"}, nil
}

func startMockGRPCServer(t *testing.T, register func(*grpc.Server)) string {
	t.Helper()
	port := tests.GetFreePort(t)
	lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	register(srv)

	go func() {
		if err := srv.Serve(lis); err != nil {
			// expected on shutdown
		}
	}()

	t.Cleanup(func() {
		srv.GracefulStop()
	})

	return fmt.Sprintf("127.0.0.1:%d", port)
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
	return result
}

func TestOrderFlow(t *testing.T) {
	ctx := context.Background()

	// Start PostgreSQL and Elasticsearch
	dsn := tests.StartPostgres(ctx, t)

	esURL, cleanupES := tests.StartElasticsearch(ctx, t)
	defer cleanupES()

	// Run migrations
	tests.RunMigrations(ctx, t, dsn,
		"../../services/user-service/migrations",
		"../../services/catalog-service/migrations",
		"../../services/order-service/migrations",
	)

	// Start mock inventory and payment servers
	invAddr := startMockGRPCServer(t, func(s *grpc.Server) {
		inventoryv1.RegisterInventoryServiceServer(s, &mockInventoryServer{})
	})
	payAddr := startMockGRPCServer(t, func(s *grpc.Server) {
		paymentv1.RegisterPaymentServiceServer(s, &mockPaymentServer{})
	})

	// Start user-service
	userPort := tests.GetFreePort(t)
	tests.StartService(t, "../../services/user-service", []string{
		"POSTGRES_DSN=" + dsn,
		fmt.Sprintf("GRPC_PORT=%d", userPort),
		"JWT_SECRET=test-secret",
	})
	userAddr := fmt.Sprintf("127.0.0.1:%d", userPort)
	tests.WaitForGRPC(t, userAddr)

	// Start catalog-service
	catalogPort := tests.GetFreePort(t)
	tests.StartService(t, "../../services/catalog-service", []string{
		"POSTGRES_DSN=" + dsn,
		fmt.Sprintf("GRPC_PORT=%d", catalogPort),
		"ES_URL=" + esURL,
	})
	catalogAddr := fmt.Sprintf("127.0.0.1:%d", catalogPort)
	tests.WaitForGRPC(t, catalogAddr)

	// Start order-service
	orderPort := tests.GetFreePort(t)
	tests.StartService(t, "../../services/order-service", []string{
		"POSTGRES_DSN=" + dsn,
		fmt.Sprintf("GRPC_PORT=%d", orderPort),
		"INVENTORY_ADDR=" + invAddr,
		"PAYMENT_ADDR=" + payAddr,
		"JWT_SECRET=test-secret",
	})
	orderAddr := fmt.Sprintf("127.0.0.1:%d", orderPort)
	tests.WaitForGRPC(t, orderAddr)

	// Start API gateway
	gatewayPort := tests.GetFreePort(t)
	tests.StartService(t, "../../services/api-gateway", []string{
		fmt.Sprintf("USER_SERVICE_ADDR=%s", userAddr),
		fmt.Sprintf("CATALOG_SERVICE_ADDR=%s", catalogAddr),
		fmt.Sprintf("PORT=%d", gatewayPort),
	})
	gatewayURL := fmt.Sprintf("http://127.0.0.1:%d", gatewayPort)
	tests.WaitForHTTP(t, gatewayURL+"/query")

	// Step 1: Register user via API Gateway
	regResult := graphqlRequest(t, gatewayURL+"/query", `mutation { register(email: "flow@example.com", password: "password123", name: "Flow User") }`)
	data, ok := regResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("register returned no data: %v", regResult)
	}
	userID, ok := data["register"].(string)
	if !ok || userID == "" {
		t.Fatalf("expected user id, got: %v", data["register"])
	}

	// Step 2: Create product via API Gateway
	prodResult := graphqlRequest(t, gatewayURL+"/query", `mutation { createProduct(name: "Test Product", description: "A test product", price: 99.99, stock: 10, categories: ["test"]) }`)
	data, ok = prodResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("createProduct returned no data: %v", prodResult)
	}
	productID, ok := data["createProduct"].(string)
	if !ok || productID == "" {
		t.Fatalf("expected product id, got: %v", data["createProduct"])
	}

	// Step 3: Create order via order-service gRPC (not exposed via gateway)
	orderConn, err := grpc.NewClient(orderAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to order-service: %v", err)
	}
	defer orderConn.Close()

	orderClient := orderv1.NewOrderServiceClient(orderConn)
	createResp, err := orderClient.CreateOrder(ctx, &orderv1.CreateOrderRequest{
		UserId: userID,
		Items: []*orderv1.OrderItem{
			{
				ProductId: productID,
				Quantity:  2,
				Price:     99.99,
			},
		},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	if createResp.OrderId == "" {
		t.Fatal("expected order id")
	}

	// Step 4: Verify order status
	// The saga should complete with mock inventory/payment, resulting in "confirmed" status.
	// However, the saga runs asynchronously after CreateOrder returns.
	// We poll for the status to become "confirmed".
	getResp, err := orderClient.GetOrder(ctx, &orderv1.GetOrderRequest{
		OrderId: createResp.OrderId,
	})
	if err != nil {
		t.Fatalf("get order failed: %v", err)
	}

	// Poll for confirmed status with timeout
	deadline := time.Now().Add(5 * time.Second)
	for getResp.Order.Status != "confirmed" && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		getResp, err = orderClient.GetOrder(ctx, &orderv1.GetOrderRequest{
			OrderId: createResp.OrderId,
		})
		if err != nil {
			t.Fatalf("get order poll failed: %v", err)
		}
	}

	if getResp.Order.Status != "confirmed" {
		t.Fatalf("expected order status 'confirmed', got '%s'", getResp.Order.Status)
	}
	if getResp.Order.UserId != userID {
		t.Fatalf("expected order user_id %s, got %s", userID, getResp.Order.UserId)
	}
	if len(getResp.Order.Items) != 1 {
		t.Fatalf("expected 1 order item, got %d", len(getResp.Order.Items))
	}

	// Also verify product exists via catalog-service gRPC
	catalogConn, err := grpc.NewClient(catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to catalog-service: %v", err)
	}
	defer catalogConn.Close()

	catalogClient := catalogv1.NewCatalogServiceClient(catalogConn)
	prodResp, err := catalogClient.GetProduct(ctx, &catalogv1.GetProductRequest{
		ProductId: productID,
	})
	if err != nil {
		t.Fatalf("get product failed: %v", err)
	}
	if prodResp.Product.Name != "Test Product" {
		t.Fatalf("expected product name 'Test Product', got '%s'", prodResp.Product.Name)
	}
}
