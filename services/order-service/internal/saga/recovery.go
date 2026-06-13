package saga

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Locker is a distributed lock used to ensure only one recovery worker runs at a time.
type Locker interface {
	TryLock(ctx context.Context, key string) (bool, error)
	Unlock(ctx context.Context, key string) error
}

type RecoveryWorker struct {
	orchestrator *Orchestrator
	interval     time.Duration
	lock         Locker
	lockKey      string
	log          *zap.Logger
	stop         chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
	started      bool
}

func NewRecoveryWorker(orchestrator *Orchestrator, log *zap.Logger, opts ...RecoveryWorkerOption) *RecoveryWorker {
	w := &RecoveryWorker{
		orchestrator: orchestrator,
		interval:     5 * time.Second,
		lockKey:      "saga-recovery-worker",
		log:          log,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

type RecoveryWorkerOption func(*RecoveryWorker)

func WithLocker(lock Locker) RecoveryWorkerOption {
	return func(w *RecoveryWorker) {
		w.lock = lock
	}
}

func WithRecoveryInterval(interval time.Duration) RecoveryWorkerOption {
	return func(w *RecoveryWorker) {
		w.interval = interval
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

	if err := w.orchestrator.Recover(ctx); err != nil {
		w.log.Error("saga recovery failed", zap.Error(err))
	}
}
