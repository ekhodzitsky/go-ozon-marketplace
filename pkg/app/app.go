// Package app provides a minimal, reusable application bootstrapper for fx-based services.
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

// Config exposes the fields every service needs to bootstrap logging and tracing.
type Config interface {
	GetLogLevel() string
	GetLogFormat() string
	GetOTELExporterOTLPEndpoint() string
}

// Run loads configuration, initializes the logger and tracer, builds the fx application
// and runs it. It blocks until the application shuts down.
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

// RunService is a typed convenience wrapper around Run. It removes the boilerplate
// of asserting the concrete config type in every service main.go.
func RunService[T Config](serviceName string, loadConfig func() (T, error), buildApp func(T) *fx.App) {
	Run(serviceName,
		func() (Config, error) { return loadConfig() },
		func(cfg Config) (*fx.App, error) { return buildApp(cfg.(T)), nil },
	)
}
