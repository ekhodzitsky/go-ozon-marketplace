// Package app — минимальный загрузчик fx-приложений для сервисов.
package app

import (
	"context"
	"fmt"
	"os"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Config — общие настройки, нужные каждому сервису для логирования и трассировки.
type Config interface {
	GetLogLevel() string
	GetLogFormat() string
	GetOTELExporterOTLPEndpoint() string
}

// Run загружает конфиг, инициализирует логгер и трассировку, собирает fx-приложение
// и запускает его. Блокируется до остановки приложения.
func Run(serviceName string, loadConfig func() (Config, error), buildApp func(Config) (*fx.App, error)) {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	log, err := logger.New(cfg.GetLogLevel(), cfg.GetLogFormat())
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}

	tp, err := tracing.InitTracer(serviceName, cfg.GetOTELExporterOTLPEndpoint())
	if err != nil {
		log.Fatal("init tracer", zap.Error(err))
	}
	defer func() {
		if err := tracing.ShutdownTracer(tp, context.Background()); err != nil {
			log.Error("shutdown tracer", zap.Error(err))
		}
	}()

	application, err := buildApp(cfg)
	if err != nil {
		log.Fatal("build application", zap.Error(err))
	}
	application.Run()
}

// RunService — типизированная обёртка над Run, чтобы не писать приведение типов в каждом main.go.
func RunService[T Config](serviceName string, loadConfig func() (T, error), buildApp func(T) *fx.App) {
	Run(serviceName,
		func() (Config, error) { return loadConfig() },
		func(cfg Config) (*fx.App, error) { return buildApp(cfg.(T)), nil },
	)
}
