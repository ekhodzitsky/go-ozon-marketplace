package tests

import (
	"fmt"

	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	notificationv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/notification/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/google/uuid"
)

func ensureIdempotencyKey(key string) string {
	if key != "" {
		return key
	}
	return uuid.New().String()
}

// --- Order builders ---

type CreateOrderRequestBuilder struct {
	items          []*orderv1.OrderItem
	idempotencyKey string
}

func NewCreateOrderRequestBuilder() *CreateOrderRequestBuilder {
	return &CreateOrderRequestBuilder{
		items: make([]*orderv1.OrderItem, 0),
	}
}

func (b *CreateOrderRequestBuilder) AddItem(productID string, quantity int32, priceCents int64) *CreateOrderRequestBuilder {
	b.items = append(b.items, &orderv1.OrderItem{
		ProductId:  productID,
		Quantity:   quantity,
		PriceCents: priceCents,
	})
	return b
}

func (b *CreateOrderRequestBuilder) WithIdempotencyKey(key string) *CreateOrderRequestBuilder {
	b.idempotencyKey = key
	return b
}

func (b *CreateOrderRequestBuilder) Build() *orderv1.CreateOrderRequest {
	if len(b.items) == 0 {
		panic("CreateOrderRequest: at least one item is required")
	}
	for i, item := range b.items {
		if item.ProductId == "" {
			panic(fmt.Sprintf("CreateOrderRequest: item %d product_id is required", i))
		}
		if item.Quantity <= 0 {
			panic(fmt.Sprintf("CreateOrderRequest: item %d quantity must be positive", i))
		}
		if item.PriceCents <= 0 {
			panic(fmt.Sprintf("CreateOrderRequest: item %d price_cents must be positive", i))
		}
	}
	return &orderv1.CreateOrderRequest{
		Items:          b.items,
		IdempotencyKey: ensureIdempotencyKey(b.idempotencyKey),
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
	if b.orderID == "" {
		panic("GetOrderRequest: order_id is required")
	}
	return &orderv1.GetOrderRequest{OrderId: b.orderID}
}

type ListOrdersRequestBuilder struct {
	page     int32
	pageSize int32
}

func NewListOrdersRequestBuilder() *ListOrdersRequestBuilder {
	return &ListOrdersRequestBuilder{page: 1, pageSize: 10}
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
	if b.page <= 0 {
		panic("ListOrdersRequest: page must be positive")
	}
	if b.pageSize <= 0 {
		panic("ListOrdersRequest: page_size must be positive")
	}
	return &orderv1.ListOrdersRequest{
		Page:     b.page,
		PageSize: b.pageSize,
	}
}

type UpdateOrderStatusRequestBuilder struct {
	orderID string
	status  orderv1.OrderStatus
}

func NewUpdateOrderStatusRequestBuilder() *UpdateOrderStatusRequestBuilder {
	return &UpdateOrderStatusRequestBuilder{}
}

func (b *UpdateOrderStatusRequestBuilder) WithOrderID(id string) *UpdateOrderStatusRequestBuilder {
	b.orderID = id
	return b
}

func (b *UpdateOrderStatusRequestBuilder) WithStatus(status orderv1.OrderStatus) *UpdateOrderStatusRequestBuilder {
	b.status = status
	return b
}

func (b *UpdateOrderStatusRequestBuilder) Build() *orderv1.UpdateOrderStatusRequest {
	if b.orderID == "" {
		panic("UpdateOrderStatusRequest: order_id is required")
	}
	if b.status == orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED {
		panic("UpdateOrderStatusRequest: status is required")
	}
	return &orderv1.UpdateOrderStatusRequest{
		OrderId: b.orderID,
		Status:  b.status,
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
	if b.email == "" {
		panic("RegisterRequest: email is required")
	}
	if b.password == "" {
		panic("RegisterRequest: password is required")
	}
	if b.name == "" {
		panic("RegisterRequest: name is required")
	}
	return &userv1.RegisterRequest{
		Email:    b.email,
		Password: b.password,
		Name:     b.name,
	}
}

func (b *UserRequestBuilder) BuildLogin() *userv1.LoginRequest {
	if b.email == "" {
		panic("LoginRequest: email is required")
	}
	if b.password == "" {
		panic("LoginRequest: password is required")
	}
	return &userv1.LoginRequest{
		Email:    b.email,
		Password: b.password,
	}
}

type GetUserRequestBuilder struct{}

func NewGetUserRequestBuilder() *GetUserRequestBuilder {
	return &GetUserRequestBuilder{}
}

func (b *GetUserRequestBuilder) Build() *userv1.GetUserRequest {
	return &userv1.GetUserRequest{}
}

// --- Payment builders ---

type ProcessPaymentRequestBuilder struct {
	orderID        string
	amountCents    int64
	idempotencyKey string
}

func NewProcessPaymentRequestBuilder() *ProcessPaymentRequestBuilder {
	return &ProcessPaymentRequestBuilder{}
}

func (b *ProcessPaymentRequestBuilder) WithOrderID(id string) *ProcessPaymentRequestBuilder {
	b.orderID = id
	return b
}

func (b *ProcessPaymentRequestBuilder) WithAmountCents(amountCents int64) *ProcessPaymentRequestBuilder {
	b.amountCents = amountCents
	return b
}

func (b *ProcessPaymentRequestBuilder) WithIdempotencyKey(key string) *ProcessPaymentRequestBuilder {
	b.idempotencyKey = key
	return b
}

func (b *ProcessPaymentRequestBuilder) Build() *paymentv1.ProcessPaymentRequest {
	if b.orderID == "" {
		panic("ProcessPaymentRequest: order_id is required")
	}
	if b.amountCents <= 0 {
		panic("ProcessPaymentRequest: amount_cents must be positive")
	}
	return &paymentv1.ProcessPaymentRequest{
		OrderId:        b.orderID,
		AmountCents:    b.amountCents,
		IdempotencyKey: ensureIdempotencyKey(b.idempotencyKey),
	}
}

type RefundRequestBuilder struct {
	paymentID      string
	amountCents    int64
	idempotencyKey string
}

func NewRefundRequestBuilder() *RefundRequestBuilder {
	return &RefundRequestBuilder{}
}

func (b *RefundRequestBuilder) WithPaymentID(id string) *RefundRequestBuilder {
	b.paymentID = id
	return b
}

func (b *RefundRequestBuilder) WithAmountCents(amount int64) *RefundRequestBuilder {
	b.amountCents = amount
	return b
}

func (b *RefundRequestBuilder) WithIdempotencyKey(key string) *RefundRequestBuilder {
	b.idempotencyKey = key
	return b
}

func (b *RefundRequestBuilder) Build() *paymentv1.RefundRequest {
	if b.paymentID == "" {
		panic("RefundRequest: payment_id is required")
	}
	return &paymentv1.RefundRequest{
		PaymentId:      b.paymentID,
		AmountCents:    b.amountCents,
		IdempotencyKey: ensureIdempotencyKey(b.idempotencyKey),
	}
}

// --- Catalog builders ---

type CreateProductRequestBuilder struct {
	name           string
	description    string
	priceCents     int64
	categories     []string
	idempotencyKey string
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

func (b *CreateProductRequestBuilder) WithPriceCents(priceCents int64) *CreateProductRequestBuilder {
	b.priceCents = priceCents
	return b
}

func (b *CreateProductRequestBuilder) WithCategories(cats []string) *CreateProductRequestBuilder {
	b.categories = cats
	return b
}

func (b *CreateProductRequestBuilder) WithIdempotencyKey(key string) *CreateProductRequestBuilder {
	b.idempotencyKey = key
	return b
}

func (b *CreateProductRequestBuilder) Build() *catalogv1.CreateProductRequest {
	if b.name == "" {
		panic("CreateProductRequest: name is required")
	}
	if b.priceCents <= 0 {
		panic("CreateProductRequest: price_cents must be positive")
	}
	return &catalogv1.CreateProductRequest{
		Name:           b.name,
		Description:    b.description,
		PriceCents:     b.priceCents,
		Categories:     b.categories,
		IdempotencyKey: ensureIdempotencyKey(b.idempotencyKey),
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
	if b.productID == "" {
		panic("GetProductRequest: product_id is required")
	}
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
	if b.page <= 0 {
		panic("ListProductsRequest: page must be positive")
	}
	if b.pageSize <= 0 {
		panic("ListProductsRequest: page_size must be positive")
	}
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
	if b.query == "" {
		panic("SearchProductsRequest: query is required")
	}
	if b.page <= 0 {
		panic("SearchProductsRequest: page must be positive")
	}
	if b.pageSize <= 0 {
		panic("SearchProductsRequest: page_size must be positive")
	}
	return &catalogv1.SearchProductsRequest{Query: b.query, Page: b.page, PageSize: b.pageSize}
}

type UpdateProductRequestBuilder struct {
	productID   string
	name        *string
	description *string
	priceCents  *int64
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
	b.name = &name
	return b
}

func (b *UpdateProductRequestBuilder) WithDescription(desc string) *UpdateProductRequestBuilder {
	b.description = &desc
	return b
}

func (b *UpdateProductRequestBuilder) WithPriceCents(priceCents int64) *UpdateProductRequestBuilder {
	b.priceCents = &priceCents
	return b
}

func (b *UpdateProductRequestBuilder) WithCategories(cats []string) *UpdateProductRequestBuilder {
	b.categories = cats
	return b
}

func (b *UpdateProductRequestBuilder) Build() *catalogv1.UpdateProductRequest {
	if b.productID == "" {
		panic("UpdateProductRequest: product_id is required")
	}
	return &catalogv1.UpdateProductRequest{
		ProductId:   b.productID,
		Name:        b.name,
		Description: b.description,
		PriceCents:  b.priceCents,
		Categories:  b.categories,
	}
}

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
	if b.productID == "" {
		panic("DeleteProductRequest: product_id is required")
	}
	return &catalogv1.DeleteProductRequest{ProductId: b.productID}
}

// --- Inventory builders ---

type ReserveRequestBuilder struct {
	productID      string
	orderID        string
	quantity       int32
	idempotencyKey string
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

func (b *ReserveRequestBuilder) WithIdempotencyKey(key string) *ReserveRequestBuilder {
	b.idempotencyKey = key
	return b
}

func (b *ReserveRequestBuilder) Build() *inventoryv1.ReserveRequest {
	if b.productID == "" {
		panic("ReserveRequest: product_id is required")
	}
	if b.orderID == "" {
		panic("ReserveRequest: order_id is required")
	}
	if b.quantity <= 0 {
		panic("ReserveRequest: quantity must be positive")
	}
	return &inventoryv1.ReserveRequest{
		ProductId:      b.productID,
		OrderId:        b.orderID,
		Quantity:       b.quantity,
		IdempotencyKey: ensureIdempotencyKey(b.idempotencyKey),
	}
}

type ReleaseRequestBuilder struct {
	productID      string
	orderID        string
	quantity       int32
	idempotencyKey string
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

func (b *ReleaseRequestBuilder) WithIdempotencyKey(key string) *ReleaseRequestBuilder {
	b.idempotencyKey = key
	return b
}

func (b *ReleaseRequestBuilder) Build() *inventoryv1.ReleaseRequest {
	if b.productID == "" {
		panic("ReleaseRequest: product_id is required")
	}
	if b.orderID == "" {
		panic("ReleaseRequest: order_id is required")
	}
	if b.quantity <= 0 {
		panic("ReleaseRequest: quantity must be positive")
	}
	return &inventoryv1.ReleaseRequest{
		ProductId:      b.productID,
		OrderId:        b.orderID,
		Quantity:       b.quantity,
		IdempotencyKey: ensureIdempotencyKey(b.idempotencyKey),
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
	if b.productID == "" {
		panic("GetStockRequest: product_id is required")
	}
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
	if b.eventType == "" {
		panic("TrackEventRequest: event_type is required")
	}
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
	if b.date == "" {
		panic("GetDailyRevenueRequest: date is required")
	}
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
	if b.to == "" {
		panic("SendEmailRequest: to is required")
	}
	if b.subject == "" {
		panic("SendEmailRequest: subject is required")
	}
	return &notificationv1.SendEmailRequest{
		To:      b.to,
		Subject: b.subject,
		Body:    b.body,
	}
}
