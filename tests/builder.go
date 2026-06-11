package tests

import (
	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	notificationv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/notification/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
)

// --- Order builders ---

type OrderRequestBuilder struct {
	userID string
	items  []*orderv1.OrderItem
}

func NewOrderRequestBuilder() *OrderRequestBuilder {
	return &OrderRequestBuilder{
		items: make([]*orderv1.OrderItem, 0),
	}
}

func (b *OrderRequestBuilder) WithUserID(id string) *OrderRequestBuilder {
	b.userID = id
	return b
}

func (b *OrderRequestBuilder) AddItem(productID string, quantity int32, price float64) *OrderRequestBuilder {
	b.items = append(b.items, &orderv1.OrderItem{
		ProductId: productID,
		Quantity:  quantity,
		Price:     price,
	})
	return b
}

func (b *OrderRequestBuilder) Build() *orderv1.CreateOrderRequest {
	return &orderv1.CreateOrderRequest{
		UserId: b.userID,
		Items:  b.items,
	}
}

type GetOrderRequestBuilder struct {
	orderID string
}

func NewGetOrderRequestBuilder() *GetOrderRequestBuilder {
	return &GetOrderRequestBuilder{}
}

func (b *GetOrderRequestBuilder) WithOrderID(id string) *GetOrderRequestBuilder {
	b.orderID = id
	return b
}

func (b *GetOrderRequestBuilder) Build() *orderv1.GetOrderRequest {
	return &orderv1.GetOrderRequest{OrderId: b.orderID}
}

type ListOrdersRequestBuilder struct {
	userID   string
	page     int32
	pageSize int32
}

func NewListOrdersRequestBuilder() *ListOrdersRequestBuilder {
	return &ListOrdersRequestBuilder{page: 1, pageSize: 10}
}

func (b *ListOrdersRequestBuilder) WithUserID(id string) *ListOrdersRequestBuilder {
	b.userID = id
	return b
}

func (b *ListOrdersRequestBuilder) WithPage(page int32) *ListOrdersRequestBuilder {
	b.page = page
	return b
}

func (b *ListOrdersRequestBuilder) WithPageSize(size int32) *ListOrdersRequestBuilder {
	b.pageSize = size
	return b
}

func (b *ListOrdersRequestBuilder) Build() *orderv1.ListOrdersRequest {
	return &orderv1.ListOrdersRequest{
		UserId:   b.userID,
		Page:     b.page,
		PageSize: b.pageSize,
	}
}

// --- User builders ---

type UserRequestBuilder struct {
	email    string
	password string
	name     string
}

func NewUserRequestBuilder() *UserRequestBuilder {
	return &UserRequestBuilder{}
}

func (b *UserRequestBuilder) WithEmail(email string) *UserRequestBuilder {
	b.email = email
	return b
}

func (b *UserRequestBuilder) WithPassword(password string) *UserRequestBuilder {
	b.password = password
	return b
}

func (b *UserRequestBuilder) WithName(name string) *UserRequestBuilder {
	b.name = name
	return b
}

func (b *UserRequestBuilder) BuildRegister() *userv1.RegisterRequest {
	return &userv1.RegisterRequest{
		Email:    b.email,
		Password: b.password,
		Name:     b.name,
	}
}

func (b *UserRequestBuilder) BuildLogin() *userv1.LoginRequest {
	return &userv1.LoginRequest{
		Email:    b.email,
		Password: b.password,
	}
}

type GetUserRequestBuilder struct {
	userID string
}

func NewGetUserRequestBuilder() *GetUserRequestBuilder {
	return &GetUserRequestBuilder{}
}

func (b *GetUserRequestBuilder) WithUserID(id string) *GetUserRequestBuilder {
	b.userID = id
	return b
}

func (b *GetUserRequestBuilder) Build() *userv1.GetUserRequest {
	return &userv1.GetUserRequest{UserId: b.userID}
}

// --- Payment builders ---

type ProcessPaymentRequestBuilder struct {
	orderID string
	userID  string
	amount  float64
}

func NewProcessPaymentRequestBuilder() *ProcessPaymentRequestBuilder {
	return &ProcessPaymentRequestBuilder{}
}

func (b *ProcessPaymentRequestBuilder) WithOrderID(id string) *ProcessPaymentRequestBuilder {
	b.orderID = id
	return b
}

func (b *ProcessPaymentRequestBuilder) WithUserID(id string) *ProcessPaymentRequestBuilder {
	b.userID = id
	return b
}

func (b *ProcessPaymentRequestBuilder) WithAmount(amount float64) *ProcessPaymentRequestBuilder {
	b.amount = amount
	return b
}

func (b *ProcessPaymentRequestBuilder) Build() *paymentv1.ProcessPaymentRequest {
	return &paymentv1.ProcessPaymentRequest{
		OrderId: b.orderID,
		UserId:  b.userID,
		Amount:  b.amount,
	}
}

type RefundRequestBuilder struct {
	paymentID string
}

func NewRefundRequestBuilder() *RefundRequestBuilder {
	return &RefundRequestBuilder{}
}

