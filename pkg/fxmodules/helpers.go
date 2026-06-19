package fxmodules

import "go.uber.org/fx"

// Config предоставляет конкретный конфиг сервиса, чтобы его можно было инжектить
// и как есть, и как более узкий модульный интерфейс.
func Config[T any](cfg T) fx.Option {
	return fx.Provide(func() T { return cfg })
}
