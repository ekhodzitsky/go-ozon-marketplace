package dlq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/kafka"
	"go.uber.org/zap"
)

const defaultSendTimeout = 5 * time.Second

// Event represents a dead-letter event.
type Event struct {
	EventType string    `json:"event_type"`
	Payload   string    `json:"payload"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

// Producer sends events to a Kafka DLQ topic.
type Producer struct {
	producer kafka.Producer
	topic    string
	log      *zap.Logger
	timeout  time.Duration
}

// NewProducer creates a new DLQ producer. It does not fail startup when Kafka
// is unavailable; instead it logs a warning and degrades to a no-op producer.
func NewProducer(brokers []string, topic string, log *zap.Logger) (*Producer, error) {
	if log == nil {
		log = zap.NewNop()
	}

	p := &Producer{
		topic:   topic,
		log:     log,
		timeout: defaultSendTimeout,
	}

	producer, err := kafka.NewSyncProducer(brokers)
	if err != nil {
		log.Warn("dlq producer unavailable; continuing with no-op dlq", zap.Error(err), zap.Strings("brokers", brokers), zap.String("topic", topic))
		return p, nil
	}
	p.producer = producer
	return p, nil
}

// NewProducerWithClient creates a DLQ producer from an existing producer.
func NewProducerWithClient(producer kafka.Producer, topic string, log *zap.Logger) *Producer {
	if log == nil {
		log = zap.NewNop()
	}
	return &Producer{producer: producer, topic: topic, log: log, timeout: defaultSendTimeout}
}

// SendToDLQ publishes an event to the DLQ topic asynchronously with a timeout.
// Errors are logged and not returned to the caller so that DLQ failures do not
// propagate into the request path.
func (p *Producer) SendToDLQ(eventType, payload, reason string) {
	if p == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- p.send(eventType, payload, reason)
		}()

		select {
		case err := <-done:
			if err != nil {
				p.log.Error("failed to send dlq event", zap.String("event_type", eventType), zap.Error(err))
			}
		case <-ctx.Done():
			p.log.Error("dlq send timed out", zap.String("event_type", eventType))
		}
	}()
}

func (p *Producer) send(eventType, payload, reason string) error {
	if p.producer == nil {
		return nil
	}

	event := Event{
		EventType: eventType,
		Payload:   payload,
		Reason:    reason,
		Timestamp: time.Now().UTC(),
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal dlq event: %w", err)
	}
	return p.producer.SendMessage(p.topic, []byte(eventType), data)
}

// Close shuts down the producer.
func (p *Producer) Close() error {
	if p == nil || p.producer == nil {
		return nil
	}
	return p.producer.Close()
}
