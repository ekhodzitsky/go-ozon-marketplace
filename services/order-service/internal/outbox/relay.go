package outbox

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"go.uber.org/zap"
)

type Relay struct {
	repo   repository.OutboxRepository
	log    *zap.Logger
	ticker *time.Ticker
	stop   chan struct{}
}

func NewRelay(repo repository.OutboxRepository, log *zap.Logger) *Relay {
	return &Relay{
		repo: repo,
		log:  log,
		stop: make(chan struct{}),
	}
}

func (r *Relay) Start(ctx context.Context) {
	r.ticker = time.NewTicker(500 * time.Millisecond)
	go r.loop(ctx)
}

func (r *Relay) Stop() {
	close(r.stop)
	if r.ticker != nil {
		r.ticker.Stop()
	}
}

func (r *Relay) loop(ctx context.Context) {
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

	for _, event := range events {
		var payload map[string]interface{}
		_ = json.Unmarshal(event.Payload, &payload)

		r.log.Info("publishing outbox event",
			zap.String("aggregate_type", event.AggregateType),
			zap.String("aggregate_id", event.AggregateID),
			zap.String("event_type", event.EventType),
			zap.Any("payload", payload),
		)

		if err := r.repo.MarkProcessed(ctx, event.ID); err != nil {
			r.log.Error("failed to mark outbox event processed", zap.Error(err), zap.String("event_id", event.ID.String()))
		}
	}
}
