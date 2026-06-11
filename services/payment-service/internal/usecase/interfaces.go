package usecase

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/domain"
	"github.com/google/uuid"
)

// PaymentUsecase defines the payment use-case boundary.
type PaymentUsecase interface {
	ProcessPayment(ctx context.Context, orderID, userID uuid.UUID, amount int64) (*domain.Payment, error)
	Refund(ctx context.Context, paymentID uuid.UUID) (*domain.Payment, error)
	GetByID(ctx context.Context, paymentID uuid.UUID) (*domain.Payment, error)
	GetRefund(ctx context.Context, refundID uuid.UUID) (*domain.Refund, error)
	ListRefunds(ctx context.Context, paymentID uuid.UUID) ([]*domain.Refund, error)
}
