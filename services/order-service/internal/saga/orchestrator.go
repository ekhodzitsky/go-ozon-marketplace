package saga

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Orchestrator гоняет сагу заказа от создания до финала или компенсации.
type Orchestrator struct {
	orderRepo    repository.OrderRepository
	sagaRepo     SagaRepository
	invClient    InventoryClient
	payClient    PaymentClient
	log          *zap.Logger
	callTimeout  time.Duration
	queryTimeout time.Duration
	retryCfg     retryConfig
}

type retryConfig struct {
	maxRetries int
	baseDelay  time.Duration
}

// NewOrchestrator собирает оркестратор со всеми зависимостями.
func NewOrchestrator(
	orderRepo repository.OrderRepository,
	sagaRepo SagaRepository,
	invClient InventoryClient,
	payClient PaymentClient,
	log *zap.Logger,
	callTimeout time.Duration,
	queryTimeout time.Duration,
) *Orchestrator {
	if callTimeout == 0 {
		callTimeout = 5 * time.Second
	}
	if queryTimeout == 0 {
		queryTimeout = 3 * time.Second
	}
	return &Orchestrator{
		orderRepo:    orderRepo,
		sagaRepo:     sagaRepo,
		invClient:    invClient,
		payClient:    payClient,
		log:          log,
		callTimeout:  callTimeout,
		queryTimeout: queryTimeout,
		retryCfg: retryConfig{
			maxRetries: 3,
			baseDelay:  200 * time.Millisecond,
		},
	}
}

func (o *Orchestrator) withQueryTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, o.queryTimeout)
}

func (o *Orchestrator) persistSaga(ctx context.Context, saga *Saga) error {
	saga.UpdatedAt = time.Now().UTC()
	qCtx, cancel := o.withQueryTimeout(ctx)
	defer cancel()
	if err := o.sagaRepo.Save(qCtx, saga); err != nil {
		o.log.Error("failed to save saga", zap.Error(err), zap.String("order_id", saga.OrderID.String()))
		return err
	}
	return nil
}

func (o *Orchestrator) saveSaga(ctx context.Context, saga *Saga, status SagaStatus, step, errMsg string) error {
	saga.Status = status
	saga.CurrentStep = step
	saga.ErrorMessage = errMsg
	return o.persistSaga(ctx, saga)
}

