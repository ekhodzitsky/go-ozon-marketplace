//go:build chaos

package chaos

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	"github.com/google/uuid"
)

func TestSagaWithOrderServiceKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	dockerComposeUp(t)
	runMigrations(t)

	ctx := context.Background()
	orderClient := newOrderClient(t)
	invClient := newInventoryClient(t)

	userID := uuid.New().String()
	productID := uuid.New().String()

	// Ensure product stock exists
	ensureStock(t, productID, 100)

	// Create order in a goroutine so we can kill the container mid-flight
	var orderID string
	var created atomic.Bool

	go func() {
		defer created.Store(true)
		resp, err := orderClient.CreateOrder(authContext(ctx, userID), &orderv1.CreateOrderRequest{
			UserId: userID,
			Items: []*orderv1.OrderItem{
				{ProductId: productID, Quantity: 1, Price: 49.99},
			},
		})
		if err != nil {
			t.Logf("create order errored as expected during chaos: %v", err)
			return
		}
		orderID = resp.OrderId
	}()

	// Give the saga time to reach payment step, then kill order-service
	time.Sleep(600 * time.Millisecond)
	dockerKill(t, "go-ozon-marketplace-order-service-1")
	// Also kill payment-service so recovery cannot complete payment and must compensate
	dockerKill(t, "go-ozon-marketplace-payment-service-1")

	// Wait for goroutine to finish
	time.Sleep(1 * time.Second)

	// Restart services
	dockerStart(t, "go-ozon-marketplace-order-service-1")
	dockerStart(t, "go-ozon-marketplace-payment-service-1")

	// Wait for recovery worker to run (interval 5s) plus compensation
	time.Sleep(8 * time.Second)

	// If orderID was not returned due to early kill, find the latest order for user
	if orderID == "" {
		listResp, err := orderClient.ListOrders(authContext(ctx, userID), &orderv1.ListOrdersRequest{
			UserId:   userID,
			Page:     1,
			PageSize: 10,
		})
		if err != nil {
			t.Fatalf("failed to list orders: %v", err)
		}
		if len(listResp.Orders) > 0 {
			orderID = listResp.Orders[0].OrderId
		}
	}

	if orderID == "" {
		t.Fatal("no order found after recovery")
	}

	getResp, err := orderClient.GetOrder(authContext(ctx, userID), &orderv1.GetOrderRequest{OrderId: orderID})
	if err != nil {
		t.Fatalf("failed to get order: %v", err)
	}

	if getResp.Order.Status != "cancelled" {
		t.Fatalf("expected order status cancelled, got %s", getResp.Order.Status)
	}

	// Verify inventory released
	stockResp, err := invClient.GetStock(serviceAuthContext(ctx), &inventoryv1.GetStockRequest{ProductId: productID})
	if err != nil {
		t.Fatalf("failed to get stock: %v", err)
	}
	if stockResp.Reserved != 0 {
		t.Fatalf("expected reserved=0, got %d", stockResp.Reserved)
	}
	if stockResp.Available != 100 {
		t.Fatalf("expected available=100, got %d", stockResp.Available)
	}
}

func TestSagaWithInventoryServiceDown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	dockerComposeUp(t)
	runMigrations(t)

	ctx := context.Background()
	orderClient := newOrderClient(t)
	invClient := newInventoryClient(t)

	userID := uuid.New().String()
	productID := uuid.New().String()

	ensureStock(t, productID, 100)

	// Kill inventory-service before creating order
	dockerKill(t, "go-ozon-marketplace-inventory-service-1")

	_, err := orderClient.CreateOrder(authContext(ctx, userID), &orderv1.CreateOrderRequest{
		UserId: userID,
		Items: []*orderv1.OrderItem{
			{ProductId: productID, Quantity: 1, Price: 49.99},
		},
	})
	if err == nil {
		t.Fatal("expected create order to fail when inventory is down")
	}

	// Restart inventory-service
	dockerStart(t, "go-ozon-marketplace-inventory-service-1")
	time.Sleep(3 * time.Second)

	// Verify order was cancelled (created in DB then compensated)
	listResp, err := orderClient.ListOrders(authContext(ctx, userID), &orderv1.ListOrdersRequest{
		UserId:   userID,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("failed to list orders: %v", err)
	}

	if len(listResp.Orders) == 0 {
		// Order might not have been committed if failure happened before commit
		// In this case we just verify no hanging reservations
	} else {
		if listResp.Orders[0].Status != "cancelled" {
			t.Fatalf("expected order status cancelled, got %s", listResp.Orders[0].Status)
		}
	}

	// Verify no hanging reservations
	stockResp, err := invClient.GetStock(serviceAuthContext(ctx), &inventoryv1.GetStockRequest{ProductId: productID})
	if err != nil {
		t.Fatalf("failed to get stock: %v", err)
	}
	if stockResp.Reserved != 0 {
		t.Fatalf("expected no hanging reservations, got reserved=%d", stockResp.Reserved)
	}
}
