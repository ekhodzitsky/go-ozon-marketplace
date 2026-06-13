//go:build integration

package tests

import (
	"context"
	"fmt"
	"net"
	"testing"

	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type fakeCatalogServer struct {
	catalogv1.UnimplementedCatalogServiceServer
	price int64
}

func (f *fakeCatalogServer) GetProduct(_ context.Context, req *catalogv1.GetProductRequest) (*catalogv1.GetProductResponse, error) {
	return &catalogv1.GetProductResponse{
		Product: &catalogv1.Product{
			ProductId:  req.GetProductId(),
			Name:       "Test Product",
			PriceCents: f.price,
		},
	}, nil
}

type fakeInventoryServer struct {
	inventoryv1.UnimplementedInventoryServiceServer
}

func (f *fakeInventoryServer) Reserve(_ context.Context, _ *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	return &inventoryv1.ReserveResponse{Success: true}, nil
}

func (f *fakeInventoryServer) Release(_ context.Context, _ *inventoryv1.ReleaseRequest) (*inventoryv1.ReleaseResponse, error) {
	return &inventoryv1.ReleaseResponse{Success: true}, nil
}

func (f *fakeInventoryServer) GetStock(_ context.Context, _ *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	return &inventoryv1.GetStockResponse{Available: 1000, Reserved: 0}, nil
}

func (f *fakeInventoryServer) GetLedger(_ context.Context, _ *inventoryv1.GetLedgerRequest) (*inventoryv1.GetLedgerResponse, error) {
	return &inventoryv1.GetLedgerResponse{}, nil
}

type fakePaymentServer struct {
	paymentv1.UnimplementedPaymentServiceServer
}

func (f *fakePaymentServer) ProcessPayment(_ context.Context, req *paymentv1.ProcessPaymentRequest) (*paymentv1.ProcessPaymentResponse, error) {
	return &paymentv1.ProcessPaymentResponse{
		PaymentId: fmt.Sprintf("payment-%s", req.GetOrderId()),
		Status:    paymentv1.PaymentStatus_PAYMENT_STATUS_SUCCESS,
	}, nil
}

func (f *fakePaymentServer) Refund(_ context.Context, _ *paymentv1.RefundRequest) (*paymentv1.RefundResponse, error) {
	return &paymentv1.RefundResponse{Status: paymentv1.PaymentStatus_PAYMENT_STATUS_REFUNDED}, nil
}

func startFakeGRPCServer(t *testing.T, register func(*grpc.Server)) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen fake server: %v", err)
	}
	grpcServer := grpc.NewServer()
	register(grpcServer)

	t.Cleanup(func() {
		grpcServer.GracefulStop()
		_ = lis.Close()
	})

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Logf("fake server exited: %v", err)
		}
	}()

	return lis.Addr().String()
}

func TestOrderServicePostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()

	dsn := StartPostgres(ctx, t)
	redisAddr := StartRedis(ctx, t)
	kafkaAddr := StartKafka(ctx, t)
	catalogAddr := startFakeGRPCServer(t, func(s *grpc.Server) {
		catalogv1.RegisterCatalogServiceServer(s, &fakeCatalogServer{price: 1050})
	})
	inventoryAddr := startFakeGRPCServer(t, func(s *grpc.Server) {
		inventoryv1.RegisterInventoryServiceServer(s, &fakeInventoryServer{})
	})
	paymentAddr := startFakeGRPCServer(t, func(s *grpc.Server) {
		paymentv1.RegisterPaymentServiceServer(s, &fakePaymentServer{})
	})

	RunMigrations(ctx, t, dsn, "../services/order-service/migrations")

	jwtSecret := "this-is-a-very-long-test-secret-for-integration-tests-only"

	grpcPort := GetFreePort(t)
	metricsPort := GetFreePort(t)
	StartService(t, "../services/order-service", []string{
		"POSTGRES_DSN=" + dsn,
		"REDIS_ADDR=" + redisAddr,
		"KAFKA_BROKERS=" + kafkaAddr,
		fmt.Sprintf("GRPC_PORT=%d", grpcPort),
		fmt.Sprintf("METRICS_PORT=%d", metricsPort),
		"INVENTORY_ADDR=" + inventoryAddr,
		"PAYMENT_ADDR=" + paymentAddr,
		"CATALOG_ADDR=" + catalogAddr,
		"JWT_SECRET=" + jwtSecret,
	})

	addr := fmt.Sprintf("127.0.0.1:%d", grpcPort)
	WaitForGRPC(t, addr)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to order-service: %v", err)
	}
	defer func() { _ = conn.Close() }()

	client := orderv1.NewOrderServiceClient(conn)

	userID := uuid.New().String()
	productID := uuid.New().String()
	authCtx := AuthContext(ctx, userID, jwtSecret)

	// Test CreateOrder (full saga: inventory reserve + payment + confirm).
	createResp, err := client.CreateOrder(authCtx, NewCreateOrderRequestBuilder().
		AddItem(productID, 2, 1050).
		Build())
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	if createResp.OrderId == "" {
		t.Fatal("expected order id after create")
	}

	// Test GetOrder
	getResp, err := client.GetOrder(authCtx, NewGetOrderRequestBuilder().
		WithOrderID(createResp.OrderId).
		Build())
	if err != nil {
		t.Fatalf("get order failed: %v", err)
	}
	if getResp.Order.UserId != userID {
		t.Fatalf("expected user id %s, got %s", userID, getResp.Order.UserId)
	}
	if len(getResp.Order.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(getResp.Order.Items))
	}
	if getResp.Order.Items[0].Quantity != 2 {
		t.Fatalf("expected quantity 2, got %d", getResp.Order.Items[0].Quantity)
	}
	if getResp.Order.Items[0].PriceCents != 1050 {
		t.Fatalf("expected price 1050, got %d", getResp.Order.Items[0].PriceCents)
	}
	if getResp.Order.TotalAmountCents != 2100 {
		t.Fatalf("expected total amount 2100, got %d", getResp.Order.TotalAmountCents)
	}
	if getResp.Order.Status != orderv1.OrderStatus_ORDER_STATUS_PAID {
		t.Fatalf("expected status paid, got %v", getResp.Order.Status)
	}

	// Test ListOrders
	listResp, err := client.ListOrders(authCtx, NewListOrdersRequestBuilder().
		WithPage(1).
		WithPageSize(10).
		Build())
	if err != nil {
		t.Fatalf("list orders failed: %v", err)
	}
	if len(listResp.Orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(listResp.Orders))
	}
	if listResp.Total != 1 {
		t.Fatalf("expected total 1, got %d", listResp.Total)
	}
}
