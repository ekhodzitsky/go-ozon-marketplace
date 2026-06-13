package saga

import "context"

type InventoryClient interface {
	Reserve(ctx context.Context, productID string, quantity int32, orderID string, idempotencyKey string) error
	Release(ctx context.Context, productID string, quantity int32, orderID string, idempotencyKey string) error
}

type PaymentClient interface {
	ProcessPayment(ctx context.Context, orderID string, amountCents int64, idempotencyKey string) (string, error)
	Refund(ctx context.Context, paymentID string, idempotencyKey string) error
}
