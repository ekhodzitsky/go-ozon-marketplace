package consumer

import (
	"context"
	"encoding/json"
	"fmt"

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

// Processor maps incoming Kafka events to analytics use-case calls.
type Processor struct {
	uc  usecase.AnalyticsUsecase
	log *zap.Logger
}

// NewProcessor creates an analytics event processor.
func NewProcessor(uc usecase.AnalyticsUsecase, log *zap.Logger) *Processor {
	if log == nil {
		log = zap.NewNop()
	}
	return &Processor{uc: uc, log: log}
}

// Process implements kafka.Processor.
func (p *Processor) Process(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var event Event
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		p.log.Warn("failed to unmarshal analytics event", zap.Error(err))
		return nil
	}

	eventType, ok := mapEventType(event.EventType)
	if !ok {
		p.log.Warn("unknown analytics event type", zap.String("event_type", event.EventType))
		return nil
	}

	amount := event.Amount
	if amount == 0 && event.AmountCents != 0 {
		amount = float64(event.AmountCents) / 100.0
	}

	return p.uc.TrackEvent(ctx, eventType, event.OrderID, string(msg.Value), aggregationKey(msg, event), amount)
}

func mapEventType(eventType string) (domain.EventType, bool) {
	switch eventType {
	case "OrderCreated":
		return domain.EventTypeOrderCreated, true
	case "OrderConfirmed":
		return domain.EventTypeOrderConfirmed, true
	case "OrderCancelled":
		return domain.EventTypeOrderCancelled, true
	case "PaymentSuccess":
		return domain.EventTypePaymentSuccess, true
	default:
		return "", false
	}
}

func aggregationKey(msg *sarama.ConsumerMessage, event Event) string {
	if len(msg.Key) > 0 {
		return string(msg.Key)
	}
	return fmt.Sprintf("%s:%d:%d", msg.Topic, msg.Partition, msg.Offset)
}
