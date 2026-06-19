package kafka

import "github.com/IBM/sarama"

// Producer отправляет сообщения в Kafka-топики.
type Producer interface {
	SendMessage(topic string, key, value []byte) error
	Close() error
}

// SyncProducer оборачивает Sarama sync producer в упрощённый интерфейс.
type SyncProducer struct {
	producer sarama.SyncProducer
}

// NewSyncProducer создаёт продюсер, подключенный к заданным брокерам.
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

// SendMessage отправляет одно сообщение в топик.
func (p *SyncProducer) SendMessage(topic string, key, value []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(value),
	}
	_, _, err := p.producer.SendMessage(msg)
	return err
}

// Close останавливает продюсер.
func (p *SyncProducer) Close() error {
	return p.producer.Close()
}
