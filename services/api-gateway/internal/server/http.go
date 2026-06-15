package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/rs/cors"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	pkgmiddleware "github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/ws"
)

// HTTP holds the gateway HTTP server.
type HTTP struct {
	server *http.Server
}

// NewHTTP creates and configures the gateway HTTP server.
func NewHTTP(
	cfg *config.Config,
	resolver *graph.Resolver,
	hub *ws.Hub,
	rl pkgmiddleware.RateLimiter,
	adminHandler http.Handler,
) *HTTP {
	srv := newGraphQLServer(resolver, rl)

	r := chi.NewRouter()
	r.Use(pkgmiddleware.RequestID)
	r.Use(pkgmiddleware.AccessLog)
	r.Use(maxBytesMiddleware(cfg.MaxBodySizeBytes))
	r.Use(rateLimitIPMiddleware(cfg.TrustedProxies))
	r.Use(pkgmiddleware.RateLimitHTTP(rl, cfg.TrustedProxies))

	c := cors.New(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})
	r.Use(c.Handler)

	if cfg.JWTSecret != "" {
		r.Use(pkgmiddleware.AuthHTTP(cfg.JWTSecret))
	}

	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWs(hub, w, r, ws.Config{
			AllowedOrigins: cfg.CORSAllowedOrigins,
			JWTSecret:      cfg.JWTSecret,
		})
	})
	r.Get("/", playground.Handler("GraphQL playground", "/query"))
	r.Handle("/query", srv)

	r.Mount("/admin", adminHandler)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	return &HTTP{
		server: &http.Server{
			Addr:         ":" + cfg.HTTPPort,
			Handler:      r,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
	}
}

// ListenAndServe starts the HTTP server.
func (h *HTTP) ListenAndServe() error {
	if err := h.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("gateway serve failed: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server.
func (h *HTTP) Shutdown(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}

func newGraphQLServer(resolver *graph.Resolver, rl pkgmiddleware.RateLimiter) *handler.Server {
	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.Websocket{})
	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{Cache: lru.New[string](100)})

	srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		opCtx := graphql.GetOperationContext(ctx)
		if opCtx != nil && opCtx.Operation != nil && opCtx.Operation.Operation == ast.Mutation {
			ip := pkgmiddleware.RateLimitIPFromContext(ctx)
			if ip == "" {
				ip = "unknown"
			}
			if !rl.Allow(ctx, "mutation:"+ip) {
				return func(ctx context.Context) *graphql.Response {
					return &graphql.Response{
						Errors: []*gqlerror.Error{{Message: "rate limit exceeded"}},
					}
				}
			}
		}
		return next(ctx)
	})
	return srv
}

func maxBytesMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return pkgmiddleware.MaxBytesHandler(next, maxBytes)
	}
}

func rateLimitIPMiddleware(trusted []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return pkgmiddleware.WithRateLimitIP(next, trusted)
	}
}
