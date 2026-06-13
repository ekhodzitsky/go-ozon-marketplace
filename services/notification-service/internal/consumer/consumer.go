package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/email"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/usecase"
	"go.uber.org/zap"
)

const (
	maxRetries     = 2
	initialBackoff = 50 * time.Millisecond
)

// Event represents a Kafka message payload.
type Event struct {
	EventType string `json:"event_type"`
	OrderID   string `json:"order_id"`
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
}

// Consumer wraps a Sarama consumer group.
type Consumer struct {
	group       sarama.ConsumerGroup
	dlqProducer sarama.SyncProducer
	dlqTopic    string
	uc          usecase.NotificationUsecase
	log         *zap.Logger
	topics      []string
}

// NewConsumer creates a new Kafka consumer.
func NewConsumer(brokers []string, groupID string, topics []string, dlqTopic string, uc usecase.NotificationUsecase, log *zap.Logger) (*Consumer, error) {
	cfg := sarama.NewConfig()
	cfg.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, fmt.Errorf("new consumer group: %w", err)
	}

	return &Consumer{
		group:       group,
		dlqProducer: newDLQProducer(brokers, log),
		dlqTopic:    dlqTopic,
		uc:          uc,
		log:         log,
		topics:      topics,
	}, nil
}

func newDLQProducer(brokers []string, log *zap.Logger) sarama.SyncProducer {
	if len(brokers) == 0 {
		return nil
	}
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		log.Warn("failed to create DLQ producer; dead-letter messages will be dropped", zap.Error(err))
		return nil
	}
	return producer
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
				uc:          c.uc,
				dlqProducer: c.dlqProducer,
				dlqTopic:    c.dlqTopic,
				log:         c.log,
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
	if c.dlqProducer != nil {
		if err := c.dlqProducer.Close(); err != nil {
			c.log.Error("dlq producer close error", zap.Error(err))
		}
	}
	return c.group.Close()
}

type consumerHandler struct {
	uc          usecase.NotificationUsecase
	dlqProducer sarama.SyncProducer
	dlqTopic    string
	log         *zap.Logger
}

func (h *consumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		if err := h.processMessage(session, msg); err != nil {
			h.log.Error("failed to process message", zap.Error(err))
		}
	}
	return nil
}

func (h *consumerHandler) processMessage(session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) error {
	var event Event
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		h.log.Error("failed to unmarshal message", zap.Error(err))
		session.MarkMessage(msg, "")
		return nil
	}

	if !isKnownEvent(event.EventType) {
		h.log.Warn("unknown event type", zap.String("event_type", event.EventType))
		session.MarkMessage(msg, "")
		return nil
	}

	ctx, cancel := context.WithTimeout(session.Context(), 10*time.Second)
	defer cancel()

	var err error
	attempts := 0
	backoff := initialBackoff
	for attempts = 0; attempts <= maxRetries; attempts++ {
		err = h.sendEmailForEvent(ctx, event)
		if err == nil {
			break
		}
		if errors.Is(err, apperrors.ErrInvalidArgument) {
			break // permanent error, no point retrying
		}
		if attempts < maxRetries {
			select {
			case <-ctx.Done():
				err = ctx.Err()
				break
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	if err == nil {
		session.MarkMessage(msg, "")
		return nil
	}

	h.log.Error("failed to send email for event",
		zap.String("event_type", event.EventType),
		zap.String("order_id", event.OrderID),
		zap.String("to", email.MaskEmail(event.Email)),
		zap.Int("attempts", attempts),
		zap.Error(err),
	)

	if errors.Is(err, apperrors.ErrInvalidArgument) || attempts > maxRetries {
		h.sendToDLQ(event, err, attempts)
		session.MarkMessage(msg, "")
	}

	return err
}

func (h *consumerHandler) sendEmailForEvent(ctx context.Context, event Event) error {
	switch event.EventType {
	case "OrderConfirmed":
		return h.uc.SendEmail(ctx, event.Email, "Order Confirmed", fmt.Sprintf("Your order %s has been confirmed.", event.OrderID))
	case "OrderCancelled":
		return h.uc.SendEmail(ctx, event.Email, "Order Cancelled", fmt.Sprintf("Your order %s has been cancelled.", event.OrderID))
	case "PaymentFailed":
		return h.uc.SendEmail(ctx, event.Email, "Payment Failed", fmt.Sprintf("Payment for order %s failed.", event.OrderID))
	default:
		return nil
	}
}

func isKnownEvent(eventType string) bool {
	switch eventType {
	case "OrderConfirmed", "OrderCancelled", "PaymentFailed":
		return true
	default:
		return false
	}
}

func (h *consumerHandler) sendToDLQ(event Event, err error, attempts int) {
	if h.dlqProducer == nil || h.dlqTopic == "" {
		return
	}

	payload, jerr := json.Marshal(map[string]any{
		"original_event": event,
		"error":          err.Error(),
		"attempts":       attempts,
	})
	if jerr != nil {
		h.log.Error("failed to marshal DLQ message", zap.Error(jerr))
		return
	}

	_, _, perr := h.dlqProducer.SendMessage(&sarama.ProducerMessage{
		Topic: h.dlqTopic,
		Value: sarama.ByteEncoder(payload),
	})
	if perr != nil {
		h.log.Error("failed to produce DLQ message", zap.Error(perr))
	}
}
