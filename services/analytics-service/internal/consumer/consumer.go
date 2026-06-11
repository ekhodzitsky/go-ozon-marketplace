package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/usecase"
	"go.uber.org/zap"
)

// Event represents a Kafka message payload.
type Event struct {
	EventType string `json:"event_type"`
	OrderID   string `json:"order_id"`
	UserID    string `json:"user_id"`
}

// Consumer wraps a Sarama consumer group.
type Consumer struct {
	group  sarama.ConsumerGroup
	uc     usecase.AnalyticsUsecase
	log    *zap.Logger
	topics []string
}

// NewConsumer creates a new Kafka consumer.
func NewConsumer(brokers []string, groupID string, topics []string, uc usecase.AnalyticsUsecase, log *zap.Logger) (*Consumer, error) {
	cfg := sarama.NewConfig()
	cfg.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, fmt.Errorf("new consumer group: %w", err)
	}

	return &Consumer{
		group:  group,
		uc:     uc,
		log:    log,
		topics: topics,
	}, nil
}

// Start begins consuming messages in a background goroutine.
func (c *Consumer) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			handler := &consumerHandler{
				uc:  c.uc,
				log: c.log,
			}

			if err := c.group.Consume(ctx, c.topics, handler); err != nil {
				c.log.Error("consumer group error", zap.Error(err))
				time.Sleep(3 * time.Second)
			}
		}
	}()
}

// Close shuts down the consumer group.
func (c *Consumer) Close() error {
	return c.group.Close()
}

type consumerHandler struct {
	uc  usecase.AnalyticsUsecase
	log *zap.Logger
}

func (h *consumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		var event Event
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			h.log.Error("failed to unmarshal message", zap.Error(err))
			session.MarkMessage(msg, "")
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var err error

		switch event.EventType {
		case "OrderCreated":
			err = h.uc.TrackEvent(ctx, domain.EventTypeOrderCreated, event.OrderID, string(msg.Value), event.UserID)
		case "OrderConfirmed":
			err = h.uc.TrackEvent(ctx, domain.EventTypeOrderConfirmed, event.OrderID, string(msg.Value), event.UserID)
		case "OrderCancelled":
			err = h.uc.TrackEvent(ctx, domain.EventTypeOrderCancelled, event.OrderID, string(msg.Value), event.UserID)
		default:
			h.log.Warn("unknown event type", zap.String("event_type", event.EventType))
		}

		if err != nil {
			h.log.Error("failed to track event", zap.String("event_type", event.EventType), zap.Error(err))
		}

		session.MarkMessage(msg, "")
		cancel()
	}
	return nil
}
