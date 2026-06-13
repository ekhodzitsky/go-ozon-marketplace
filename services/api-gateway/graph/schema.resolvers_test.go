package graph_test

import (
	"context"
	"errors"
	"testing"
	"time"

	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/abtesting"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
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

type mockCatalogServiceClient struct {
	createProductFunc  func(ctx context.Context, req *catalogv1.CreateProductRequest, opts ...grpc.CallOption) (*catalogv1.CreateProductResponse, error)
	getProductFunc     func(ctx context.Context, req *catalogv1.GetProductRequest, opts ...grpc.CallOption) (*catalogv1.GetProductResponse, error)
	searchProductsFunc func(ctx context.Context, req *catalogv1.SearchProductsRequest, opts ...grpc.CallOption) (*catalogv1.SearchProductsResponse, error)
}

func (m *mockCatalogServiceClient) CreateProduct(ctx context.Context, req *catalogv1.CreateProductRequest, opts ...grpc.CallOption) (*catalogv1.CreateProductResponse, error) {
	if m.createProductFunc != nil {
		return m.createProductFunc(ctx, req, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCatalogServiceClient) UpdateProduct(ctx context.Context, req *catalogv1.UpdateProductRequest, opts ...grpc.CallOption) (*catalogv1.UpdateProductResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockCatalogServiceClient) DeleteProduct(ctx context.Context, req *catalogv1.DeleteProductRequest, opts ...grpc.CallOption) (*catalogv1.DeleteProductResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockCatalogServiceClient) GetProduct(ctx context.Context, req *catalogv1.GetProductRequest, opts ...grpc.CallOption) (*catalogv1.GetProductResponse, error) {
	if m.getProductFunc != nil {
		return m.getProductFunc(ctx, req, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCatalogServiceClient) ListProducts(ctx context.Context, req *catalogv1.ListProductsRequest, opts ...grpc.CallOption) (*catalogv1.ListProductsResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockCatalogServiceClient) SearchProducts(ctx context.Context, req *catalogv1.SearchProductsRequest, opts ...grpc.CallOption) (*catalogv1.SearchProductsResponse, error) {
	if m.searchProductsFunc != nil {
		return m.searchProductsFunc(ctx, req, opts...)
	}
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

type mockUserServiceClient struct {
	registerFunc func(ctx context.Context, req *userv1.RegisterRequest, opts ...grpc.CallOption) (*userv1.RegisterResponse, error)
	loginFunc    func(ctx context.Context, req *userv1.LoginRequest, opts ...grpc.CallOption) (*userv1.LoginResponse, error)
	getUserFunc  func(ctx context.Context, req *userv1.GetUserRequest, opts ...grpc.CallOption) (*userv1.GetUserResponse, error)
}

func (m *mockUserServiceClient) Register(ctx context.Context, req *userv1.RegisterRequest, opts ...grpc.CallOption) (*userv1.RegisterResponse, error) {
	if m.registerFunc != nil {
		return m.registerFunc(ctx, req, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserServiceClient) Login(ctx context.Context, req *userv1.LoginRequest, opts ...grpc.CallOption) (*userv1.LoginResponse, error) {
	if m.loginFunc != nil {
		return m.loginFunc(ctx, req, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserServiceClient) GetUser(ctx context.Context, req *userv1.GetUserRequest, opts ...grpc.CallOption) (*userv1.GetUserResponse, error) {
	if m.getUserFunc != nil {
		return m.getUserFunc(ctx, req, opts...)
	}
	return nil, errors.New("not implemented")
}

type mockAnalyticsServiceClient struct {
	trackEventFunc       func(ctx context.Context, req *analyticsv1.TrackEventRequest, opts ...grpc.CallOption) (*analyticsv1.TrackEventResponse, error)
	getDailyRevenueFunc  func(ctx context.Context, req *analyticsv1.GetDailyRevenueRequest, opts ...grpc.CallOption) (*analyticsv1.GetDailyRevenueResponse, error)
	trackABTestEventFunc func(ctx context.Context, req *analyticsv1.TrackABTestEventRequest, opts ...grpc.CallOption) (*analyticsv1.TrackABTestEventResponse, error)
}

func (m *mockAnalyticsServiceClient) TrackEvent(ctx context.Context, req *analyticsv1.TrackEventRequest, opts ...grpc.CallOption) (*analyticsv1.TrackEventResponse, error) {
	if m.trackEventFunc != nil {
		return m.trackEventFunc(ctx, req, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockAnalyticsServiceClient) GetDailyRevenue(ctx context.Context, req *analyticsv1.GetDailyRevenueRequest, opts ...grpc.CallOption) (*analyticsv1.GetDailyRevenueResponse, error) {
	if m.getDailyRevenueFunc != nil {
		return m.getDailyRevenueFunc(ctx, req, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockAnalyticsServiceClient) TrackABTestEvent(ctx context.Context, req *analyticsv1.TrackABTestEventRequest, opts ...grpc.CallOption) (*analyticsv1.TrackABTestEventResponse, error) {
	if m.trackABTestEventFunc != nil {
		return m.trackABTestEventFunc(ctx, req, opts...)
	}
	return nil, errors.New("not implemented")
}

func userContext(userID string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, userID)
	ctx = context.WithValue(ctx, middleware.ContextKeyRole, string(middleware.RoleUser))
	return ctx
}

func adminContext(userID string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, userID)
	ctx = context.WithValue(ctx, middleware.ContextKeyRole, string(middleware.RoleAdmin))
	return ctx
}

func newResolver() *graph.Resolver {
	return &graph.Resolver{
		CallTimeout:  5 * time.Second,
		QueryTimeout: 5 * time.Second,
	}
}

func TestMutationResolver_CreateOrder(t *testing.T) {
	t.Parallel()

	catalogClient := &mockCatalogServiceClient{
		getProductFunc: func(_ context.Context, req *catalogv1.GetProductRequest, _ ...grpc.CallOption) (*catalogv1.GetProductResponse, error) {
			assert.Equal(t, "product-1", req.ProductId)
			return &catalogv1.GetProductResponse{Product: &catalogv1.Product{ProductId: "product-1", PriceCents: 999}}, nil
		},
	}

	orderClient := &mockOrderServiceClient{
		createOrderFunc: func(_ context.Context, req *orderv1.CreateOrderRequest, _ ...grpc.CallOption) (*orderv1.CreateOrderResponse, error) {
			require.Len(t, req.Items, 1)
			assert.Equal(t, "product-1", req.Items[0].ProductId)
			assert.Equal(t, int32(2), req.Items[0].Quantity)
			assert.Equal(t, int64(999), req.Items[0].PriceCents)
			assert.NotEmpty(t, req.IdempotencyKey)
			return &orderv1.CreateOrderResponse{OrderId: "order-123"}, nil
		},
	}

	r := newResolver()
	r.CatalogService = catalogClient
	r.OrderService = orderClient

	mut := r.Mutation()
	id, err := mut.CreateOrder(userContext("user-1"), []*model.OrderItemInput{
		{ProductID: "product-1", Quantity: 2, Price: 9.99},
	})
	require.NoError(t, err)
	assert.Equal(t, "order-123", id)
}

func TestMutationResolver_CreateOrder_Unauthenticated(t *testing.T) {
	t.Parallel()

	r := newResolver()
	mut := r.Mutation()
	_, err := mut.CreateOrder(context.Background(), []*model.OrderItemInput{
		{ProductID: "product-1", Quantity: 2, Price: 9.99},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthenticated")
}

func TestMutationResolver_CreateOrder_EmptyItems(t *testing.T) {
	t.Parallel()

	r := newResolver()
	mut := r.Mutation()
	_, err := mut.CreateOrder(userContext("user-1"), []*model.OrderItemInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid argument")
}

func TestMutationResolver_CreateOrder_PriceMismatch_Lower(t *testing.T) {
	t.Parallel()

	catalogClient := &mockCatalogServiceClient{
		getProductFunc: func(_ context.Context, req *catalogv1.GetProductRequest, _ ...grpc.CallOption) (*catalogv1.GetProductResponse, error) {
			return &catalogv1.GetProductResponse{Product: &catalogv1.Product{ProductId: req.ProductId, PriceCents: 999}}, nil
		},
	}

	r := newResolver()
	r.CatalogService = catalogClient

	mut := r.Mutation()
	_, err := mut.CreateOrder(userContext("user-1"), []*model.OrderItemInput{
		{ProductID: "product-1", Quantity: 1, Price: 8.99},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid argument")
	assert.Contains(t, err.Error(), "price mismatch")
}

func TestMutationResolver_CreateOrder_PriceMismatch_Higher(t *testing.T) {
	t.Parallel()

	catalogClient := &mockCatalogServiceClient{
		getProductFunc: func(_ context.Context, req *catalogv1.GetProductRequest, _ ...grpc.CallOption) (*catalogv1.GetProductResponse, error) {
			return &catalogv1.GetProductResponse{Product: &catalogv1.Product{ProductId: req.ProductId, PriceCents: 999}}, nil
		},
	}

	r := newResolver()
	r.CatalogService = catalogClient

	mut := r.Mutation()
	_, err := mut.CreateOrder(userContext("user-1"), []*model.OrderItemInput{
		{ProductID: "product-1", Quantity: 1, Price: 10.99},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid argument")
	assert.Contains(t, err.Error(), "price mismatch")
}

func TestMutationResolver_CreateOrder_ProductNotFound(t *testing.T) {
	t.Parallel()

	catalogClient := &mockCatalogServiceClient{
		getProductFunc: func(_ context.Context, req *catalogv1.GetProductRequest, _ ...grpc.CallOption) (*catalogv1.GetProductResponse, error) {
			return &catalogv1.GetProductResponse{Product: nil}, nil
		},
	}

	r := newResolver()
	r.CatalogService = catalogClient

	mut := r.Mutation()
	_, err := mut.CreateOrder(userContext("user-1"), []*model.OrderItemInput{
		{ProductID: "missing", Quantity: 1, Price: 9.99},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid argument")
	assert.Contains(t, err.Error(), "not found")
}

func TestMutationResolver_CancelOrder(t *testing.T) {
	t.Parallel()

	orderClient := &mockOrderServiceClient{
		getOrderFunc: func(_ context.Context, req *orderv1.GetOrderRequest, _ ...grpc.CallOption) (*orderv1.GetOrderResponse, error) {
			assert.Equal(t, "order-123", req.OrderId)
			return &orderv1.GetOrderResponse{Order: &orderv1.Order{OrderId: "order-123", UserId: "user-1", Status: orderv1.OrderStatus_ORDER_STATUS_PENDING}}, nil
		},
		cancelOrderFunc: func(_ context.Context, req *orderv1.CancelOrderRequest, _ ...grpc.CallOption) (*orderv1.CancelOrderResponse, error) {
			assert.Equal(t, "order-123", req.OrderId)
			return &orderv1.CancelOrderResponse{Success: true}, nil
		},
	}

	r := newResolver()
	r.OrderService = orderClient

	mut := r.Mutation()
	ok, err := mut.CancelOrder(userContext("user-1"), "order-123")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestMutationResolver_CancelOrder_Forbidden(t *testing.T) {
	t.Parallel()

	orderClient := &mockOrderServiceClient{
		getOrderFunc: func(_ context.Context, req *orderv1.GetOrderRequest, _ ...grpc.CallOption) (*orderv1.GetOrderResponse, error) {
			return &orderv1.GetOrderResponse{Order: &orderv1.Order{OrderId: "order-123", UserId: "user-2", Status: orderv1.OrderStatus_ORDER_STATUS_PENDING}}, nil
		},
	}

	r := newResolver()
	r.OrderService = orderClient

	mut := r.Mutation()
	_, err := mut.CancelOrder(userContext("user-1"), "order-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestQueryResolver_Order(t *testing.T) {
	t.Parallel()

	orderClient := &mockOrderServiceClient{
		getOrderFunc: func(_ context.Context, req *orderv1.GetOrderRequest, _ ...grpc.CallOption) (*orderv1.GetOrderResponse, error) {
			assert.Equal(t, "order-123", req.OrderId)
			return &orderv1.GetOrderResponse{Order: &orderv1.Order{OrderId: "order-123", UserId: "user-1", Status: orderv1.OrderStatus_ORDER_STATUS_PENDING, TotalAmountCents: 1998}}, nil
		},
	}

	r := newResolver()
	r.OrderService = orderClient

	q := r.Query()
	order, err := q.Order(userContext("user-1"), "order-123")
	require.NoError(t, err)
	assert.Equal(t, "order-123", order.ID)
	assert.Equal(t, "pending", order.Status)
	assert.InDelta(t, 19.98, order.TotalAmount, 0.001)
}

func TestQueryResolver_Order_Forbidden(t *testing.T) {
	t.Parallel()

	orderClient := &mockOrderServiceClient{
		getOrderFunc: func(_ context.Context, req *orderv1.GetOrderRequest, _ ...grpc.CallOption) (*orderv1.GetOrderResponse, error) {
			return &orderv1.GetOrderResponse{Order: &orderv1.Order{OrderId: "order-123", UserId: "user-2", Status: orderv1.OrderStatus_ORDER_STATUS_PENDING}}, nil
		},
	}

	r := newResolver()
	r.OrderService = orderClient

	q := r.Query()
	_, err := q.Order(userContext("user-1"), "order-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestQueryResolver_Orders(t *testing.T) {
	t.Parallel()

	orderClient := &mockOrderServiceClient{
		listOrdersFunc: func(_ context.Context, req *orderv1.ListOrdersRequest, _ ...grpc.CallOption) (*orderv1.ListOrdersResponse, error) {
			return &orderv1.ListOrdersResponse{
				Orders: []*orderv1.Order{{OrderId: "order-1", UserId: "user-1", Status: orderv1.OrderStatus_ORDER_STATUS_DELIVERED, TotalAmountCents: 1000}},
				Total:  1,
			}, nil
		},
	}

	r := newResolver()
	r.OrderService = orderClient

	q := r.Query()
	conn, err := q.Orders(userContext("user-1"), "user-1", nil, nil)
	require.NoError(t, err)
	assert.Len(t, conn.Orders, 1)
	assert.Equal(t, "order-1", conn.Orders[0].ID)
	assert.InDelta(t, 10.0, conn.Orders[0].TotalAmount, 0.001)
	assert.Equal(t, int32(1), conn.Total)
}

func TestQueryResolver_Orders_Forbidden(t *testing.T) {
	t.Parallel()

	r := newResolver()
	q := r.Query()
	_, err := q.Orders(userContext("user-1"), "user-2", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
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

func TestMutationResolver_CreateProduct(t *testing.T) {
	t.Parallel()

	catalogClient := &mockCatalogServiceClient{
		createProductFunc: func(_ context.Context, req *catalogv1.CreateProductRequest, _ ...grpc.CallOption) (*catalogv1.CreateProductResponse, error) {
			assert.Equal(t, "Widget", req.Name)
			assert.Equal(t, int64(999), req.PriceCents)
			assert.NotEmpty(t, req.IdempotencyKey)
			return &catalogv1.CreateProductResponse{ProductId: "product-123"}, nil
		},
	}

	r := newResolver()
	r.CatalogService = catalogClient

	mut := r.Mutation()
	id, err := mut.CreateProduct(adminContext("admin-1"), "Widget", "A useful widget", 9.99, []string{"gadgets"})
	require.NoError(t, err)
	assert.Equal(t, "product-123", id)
}

func TestMutationResolver_CreateProduct_ForbiddenForUser(t *testing.T) {
	t.Parallel()

	r := newResolver()
	mut := r.Mutation()
	_, err := mut.CreateProduct(userContext("user-1"), "Widget", "A useful widget", 9.99, []string{"gadgets"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestMutationResolver_Register(t *testing.T) {
	t.Parallel()

	userClient := &mockUserServiceClient{
		registerFunc: func(_ context.Context, req *userv1.RegisterRequest, _ ...grpc.CallOption) (*userv1.RegisterResponse, error) {
			assert.Equal(t, "test@example.com", req.Email)
			assert.Equal(t, "password123", req.Password)
			assert.Equal(t, "Test User", req.Name)
			return &userv1.RegisterResponse{UserId: "user-123"}, nil
		},
	}

	r := newResolver()
	r.UserService = userClient

	mut := r.Mutation()
	id, err := mut.Register(context.Background(), "test@example.com", "password123", "Test User")
	require.NoError(t, err)
	assert.Equal(t, "user-123", id)
}

func TestMutationResolver_Register_InvalidInput(t *testing.T) {
	t.Parallel()

	r := newResolver()
	mut := r.Mutation()

	_, err := mut.Register(context.Background(), "not-an-email", "password123", "Test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input")

	_, err = mut.Register(context.Background(), "test@example.com", "short", "Test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input")
}

func TestMutationResolver_Login(t *testing.T) {
	t.Parallel()

	userClient := &mockUserServiceClient{
		loginFunc: func(_ context.Context, req *userv1.LoginRequest, _ ...grpc.CallOption) (*userv1.LoginResponse, error) {
			assert.Equal(t, "test@example.com", req.Email)
			assert.Equal(t, "password123", req.Password)
			return &userv1.LoginResponse{Token: "jwt-token"}, nil
		},
	}

	r := newResolver()
	r.UserService = userClient

	mut := r.Mutation()
	token, err := mut.Login(context.Background(), "test@example.com", "password123")
	require.NoError(t, err)
	assert.Equal(t, "jwt-token", token)
}

func TestMutationResolver_CreateProduct_InvalidPrice(t *testing.T) {
	t.Parallel()

	r := newResolver()
	mut := r.Mutation()
	_, err := mut.CreateProduct(adminContext("admin-1"), "Widget", "desc", -5, []string{"gadgets"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input")
}

func TestMutationResolver_CreateOrder_InvalidQuantity(t *testing.T) {
	t.Parallel()

	r := newResolver()
	mut := r.Mutation()
	_, err := mut.CreateOrder(userContext("user-1"), []*model.OrderItemInput{
		{ProductID: "product-1", Quantity: 0, Price: 9.99},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input")
}

func TestMutationResolver_CreateOrder_InvalidPrice(t *testing.T) {
	t.Parallel()

	r := newResolver()
	mut := r.Mutation()
	_, err := mut.CreateOrder(userContext("user-1"), []*model.OrderItemInput{
		{ProductID: "product-1", Quantity: 1, Price: 0},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input")
}

func TestQueryResolver_Me(t *testing.T) {
	t.Parallel()

	userClient := &mockUserServiceClient{
		getUserFunc: func(_ context.Context, _ *userv1.GetUserRequest, _ ...grpc.CallOption) (*userv1.GetUserResponse, error) {
			return &userv1.GetUserResponse{UserId: "user-1", Email: "test@example.com", Name: "Test", CreatedAt: "2024-01-01T00:00:00Z"}, nil
		},
	}

	r := newResolver()
	r.UserService = userClient

	q := r.Query()
	user, err := q.Me(userContext("user-1"))
	require.NoError(t, err)
	assert.Equal(t, "user-1", user.ID)
	assert.Equal(t, "test@example.com", user.Email)
}

func TestQueryResolver_User_AdminCanViewOthers(t *testing.T) {
	t.Parallel()

	userClient := &mockUserServiceClient{
		getUserFunc: func(_ context.Context, _ *userv1.GetUserRequest, _ ...grpc.CallOption) (*userv1.GetUserResponse, error) {
			return &userv1.GetUserResponse{UserId: "user-2", Email: "other@example.com", Name: "Other"}, nil
		},
	}

	r := newResolver()
	r.UserService = userClient

	q := r.Query()
	user, err := q.User(adminContext("admin-1"), "user-2")
	require.NoError(t, err)
	assert.Equal(t, "user-2", user.ID)
}

func TestQueryResolver_Product(t *testing.T) {
	t.Parallel()

	catalogClient := &mockCatalogServiceClient{
		getProductFunc: func(_ context.Context, req *catalogv1.GetProductRequest, _ ...grpc.CallOption) (*catalogv1.GetProductResponse, error) {
			assert.Equal(t, "product-1", req.ProductId)
			return &catalogv1.GetProductResponse{Product: &catalogv1.Product{ProductId: "product-1", Name: "Widget", PriceCents: 999, Categories: []string{"gadgets"}, CreatedAt: "2024-01-01T00:00:00Z"}}, nil
		},
	}

	r := newResolver()
	r.CatalogService = catalogClient

	q := r.Query()
	product, err := q.Product(context.Background(), "product-1")
	require.NoError(t, err)
	assert.Equal(t, "product-1", product.ID)
	assert.InDelta(t, 9.99, product.Price, 0.001)
}

func TestQueryResolver_SearchProducts(t *testing.T) {
	t.Parallel()

	catalogClient := &mockCatalogServiceClient{
		searchProductsFunc: func(_ context.Context, req *catalogv1.SearchProductsRequest, _ ...grpc.CallOption) (*catalogv1.SearchProductsResponse, error) {
			assert.Equal(t, "widget", req.Query)
			assert.Equal(t, int32(1), req.Page)
			assert.Equal(t, int32(10), req.PageSize)
			return &catalogv1.SearchProductsResponse{
				Products: []*catalogv1.Product{{ProductId: "product-1", Name: "Widget", PriceCents: 999}},
				Total:    1,
			}, nil
		},
	}

	r := newResolver()
	r.CatalogService = catalogClient

	q := r.Query()
	page := int32(1)
	pageSize := int32(10)
	conn, err := q.SearchProducts(context.Background(), "widget", &page, &pageSize)
	require.NoError(t, err)
	assert.Len(t, conn.Products, 1)
	assert.Equal(t, int32(1), conn.Total)
}

func TestQueryResolver_SearchProducts_InvalidPageSize(t *testing.T) {
	t.Parallel()

	r := newResolver()
	q := r.Query()
	pageSize := int32(200)
	_, err := q.SearchProducts(context.Background(), "widget", nil, &pageSize)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input")
}

func TestQueryResolver_FeatureFlags(t *testing.T) {
	t.Parallel()

	engine := featureflags.NewEngine(nil)
	engine.Register(&featureflags.Flag{Name: "new-checkout-flow", Enabled: true, Strategy: "default"})

	r := newResolver()
	r.FeatureFlagsEngine = engine

	q := r.Query()
	flags, err := q.FeatureFlags(userContext("user-1"))
	require.NoError(t, err)
	assert.True(t, flags.NewCheckoutFlow)
}

func TestQueryResolver_AbTestAssignments(t *testing.T) {
	t.Parallel()

	r := newResolver()
	r.ABExperiments = []*abtesting.Experiment{
		{Name: "checkout-button-color", Variations: []abtesting.Variation{{Name: "control", Weight: 100}}},
	}

	q := r.Query()
	assignments, err := q.AbTestAssignments(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Len(t, assignments, 1)
	assert.Equal(t, "checkout-button-color", assignments[0].Experiment)
	assert.Equal(t, "control", assignments[0].Variation)
}

func TestMutationResolver_CreateOrder_TracksABTest(t *testing.T) {
	t.Parallel()

	catalogClient := &mockCatalogServiceClient{
		getProductFunc: func(_ context.Context, req *catalogv1.GetProductRequest, _ ...grpc.CallOption) (*catalogv1.GetProductResponse, error) {
			return &catalogv1.GetProductResponse{Product: &catalogv1.Product{ProductId: req.ProductId, PriceCents: 999}}, nil
		},
	}

	orderClient := &mockOrderServiceClient{
		createOrderFunc: func(_ context.Context, _ *orderv1.CreateOrderRequest, _ ...grpc.CallOption) (*orderv1.CreateOrderResponse, error) {
			return &orderv1.CreateOrderResponse{OrderId: "order-123"}, nil
		},
	}

	tracked := make(chan *analyticsv1.TrackABTestEventRequest, 1)
	analyticsClient := &mockAnalyticsServiceClient{
		trackABTestEventFunc: func(_ context.Context, req *analyticsv1.TrackABTestEventRequest, _ ...grpc.CallOption) (*analyticsv1.TrackABTestEventResponse, error) {
			tracked <- req
			return &analyticsv1.TrackABTestEventResponse{}, nil
		},
	}

	r := newResolver()
	r.CatalogService = catalogClient
	r.OrderService = orderClient
	r.AnalyticsService = analyticsClient
	r.ABExperiments = []*abtesting.Experiment{
		{Name: "checkout-button-color", Variations: []abtesting.Variation{{Name: "control", Weight: 100}}},
	}

	mut := r.Mutation()
	_, err := mut.CreateOrder(userContext("user-1"), []*model.OrderItemInput{
		{ProductID: "product-1", Quantity: 1, Price: 9.99},
	})
	require.NoError(t, err)

	select {
	case req := <-tracked:
		assert.Equal(t, "checkout-button-color", req.Experiment)
		assert.Equal(t, "user-1", req.UserId)
		assert.True(t, req.Conversion)
	case <-time.After(2 * time.Second):
		t.Fatal("expected A/B test event to be tracked")
	}
}
