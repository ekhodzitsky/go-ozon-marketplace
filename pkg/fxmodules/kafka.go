package fxmodules

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/kafka"
	"go.uber.org/fx"
)

// KafkaProducerConfig — настройки Kafka-брокеров.
type KafkaProducerConfig interface {
	GetKafkaBrokers() []string
}

// KafkaProducer отдаёт общий Kafka-продюсер как fx-модуль.
// Настройки берутся из KafkaProducerConfig через DI, чтобы тесты могли их переопределять.
func KafkaProducer(cfg KafkaProducerConfig) fx.Option {
	return fx.Options(
		fx.Provide(func() KafkaProducerConfig { return cfg }),
		fx.Provide(func(lc fx.Lifecycle, cfg KafkaProducerConfig) (kafka.Producer, error) {
			p, err := kafka.NewSyncProducer(cfg.GetKafkaBrokers())
			if err != nil {
				return nil, err
			}
			lc.Append(fx.Hook{OnStop: func(ctx context.Context) error { return p.Close() }})
			return p, nil
		}),
	)
}