func (b *RefundRequestBuilder) WithPaymentID(id string) *RefundRequestBuilder {
	b.paymentID = id
	return b
}

func (b *RefundRequestBuilder) Build() *paymentv1.RefundRequest {
	return &paymentv1.RefundRequest{PaymentId: b.paymentID}
}

// --- Catalog builders ---

type CreateProductRequestBuilder struct {
	name        string
	description string
	price       float64
	categories  []string
}

func NewCreateProductRequestBuilder() *CreateProductRequestBuilder {
	return &CreateProductRequestBuilder{}
}

func (b *CreateProductRequestBuilder) WithName(name string) *CreateProductRequestBuilder {
	b.name = name
	return b
}

func (b *CreateProductRequestBuilder) WithDescription(desc string) *CreateProductRequestBuilder {
	b.description = desc
	return b
}

func (b *CreateProductRequestBuilder) WithPrice(price float64) *CreateProductRequestBuilder {
	b.price = price
	return b
}

func (b *CreateProductRequestBuilder) WithCategories(cats []string) *CreateProductRequestBuilder {
	b.categories = cats
	return b
}

func (b *CreateProductRequestBuilder) Build() *catalogv1.CreateProductRequest {
	return &catalogv1.CreateProductRequest{
		Name:        b.name,
		Description: b.description,
		Price:       b.price,
		Categories:  b.categories,
	}
}

type GetProductRequestBuilder struct {
	productID string
}

func NewGetProductRequestBuilder() *GetProductRequestBuilder {
	return &GetProductRequestBuilder{}
}

func (b *GetProductRequestBuilder) WithProductID(id string) *GetProductRequestBuilder {
	b.productID = id
	return b
}

func (b *GetProductRequestBuilder) Build() *catalogv1.GetProductRequest {
	return &catalogv1.GetProductRequest{ProductId: b.productID}
}

type ListProductsRequestBuilder struct {
	page     int32
	pageSize int32
}

func NewListProductsRequestBuilder() *ListProductsRequestBuilder {
	return &ListProductsRequestBuilder{page: 1, pageSize: 10}
}

func (b *ListProductsRequestBuilder) WithPage(page int32) *ListProductsRequestBuilder {
	b.page = page
	return b
}

func (b *ListProductsRequestBuilder) WithPageSize(size int32) *ListProductsRequestBuilder {
	b.pageSize = size
	return b
}

func (b *ListProductsRequestBuilder) Build() *catalogv1.ListProductsRequest {
	return &catalogv1.ListProductsRequest{Page: b.page, PageSize: b.pageSize}
}

type SearchProductsRequestBuilder struct {
	query    string
	page     int32
	pageSize int32
}

func NewSearchProductsRequestBuilder() *SearchProductsRequestBuilder {
	return &SearchProductsRequestBuilder{page: 1, pageSize: 10}
}

func (b *SearchProductsRequestBuilder) WithQuery(query string) *SearchProductsRequestBuilder {
	b.query = query
	return b
}

func (b *SearchProductsRequestBuilder) WithPage(page int32) *SearchProductsRequestBuilder {
	b.page = page
	return b
}

func (b *SearchProductsRequestBuilder) WithPageSize(size int32) *SearchProductsRequestBuilder {
	b.pageSize = size
	return b
}

func (b *SearchProductsRequestBuilder) Build() *catalogv1.SearchProductsRequest {
	return &catalogv1.SearchProductsRequest{Query: b.query, Page: b.page, PageSize: b.pageSize}
}

// --- Inventory builders ---

type ReserveRequestBuilder struct {
	productID string
	orderID   string
	quantity  int32
}

func NewReserveRequestBuilder() *ReserveRequestBuilder {
	return &ReserveRequestBuilder{}
}

func (b *ReserveRequestBuilder) WithProductID(id string) *ReserveRequestBuilder {
	b.productID = id
	return b
}

func (b *ReserveRequestBuilder) WithOrderID(id string) *ReserveRequestBuilder {
	b.orderID = id
	return b
}

func (b *ReserveRequestBuilder) WithQuantity(qty int32) *ReserveRequestBuilder {
	b.quantity = qty
	return b
}

func (b *ReserveRequestBuilder) Build() *inventoryv1.ReserveRequest {
	return &inventoryv1.ReserveRequest{
		ProductId: b.productID,
		OrderId:   b.orderID,
		Quantity:  b.quantity,
	}
}

type ReleaseRequestBuilder struct {
	productID string
	orderID   string
	quantity  int32
}

func NewReleaseRequestBuilder() *ReleaseRequestBuilder {
	return &ReleaseRequestBuilder{}
}

func (b *ReleaseRequestBuilder) WithProductID(id string) *ReleaseRequestBuilder {
	b.productID = id
	return b
}

func (b *ReleaseRequestBuilder) WithOrderID(id string) *ReleaseRequestBuilder {
	b.orderID = id
	return b
}

func (b *ReleaseRequestBuilder) WithQuantity(qty int32) *ReleaseRequestBuilder {
	b.quantity = qty
	return b
}

