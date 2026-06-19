package saga

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/store"
)

// NewRepositoryStore is the adapter constructor for the Store seam. It wraps
// the repository in a small, focused module so the orchestrator does not need
// to know about repository details.
func NewRepositoryStore(repo repository.SagaRepository) Store {
	return store.NewRepositoryStore(repo)
}
