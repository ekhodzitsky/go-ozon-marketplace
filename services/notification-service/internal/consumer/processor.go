package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/email"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/usecase"
	"go.uber.org/zap"
)

// Event represents a Kafka message payload.
type Event struct {
	EventType string `json:"event_type"`
	OrderID   string `json:"order_id"`
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
}

// Processor dispatches incoming Kafka events to the notification use case.
type Processor struct {
	uc  usecase.NotificationUsecase
	log *zap.Logger
}

// NewProcessor creates a notification event processor.
func NewProcessor(uc usecase.NotificationUsecase, log *zap.Logger) *Processor {
	if log == nil {
		log = zap.NewNop()
	}
	return &Processor{uc: uc, log: log}
}

// Process implements kafka.Processor.
func (p *Processor) Process(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var event Event
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		p.log.Warn("failed to unmarshal notification event", zap.Error(err))
		return nil // poison message, do not retry
	}

	subject, body, ok := messageFor(event)
	if !ok {
		p.log.Warn("unknown notification event type", zap.String("event_type", event.EventType))
		return nil
	}

	if err := p.uc.SendEmail(ctx, event.Email, subject, body); err != nil {
		return fmt.Errorf("send email for %s: %w", event.EventType, err)
	}
	return nil
}

func messageFor(event Event) (subject, body string, ok bool) {
	switch event.EventType {
	case "OrderConfirmed":
		return "Order Confirmed", fmt.Sprintf("Your order %s has been confirmed.", event.OrderID), true
	case "OrderCancelled":
		return "Order Cancelled", fmt.Sprintf("Your order %s has been cancelled.", event.OrderID), true
	case "PaymentFailed":
		return "Payment Failed", fmt.Sprintf("Payment for order %s failed.", event.OrderID), true
	default:
		return "", "", false
	}
}

// MaskEmail is kept exported so callers can redact addresses in logs/tests.
var MaskEmail = email.MaskEmail
