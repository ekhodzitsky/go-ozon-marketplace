package outbox

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Relay struct {
	outboxRepo   repository.OutboxRepository
	searchRepo   repository.ProductSearchRepository
	log          *zap.Logger
	ticker       *time.Ticker
	stop         chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
	started      bool
	queryTimeout time.Duration
}

func NewRelay(outboxRepo repository.OutboxRepository, searchRepo repository.ProductSearchRepository, log *zap.Logger, queryTimeout time.Duration) *Relay {
	return &Relay{
		outboxRepo:   outboxRepo,
		searchRepo:   searchRepo,
		log:          log,
		queryTimeout: queryTimeout,
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
	events, err := r.outboxRepo.GetUnprocessed(ctx, 100)
	if err != nil {
		r.log.Error("failed to get unprocessed outbox events", zap.Error(err))
		return
	}

	var successIDs []uuid.UUID
	var poisonIDs []uuid.UUID

	for _, event := range events {
		var product domain.Product
		if err := json.Unmarshal(event.Payload, &product); err != nil {
			r.log.Error("failed to unmarshal outbox payload", zap.Error(err), zap.String("event_id", event.ID.String()))
			poisonIDs = append(poisonIDs, event.ID)
			continue
		}

		if err := r.searchRepo.Index(ctx, &product); err != nil {
			r.log.Error("failed to index product in ES", zap.Error(err), zap.String("event_id", event.ID.String()), zap.String("product_id", product.ID.String()))
			continue
		}

		successIDs = append(successIDs, event.ID)
	}

	if len(poisonIDs) > 0 {
		if err := r.outboxRepo.BatchMarkProcessed(ctx, poisonIDs); err != nil {
			r.log.Error("failed to mark poison outbox events processed", zap.Error(err))
		}
	}
	if len(successIDs) > 0 {
		if err := r.outboxRepo.BatchMarkProcessed(ctx, successIDs); err != nil {
			r.log.Error("failed to batch mark outbox events processed", zap.Error(err))
		}
	}
}
