//go:build e2e

package e2e

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
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

func authContext(ctx context.Context, userID, secret string) context.Context {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
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
		wantStatus   string
		wantPayment  bool
		pollTimeout  time.Duration
		pollInterval time.Duration
	}{
		{
			name:         "inventory_reserve_failure_triggers_compensation",
			wantStatus:   "cancelled",
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

			jwtSecret := "test-secret"

			// Start order-service
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

			// Connect to order-service
			orderConn, err := grpc.NewClient(orderAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				t.Fatalf("failed to connect to order-service: %v", err)
			}
			defer orderConn.Close()

			orderClient := orderv1.NewOrderServiceClient(orderConn)

			userID := uuid.New().String()
			productID := uuid.New().String()

			// Create order with auth
			createCtx := authContext(ctx, userID, jwtSecret)
			createResp, err := orderClient.CreateOrder(createCtx, &orderv1.CreateOrderRequest{
				UserId: userID,
				Items: []*orderv1.OrderItem{
					{
						ProductId: productID,
						Quantity:  1,
						Price:     49.99,
					},
				},
			})
			if err != nil {
				t.Fatalf("create order failed: %v", err)
			}
			if createResp.OrderId == "" {
				t.Fatal("expected order id")
			}

			// Poll for expected status
			var status string
			deadline := time.Now().Add(tt.pollTimeout)
			for time.Now().Before(deadline) {
				getCtx := authContext(ctx, userID, jwtSecret)
				getResp, err := orderClient.GetOrder(getCtx, &orderv1.GetOrderRequest{
					OrderId: createResp.OrderId,
				})
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
				t.Fatalf("expected order status %q, got %q", tt.wantStatus, status)
			}

			// Verify payment was (not) processed
			if got := payMock.processPaymentCalled.Load(); got != tt.wantPayment {
				t.Fatalf("expected payment processed=%v, got %v", tt.wantPayment, got)
			}
		})
	}
}
