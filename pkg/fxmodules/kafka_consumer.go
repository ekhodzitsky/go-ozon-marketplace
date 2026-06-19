package fxmodules

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/kafka"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// KafkaConsumerConfig — настройки для запуска Kafka consumer group.
type KafkaConsumerConfig interface {
	GetKafkaBrokers() []string
	GetKafkaConsumerGroup() string
	GetKafkaTopics() []string
	GetKafkaDLQTopic() string
	GetKafkaMaxRetries() int
	GetKafkaInitialBackoff() time.Duration
	GetKafkaProcessTimeout() time.Duration
}

// KafkaConsumer подключает kafka.Consumer к fx-приложению.
// Вызывающий должен предоставить реализацию kafka.Processor.
// Можно опционально передать kafka.IsPermanentError, чтобы переопределить поведение ретраев.
// Настройки берутся из KafkaConsumerConfig через DI, чтобы тесты могли их переопределять.
func KafkaConsumer(cfg KafkaConsumerConfig) fx.Option {
	return fx.Options(
		fx.Provide(func() KafkaConsumerConfig { return cfg }),
		fx.Provide(func(
			cfg KafkaConsumerConfig,
			processor kafka.Processor,
			log *zap.Logger,
			optional struct {
				fx.In
				IsPermanent kafka.IsPermanentError `optional:"true"`
			},
		) (*kafka.Consumer, error) {
			kcfg := kafka.Config{
				Brokers:        cfg.GetKafkaBrokers(),
				GroupID:        cfg.GetKafkaConsumerGroup(),
				Topics:         cfg.GetKafkaTopics(),
				DLQTopic:       cfg.GetKafkaDLQTopic(),
				MaxRetries:     defaultIfZero(cfg.GetKafkaMaxRetries(), 3),
				InitialBackoff: defaultIfZero(cfg.GetKafkaInitialBackoff(), 100*time.Millisecond),
				ProcessTimeout: defaultIfZero(cfg.GetKafkaProcessTimeout(), 10*time.Second),
			}
			if optional.IsPermanent != nil {
				kcfg.IsPermanent = optional.IsPermanent
			}
			return kafka.NewConsumer(kcfg, processor, log)
		}),
		fx.Invoke(func(lc fx.Lifecycle, c *kafka.Consumer, log *zap.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					c.Start(ctx)
					return nil
				},
				OnStop: func(ctx context.Context) error {
					if err := c.Close(); err != nil {
						log.Error("consumer close error", zap.Error(err))
					}
					return nil
				},
			})
		}),
	)
}

func defaultIfZero[T comparable](v, d T) T {
	var zero T
	if v == zero {
		return d
	}
	return v
}
