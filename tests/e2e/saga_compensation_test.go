//go:build e2e

package e2e

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type failingInventoryServer struct {
	inventoryv1.UnimplementedInventoryServiceServer
}

func (s *failingInventoryServer) Reserve(_ context.Context, _ *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	return nil, errors.New("inventory reserve failed")
}

func (s *failingInventoryServer) Release(_ context.Context, _ *inventoryv1.ReleaseRequest) (*inventoryv1.ReleaseResponse, error) {
	return &inventoryv1.ReleaseResponse{Success: true}, nil
}

func (s *failingInventoryServer) GetStock(_ context.Context, _ *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	return &inventoryv1.GetStockResponse{Available: 100, Reserved: 0}, nil
}

type trackingPaymentServer struct {
	paymentv1.UnimplementedPaymentServiceServer
	processPaymentCalled atomic.Bool
}

func (s *trackingPaymentServer) ProcessPayment(_ context.Context, _ *paymentv1.ProcessPaymentRequest) (*paymentv1.ProcessPaymentResponse, error) {
	s.processPaymentCalled.Store(true)
	return &paymentv1.ProcessPaymentResponse{PaymentId: uuid.New().String(), Status: paymentv1.PaymentStatus_PAYMENT_STATUS_SUCCESS}, nil
}

func (s *trackingPaymentServer) Refund(_ context.Context, _ *paymentv1.RefundRequest) (*paymentv1.RefundResponse, error) {
	return &paymentv1.RefundResponse{Status: paymentv1.PaymentStatus_PAYMENT_STATUS_REFUNDED}, nil
}

type mockCatalogServer struct {
	catalogv1.UnimplementedCatalogServiceServer
}

func (s *mockCatalogServer) GetProduct(_ context.Context, req *catalogv1.GetProductRequest) (*catalogv1.GetProductResponse, error) {
	return &catalogv1.GetProductResponse{
		Product: &catalogv1.Product{
			ProductId:  req.GetProductId(),
			PriceCents: 4999,
		},
	}, nil
}

func authContext(ctx context.Context, userID, secret string) context.Context {
	now := time.Now().UTC()
	claims := middleware.CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
		Role: string(middleware.RoleUser),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		panic(fmt.Sprintf("failed to sign token: %v", err))
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tokenStr)
}

func TestSagaCompensation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}
	cases := []struct {
		name         string
		wantStatus   orderv1.OrderStatus
		wantPayment  bool
		pollTimeout  time.Duration
		pollInterval time.Duration
	}{
		{
			name:         "inventory_reserve_failure_triggers_compensation",
			wantStatus:   orderv1.OrderStatus_ORDER_STATUS_CANCELLED,
			wantPayment:  false,
			pollTimeout:  5 * time.Second,
			pollInterval: 200 * time.Millisecond,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Start PostgreSQL
			dsn := tests.StartPostgres(ctx, t)

			redisAddr := tests.StartRedis(ctx, t)
			kafkaAddr := tests.StartKafka(ctx, t)

			// Run migrations
			tests.RunMigrations(ctx, t, dsn, "../../services/order-service/migrations")

			// Start failing inventory mock
			invAddr := startMockGRPCServer(t, func(s *grpc.Server) {
				inventoryv1.RegisterInventoryServiceServer(s, &failingInventoryServer{})
			})

			// Start tracking payment mock
			payMock := &trackingPaymentServer{}
			payAddr := startMockGRPCServer(t, func(s *grpc.Server) {
				paymentv1.RegisterPaymentServiceServer(s, payMock)
			})

			// Start catalog mock (order-service validates item prices against catalog)
			catalogAddr := startMockGRPCServer(t, func(s *grpc.Server) {
				catalogv1.RegisterCatalogServiceServer(s, &mockCatalogServer{})
			})

			// Start order-service
			orderPort := tests.GetFreePort(t)
			orderMetricsPort := tests.GetFreePort(t)
			tests.StartService(t, "../../services/order-service", []string{
				"POSTGRES_DSN=" + dsn,
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

			// Connect to order-service
			orderConn, err := grpc.NewClient(orderAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				t.Fatalf("failed to connect to order-service: %v", err)
			}
			defer func() { _ = orderConn.Close() }()

			orderClient := orderv1.NewOrderServiceClient(orderConn)

			userID := uuid.New().String()
			productID := uuid.New().String()

			// Create order with auth
			createCtx := authContext(ctx, userID, e2eJWTSecret)
			createResp, err := orderClient.CreateOrder(createCtx, tests.NewCreateOrderRequestBuilder().
				AddItem(productID, 1, 4999).
				Build())
			if err != nil {
				t.Fatalf("create order failed: %v", err)
			}
			if createResp.OrderId == "" {
				t.Fatal("expected order id")
			}

			// Poll for expected status
			status := orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
			deadline := time.Now().Add(tt.pollTimeout)
			for time.Now().Before(deadline) {
				getCtx := authContext(ctx, userID, e2eJWTSecret)
				getResp, err := orderClient.GetOrder(getCtx, tests.NewGetOrderRequestBuilder().
					WithOrderID(createResp.OrderId).
					Build())
				if err != nil {
					t.Fatalf("get order failed: %v", err)
				}
				status = getResp.Order.Status
				if status == tt.wantStatus {
					break
				}
				time.Sleep(tt.pollInterval)
			}

			if status != tt.wantStatus {
				t.Fatalf("expected order status %v, got %v", tt.wantStatus, status)
			}

			// Verify payment was (not) processed
			if got := payMock.processPaymentCalled.Load(); got != tt.wantPayment {
				t.Fatalf("expected payment processed=%v, got %v", tt.wantPayment, got)
			}
		})
	}
}
