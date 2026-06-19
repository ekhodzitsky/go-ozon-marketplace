package store

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/google/uuid"
)

// Store is the seam that insulates saga persistence logic from the concrete
// repository implementation. It exposes a narrow, deep interface that the
// orchestrator can leverage without depending on repository details.
type Store interface {
	Create(ctx context.Context, saga *domain.Saga) error
	GetByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.Saga, error)
	Save(ctx context.Context, saga *domain.Saga) error
	ListIncomplete(ctx context.Context, limit int) ([]domain.Saga, error)
}
