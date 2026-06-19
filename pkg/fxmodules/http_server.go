package fxmodules

import (
	"context"
	"errors"
	"net/http"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// HTTPService — минимальный интерфейс HTTP-сервера для управления жизненным циклом.
type HTTPService interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

// HTTPServer регистрирует HTTP-сервис в жизненном цикле fx.
// Стартует сервер в фоновой горутине на OnStart и аккуратно останавливает на OnStop.
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
