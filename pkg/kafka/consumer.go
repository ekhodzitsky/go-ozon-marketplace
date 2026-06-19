package kafka

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// Processor handles a single Kafka message. Implementations decide how to parse
// the message and what business action to perform.
type Processor interface {
	Process(ctx context.Context, msg *sarama.ConsumerMessage) error
}

// ProcessorFunc adapts a plain function to the Processor interface.
type ProcessorFunc func(ctx context.Context, msg *sarama.ConsumerMessage) error

func (f ProcessorFunc) Process(ctx context.Context, msg *sarama.ConsumerMessage) error {
	return f(ctx, msg)
}

// IsPermanentError is used by the consumer to decide whether a processing error
// should skip retries and go straight to the DLQ.
type IsPermanentError func(error) bool

// Consumer wraps a Sarama consumer group with retries, DLQ support and graceful shutdown.
type Consumer struct {
	group     sarama.ConsumerGroup
	dlq       Producer
	cfg       Config
	processor Processor
	log       *zap.Logger
}

// NewConsumer builds a consumer from a list of brokers.
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

// NewConsumerFromGroup builds a consumer from an existing Sarama consumer group.
// Useful for tests and for callers that manage their own Sarama client.
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

// SetDLQProducer attaches a DLQ producer after construction. If DLQTopic is empty
// the producer is never used.
func (c *Consumer) SetDLQProducer(p Producer) {
	c.dlq = p
}

// Start begins consuming messages in a background goroutine. The consumer restarts
// automatically after recoverable errors until ctx is cancelled.
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

// Close shuts down the consumer group and the optional DLQ producer.
func (c *Consumer) Close() error {
	if c.dlq != nil {
		if err := c.dlq.Close(); err != nil {
			c.log.Error("dlq producer close error", zap.Error(err))
		}
	}
	return c.group.Close()
}
