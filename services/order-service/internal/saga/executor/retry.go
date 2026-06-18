package executor

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
	"go.uber.org/zap"
)

// RetryExecutor runs steps with exponential back-off. It is an adapter around
// the StepExecutor seam that adds locality for the retry policy.
type RetryExecutor struct {
	cfg RetryConfig
	log *zap.Logger
}

// NewRetryExecutor creates a RetryExecutor with sensible defaults.
func NewRetryExecutor(cfg RetryConfig, log *zap.Logger) *RetryExecutor {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = 200 * time.Millisecond
	}
	return &RetryExecutor{cfg: cfg, log: log}
}

var _ StepExecutor = (*RetryExecutor)(nil)

// Execute runs the step with retries and a per-attempt call timeout.
func (e *RetryExecutor) Execute(ctx context.Context, step steps.Step, saga *domain.Saga, order *domain.Order, idempotencyKey string) error {
	return e.retry(ctx, func(ctx context.Context) error {
		return step.Execute(ctx, saga, order, idempotencyKey)
	})
}

// Compensate runs the step compensation with retries and a per-attempt call
// timeout.
func (e *RetryExecutor) Compensate(ctx context.Context, step steps.Step, saga *domain.Saga, order *domain.Order, idempotencyKey string) error {
	return e.retry(ctx, func(ctx context.Context) error {
		return step.Compensate(ctx, saga, order, idempotencyKey)
	})
}

func (e *RetryExecutor) retry(ctx context.Context, fn func(context.Context) error) error {
	var lastErr error
	for i := 0; i < e.cfg.MaxRetries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(e.cfg.BaseDelay * time.Duration(1<<(i-1))):
			}
		}

		cCtx, cancel := context.WithTimeout(ctx, e.cfg.CallTimeout)
		lastErr = fn(cCtx)
		cancel()

		if lastErr == nil {
			return nil
		}
		e.log.Warn("retryable error", zap.Error(lastErr), zap.Int("attempt", i+1))
	}
	return lastErr
}
