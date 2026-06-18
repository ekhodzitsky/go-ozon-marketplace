package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/mocks"
	"github.com/google/uuid"
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

func TestRelay_Poll_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: []byte("{}")}

	outboxRepo.EXPECT().Begin(gomock.Any()).Return(nil)
	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(nil)
	outboxRepo.EXPECT().Commit(gomock.Any()).Return(nil)

	handler := &stubHandler{handleFunc: func(context.Context, domain.OutboxEvent) error { return nil }}
	r := NewRelay(outboxRepo, handler, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_HandlerErrorRetries(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: []byte("{}"), RetryCount: 0}

	outboxRepo.EXPECT().Begin(gomock.Any()).Return(nil)
	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().IncrementRetryAndSetError(gomock.Any(), event.ID, "es down", gomock.Any()).Return(nil)
	outboxRepo.EXPECT().Commit(gomock.Any()).Return(nil)

	handler := &stubHandler{handleFunc: func(context.Context, domain.OutboxEvent) error { return errors.New("es down") }}
	r := NewRelay(outboxRepo, handler, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_HandlerErrorMaxRetriesMovesToDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: []byte("{}"), RetryCount: 4}

	outboxRepo.EXPECT().Begin(gomock.Any()).Return(nil)
	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().MoveToDLQ(gomock.Any(), gomock.Any(), gomock.Any(), "es down").Return(nil)
	outboxRepo.EXPECT().Commit(gomock.Any()).Return(nil)

	handler := &stubHandler{handleFunc: func(context.Context, domain.OutboxEvent) error { return errors.New("es down") }}
	r := NewRelay(outboxRepo, handler, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_PoisonMovesToDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "Unknown", RetryCount: 0}

	outboxRepo.EXPECT().Begin(gomock.Any()).Return(nil)
	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().MoveToDLQ(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	outboxRepo.EXPECT().Commit(gomock.Any()).Return(nil)

	handler := &stubHandler{handleFunc: func(context.Context, domain.OutboxEvent) error { return ErrPoison }}
	r := NewRelay(outboxRepo, handler, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_GetUnprocessedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	outboxRepo.EXPECT().Begin(gomock.Any()).Return(nil)
	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return(nil, errors.New("db down"))
	outboxRepo.EXPECT().Rollback(gomock.Any()).Return(nil)

	handler := &stubHandler{handleFunc: func(context.Context, domain.OutboxEvent) error { return nil }}
	r := NewRelay(outboxRepo, handler, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_BatchMarkProcessedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: []byte("{}")}

	outboxRepo.EXPECT().Begin(gomock.Any()).Return(nil)
	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(errors.New("db down"))
	outboxRepo.EXPECT().Rollback(gomock.Any()).Return(nil)

	handler := &stubHandler{handleFunc: func(context.Context, domain.OutboxEvent) error { return nil }}
	r := NewRelay(outboxRepo, handler, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: []byte("{}")}

	outboxRepo.EXPECT().Begin(gomock.Any()).Return(nil)
	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(nil)
	outboxRepo.EXPECT().Commit(gomock.Any()).Return(errors.New("commit failed"))
	outboxRepo.EXPECT().Rollback(gomock.Any()).Return(nil)

	handler := &stubHandler{handleFunc: func(context.Context, domain.OutboxEvent) error { return nil }}
	r := NewRelay(outboxRepo, handler, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_StartStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	r := NewRelay(outboxRepo, &stubHandler{}, log, 5*time.Second)

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

	r := NewRelay(outboxRepo, &stubHandler{}, log, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r.Start(ctx)
	r.Start(ctx) // idempotent
	require.True(t, r.started)

	r.Stop()
	r.Stop() // idempotent
	assert.False(t, r.started)
}
