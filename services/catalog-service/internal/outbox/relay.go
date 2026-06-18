package outbox

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Relay polls the outbox table and delegates events to a Handler.
type Relay struct {
	outboxRepo   repository.OutboxRepository
	handler      Handler
	log          *zap.Logger
	ticker       *time.Ticker
	stop         chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
	started      bool
	queryTimeout time.Duration
	maxRetries   int
	baseBackoff  time.Duration
	maxBackoff   time.Duration
}

// NewRelay constructs a catalog outbox relay.
func NewRelay(outboxRepo repository.OutboxRepository, handler Handler, log *zap.Logger, queryTimeout time.Duration) *Relay {
	return &Relay{
		outboxRepo:   outboxRepo,
		handler:      handler,
		log:          log,
		queryTimeout: queryTimeout,
		maxRetries:   5,
		baseBackoff:  500 * time.Millisecond,
		maxBackoff:   30 * time.Second,
	}
}

func (r *Relay) Start(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return
	}
	r.started = true
	r.stop = make(chan struct{})
	r.ticker = time.NewTicker(500 * time.Millisecond)
	r.wg.Add(1)
	go r.loop(ctx)
}

func (r *Relay) Stop() {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return
	}
	r.started = false
	close(r.stop)
	r.ticker.Stop()
	r.mu.Unlock()
	r.wg.Wait()
}

func (r *Relay) loop(ctx context.Context) {
	defer r.wg.Done()
	for {
		select {
		case <-r.ticker.C:
			r.poll(ctx)
		case <-r.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (r *Relay) poll(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	if err := r.outboxRepo.Begin(ctx); err != nil {
		r.log.Error("failed to begin outbox transaction", zap.Error(err))
		return
	}

	events, err := r.outboxRepo.GetUnprocessed(ctx, 100)
	if err != nil {
		r.log.Error("failed to get unprocessed outbox events", zap.Error(err))
		_ = r.outboxRepo.Rollback(ctx)
		return
	}

	now := time.Now().UTC()
	processed := make([]uuid.UUID, 0, len(events))

	for _, event := range events {
		if err := r.handler.Handle(ctx, event); err != nil {
			r.log.Error("failed to handle outbox event",
				zap.Error(err),
				zap.String("event_id", event.ID.String()),
				zap.Int("retry_count", event.RetryCount),
			)

			if errors.Is(err, ErrPoison) || event.RetryCount+1 >= r.maxRetries {
				if dlqErr := r.outboxRepo.MoveToDLQ(ctx, &event, now, err.Error()); dlqErr != nil {
					r.log.Error("failed to move outbox event to DLQ",
						zap.Error(dlqErr),
						zap.String("event_id", event.ID.String()),
					)
				}
				continue
			}

			nextRetry := r.calculateNextRetry(event.RetryCount)
			if retryErr := r.outboxRepo.IncrementRetryAndSetError(ctx, event.ID, err.Error(), nextRetry); retryErr != nil {
				r.log.Error("failed to increment outbox retry",
					zap.Error(retryErr),
					zap.String("event_id", event.ID.String()),
				)
			}
			continue
		}

		processed = append(processed, event.ID)
	}

	if len(processed) > 0 {
		if err := r.outboxRepo.BatchMarkProcessed(ctx, processed); err != nil {
			r.log.Error("failed to batch mark outbox events processed", zap.Error(err))
			_ = r.outboxRepo.Rollback(ctx)
			return
		}
	}

	if err := r.outboxRepo.Commit(ctx); err != nil {
		r.log.Error("failed to commit outbox transaction", zap.Error(err))
		_ = r.outboxRepo.Rollback(ctx)
	}
}

func (r *Relay) calculateNextRetry(retryCount int) time.Time {
	factor := time.Duration(1) << min(retryCount, 10)
	backoff := r.baseBackoff * factor
	if backoff > r.maxBackoff {
		backoff = r.maxBackoff
	}
	return time.Now().UTC().Add(backoff)
}
