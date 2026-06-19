package main

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()
	log, err := logger.New(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		panic(err)
	}

	tp, err := tracing.InitTracer("api-gateway", cfg.OTELExporterOTLPEndpoint)
	if err != nil {
		log.Fatal("init tracer", zap.Error(err))
	}
	defer func() {
		if err := tracing.ShutdownTracer(tp, context.Background()); err != nil {
			log.Error("shutdown tracer", zap.Error(err))
		}
	}()

	application, cleanup, err := app.New(cfg)
	if err != nil {
		log.Fatal("init app", zap.Error(err))
	}
	defer cleanup()

	if err := application.Run(); err != nil {
		log.Fatal("gateway error", zap.Error(err))
	}
}
