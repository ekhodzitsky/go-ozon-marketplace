package outbox

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Relay struct {
	repo    repository.OutboxRepository
	log     *zap.Logger
	ticker  *time.Ticker
	stop    chan struct{}
	wg      sync.WaitGroup
	mu      sync.Mutex
	started bool
}

func NewRelay(repo repository.OutboxRepository, log *zap.Logger) *Relay {
	return &Relay{
		repo: repo,
		log:  log,
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
	events, err := r.repo.GetUnprocessed(ctx, 100)
	if err != nil {
		r.log.Error("failed to get unprocessed outbox events", zap.Error(err))
		return
	}

	ids := make([]uuid.UUID, 0, len(events))
	for _, event := range events {
		var payload map[string]interface{}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			r.log.Error("failed to unmarshal outbox payload", zap.Error(err), zap.String("event_id", event.ID.String()))
			continue
		}

		r.log.Info("publishing outbox event",
			zap.String("aggregate_type", event.AggregateType),
			zap.String("aggregate_id", event.AggregateID),
			zap.String("event_type", event.EventType),
			zap.Any("payload", payload),
		)

		ids = append(ids, event.ID)
	}

	if len(ids) > 0 {
		if err := r.repo.BatchMarkProcessed(ctx, ids); err != nil {
			r.log.Error("failed to batch mark outbox events processed", zap.Error(err))
		}
	}
}
