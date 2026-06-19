package kafka

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// Processor обрабатывает одно Kafka-сообщение. Реализации решают, как парсить
// сообщение и какое бизнес-действие выполнить.
type Processor interface {
	Process(ctx context.Context, msg *sarama.ConsumerMessage) error
}

// ProcessorFunc адаптирует обычную функцию к интерфейсу Processor.
type ProcessorFunc func(ctx context.Context, msg *sarama.ConsumerMessage) error

func (f ProcessorFunc) Process(ctx context.Context, msg *sarama.ConsumerMessage) error {
	return f(ctx, msg)
}

// IsPermanentError помогает консьюмеру решить, стоит ли пропустить ретраи
// и сразу отправить сообщение в DLQ.
type IsPermanentError func(error) bool

// Consumer оборачивает Sarama consumer group с ретраями, DLQ и graceful shutdown.
type Consumer struct {
	group     sarama.ConsumerGroup
	dlq       Producer
	cfg       Config
	processor Processor
	log       *zap.Logger
}

// NewConsumer собирает консьюмер из списка брокеров.
func NewConsumer(cfg Config, processor Processor, log *zap.Logger) (*Consumer, error) {
	cfg.setDefaults()

	sc, err := cfg.saramaConfig()
	if err != nil {
		return nil, err
	}

	group, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, sc)
	if err != nil {
		return nil, fmt.Errorf("new consumer group: %w", err)
	}

	c := newConsumer(group, cfg, processor, log)
	if cfg.DLQTopic != "" {
		c.dlq = newDLQProducer(cfg.Brokers, log)
	}
	return c, nil
}

func newDLQProducer(brokers []string, log *zap.Logger) Producer {
	if len(brokers) == 0 {
		return nil
	}
	producer, err := NewSyncProducer(brokers)
	if err != nil {
		log.Warn("failed to create dlq producer; dead-letter messages will be dropped", zap.Error(err))
		return nil
	}
	return producer
}

// NewConsumerFromGroup собирает консьюмер из готовой Sarama consumer group.
// Удобно для тестов и для тех, кто управляет Sarama-клиентом сам.
func NewConsumerFromGroup(group sarama.ConsumerGroup, cfg Config, processor Processor, log *zap.Logger) *Consumer {
	cfg.setDefaults()
	return newConsumer(group, cfg, processor, log)
}

func newConsumer(group sarama.ConsumerGroup, cfg Config, processor Processor, log *zap.Logger) *Consumer {
	if log == nil {
		log = zap.NewNop()
	}
	return &Consumer{
		group:     group,
		cfg:       cfg,
		processor: processor,
		log:       log,
	}
}

// SetDLQProducer прицепляет DLQ-продюсер после создания консьюмера.
// Если DLQTopic пустой, продюсер не используется.
func (c *Consumer) SetDLQProducer(p Producer) {
	c.dlq = p
}

// Start начинает чтение сообщений в фоновой горутине. После recoverable-ошибок
// консьюмер перезапускается сам, пока не отменится ctx.
func (c *Consumer) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			handler := &consumerHandler{consumer: c}

			if err := c.group.Consume(ctx, c.cfg.Topics, handler); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, sarama.ErrClosedConsumerGroup) {
					return
				}
				c.log.Error("consumer group error", zap.Error(err))
				select {
				case <-ctx.Done():
					return
				case <-time.After(3 * time.Second):
				}
			}
		}
	}()
}

// Close останавливает consumer group и опциональный DLQ-продюсер.
func (c *Consumer) Close() error {
	if c.dlq != nil {
		if err := c.dlq.Close(); err != nil {
			c.log.Error("dlq producer close error", zap.Error(err))
		}
	}
	return c.group.Close()
}
