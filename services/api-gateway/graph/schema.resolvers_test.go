package graph_test

import (
	"context"
	"errors"
	"testing"
	"time"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// --- manual mocks for gRPC clients ---

type mockOrderServiceClient struct {
	createOrderFunc func(ctx context.Context, req *orderv1.CreateOrderRequest, opts ...grpc.CallOption) (*orderv1.CreateOrderResponse, error)
	getOrderFunc    func(ctx context.Context, req *orderv1.GetOrderRequest, opts ...grpc.CallOption) (*orderv1.GetOrderResponse, error)
	listOrdersFunc  func(ctx context.Context, req *orderv1.ListOrdersRequest, opts ...grpc.CallOption) (*orderv1.ListOrdersResponse, error)
	cancelOrderFunc func(ctx context.Context, req *orderv1.CancelOrderRequest, opts ...grpc.CallOption) (*orderv1.CancelOrderResponse, error)
}

func (m *mockOrderServiceClient) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest, opts ...grpc.CallOption) (*orderv1.CreateOrderResponse, error) {
	return m.createOrderFunc(ctx, req, opts...)
}

func (m *mockOrderServiceClient) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest, opts ...grpc.CallOption) (*orderv1.GetOrderResponse, error) {
	return m.getOrderFunc(ctx, req, opts...)
}

func (m *mockOrderServiceClient) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest, opts ...grpc.CallOption) (*orderv1.ListOrdersResponse, error) {
	return m.listOrdersFunc(ctx, req, opts...)
}

func (m *mockOrderServiceClient) CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest, opts ...grpc.CallOption) (*orderv1.CancelOrderResponse, error) {
	return m.cancelOrderFunc(ctx, req, opts...)
}

func (m *mockOrderServiceClient) UpdateOrderStatus(ctx context.Context, req *orderv1.UpdateOrderStatusRequest, opts ...grpc.CallOption) (*orderv1.UpdateOrderStatusResponse, error) {
	return nil, errors.New("not implemented")
}

type mockInventoryServiceClient struct {
	getStockFunc  func(ctx context.Context, req *inventoryv1.GetStockRequest, opts ...grpc.CallOption) (*inventoryv1.GetStockResponse, error)
	getLedgerFunc func(ctx context.Context, req *inventoryv1.GetLedgerRequest, opts ...grpc.CallOption) (*inventoryv1.GetLedgerResponse, error)
}

func (m *mockInventoryServiceClient) Reserve(ctx context.Context, req *inventoryv1.ReserveRequest, opts ...grpc.CallOption) (*inventoryv1.ReserveResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockInventoryServiceClient) Release(ctx context.Context, req *inventoryv1.ReleaseRequest, opts ...grpc.CallOption) (*inventoryv1.ReleaseResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockInventoryServiceClient) GetStock(ctx context.Context, req *inventoryv1.GetStockRequest, opts ...grpc.CallOption) (*inventoryv1.GetStockResponse, error) {
	return m.getStockFunc(ctx, req, opts...)
}

func (m *mockInventoryServiceClient) GetLedger(ctx context.Context, req *inventoryv1.GetLedgerRequest, opts ...grpc.CallOption) (*inventoryv1.GetLedgerResponse, error) {
	if m.getLedgerFunc != nil {
		return m.getLedgerFunc(ctx, req, opts...)
	}
	return nil, errors.New("not implemented")
}

func newResolver() *graph.Resolver {
	return &graph.Resolver{
		CallTimeout:  5 * time.Second,
		QueryTimeout: 5 * time.Second,
	}
}

func TestMutationResolver_CreateOrder(t *testing.T) {
	t.Parallel()

	orderClient := &mockOrderServiceClient{
		createOrderFunc: func(_ context.Context, req *orderv1.CreateOrderRequest, _ ...grpc.CallOption) (*orderv1.CreateOrderResponse, error) {
			require.Len(t, req.Items, 1)
			assert.Equal(t, "product-1", req.Items[0].ProductId)
			return &orderv1.CreateOrderResponse{OrderId: "order-123"}, nil
		},
	}

	r := newResolver()
	r.OrderService = orderClient

	mut := r.Mutation()
	id, err := mut.CreateOrder(context.Background(), "user-1", []*model.OrderItemInput{
		{ProductID: "product-1", Quantity: 2, Price: 9.99},
	})
	require.NoError(t, err)
	assert.Equal(t, "order-123", id)
}

func TestMutationResolver_CancelOrder(t *testing.T) {
	t.Parallel()

	orderClient := &mockOrderServiceClient{
		cancelOrderFunc: func(_ context.Context, req *orderv1.CancelOrderRequest, _ ...grpc.CallOption) (*orderv1.CancelOrderResponse, error) {
			assert.Equal(t, "order-123", req.OrderId)
			return &orderv1.CancelOrderResponse{Success: true}, nil
		},
	}

	r := newResolver()
	r.OrderService = orderClient

	mut := r.Mutation()
	ok, err := mut.CancelOrder(context.Background(), "order-123")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestQueryResolver_Order(t *testing.T) {
	t.Parallel()

	orderClient := &mockOrderServiceClient{
		getOrderFunc: func(_ context.Context, req *orderv1.GetOrderRequest, _ ...grpc.CallOption) (*orderv1.GetOrderResponse, error) {
			assert.Equal(t, "order-123", req.OrderId)
			return &orderv1.GetOrderResponse{Order: &orderv1.Order{OrderId: "order-123", UserId: "user-1", Status: "pending"}}, nil
		},
	}

	r := newResolver()
	r.OrderService = orderClient

	q := r.Query()
	order, err := q.Order(context.Background(), "order-123")
	require.NoError(t, err)
	assert.Equal(t, "order-123", order.ID)
	assert.Equal(t, "pending", order.Status)
}

func TestQueryResolver_Orders(t *testing.T) {
	t.Parallel()

	orderClient := &mockOrderServiceClient{
		listOrdersFunc: func(_ context.Context, req *orderv1.ListOrdersRequest, _ ...grpc.CallOption) (*orderv1.ListOrdersResponse, error) {
			assert.Equal(t, "user-1", req.UserId)
			return &orderv1.ListOrdersResponse{
				Orders: []*orderv1.Order{{OrderId: "order-1", UserId: "user-1", Status: "delivered"}},
				Total:  1,
			}, nil
		},
	}

	r := newResolver()
	r.OrderService = orderClient

	q := r.Query()
	conn, err := q.Orders(context.Background(), "user-1", nil, nil)
	require.NoError(t, err)
	assert.Len(t, conn.Orders, 1)
	assert.Equal(t, "order-1", conn.Orders[0].ID)
	assert.Equal(t, int32(1), conn.Total)
}

func TestQueryResolver_Inventory(t *testing.T) {
	t.Parallel()

	invClient := &mockInventoryServiceClient{
		getStockFunc: func(_ context.Context, req *inventoryv1.GetStockRequest, _ ...grpc.CallOption) (*inventoryv1.GetStockResponse, error) {
			assert.Equal(t, "product-1", req.ProductId)
			return &inventoryv1.GetStockResponse{Available: 100, Reserved: 5}, nil
		},
	}

	r := newResolver()
	r.InventoryService = invClient

	q := r.Query()
	inv, err := q.Inventory(context.Background(), "product-1")
	require.NoError(t, err)
	assert.Equal(t, "product-1", inv.ProductID)
	assert.Equal(t, int32(100), inv.Available)
	assert.Equal(t, int32(5), inv.Reserved)
}
