package saga

import (
	"context"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"google.golang.org/grpc"
)

type InventoryClient interface {
	Reserve(ctx context.Context, in *inventoryv1.ReserveRequest, opts ...grpc.CallOption) (*inventoryv1.ReserveResponse, error)
	Release(ctx context.Context, in *inventoryv1.ReleaseRequest, opts ...grpc.CallOption) (*inventoryv1.ReleaseResponse, error)
}

type PaymentClient interface {
	ProcessPayment(ctx context.Context, in *paymentv1.ProcessPaymentRequest, opts ...grpc.CallOption) (*paymentv1.ProcessPaymentResponse, error)
}
