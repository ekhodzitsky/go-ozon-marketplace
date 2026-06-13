package outbox

import (
	"context"
	"encoding/json"
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

func TestRelay_Poll_CreatedEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)
	log := zap.NewNop()

	product := domain.Product{ID: uuid.New(), Name: "P", Price: 1000}
	payload, _ := json.Marshal(product)
	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: payload}

	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	searchRepo.EXPECT().Index(gomock.Any(), &product).Return(nil)
	outboxRepo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(nil)

	r := NewRelay(outboxRepo, searchRepo, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_UpdatedEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)
	log := zap.NewNop()

	product := domain.Product{ID: uuid.New(), Name: "P", Price: 2000}
	payload, _ := json.Marshal(product)
	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductUpdated", Payload: payload}

	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	searchRepo.EXPECT().Index(gomock.Any(), &product).Return(nil)
	outboxRepo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(nil)

	r := NewRelay(outboxRepo, searchRepo, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_DeletedEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)
	log := zap.NewNop()

	id := uuid.New()
	payload, _ := json.Marshal(map[string]string{"product_id": id.String()})
	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductDeleted", Payload: payload}

	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	searchRepo.EXPECT().Delete(gomock.Any(), id).Return(nil)
	outboxRepo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(nil)

	r := NewRelay(outboxRepo, searchRepo, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_UnknownEventMarkedPoison(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "Unknown"}

	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(nil)

	r := NewRelay(outboxRepo, searchRepo, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_UnmarshalPoison(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)
	log := zap.NewNop()

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: []byte("not-json")}

	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(nil)

	r := NewRelay(outboxRepo, searchRepo, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_InvalidProductIDInDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)
	log := zap.NewNop()

	payload, _ := json.Marshal(map[string]string{"product_id": "not-a-uuid"})
	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductDeleted", Payload: payload}

	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	outboxRepo.EXPECT().BatchMarkProcessed(gomock.Any(), []uuid.UUID{event.ID}).Return(nil)

	r := NewRelay(outboxRepo, searchRepo, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_IndexErrorContinues(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)
	log := zap.NewNop()

	product := domain.Product{ID: uuid.New(), Name: "P"}
	payload, _ := json.Marshal(product)
	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: payload}

	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return([]domain.OutboxEvent{event}, nil)
	searchRepo.EXPECT().Index(gomock.Any(), &product).Return(errors.New("es down"))

	r := NewRelay(outboxRepo, searchRepo, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_Poll_GetUnprocessedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)
	log := zap.NewNop()

	outboxRepo.EXPECT().GetUnprocessed(gomock.Any(), 100).Return(nil, errors.New("db down"))

	r := NewRelay(outboxRepo, searchRepo, log, 5*time.Second)
	r.poll(context.Background())
}

func TestRelay_StartStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)
	log := zap.NewNop()

	r := NewRelay(outboxRepo, searchRepo, log, 5*time.Second)

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
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)
	log := zap.NewNop()

	r := NewRelay(outboxRepo, searchRepo, log, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r.Start(ctx)
	r.Start(ctx) // idempotent
	require.True(t, r.started)

	r.Stop()
	r.Stop() // idempotent
	assert.False(t, r.started)
}
