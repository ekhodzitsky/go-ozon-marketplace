package saga

import (
	"go.uber.org/zap"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/executor"
)

// NewRetryExecutor is the adapter constructor for the StepExecutor seam. It
// exposes a retrying executor with the configured policy.
func NewRetryExecutor(cfg RetryConfig, log *zap.Logger) StepExecutor {
	return executor.NewRetryExecutor(executor.RetryConfig(cfg), log)
}
