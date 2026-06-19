package outbox

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// txRunner выполняет функцию внутри БД-транзакции.
// В продакшене это txmanager.RunTx, в тестах — заглушка.
type txRunner func(ctx context.Context, fn func(pgx.Tx) error) error

// Relay polls the outbox table and delegates events to a Handler.
type Relay struct {
	runTx        txRunner
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
func NewRelay(runTx txRunner, outboxRepo repository.OutboxRepository, handler Handler, log *zap.Logger, queryTimeout time.Duration) *Relay {
	return &Relay{
		runTx:        runTx,
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

// poll читает события из outbox и отправляет их в Elasticsearch.
// Транзакция управляется напрямую в relay: чтение событий и внешний вызов
// разнесены по разным транзакциям, чтобы Elasticsearch не вызывался внутри БД-транзакции.
// В текущей схеме без бронирования событий между транзакциями реле считается единственным
// или обработчик — идемпотентным.
func (r *Relay) poll(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	var events []domain.OutboxEvent
	if err := r.runTx(ctx, func(tx pgx.Tx) error {
		var err error
		events, err = r.outboxRepo.WithTx(tx).GetUnprocessed(ctx, 100)
		return err
	}); err != nil {
		r.log.Error("failed to get unprocessed outbox events", zap.Error(err))
		return
	}

	if len(events) == 0 {
		return
	}

	// Внешний вызов делаем после коммита первой транзакции.
	type result struct {
		event domain.OutboxEvent
		err   error
	}
	results := make([]result, 0, len(events))
	for _, event := range events {
		err := r.handler.Handle(ctx, event)
		results = append(results, result{event: event, err: err})
		if err != nil {
			r.log.Error("failed to handle outbox event",
				zap.Error(err),
				zap.String("event_id", event.ID.String()),
				zap.Int("retry_count", event.RetryCount),
			)
		}
	}

	now := time.Now().UTC()
	if err := r.runTx(ctx, func(tx pgx.Tx) error {
		txRepo := r.outboxRepo.WithTx(tx)
		for _, res := range results {
			switch {
			case res.err == nil:
				if err := txRepo.BatchMarkProcessed(ctx, []uuid.UUID{res.event.ID}); err != nil {
					return fmt.Errorf("mark outbox event processed: %w", err)
				}
			case errors.Is(res.err, ErrPoison) || res.event.RetryCount+1 >= r.maxRetries:
				if err := txRepo.MoveToDLQ(ctx, &res.event, now, res.err.Error()); err != nil {
					return fmt.Errorf("move outbox event to DLQ: %w", err)
				}
			default:
				nextRetry := r.calculateNextRetry(res.event.RetryCount)
				if err := txRepo.IncrementRetryAndSetError(ctx, res.event.ID, res.err.Error(), nextRetry); err != nil {
					return fmt.Errorf("increment outbox retry: %w", err)
				}
			}
		}
		return nil
	}); err != nil {
		r.log.Error("failed to commit outbox results", zap.Error(err))
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