func (b *ReleaseRequestBuilder) Build() *inventoryv1.ReleaseRequest {
	return &inventoryv1.ReleaseRequest{
		ProductId: b.productID,
		OrderId:   b.orderID,
		Quantity:  b.quantity,
	}
}

type GetStockRequestBuilder struct {
	productID string
}

func NewGetStockRequestBuilder() *GetStockRequestBuilder {
	return &GetStockRequestBuilder{}
}

func (b *GetStockRequestBuilder) WithProductID(id string) *GetStockRequestBuilder {
	b.productID = id
	return b
}

func (b *GetStockRequestBuilder) Build() *inventoryv1.GetStockRequest {
	return &inventoryv1.GetStockRequest{ProductId: b.productID}
}

// --- Analytics builders ---

type TrackEventRequestBuilder struct {
	eventType      string
	aggregateID    string
	payload        string
	aggregationKey string
}

func NewTrackEventRequestBuilder() *TrackEventRequestBuilder {
	return &TrackEventRequestBuilder{}
}

func (b *TrackEventRequestBuilder) WithEventType(et string) *TrackEventRequestBuilder {
	b.eventType = et
	return b
}

func (b *TrackEventRequestBuilder) WithAggregateID(id string) *TrackEventRequestBuilder {
	b.aggregateID = id
	return b
}

func (b *TrackEventRequestBuilder) WithPayload(payload string) *TrackEventRequestBuilder {
	b.payload = payload
	return b
}

func (b *TrackEventRequestBuilder) WithAggregationKey(key string) *TrackEventRequestBuilder {
	b.aggregationKey = key
	return b
}

func (b *TrackEventRequestBuilder) Build() *analyticsv1.TrackEventRequest {
	return &analyticsv1.TrackEventRequest{
		EventType:      b.eventType,
		AggregateId:    b.aggregateID,
		Payload:        b.payload,
		AggregationKey: b.aggregationKey,
	}
}

type GetDailyRevenueRequestBuilder struct {
	date string
}

func NewGetDailyRevenueRequestBuilder() *GetDailyRevenueRequestBuilder {
	return &GetDailyRevenueRequestBuilder{}
}

func (b *GetDailyRevenueRequestBuilder) WithDate(date string) *GetDailyRevenueRequestBuilder {
	b.date = date
	return b
}

func (b *GetDailyRevenueRequestBuilder) Build() *analyticsv1.GetDailyRevenueRequest {
	return &analyticsv1.GetDailyRevenueRequest{Date: b.date}
}

// --- Notification builders ---

type SendEmailRequestBuilder struct {
	to      string
	subject string
	body    string
}

func NewSendEmailRequestBuilder() *SendEmailRequestBuilder {
	return &SendEmailRequestBuilder{}
}

func (b *SendEmailRequestBuilder) WithTo(to string) *SendEmailRequestBuilder {
	b.to = to
	return b
}

func (b *SendEmailRequestBuilder) WithSubject(subject string) *SendEmailRequestBuilder {
	b.subject = subject
	return b
}

func (b *SendEmailRequestBuilder) WithBody(body string) *SendEmailRequestBuilder {
	b.body = body
	return b
}

func (b *SendEmailRequestBuilder) Build() *notificationv1.SendEmailRequest {
	return &notificationv1.SendEmailRequest{
		To:      b.to,
		Subject: b.subject,
		Body:    b.body,
	}
}

// UpdateProduct builder

type UpdateProductRequestBuilder struct {
	productID   string
	name        string
	description string
	price       float64
	categories  []string
}

func NewUpdateProductRequestBuilder() *UpdateProductRequestBuilder {
	return &UpdateProductRequestBuilder{}
}

func (b *UpdateProductRequestBuilder) WithProductID(id string) *UpdateProductRequestBuilder {
	b.productID = id
	return b
}

func (b *UpdateProductRequestBuilder) WithName(name string) *UpdateProductRequestBuilder {
	b.name = name
	return b
}

func (b *UpdateProductRequestBuilder) WithDescription(desc string) *UpdateProductRequestBuilder {
	b.description = desc
	return b
}

func (b *UpdateProductRequestBuilder) WithPrice(price float64) *UpdateProductRequestBuilder {
	b.price = price
	return b
}

func (b *UpdateProductRequestBuilder) WithCategories(cats []string) *UpdateProductRequestBuilder {
	b.categories = cats
	return b
}

func (b *UpdateProductRequestBuilder) Build() *catalogv1.UpdateProductRequest {
	return &catalogv1.UpdateProductRequest{
		ProductId:   b.productID,
		Name:        b.name,
		Description: b.description,
		Price:       b.price,
		Categories:  b.categories,
	}
}

// DeleteProduct builder

type DeleteProductRequestBuilder struct {
	productID string
}

func NewDeleteProductRequestBuilder() *DeleteProductRequestBuilder {
	return &DeleteProductRequestBuilder{}
}

func (b *DeleteProductRequestBuilder) WithProductID(id string) *DeleteProductRequestBuilder {
	b.productID = id
	return b
}

func (b *DeleteProductRequestBuilder) Build() *catalogv1.DeleteProductRequest {
	return &catalogv1.DeleteProductRequest{ProductId: b.productID}
}
