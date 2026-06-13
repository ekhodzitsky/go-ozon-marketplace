//go:build chaos

package chaos

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
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
		resp, err := orderClient.CreateOrder(authContext(ctx, userID), tests.NewCreateOrderRequestBuilder().
			AddItem(productID, 1, 4999).
			Build())
		if err != nil {
			t.Logf("create order errored as expected during chaos: %v", err)
			return
		}
		orderID = resp.OrderId
	}()

	// Give the saga time to reach payment step, then kill order-service
	time.Sleep(600 * time.Millisecond)
	dockerKill(t, containerName("order-service"))
	// Also kill payment-service so recovery cannot complete payment and must compensate
	dockerKill(t, containerName("payment-service"))

	// Wait for goroutine to finish
	time.Sleep(1 * time.Second)

	// Restart services and wait for them to be running
	dockerStart(t, containerName("order-service"))
	dockerStart(t, containerName("payment-service"))
	waitForContainer(t, containerName("order-service"), 30*time.Second)
	waitForContainer(t, containerName("payment-service"), 30*time.Second)

	// Wait for recovery worker to run (interval 5s) plus compensation
	time.Sleep(8 * time.Second)

	// If orderID was not returned due to early kill, find the latest order for user
	if orderID == "" {
		listResp, err := orderClient.ListOrders(authContext(ctx, userID), tests.NewListOrdersRequestBuilder().
			WithPage(1).
			WithPageSize(10).
			Build())
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

	getResp, err := orderClient.GetOrder(authContext(ctx, userID), tests.NewGetOrderRequestBuilder().
		WithOrderID(orderID).
		Build())
	if err != nil {
		t.Fatalf("failed to get order: %v", err)
	}

	if getResp.Order.Status != orderv1.OrderStatus_ORDER_STATUS_CANCELLED {
		t.Fatalf("expected order status cancelled, got %v", getResp.Order.Status)
	}

	// Verify inventory released
	stockResp, err := invClient.GetStock(serviceAuthContext(ctx), tests.NewGetStockRequestBuilder().
		WithProductID(productID).
		Build())
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
	dockerKill(t, containerName("inventory-service"))

	_, err := orderClient.CreateOrder(authContext(ctx, userID), tests.NewCreateOrderRequestBuilder().
		AddItem(productID, 1, 4999).
		Build())
	if err == nil {
		t.Fatal("expected create order to fail when inventory is down")
	}

	// Restart inventory-service and wait for it to be running
	dockerStart(t, containerName("inventory-service"))
	waitForContainer(t, containerName("inventory-service"), 30*time.Second)
	time.Sleep(3 * time.Second)

	// Verify order was cancelled (created in DB then compensated)
	listResp, err := orderClient.ListOrders(authContext(ctx, userID), tests.NewListOrdersRequestBuilder().
		WithPage(1).
		WithPageSize(10).
		Build())
	if err != nil {
		t.Fatalf("failed to list orders: %v", err)
	}

	if len(listResp.Orders) == 0 {
		// Order might not have been committed if failure happened before commit
		// In this case we just verify no hanging reservations
	} else {
		if listResp.Orders[0].Status != orderv1.OrderStatus_ORDER_STATUS_CANCELLED {
			t.Fatalf("expected order status cancelled, got %v", listResp.Orders[0].Status)
		}
	}

	// Verify no hanging reservations
	stockResp, err := invClient.GetStock(serviceAuthContext(ctx), tests.NewGetStockRequestBuilder().
		WithProductID(productID).
		Build())
	if err != nil {
		t.Fatalf("failed to get stock: %v", err)
	}
	if stockResp.Reserved != 0 {
		t.Fatalf("expected no hanging reservations, got reserved=%d", stockResp.Reserved)
	}
}
