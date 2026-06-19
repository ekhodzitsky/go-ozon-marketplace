package usecase

import (
	"context"
	"encoding/json"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph/model"
	"github.com/redis/go-redis/v9"
)

func OrderStatusChanged(ctx context.Context, redisClient *redis.Client, userID string) (<-chan *model.Order, error) {
	if err := requireOwnerOrAdmin(ctx, userID); err != nil {
		return nil, err
	}
	ch := make(chan *model.Order, 1)
	pubsub := redisClient.PSubscribe(ctx, "order-events")
	go func() {
		defer close(ch)
		defer func() { _ = pubsub.Close() }()
		msgCh := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				var envelope struct {
					Topic   string `json:"topic"`
					UserID  string `json:"user_id"`
					Payload struct {
						OrderID string `json:"order_id"`
						Status  string `json:"status"`
						UserID  string `json:"user_id"`
					} `json:"payload"`
				}
				if err := json.Unmarshal([]byte(msg.Payload), &envelope); err != nil {
					continue
				}
				if envelope.UserID != "" && envelope.UserID != userID {
					continue
				}
				ch <- &model.Order{
					ID:     envelope.Payload.OrderID,
					UserID: envelope.Payload.UserID,
					Status: envelope.Payload.Status,
				}
			}
		}
	}()
	return ch, nil
}

func InventoryChanged(ctx context.Context, redisClient *redis.Client, productID string) (<-chan *model.Inventory, error) {
	ch := make(chan *model.Inventory, 1)
	pubsub := redisClient.PSubscribe(ctx, "inventory-events")
	go func() {
		defer close(ch)
		defer func() { _ = pubsub.Close() }()
		msgCh := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				var envelope struct {
					Topic   string `json:"topic"`
					Payload struct {
						ProductID string `json:"product_id"`
						Available int32  `json:"available"`
						Reserved  int32  `json:"reserved"`
					} `json:"payload"`
				}
				if err := json.Unmarshal([]byte(msg.Payload), &envelope); err != nil {
					continue
				}
				if envelope.Payload.ProductID != "" && envelope.Payload.ProductID != productID {
					continue
				}
				ch <- &model.Inventory{
					ProductID: envelope.Payload.ProductID,
					Available: envelope.Payload.Available,
					Reserved:  envelope.Payload.Reserved,
				}
			}
		}
	}()
	return ch, nil
}
