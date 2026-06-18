package saga

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RecoveryWorker periodically invokes the Recoverer seam to continue
// incomplete sagas. It uses Locker to ensure only one worker runs at a time.
type RecoveryWorker struct {
	recoverer Recoverer
	interval  time.Duration
	lock      Locker
	lockKey   string
	log       *zap.Logger
	stop      chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
	started   bool
}

// NewRecoveryWorker builds a worker that depends on the Recoverer seam rather
// than the concrete Orchestrator. *Orchestrator satisfies Recoverer, so the
// public call site remains unchanged.
func NewRecoveryWorker(recoverer Recoverer, log *zap.Logger, opts ...RecoveryWorkerOption) *RecoveryWorker {
	w := &RecoveryWorker{
		recoverer: recoverer,
		interval:  5 * time.Second,
		lockKey:   "saga-recovery-worker",
		log:       log,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// RecoveryWorkerOption configures a RecoveryWorker.
type RecoveryWorkerOption func(*RecoveryWorker)

// WithLocker injects a distributed lock into the worker.
func WithLocker(lock Locker) RecoveryWorkerOption {
	return func(w *RecoveryWorker) {
		w.lock = lock
	}
}

// WithRecoveryInterval sets the polling interval.
func WithRecoveryInterval(interval time.Duration) RecoveryWorkerOption {
	return func(w *RecoveryWorker) {
		w.interval = interval
	}
}

// Start begins the recovery loop. It is idempotent.
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

// Stop halts the recovery loop. It is idempotent.
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
	if w.lock != nil {
		locked, err := w.lock.TryLock(ctx, w.lockKey)
		if err != nil {
			w.log.Error("failed to acquire recovery lock", zap.Error(err))
			return
		}
		if !locked {
			w.log.Debug("recovery lock held by another instance")
			return
		}
		defer func() {
			if err := w.lock.Unlock(ctx, w.lockKey); err != nil {
				w.log.Error("failed to release recovery lock", zap.Error(err))
			}
		}()
	}

	if err := w.recoverer.Recover(ctx); err != nil {
		w.log.Error("saga recovery failed", zap.Error(err))
	}
}
