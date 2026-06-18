package saga

import (
	"go.uber.org/zap"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
)

// NewStartStep is the adapter constructor for the start step seam.
func NewStartStep(orderRepo repository.OrderRepository, log *zap.Logger) Step {
	return steps.NewStartStep(orderRepo, log)
}

// NewReserveInventoryStep is the adapter constructor for the reserve step seam.
func NewReserveInventoryStep(client InventoryClient) Step {
	return steps.NewReserveInventoryStep(client)
}

// NewProcessPaymentStep is the adapter constructor for the payment step seam.
func NewProcessPaymentStep(client PaymentClient) Step {
	return steps.NewProcessPaymentStep(client)
}

// NewConfirmOrderStep is the adapter constructor for the confirm step seam.
func NewConfirmOrderStep(orderRepo repository.OrderRepository) Step {
	return steps.NewConfirmOrderStep(orderRepo)
}
