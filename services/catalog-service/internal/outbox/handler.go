package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
	"github.com/google/uuid"
)

// Handler is the local seam between the outbox relay and the downstream sink.
type Handler interface {
	Handle(ctx context.Context, event domain.OutboxEvent) error
}

// ErrPoison signals an outbox event that can never be handled successfully.
var ErrPoison = errors.New("poison outbox event")

// ESHandler is an adapter that projects outbox events into the Elasticsearch index.
type ESHandler struct {
	searchRepo repository.ProductSearchRepository
}

// NewESHandler creates an Elasticsearch projection handler.
func NewESHandler(searchRepo repository.ProductSearchRepository) *ESHandler {
	return &ESHandler{searchRepo: searchRepo}
}

// Handle dispatches catalog outbox events to the search index.
// Unknown or unparseable events return ErrPoison so the relay moves them to the DLQ.
func (h *ESHandler) Handle(ctx context.Context, event domain.OutboxEvent) error {
	switch event.EventType {
	case "ProductCreated", "ProductUpdated":
		var product domain.Product
		if err := json.Unmarshal(event.Payload, &product); err != nil {
			return fmt.Errorf("%w: unmarshal product payload: %w", ErrPoison, err)
		}
		if err := h.searchRepo.Index(ctx, &product); err != nil {
			return fmt.Errorf("index product: %w", err)
		}
		return nil
	case "ProductDeleted":
		var payload struct {
			ProductID string `json:"product_id"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return fmt.Errorf("%w: unmarshal delete payload: %w", ErrPoison, err)
		}
		id, err := uuid.Parse(payload.ProductID)
		if err != nil {
			return fmt.Errorf("%w: invalid product id: %w", ErrPoison, err)
		}
		if err := h.searchRepo.Delete(ctx, id); err != nil {
			return fmt.Errorf("delete product from index: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown event type %q", ErrPoison, event.EventType)
	}
}
