package postgres

import (
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/txmanager"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/repository"
	"github.com/stretchr/testify/assert"
)

// TestNewPaymentTxManager_UsesGenericManager documents the seam:
// the local constructor is a thin adapter that returns the generic
// txmanager.Manager[repository.PaymentRepository] directly.
func TestNewPaymentTxManager_UsesGenericManager(t *testing.T) {
	repo := NewPaymentPostgres(nil)

	tm := NewPaymentTxManager(nil, repo)

	assert.NotNil(t, tm)
	assert.IsType(t, (*txmanager.Manager[repository.PaymentRepository])(nil), tm)
	var _ repository.TxManager = tm
}
