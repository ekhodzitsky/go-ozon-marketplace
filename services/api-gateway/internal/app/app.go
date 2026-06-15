package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/server"
	"go.uber.org/zap"
)

// App encapsulates the gateway application.
type App struct {
	log     *zap.Logger
	http    *server.HTTP
	metrics *server.Metrics
}

// New creates a new App via wire-generated dependency injection.
func New(cfg *config.Config) (*App, func(), error) {
	return InitializeApp(cfg)
}

// Run starts servers and waits for shutdown signal.
func (a *App) Run() error {
	go func() {
		a.log.Info("starting gateway")
		if err := a.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.log.Fatal("gateway serve failed", zap.Error(err))
		}
	}()

	go func() {
		a.log.Info("starting metrics server")
		if err := a.metrics.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.log.Fatal("metrics server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	a.log.Info("shutting down gateway")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.metrics.Shutdown(shutdownCtx); err != nil {
		a.log.Error("metrics server shutdown error", zap.Error(err))
	}
	return a.http.Shutdown(shutdownCtx)
}
