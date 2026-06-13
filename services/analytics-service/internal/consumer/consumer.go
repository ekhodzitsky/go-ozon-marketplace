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
	EventType   string  `json:"event_type"`
	OrderID     string  `json:"order_id"`
	UserID      string  `json:"user_id"`
	AmountCents int64   `json:"amount_cents"`
	Amount      float64 `json:"amount"`
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
	cfg.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, fmt.Errorf("new consumer group: %w", err)
	}

	return NewConsumerFromGroup(group, topics, uc, log), nil
}

// NewConsumerFromGroup builds a Consumer from an existing Sarama consumer group.
// It is useful for tests and for callers that manage their own Sarama client.
func NewConsumerFromGroup(group sarama.ConsumerGroup, topics []string, uc usecase.AnalyticsUsecase, log *zap.Logger) *Consumer {
	return &Consumer{
		group:  group,
		uc:     uc,
		log:    log,
		topics: topics,
	}
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

		aggKey := aggregationKey(msg, event)
		amount := event.Amount
		if amount == 0 && event.AmountCents != 0 {
			amount = float64(event.AmountCents) / 100.0
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var err error

		switch event.EventType {
		case "OrderCreated":
			err = h.trackWithRetry(ctx, domain.EventTypeOrderCreated, event.OrderID, string(msg.Value), aggKey, amount)
		case "OrderConfirmed":
			err = h.trackWithRetry(ctx, domain.EventTypeOrderConfirmed, event.OrderID, string(msg.Value), aggKey, amount)
		case "OrderCancelled":
			err = h.trackWithRetry(ctx, domain.EventTypeOrderCancelled, event.OrderID, string(msg.Value), aggKey, amount)
		case "PaymentSuccess":
			err = h.trackWithRetry(ctx, domain.EventTypePaymentSuccess, event.OrderID, string(msg.Value), aggKey, amount)
		default:
			h.log.Warn("unknown event type", zap.String("event_type", event.EventType))
		}

		if err != nil {
			h.log.Error("failed to track event after retries",
				zap.String("event_type", event.EventType),
				zap.Error(err),
			)
		} else {
			session.MarkMessage(msg, "")
		}

		cancel()
	}
	return nil
}

func (h *consumerHandler) trackWithRetry(ctx context.Context, eventType domain.EventType, aggregateID, payload, aggregationKey string, amount float64) error {
	const maxRetries = 3
	var err error
	backoff := 100 * time.Millisecond
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}
		err = h.uc.TrackEvent(ctx, eventType, aggregateID, payload, aggregationKey, amount)
		if err == nil {
			return nil
		}
		h.log.Warn("track event failed, retrying", zap.Int("attempt", i+1), zap.Error(err))
	}
	return err
}

func aggregationKey(msg *sarama.ConsumerMessage, event Event) string {
	if len(msg.Key) > 0 {
		return string(msg.Key)
	}
	return fmt.Sprintf("%s:%d:%d", msg.Topic, msg.Partition, msg.Offset)
}
