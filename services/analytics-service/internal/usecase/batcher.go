package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"go.uber.org/zap"
)

// eventBatcher buffers analytics events and flushes them to ClickHouse in batches.
type eventBatcher struct {
	repo          EventRepository
	log           *zap.Logger
	mu            sync.Mutex
	buffer        []domain.Event
	seen          map[string]struct{}
	size          int
	flushInterval time.Duration
	callTimeout   time.Duration
	stopCh        chan struct{}
	doneCh        chan struct{}
}

func newEventBatcher(repo EventRepository, log *zap.Logger, size int, flushInterval, callTimeout time.Duration) *eventBatcher {
	b := &eventBatcher{
		repo:          repo,
		log:           log,
		buffer:        make([]domain.Event, 0, size),
		seen:          make(map[string]struct{}),
		size:          size,
		flushInterval: flushInterval,
		callTimeout:   callTimeout,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}
	go b.loop()
	return b
}

// Add buffers an event. Duplicate aggregation keys are dropped in-process.
func (b *eventBatcher) Add(ctx context.Context, event domain.Event) error {
	b.mu.Lock()
	if event.AggregationKey != "" {
		if _, exists := b.seen[event.AggregationKey]; exists {
			b.mu.Unlock()
			return nil
		}
		b.seen[event.AggregationKey] = struct{}{}
	}
	b.buffer = append(b.buffer, event)
	shouldFlush := len(b.buffer) >= b.size
	b.mu.Unlock()

	if shouldFlush {
		return b.Flush(ctx)
	}
	return nil
}

// Flush sends all buffered events to ClickHouse.
// On failure the events are kept in the buffer so they can be retried.
func (b *eventBatcher) Flush(ctx context.Context) error {
	b.mu.Lock()
	if len(b.buffer) == 0 {
		b.mu.Unlock()
		return nil
	}
	events := b.buffer
	b.buffer = make([]domain.Event, 0, b.size)
	b.mu.Unlock()

	timeout := b.callTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := b.repo.BatchInsert(ctx, events); err != nil {
		b.mu.Lock()
		// Prepend the failed events back so they are retried before newer ones.
		b.buffer = append(events, b.buffer...)
		b.mu.Unlock()
		return err
	}
	return nil
}

func (b *eventBatcher) loop() {
	defer close(b.doneCh)
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()
	timeout := b.callTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			if err := b.Flush(ctx); err != nil {
				b.log.Error("periodic event flush failed", zap.Error(err))
			}
			cancel()
		case <-b.stopCh:
			return
		}
	}
}

// Stop halts the background ticker and flushes remaining events.
func (b *eventBatcher) Stop(ctx context.Context) error {
	close(b.stopCh)
	<-b.doneCh
	return b.Flush(ctx)
}
