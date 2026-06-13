package main

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/config"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log, err := logger.New(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		panic(err)
	}

	tp, err := tracing.InitTracer("analytics-service", cfg.OTELExporterOTLPEndpoint)
	if err != nil {
		log.Fatal("init tracer", zap.Error(err))
	}
	defer func() {
		if err := tracing.ShutdownTracer(tp, context.Background()); err != nil {
			log.Error("shutdown tracer", zap.Error(err))
		}
	}()

	application, err := app.New()
	if err != nil {
		log.Fatal("init application", zap.Error(err))
	}
	application.Run()
}
