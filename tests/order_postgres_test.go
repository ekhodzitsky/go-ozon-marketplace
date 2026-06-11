//go:build integration

package tests

import (
	"context"
	"fmt"
	"testing"

	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestOrderServicePostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()

	dsn := StartPostgres(ctx, t)

	RunMigrations(ctx, t, dsn, "../services/order-service/migrations")

	grpcPort := GetFreePort(t)
	StartService(t, "../services/order-service", []string{
		"POSTGRES_DSN=" + dsn,
		fmt.Sprintf("GRPC_PORT=%d", grpcPort),
		"INVENTORY_ADDR=127.0.0.1:1",
		"PAYMENT_ADDR=127.0.0.1:1",
		"JWT_SECRET=test-secret",
	})

	addr := fmt.Sprintf("127.0.0.1:%d", grpcPort)
	WaitForGRPC(t, addr)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to order-service: %v", err)
	}
	defer conn.Close()

	client := orderv1.NewOrderServiceClient(conn)

	userID := uuid.New().String()
	productID := uuid.New().String()

	// Test CreateOrder
	createResp, err := client.CreateOrder(ctx, &orderv1.CreateOrderRequest{
		UserId: userID,
		Items: []*orderv1.OrderItem{
			{
				ProductId: productID,
				Quantity:  2,
				Price:     10.5,
			},
		},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	if createResp.OrderId == "" {
		t.Fatal("expected order id after create")
	}

	// Test GetOrder
	getResp, err := client.GetOrder(ctx, &orderv1.GetOrderRequest{
		OrderId: createResp.OrderId,
	})
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
	if getResp.Order.Items[0].Price != 1050.0 {
		t.Fatalf("expected price 1050.0, got %f", getResp.Order.Items[0].Price)
	}

	// Test ListOrders
	listResp, err := client.ListOrders(ctx, &orderv1.ListOrdersRequest{
		UserId:   userID,
		Page:     1,
		PageSize: 10,
	})
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
