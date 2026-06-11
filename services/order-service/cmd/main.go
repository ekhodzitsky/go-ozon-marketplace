package main

import (
	"context"
	"net/http"
	_ "net/http/pprof"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/config"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()
	log, err := logger.New(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		panic(err)
	}

	tp, err := tracing.InitTracer("order-service", cfg.OTELExporterOTLPEndpoint)
	if err != nil {
		log.Fatal("init tracer", zap.Error(err))
	}
	defer func() {
		if err := tracing.ShutdownTracer(tp, context.Background()); err != nil {
			log.Error("shutdown tracer", zap.Error(err))
		}
	}()

	go func() {
		log.Info("starting pprof server", zap.String("addr", "localhost:6060"))
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			log.Error("pprof server error", zap.Error(err))
		}
	}()

	application := app.New()
	application.Run()
}
