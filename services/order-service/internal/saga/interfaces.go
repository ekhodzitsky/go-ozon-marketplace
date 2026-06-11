package saga

import "context"

type InventoryClient interface {
	Reserve(ctx context.Context, productID string, quantity int32, orderID string) error
	Release(ctx context.Context, productID string, quantity int32, orderID string) error
}

type PaymentClient interface {
	ProcessPayment(ctx context.Context, orderID, userID string, amount int64) (string, error)
	Refund(ctx context.Context, paymentID string) error
}
