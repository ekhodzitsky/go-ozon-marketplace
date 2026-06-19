package fxmodules

import (
	"context"
	"errors"
	"net/http"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// HTTPService is the minimal interface an HTTP server needs for lifecycle management.
type HTTPService interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

// HTTPServer registers an HTTP service with the fx lifecycle.
// It starts the server in a background goroutine on OnStart and gracefully
// shuts it down on OnStop.
func HTTPServer[T HTTPService](name string) fx.Option {
	return fx.Invoke(func(lc fx.Lifecycle, srv T, log *zap.Logger) {
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				go func() {
					log.Info("starting http server", zap.String("server", name))
					if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
						log.Fatal("http server serve failed", zap.String("server", name), zap.Error(err))
					}
				}()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				log.Info("shutting down http server", zap.String("server", name))
				return srv.Shutdown(ctx)
			},
		})
	})
}
