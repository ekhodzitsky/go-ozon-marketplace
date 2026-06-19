package fxmodules

import "go.uber.org/fx"

// Config provides the concrete service configuration so it can be injected both
// as-is and as the narrower module-specific config interfaces.
func Config[T any](cfg T) fx.Option {
	return fx.Provide(func() T { return cfg })
}
