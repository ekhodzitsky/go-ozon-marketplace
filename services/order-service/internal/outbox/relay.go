package outbox

import (
	"context"
	"sync"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Relay struct {
	repo         repository.OutboxRepository
	producer     Producer
	log          *zap.Logger
	ticker       *time.Ticker
	stop         chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
	started      bool
	queryTimeout time.Duration
	topic        string
	maxRetries   int
	baseBackoff  time.Duration
	maxBackoff   time.Duration
}

func NewRelay(repo repository.OutboxRepository, producer Producer, log *zap.Logger, queryTimeout time.Duration, topic string) *Relay {
	return &Relay{
		repo:         repo,
		producer:     producer,
		log:          log,
		queryTimeout: queryTimeout,
		topic:        topic,
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

	events, err := r.repo.GetUnprocessed(ctx, 100)
	if err != nil {
		r.log.Error("failed to get unprocessed outbox events", zap.Error(err))
		return
	}

	processed := make([]uuid.UUID, 0, len(events))
	now := time.Now().UTC()

	for _, event := range events {
		if err := r.producer.SendMessage(r.topic, []byte(event.AggregateID), event.Payload); err != nil {
			r.log.Error("failed to publish outbox event",
				zap.Error(err),
				zap.String("event_id", event.ID.String()),
				zap.Int("retry_count", event.RetryCount),
			)

			nextRetry := r.calculateNextRetry(event.RetryCount)
			if event.RetryCount+1 >= r.maxRetries {
				if dlqErr := r.repo.MoveToDLQ(ctx, &event, now, err.Error()); dlqErr != nil {
					r.log.Error("failed to move outbox event to DLQ",
						zap.Error(dlqErr),
						zap.String("event_id", event.ID.String()),
					)
				}
				continue
			}

			if retryErr := r.repo.IncrementRetryAndSetError(ctx, event.ID, err.Error(), nextRetry); retryErr != nil {
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
		if err := r.repo.BatchMarkProcessed(ctx, processed); err != nil {
			r.log.Error("failed to batch mark outbox events processed", zap.Error(err))
		}
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