// ProcessOrder проходит сагу от текущего состояния до конца.
func (o *Orchestrator) ProcessOrder(ctx context.Context, order *domain.Order, idempotencyKey string) error {
	qCtx, cancel := o.withQueryTimeout(ctx)
	defer cancel()

	saga, err := o.sagaRepo.GetByOrderID(qCtx, order.ID)
	if err != nil {
		if !errors.Is(err, apperrors.ErrNotFound) {
			return fmt.Errorf("get saga: %w", err)
		}
		saga = &Saga{
			ID:        uuid.New(),
			OrderID:   order.ID,
			Status:    SagaStatusPending,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := o.sagaRepo.Create(qCtx, saga); err != nil {
			return fmt.Errorf("create saga: %w", err)
		}
	}

	if o.isTerminal(saga) {
		return nil
	}

	for {
		step := o.nextStep(saga)
		if step == nil {
			break
		}

		if p, ok := step.(Preparable); ok {
			if p.Prepare(saga) {
				if err := o.persistSaga(ctx, saga); err != nil {
					return err
				}
			}
		}

		if err := o.executeWithRetry(ctx, step, saga, order, idempotencyKey); err != nil {
			return o.handleStepFailure(ctx, saga, order, step, idempotencyKey, err)
		}

		if err := o.persistSaga(ctx, saga); err != nil {
			return err
		}
	}

	return nil
}

func (o *Orchestrator) nextStep(saga *Saga) Step {
	switch saga.Status {
	case SagaStatusPending:
		return &startStep{orderRepo: o.orderRepo, log: o.log}
	case SagaStatusReserving:
		return &reserveInventoryStep{client: o.invClient}
	case SagaStatusReserved, SagaStatusPaying:
		return &processPaymentStep{client: o.payClient}
	case SagaStatusPaid, SagaStatusConfirming:
		return &confirmOrderStep{orderRepo: o.orderRepo}
	case SagaStatusConfirmed, SagaStatusCancelled, SagaStatusFailed, SagaStatusCompensating:
		return nil
	default:
		return nil
	}
}

func (o *Orchestrator) isTerminal(saga *Saga) bool {
	switch saga.Status {
	case SagaStatusConfirmed, SagaStatusCancelled, SagaStatusFailed:
		return true
	}
	return false
}

func (o *Orchestrator) handleStepFailure(ctx context.Context, saga *Saga, order *domain.Order, step Step, idempotencyKey string, execErr error) error {
	// Если упал самый первый шаг — компенсировать нечего, просто падаем.
	if saga.Status == SagaStatusPending {
		if err := o.saveSaga(ctx, saga, SagaStatusFailed, step.Name(), execErr.Error()); err != nil {
			o.log.Error("failed to mark saga failed", zap.Error(err), zap.String("order_id", order.ID.String()))
		}
		return fmt.Errorf("start step failed: %w", execErr)
	}

	plan := o.compensationPlan(step)
	currentStep := "compensate_" + joinStepNames(plan)
	if err := o.saveSaga(ctx, saga, SagaStatusCompensating, currentStep, execErr.Error()); err != nil {
		o.log.Error("failed to mark saga compensating", zap.Error(err), zap.String("order_id", order.ID.String()))
	}

	for _, compStep := range plan {
		if err := o.compensateWithRetry(ctx, compStep, saga, order, idempotencyKey); err != nil {
			o.log.Error("compensation step failed", zap.Error(err), zap.String("step", compStep.Name()))
		}
	}

	qCtx, cancel := o.withQueryTimeout(ctx)
	defer cancel()
	if err := o.orderRepo.UpdateStatus(qCtx, order.ID, domain.OrderStatusCancelled); err != nil {
		o.log.Error("failed to cancel order", zap.Error(err), zap.String("order_id", order.ID.String()))
	}

	if err := o.saveSaga(ctx, saga, SagaStatusCancelled, "cancelled", ""); err != nil {
		o.log.Error("failed to mark saga cancelled", zap.Error(err), zap.String("order_id", order.ID.String()))
	}

	o.log.Warn("order cancelled after step failure", zap.String("order_id", order.ID.String()), zap.String("step", step.Name()), zap.Error(execErr))
	return nil
}

func (o *Orchestrator) compensationPlan(failed Step) []Step {
	switch failed.Name() {
	case "confirm":
		return []Step{
			&processPaymentStep{client: o.payClient},
			&reserveInventoryStep{client: o.invClient},
		}
	case "inventory", "payment":
		return []Step{&reserveInventoryStep{client: o.invClient}}
	default:
		return []Step{&reserveInventoryStep{client: o.invClient}}
	}
}

func joinStepNames(plan []Step) string {
	names := make([]string, len(plan))
	for i, s := range plan {
		names[i] = s.Name()
	}
	return strings.Join(names, "+")
}

func (o *Orchestrator) executeWithRetry(ctx context.Context, step Step, saga *Saga, order *domain.Order, idempotencyKey string) error {
	return o.retry(ctx, func(ctx context.Context) error {
		return step.Execute(ctx, saga, order, idempotencyKey)
	})
}

func (o *Orchestrator) compensateWithRetry(ctx context.Context, step Step, saga *Saga, order *domain.Order, idempotencyKey string) error {
	return o.retry(ctx, func(ctx context.Context) error {
		return step.Compensate(ctx, saga, order, idempotencyKey)
	})
}

func (o *Orchestrator) retry(ctx context.Context, fn func(context.Context) error) error {
	var lastErr error
	for i := 0; i < o.retryCfg.maxRetries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(o.retryCfg.baseDelay * time.Duration(1<<(i-1))):
			}
		}

		cCtx, cancel := context.WithTimeout(ctx, o.callTimeout)
		lastErr = fn(cCtx)
		cancel()

		if lastErr == nil {
			return nil
		}
		o.log.Warn("retryable error", zap.Error(lastErr), zap.Int("attempt", i+1))
	}
	return lastErr
}

