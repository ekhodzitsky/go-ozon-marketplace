package app

import (
	"fmt"
	"net/http"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/admin"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/ws"
)

func provideAdminHandler(ffEngine *featureflags.Engine, cfg *config.Config) http.Handler {
	var verifier auth.Verifier
	if cfg.JWTSecret != "" {
		verifier = auth.NewJWTVerifier(cfg.JWTSecret)
	}
	return admin.NewRouter(admin.NewHandler(ffEngine), verifier)
}

func provideHTTPServer(
	cfg *config.Config,
	resolver *graph.Resolver,
	hub *ws.Hub,
	rl middleware.RateLimiter,
	adminHandler http.Handler,
) *server.HTTP {
	return server.NewHTTP(cfg, resolver, hub, rl, adminHandler)
}

func provideMetricsServer(cfg *config.Config) *server.Metrics {
	return server.NewMetrics(fmt.Sprintf(":%d", cfg.MetricsPort))
}
