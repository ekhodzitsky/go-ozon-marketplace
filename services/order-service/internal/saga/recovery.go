package saga

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

type RecoveryWorker struct {
	orchestrator *Orchestrator
	interval     time.Duration
	log          *zap.Logger
	stop         chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
	started      bool
}

func NewRecoveryWorker(orchestrator *Orchestrator, log *zap.Logger) *RecoveryWorker {
	return &RecoveryWorker{
		orchestrator: orchestrator,
		interval:     5 * time.Second,
		log:          log,
	}
}

func (w *RecoveryWorker) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started {
		return
	}
	w.started = true
	w.stop = make(chan struct{})
	w.wg.Add(1)

	w.recoverOnce(ctx)

	go w.loop(ctx)
}

func (w *RecoveryWorker) Stop() {
	w.mu.Lock()
	if !w.started {
		w.mu.Unlock()
		return
	}
	w.started = false
	close(w.stop)
	w.mu.Unlock()
	w.wg.Wait()
}

func (w *RecoveryWorker) loop(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.recoverOnce(ctx)
		case <-w.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (w *RecoveryWorker) recoverOnce(ctx context.Context) {
	if err := w.orchestrator.Recover(ctx); err != nil {
		w.log.Error("saga recovery failed", zap.Error(err))
	}
}
