package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestESHandler_ProductCreated(t *testing.T) {
	ctrl := gomock.NewController(t)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)

	product := domain.Product{ID: uuid.New(), Name: "P", Price: 1000}
	payload, _ := json.Marshal(product)
	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: payload}

	searchRepo.EXPECT().Index(gomock.Any(), &product).Return(nil)

	h := NewESHandler(searchRepo)
	err := h.Handle(context.Background(), event)
	assert.NoError(t, err)
}

func TestESHandler_ProductUpdated(t *testing.T) {
	ctrl := gomock.NewController(t)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)

	product := domain.Product{ID: uuid.New(), Name: "P", Price: 2000}
	payload, _ := json.Marshal(product)
	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductUpdated", Payload: payload}

	searchRepo.EXPECT().Index(gomock.Any(), &product).Return(nil)

	h := NewESHandler(searchRepo)
	err := h.Handle(context.Background(), event)
	assert.NoError(t, err)
}

func TestESHandler_ProductDeleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)

	id := uuid.New()
	payload, _ := json.Marshal(map[string]string{"product_id": id.String()})
	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductDeleted", Payload: payload}

	searchRepo.EXPECT().Delete(gomock.Any(), id).Return(nil)

	h := NewESHandler(searchRepo)
	err := h.Handle(context.Background(), event)
	assert.NoError(t, err)
}

func TestESHandler_UnknownEventReturnsPoison(t *testing.T) {
	ctrl := gomock.NewController(t)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "Unknown"}

	h := NewESHandler(searchRepo)
	err := h.Handle(context.Background(), event)
	assert.ErrorIs(t, err, ErrPoison)
}

func TestESHandler_UnmarshalablePayloadReturnsPoison(t *testing.T) {
	ctrl := gomock.NewController(t)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)

	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: []byte("not-json")}

	h := NewESHandler(searchRepo)
	err := h.Handle(context.Background(), event)
	assert.ErrorIs(t, err, ErrPoison)
}

func TestESHandler_InvalidDeleteIDReturnsPoison(t *testing.T) {
	ctrl := gomock.NewController(t)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)

	payload, _ := json.Marshal(map[string]string{"product_id": "not-a-uuid"})
	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductDeleted", Payload: payload}

	h := NewESHandler(searchRepo)
	err := h.Handle(context.Background(), event)
	assert.ErrorIs(t, err, ErrPoison)
}

func TestESHandler_IndexErrorNotPoison(t *testing.T) {
	ctrl := gomock.NewController(t)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)

	product := domain.Product{ID: uuid.New(), Name: "P"}
	payload, _ := json.Marshal(product)
	event := domain.OutboxEvent{ID: uuid.New(), EventType: "ProductCreated", Payload: payload}

	searchRepo.EXPECT().Index(gomock.Any(), &product).Return(errors.New("es down"))

	h := NewESHandler(searchRepo)
	err := h.Handle(context.Background(), event)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrPoison)
}
