package domain_test

import (
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderStatus_CanTransition_SameStatus(t *testing.T) {
	assert.True(t, domain.OrderStatusPending.CanTransition(domain.OrderStatusPending))
}

func TestOrderStatus_CanTransition_Valid(t *testing.T) {
	assert.True(t, domain.OrderStatusPending.CanTransition(domain.OrderStatusAwaitingPayment))
	assert.True(t, domain.OrderStatusPending.CanTransition(domain.OrderStatusCancelled))
	assert.True(t, domain.OrderStatusAwaitingPayment.CanTransition(domain.OrderStatusPaid))
	assert.True(t, domain.OrderStatusPaid.CanTransition(domain.OrderStatusProcessing))
	assert.True(t, domain.OrderStatusShipped.CanTransition(domain.OrderStatusDelivered))
}

func TestOrderStatus_CanTransition_Invalid(t *testing.T) {
	assert.False(t, domain.OrderStatusPending.CanTransition(domain.OrderStatusPaid))
	assert.False(t, domain.OrderStatusCancelled.CanTransition(domain.OrderStatusPaid))
	assert.False(t, domain.OrderStatusDelivered.CanTransition(domain.OrderStatusShipped))
}

func TestOrderStatus_ValidateTransition_Error(t *testing.T) {
	err := domain.OrderStatusPending.ValidateTransition(domain.OrderStatusPaid)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status transition")
}

func TestOrderStatus_ValidateTransition_Success(t *testing.T) {
	require.NoError(t, domain.OrderStatusPending.ValidateTransition(domain.OrderStatusAwaitingPayment))
}

func TestOrderStatus_CancellableDirectly(t *testing.T) {
	assert.True(t, domain.OrderStatusPending.CancellableDirectly())
	assert.True(t, domain.OrderStatusAwaitingPayment.CancellableDirectly())
	assert.False(t, domain.OrderStatusPaid.CancellableDirectly())
	assert.False(t, domain.OrderStatusDelivered.CancellableDirectly())
}

func TestParseOrderStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected domain.OrderStatus
	}{
		{"", domain.OrderStatusUnspecified},
		{"unspecified", domain.OrderStatusUnspecified},
		{"pending", domain.OrderStatusPending},
		{"awaiting_payment", domain.OrderStatusAwaitingPayment},
		{"paid", domain.OrderStatusPaid},
		{"processing", domain.OrderStatusProcessing},
		{"shipped", domain.OrderStatusShipped},
		{"delivered", domain.OrderStatusDelivered},
		{"cancelled", domain.OrderStatusCancelled},
		{"refunded", domain.OrderStatusRefunded},
	}

	for _, tt := range tests {
		got, err := domain.ParseOrderStatus(tt.input)
		require.NoError(t, err)
		assert.Equal(t, tt.expected, got)
	}
}

func TestParseOrderStatus_CaseInsensitive(t *testing.T) {
	got, err := domain.ParseOrderStatus("PENDING")
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusPending, got)
}

func TestParseOrderStatus_Unknown(t *testing.T) {
	_, err := domain.ParseOrderStatus("unknown")
	require.Error(t, err)
}
