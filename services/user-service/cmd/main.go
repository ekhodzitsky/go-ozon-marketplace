package main

import (
	"context"
	"log"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/config"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logStd, err := logger.New(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}

	tp, err := tracing.InitTracer("user-service", cfg.OTELExporterOTLPEndpoint)
	if err != nil {
		logStd.Fatal("init tracer", zap.Error(err))
	}
	defer func() {
		if err := tracing.ShutdownTracer(tp, context.Background()); err != nil {
			logStd.Error("shutdown tracer", zap.Error(err))
		}
	}()

	application := app.New()
	application.Run()
}
