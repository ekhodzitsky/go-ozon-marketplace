package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/mocks"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

type stubHandler struct {
	handleFunc func(context.Context, domain.OutboxEvent) error
}

func (s *stubHandler) Handle(ctx context.Context, e domain.OutboxEvent) error {
	return s.handleFunc(ctx, e)
}

// txCall описывает поведение одного вызова fakeTxRunner.run.
type txCall struct {
	err    error
	skipFn bool // если true, callback не выполняется (имитирует ошибку до исполнения)
}

// fakeTxRunner имитирует менеджер транзакций: выполняет callback с nil-транзакцией.
// calls позволяет задать поведение для конкретного вызова runTx по порядку.
type fakeTxRunner struct {
	calls []txCall
	idx   int
}

func (f *fakeTxRunner) run(ctx context.Context, fn func(pgx.Tx) error) error {
	if f.idx >= len(f.calls) {
		return fn(nil)
	}
	call := f.calls[f.idx]
	f.idx++
	if call.skipFn {
		return call.err
	}
	if err := fn(nil); err != nil {
		return err
	}
	return call.err
}

func TestRelay_Poll_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: []byte("{}")}

	outboxRepo.EXPECT().WithTx(gomock.Any()).Return(outboxRepo).AnyTimes()
	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(nil)

	txRunner := &fakeTxRunner{}
	handler := &stubHandler{handleFunc: func(context.Context, domain.OutboxEvent) error { return nil }}
	r := NewRelay(txRunner.run, outboxRepo, handler, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_HandlerErrorRetries(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: []byte("{}"), RetryCount: 0}

	outboxRepo.EXPECT().WithTx(gomock.Any()).Return(outboxRepo).AnyTimes()
	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().IncrementRetryAndSetError(gomock.Any(), event.ID, "es down", gomock.Any()).Return(nil)

	txRunner := &fakeTxRunner{}
	handler := &stubHandler{handleFunc: func(context.Context, domain.OutboxEvent) error { return errors.New("es down") }}
	r := NewRelay(txRunner.run, outboxRepo, handler, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_HandlerErrorMaxRetriesMovesToDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: []byte("{}"), RetryCount: 4}

	outboxRepo.EXPECT().WithTx(gomock.Any()).Return(outboxRepo).AnyTimes()
	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().MoveToDLQ(gomock.Any(), gomock.Any(), gomock.Any(), "es down").Return(nil)

	txRunner := &fakeTxRunner{}
	handler := &stubHandler{handleFunc: func(context.Context, domain.OutboxEvent) error { return errors.New("es down") }}
	r := NewRelay(txRunner.run, outboxRepo, handler, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_PoisonMovesToDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "Unknown", RetryCount: 0}

	outboxRepo.EXPECT().WithTx(gomock.Any()).Return(outboxRepo).AnyTimes()
	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().MoveToDLQ(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	txRunner := &fakeTxRunner{}
	handler := &stubHandler{handleFunc: func(context.Context, domain.OutboxEvent) error { return ErrPoison }}
	r := NewRelay(txRunner.run, outboxRepo, handler, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_GetUnprocessedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	outboxRepo.EXPECT().WithTx(gomock.Any()).Return(outboxRepo).AnyTimes()
	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return(nil, errors.New("db down"))

	txRunner := &fakeTxRunner{}
	handler := &stubHandler{handleFunc: func(context.Context, domain.OutboxEvent) error { return nil }}
	r := NewRelay(txRunner.run, outboxRepo, handler, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_BatchMarkProcessedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: []byte("{}")}

	outboxRepo.EXPECT().WithTx(gomock.Any()).Return(outboxRepo).AnyTimes()
	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(errors.New("db down"))

	txRunner := &fakeTxRunner{}
	handler := &stubHandler{handleFunc: func(context.Context, domain.OutboxEvent) error { return nil }}
	r := NewRelay(txRunner.run, outboxRepo, handler, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: []byte("{}")}

	outboxRepo.EXPECT().WithTx(gomock.Any()).Return(outboxRepo).AnyTimes()
	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(nil)

	// Ошибка на второй транзакции: callback выполняется, но сама транзакция не коммитится.
	txRunner := &fakeTxRunner{calls: []txCall{{}, {err: errors.New("commit failed")}}}
	handler := &stubHandler{handleFunc: func(context.Context, domain.OutboxEvent) error { return nil }}
	r := NewRelay(txRunner.run, outboxRepo, handler, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_StartStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	txRunner := &fakeTxRunner{}
	r := NewRelay(txRunner.run, outboxRepo, &stubHandler{}, log, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r.Start(ctx)
	assert.True(t, r.started)

	r.Stop()
	assert.False(t, r.started)
}

func TestRelay_StartStop_Idempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	txRunner := &fakeTxRunner{}
	r := NewRelay(txRunner.run, outboxRepo, &stubHandler{}, log, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r.Start(ctx)
	r.Start(ctx) // idempotent
	require.True(t, r.started)

	r.Stop()
	r.Stop() // idempotent
	assert.False(t, r.started)
}
