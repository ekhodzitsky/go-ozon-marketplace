package fxmodules

import (
	"context"

	"go.uber.org/fx"
)

// LifecycleRunner — фоновый воркер, который можно стартовать и остановить.
type LifecycleRunner interface {
	Start(ctx context.Context)
	Stop()
}

// Runner регистрирует LifecycleRunner в жизненном цикле fx.
// Используй для фоновых задач: outbox-релеев, recovery-циклов и т.п.
func Runner[T LifecycleRunner]() fx.Option {
	return fx.Invoke(func(lc fx.Lifecycle, r T) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				r.Start(ctx)
				return nil
			},
			OnStop: func(ctx context.Context) error {
				r.Stop()
				return nil
			},
		})
	})
}
