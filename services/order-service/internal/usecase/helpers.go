package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/google/uuid"
)

func (u *orderUsecase) publishOrderEvent(ctx context.Context, orderID, status, userID string) {
	if u.redis == nil {
		return
	}
	event := map[string]interface{}{
		"topic":   "orders",
		"user_id": userID,
		"payload": map[string]interface{}{
			"order_id": orderID,
			"status":   status,
			"user_id":  userID,
		},
	}
	data, _ := json.Marshal(event)
	pubCtx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()
	u.redis.Publish(pubCtx, "order-events", string(data))
}

func outboxEventFromOrder(order *domain.Order) (*domain.OutboxEvent, error) {
	payload, err := json.Marshal(order)
	if err != nil {
		return nil, fmt.Errorf("marshal outbox payload: %w", err)
	}
	return &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "order",
		AggregateID:   order.ID.String(),
		EventType:     "OrderCreated",
		Payload:       payload,
		CreatedAt:     order.CreatedAt,
	}, nil
}

func releaseIdempotencyKey(orderID, productID string) string {
	return fmt.Sprintf("release:%s:%s", orderID, productID)
}

func refundIdempotencyKey(orderID, paymentID string) string {
	return fmt.Sprintf("refund:%s:%s", orderID, paymentID)
}
