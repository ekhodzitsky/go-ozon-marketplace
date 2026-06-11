package dlq

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
)

// Event represents a dead-letter event.
type Event struct {
	EventType string    `json:"event_type"`
	Payload   string    `json:"payload"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

// Producer sends events to a Kafka DLQ topic.
type Producer struct {
	producer sarama.SyncProducer
	topic    string
}

// NewProducer creates a new DLQ producer.
func NewProducer(brokers []string, topic string) (*Producer, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForLocal
	cfg.Producer.Retry.Max = 3
	cfg.Producer.Return.Successes = true
	p, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		return nil, fmt.Errorf("new sync producer: %w", err)
	}
	return &Producer{producer: p, topic: topic}, nil
}

// NewProducerWithClient creates a DLQ producer from an existing sync producer.
func NewProducerWithClient(producer sarama.SyncProducer, topic string) *Producer {
	return &Producer{producer: producer, topic: topic}
}

// SendToDLQ publishes an event to the DLQ topic.
func (p *Producer) SendToDLQ(eventType, payload, reason string) error {
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
	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.ByteEncoder([]byte(eventType)),
		Value: sarama.ByteEncoder(data),
	}
	_, _, err = p.producer.SendMessage(msg)
	return err
}

// Close shuts down the producer.
func (p *Producer) Close() error {
	return p.producer.Close()
}
