package fxmodules

import (
	"context"

	"go.uber.org/fx"
)

// LifecycleRunner is a background worker that can be started and stopped.
type LifecycleRunner interface {
	Start(ctx context.Context)
	Stop()
}

// Runner registers a LifecycleRunner with the fx lifecycle.
// Use it for background workers such as outbox relays or recovery loops.
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
