package outbox

import "github.com/IBM/sarama"

type Producer interface {
	SendMessage(topic string, key, value []byte) error
	Close() error
}

type SaramaProducer struct {
	producer sarama.SyncProducer
}

func NewSaramaProducer(brokers []string) (*SaramaProducer, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForLocal
	cfg.Producer.Retry.Max = 3
	cfg.Producer.Return.Successes = true
	p, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		return nil, err
	}
	return &SaramaProducer{producer: p}, nil
}

func (p *SaramaProducer) SendMessage(topic string, key, value []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(value),
	}
	_, _, err := p.producer.SendMessage(msg)
	return err
}

func (p *SaramaProducer) Close() error {
	return p.producer.Close()
}
