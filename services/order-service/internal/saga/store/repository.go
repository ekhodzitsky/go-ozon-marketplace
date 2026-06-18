package store

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/google/uuid"
)

// RepositoryStore is an adapter that exposes repository.SagaRepository through
// the Store seam. It keeps repository locality inside the store module so the
// orchestrator stays shallow.
type RepositoryStore struct {
	repo repository.SagaRepository
}

// NewRepositoryStore constructs a Store adapter from a SagaRepository.
func NewRepositoryStore(repo repository.SagaRepository) *RepositoryStore {
	return &RepositoryStore{repo: repo}
}

var _ Store = (*RepositoryStore)(nil)

// Create persists a new saga.
func (s *RepositoryStore) Create(ctx context.Context, saga *domain.Saga) error {
	return s.repo.Create(ctx, saga)
}

// GetByOrderID loads a saga by order identifier.
func (s *RepositoryStore) GetByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.Saga, error) {
	return s.repo.GetByOrderID(ctx, orderID)
}

// Save updates an existing saga.
func (s *RepositoryStore) Save(ctx context.Context, saga *domain.Saga) error {
	return s.repo.Save(ctx, saga)
}

// ListIncomplete returns up to limit sagas that are not in a terminal state.
func (s *RepositoryStore) ListIncomplete(ctx context.Context, limit int) ([]domain.Saga, error) {
	return s.repo.ListIncomplete(ctx, limit)
}
