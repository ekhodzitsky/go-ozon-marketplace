package fxmodules

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// LoggerConfig exposes logging settings.
type LoggerConfig interface {
	GetLogLevel() string
	GetLogFormat() string
}

// Logger provides a zap logger as an fx module.
// Settings are resolved from LoggerConfig via DI, so tests can override them.
func Logger(cfg LoggerConfig) fx.Option {
	return fx.Options(
		fx.Provide(func() LoggerConfig { return cfg }),
		fx.Provide(func(cfg LoggerConfig) (*zap.Logger, error) {
			return logger.New(cfg.GetLogLevel(), cfg.GetLogFormat())
		}),
	)
}
