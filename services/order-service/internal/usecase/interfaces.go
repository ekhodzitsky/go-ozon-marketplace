package usecase

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/google/uuid"
)

// OrderUsecase defines the order use-case boundary.
type OrderUsecase interface {
	CreateOrder(ctx context.Context, userID uuid.UUID, items []domain.OrderItem) (uuid.UUID, error)
	GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error)
	ListOrders(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]domain.Order, int, error)
}
