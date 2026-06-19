package repository

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PaymentRepository interface {
	Create(ctx context.Context, payment *domain.Payment) error
	CreateOrGet(ctx context.Context, payment *domain.Payment) (*domain.Payment, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status) error
	UpdateStatusIf(ctx context.Context, id uuid.UUID, newStatus, expectedStatus domain.Status) (bool, error)
	CreateRefund(ctx context.Context, refund *domain.Refund) error
	GetRefund(ctx context.Context, id uuid.UUID) (*domain.Refund, error)
	GetRefundByIdempotencyKey(ctx context.Context, key string) (*domain.Refund, error)
	ListRefunds(ctx context.Context, paymentID uuid.UUID) ([]*domain.Refund, error)
	WithTx(tx pgx.Tx) PaymentRepository
}

type TxManager interface {
	Run(ctx context.Context, fn func(repo PaymentRepository) error) error
}
