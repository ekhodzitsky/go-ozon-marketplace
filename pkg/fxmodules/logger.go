package fxmodules

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// LoggerConfig — настройки логирования.
type LoggerConfig interface {
	GetLogLevel() string
	GetLogFormat() string
}

// Logger отдаёт zap-логгер как fx-модуль.
// Настройки берутся из LoggerConfig через DI, чтобы тесты могли их переопределять.
func Logger(cfg LoggerConfig) fx.Option {
	return fx.Options(
		fx.Provide(func() LoggerConfig { return cfg }),
		fx.Provide(func(cfg LoggerConfig) (*zap.Logger, error) {
			return logger.New(cfg.GetLogLevel(), cfg.GetLogFormat())
		}),
	)
}
