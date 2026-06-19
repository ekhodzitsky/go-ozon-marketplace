package kafka

import "github.com/IBM/sarama"

// Producer sends messages to Kafka topics.
type Producer interface {
	SendMessage(topic string, key, value []byte) error
	Close() error
}

// SyncProducer wraps a Sarama sync producer with a simplified interface.
type SyncProducer struct {
	producer sarama.SyncProducer
}

// NewSyncProducer creates a producer connected to the given brokers.
func NewSyncProducer(brokers []string) (*SyncProducer, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForLocal
	cfg.Producer.Retry.Max = 3
	cfg.Producer.Return.Successes = true
	p, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		return nil, err
	}
	return &SyncProducer{producer: p}, nil
}

// SendMessage produces a single message to topic.
func (p *SyncProducer) SendMessage(topic string, key, value []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(value),
	}
	_, _, err := p.producer.SendMessage(msg)
	return err
}

// Close shuts down the producer.
func (p *SyncProducer) Close() error {
	return p.producer.Close()
}
