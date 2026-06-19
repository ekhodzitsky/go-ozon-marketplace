package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

type fakeProducer struct {
	sendFunc func(topic string, key, value []byte) error
}

func (f *fakeProducer) SendMessage(topic string, key, value []byte) error {
	return f.sendFunc(topic, key, value)
}

func (f *fakeProducer) Close() error { return nil }

func TestOrderRelay_Poll_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), AggregateID: uuid.NewString(), Payload: []byte("payload")}

	repo.EXPECT().Begin(gomock.Any()).Return(nil)
	repo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	repo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(nil)
	repo.EXPECT().Commit(gomock.Any()).Return(nil)

	var called bool
	producer := &fakeProducer{
		sendFunc: func(topic string, key, value []byte) error {
			called = true
			assert.Equal(t, "orders", topic)
			assert.Equal(t, []byte(event.AggregateID), key)
			assert.Equal(t, event.Payload, value)
			return nil
		},
	}

	r := NewRelay(repo, producer, log, 5*time.Second, "orders")
	r.poll(context.Background())
	assert.True(t, called)
}

func TestOrderRelay_Poll_SendErrorRetries(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), AggregateID: uuid.NewString(), Payload: []byte("payload"), RetryCount: 0}

	repo.EXPECT().Begin(gomock.Any()).Return(nil)
	repo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	repo.EXPECT().IncrementRetryAndSetError(gomock.Any(), event.ID, "kafka down", gomock.Any()).Return(nil)
	repo.EXPECT().Commit(gomock.Any()).Return(nil)

	producer := &fakeProducer{sendFunc: func(string, []byte, []byte) error { return errors.New("kafka down") }}

	r := NewRelay(repo, producer, log, 5*time.Second, "orders")
	r.poll(context.Background())
}

func TestOrderRelay_Poll_SendErrorMaxRetriesMovesToDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), AggregateID: uuid.NewString(), Payload: []byte("payload"), RetryCount: 4}

	repo.EXPECT().Begin(gomock.Any()).Return(nil)
	repo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	repo.EXPECT().MoveToDLQ(gomock.Any(), gomock.Any(), gomock.Any(), "kafka down").Return(nil)
	repo.EXPECT().Commit(gomock.Any()).Return(nil)

	producer := &fakeProducer{sendFunc: func(string, []byte, []byte) error { return errors.New("kafka down") }}

	r := NewRelay(repo, producer, log, 5*time.Second, "orders")
	r.poll(context.Background())
}

func TestOrderRelay_Poll_BeginError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	repo.EXPECT().Begin(gomock.Any()).Return(errors.New("begin failed"))

	producer := &fakeProducer{sendFunc: func(string, []byte, []byte) error { return nil }}
	r := NewRelay(repo, producer, log, 5*time.Second, "orders")
	r.poll(context.Background())
}

func TestOrderRelay_Poll_GetUnprocessedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	repo.EXPECT().Begin(gomock.Any()).Return(nil)
	repo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return(nil, errors.New("db down"))
	repo.EXPECT().Rollback(gomock.Any()).Return(nil)

	producer := &fakeProducer{sendFunc: func(string, []byte, []byte) error { return nil }}
	r := NewRelay(repo, producer, log, 5*time.Second, "orders")
	r.poll(context.Background())
}

func TestOrderRelay_Poll_BatchMarkProcessedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), AggregateID: uuid.NewString(), Payload: []byte("payload")}

	repo.EXPECT().Begin(gomock.Any()).Return(nil)
	repo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	repo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(errors.New("db down"))
	repo.EXPECT().Rollback(gomock.Any()).Return(nil)

	producer := &fakeProducer{sendFunc: func(string, []byte, []byte) error { return nil }}
	r := NewRelay(repo, producer, log, 5*time.Second, "orders")
	r.poll(context.Background())
}

func TestOrderRelay_Poll_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), AggregateID: uuid.NewString(), Payload: []byte("payload")}

	repo.EXPECT().Begin(gomock.Any()).Return(nil)
	repo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	repo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(nil)
	repo.EXPECT().Commit(gomock.Any()).Return(errors.New("commit failed"))
	repo.EXPECT().Rollback(gomock.Any()).Return(nil)

	producer := &fakeProducer{sendFunc: func(string, []byte, []byte) error { return nil }}
	r := NewRelay(repo, producer, log, 5*time.Second, "orders")
	r.poll(context.Background())
}

func TestOrderRelay_StartStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOutboxRepository(ctrl)
	log := zap.NewNop()

	r := NewRelay(repo, &fakeProducer{}, log, 5*time.Second, "orders")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r.Start(ctx)
	require.True(t, r.started)

	r.Stop()
	assert.False(t, r.started)
}
