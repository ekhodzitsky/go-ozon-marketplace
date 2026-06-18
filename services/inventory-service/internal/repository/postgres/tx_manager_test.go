package postgres

import (
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/txmanager"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/repository"
	"github.com/stretchr/testify/assert"
)

// TestNewInventoryTxManager_UsesGenericManager documents the seam:
// the local constructor is a thin adapter that returns the generic
// txmanager.Manager[repository.InventoryRepository] directly.
func TestNewInventoryTxManager_UsesGenericManager(t *testing.T) {
	repo := NewInventoryPostgres(nil)

	tm := NewInventoryTxManager(nil, repo)

	assert.NotNil(t, tm)
	assert.IsType(t, (*txmanager.Manager[repository.InventoryRepository])(nil), tm)
	var _ repository.TxManager = tm
}
