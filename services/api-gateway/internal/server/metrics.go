package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the metrics HTTP server.
type Metrics struct {
	server *http.Server
}

// NewMetrics creates a metrics server on the given port.
func NewMetrics(port int) *Metrics {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return &Metrics{
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		},
	}
}

// ListenAndServe starts the metrics server.
func (m *Metrics) ListenAndServe() error {
	if err := m.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("metrics server failed: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the metrics server.
func (m *Metrics) Shutdown(ctx context.Context) error {
	return m.server.Shutdown(ctx)
}
