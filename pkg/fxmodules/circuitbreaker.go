package fxmodules

import (
	"time"

	"github.com/sony/gobreaker"
	"go.uber.org/fx"
)

// CircuitBreaker отдаёт настроенный sony/gobreaker как fx-модуль.
func CircuitBreaker(name string) fx.Option {
	return fx.Provide(func() *gobreaker.CircuitBreaker {
		return gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:        name,
			MaxRequests: 2,
			Interval:    0,
			Timeout:     30 * time.Second,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 5
			},
		})
	})
}