// Recover доигрывает незавершенные саги.
func (o *Orchestrator) Recover(ctx context.Context) error {
	qCtx, cancel := o.withQueryTimeout(ctx)
	defer cancel()
	sagas, err := o.sagaRepo.ListIncomplete(qCtx, 100)
	if err != nil {
		return fmt.Errorf("list incomplete sagas: %w", err)
	}
	for _, s := range sagas {
		func(s Saga) {
			qCtx, cancel := o.withQueryTimeout(ctx)
			defer cancel()
			order, err := o.orderRepo.GetByID(qCtx, s.OrderID)
			if err != nil {
				o.log.Error("recover: failed to get order", zap.Error(err), zap.String("order_id", s.OrderID.String()))
				return
			}
			// recovery оборачивает вызов в таймаут, а не передает исходный бесконечный ctx
			procCtx, procCancel := o.withQueryTimeout(ctx)
			defer procCancel()
			if err := o.ProcessOrder(procCtx, order, recoveryIdempotencyKey(s.OrderID.String())); err != nil {
				o.log.Error("recover: process order failed", zap.Error(err), zap.String("order_id", s.OrderID.String()))
			}
		}(s)
	}
	return nil
}

func recoveryIdempotencyKey(orderID string) string {
	return fmt.Sprintf("recovery:%s", orderID)
}

// CancelOrder отменяет уже оплаченный/обработанный заказ: refund, release inventory, cancel.
func (o *Orchestrator) CancelOrder(ctx context.Context, order *domain.Order) error {
	qCtx, cancel := o.withQueryTimeout(ctx)
	defer cancel()

	s, err := o.sagaRepo.GetByOrderID(qCtx, order.ID)
	if err != nil {
		return fmt.Errorf("%w: saga not found for refund: %v", apperrors.ErrFailedPrecondition, err)
	}

	// Ретраи идут через исходный ctx, чтобы бэкофы не убивались query-таймаутом.
	if s.PaymentID != "" {
		if err := o.retry(ctx, func(ctx context.Context) error {
			return o.payClient.Refund(ctx, s.PaymentID, refundKey(order.ID.String(), s.PaymentID))
		}); err != nil {
			return fmt.Errorf("refund payment %s: %w", s.PaymentID, err)
		}
	}

	if err := o.compensateWithRetry(ctx, &reserveInventoryStep{client: o.invClient}, s, order, ""); err != nil {
		return fmt.Errorf("release inventory: %w", err)
	}

	if err := o.orderRepo.UpdateStatus(qCtx, order.ID, domain.OrderStatusCancelled); err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}

	s.Status = SagaStatusCancelled
	s.CurrentStep = "cancelled"
	if err := o.persistSaga(qCtx, s); err != nil {
		o.log.Error("failed to persist cancelled saga", zap.Error(err), zap.String("order_id", order.ID.String()))
	}
	return nil
}

// PostgresAdvisoryLock — распределенный замок через PostgreSQL advisory locks.
type PostgresAdvisoryLock struct {
	pool *pgxpool.Pool
}

// NewPostgresAdvisoryLock создает Locker из пула pgx.
func NewPostgresAdvisoryLock(pool *pgxpool.Pool) *PostgresAdvisoryLock {
	return &PostgresAdvisoryLock{pool: pool}
}

func (l *PostgresAdvisoryLock) TryLock(ctx context.Context, key string) (bool, error) {
	lockID := advisoryLockID(key)
	var acquired bool
	if err := l.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired); err != nil {
		return false, fmt.Errorf("pg_try_advisory_lock: %w", err)
	}
	return acquired, nil
}

func (l *PostgresAdvisoryLock) Unlock(ctx context.Context, key string) error {
	lockID := advisoryLockID(key)
	_, err := l.pool.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockID)
	if err != nil {
		return fmt.Errorf("pg_advisory_unlock: %w", err)
	}
	return nil
}

func advisoryLockID(key string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return int64(h.Sum64())
}

// RecoveryWorker периодически вызывает Recoverer.
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

// NewRecoveryWorker собирает воркер восстановления саг.
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

// RecoveryWorkerOption настраивает RecoveryWorker.
type RecoveryWorkerOption func(*RecoveryWorker)

// WithLocker внедряет распределенный замок.
func WithLocker(lock Locker) RecoveryWorkerOption {
	return func(w *RecoveryWorker) {
		w.lock = lock
	}
}

// WithRecoveryInterval задает интервал опроса.
func WithRecoveryInterval(interval time.Duration) RecoveryWorkerOption {
	return func(w *RecoveryWorker) {
		w.interval = interval
	}
}

// Start запускает цикл восстановления. Можно вызывать повторно — эффекта не будет.
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

// Stop останавливает цикл восстановления. Можно вызывать повторно.
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
