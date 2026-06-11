package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/abtesting"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/circuitbreaker"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/redis"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/admin"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/ws"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// App encapsulates the gateway application.
type App struct {
	cfg *config.Config
}

// New creates a new App.
func New(cfg *config.Config) *App {
	return &App{cfg: cfg}
}

func authClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if opCtx := graphql.GetOperationContext(ctx); opCtx != nil {
		if auth := opCtx.Headers.Get("Authorization"); auth != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", auth)
		}
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

func circuitBreakerClientInterceptor(cb *circuitbreaker.CircuitBreaker) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return cb.Call(func() error {
			return invoker(ctx, method, req, reply, cc, opts...)
		})
	}
}

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := ""
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allowed = o
					break
				}
			}
			if allowed != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowed)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientCreds(cfg *config.Config) (credentials.TransportCredentials, error) {
	if cfg.CertPath != "" {
		return server.LoadClientMTLSCredentials(
			filepath.Join(cfg.CertPath, "server-cert.pem"),
			filepath.Join(cfg.CertPath, "server-key.pem"),
			filepath.Join(cfg.CertPath, "ca-cert.pem"),
			"",
		)
	}
	return insecure.NewCredentials(), nil
}

// Run starts the HTTP server with GraphQL playground and query endpoint.
func (a *App) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.DefaultCallTimeout)
	defer cancel()

	log, err := logger.New(a.cfg.LogLevel, a.cfg.LogFormat)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	creds, err := clientCreds(a.cfg)
	if err != nil {
		return fmt.Errorf("load tls credentials: %w", err)
	}

	cb := circuitbreaker.New(5, 2, 30*time.Second)
	interceptorChain := grpc.WithChainUnaryInterceptor(authClientInterceptor, circuitBreakerClientInterceptor(cb), tracing.UnaryClientInterceptor())

	userConn, err := grpc.DialContext(ctx, a.cfg.UserServiceAddr,
		grpc.WithTransportCredentials(creds),
		interceptorChain,
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("dial user-service: %w", err)
	}
	defer userConn.Close()

	catalogConn, err := grpc.DialContext(ctx, a.cfg.CatalogServiceAddr,
		grpc.WithTransportCredentials(creds),
		interceptorChain,
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("dial catalog-service: %w", err)
	}
	defer catalogConn.Close()

	orderConn, err := grpc.DialContext(ctx, a.cfg.OrderServiceAddr,
		grpc.WithTransportCredentials(creds),
		interceptorChain,
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("dial order-service: %w", err)
	}
	defer orderConn.Close()

	inventoryConn, err := grpc.DialContext(ctx, a.cfg.InventoryServiceAddr,
		grpc.WithTransportCredentials(creds),
		interceptorChain,
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("dial inventory-service: %w", err)
	}
	defer inventoryConn.Close()

	paymentConn, err := grpc.DialContext(ctx, a.cfg.PaymentServiceAddr,
		grpc.WithTransportCredentials(creds),
		interceptorChain,
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("dial payment-service: %w", err)
	}
	defer paymentConn.Close()

	analyticsConn, err := grpc.DialContext(ctx, a.cfg.AnalyticsServiceAddr,
		grpc.WithTransportCredentials(creds),
		interceptorChain,
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("dial analytics-service: %w", err)
	}
	defer analyticsConn.Close()

	redisClient, err := redis.NewClient(context.Background(), a.cfg.RedisAddr)
	if err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	defer redisClient.Close()

	// Feature flags engine.
	ffEngine := featureflags.NewEngine(redisClient)
	_ = ffEngine.LoadFromRedis()
	ffEngine.Register(&featureflags.Flag{Name: "new-checkout-flow", Enabled: false, Strategy: "default"})
	ffEngine.Register(&featureflags.Flag{Name: "fast-search", Enabled: false, Strategy: "default"})
	ffEngine.Register(&featureflags.Flag{Name: "discount-system", Enabled: false, Strategy: "default"})
	ffEngine.Register(&featureflags.Flag{Name: "real-time-updates", Enabled: false, Strategy: "default"})
	_ = ffEngine.SaveToRedis()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = ffEngine.LoadFromRedis()
			case <-ffEngine.Done():
				return
			}
		}
	}()

	// A/B testing experiments.
	abExperiments := []*abtesting.Experiment{
		{
			Name: "checkout-button-color",
			Variations: []abtesting.Variation{
				{Name: "control", Weight: 50},
				{Name: "green", Weight: 50},
			},
		},
		{
			Name: "search-algorithm",
			Variations: []abtesting.Variation{
				{Name: "v1", Weight: 70},
				{Name: "v2", Weight: 30},
			},
		},
	}

	rl := middleware.NewRoleRateLimiter(redisClient, a.cfg.RateLimitUserRPS, a.cfg.RateLimitAdminRPS, a.cfg.RateLimitWindow)

	hub := ws.NewHub()
	go hub.Run()

	go func() {
		ws.StartRedisPubSub(context.Background(), redisClient, hub)
	}()

	resolver := &graph.Resolver{
		UserService:        userv1.NewUserServiceClient(userConn),
		CatalogService:     catalogv1.NewCatalogServiceClient(catalogConn),
		OrderService:       orderv1.NewOrderServiceClient(orderConn),
		InventoryService:   inventoryv1.NewInventoryServiceClient(inventoryConn),
		PaymentService:     paymentv1.NewPaymentServiceClient(paymentConn),
		AnalyticsService:   analyticsv1.NewAnalyticsServiceClient(analyticsConn),
		FeatureFlagsEngine: ffEngine,
		ABExperiments:      abExperiments,
		Hub:                hub,
		Redis:              redisClient,
		CallTimeout:        a.cfg.DefaultCallTimeout,
		QueryTimeout:       a.cfg.DefaultQueryTimeout,
	}

	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.Websocket{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		opCtx := graphql.GetOperationContext(ctx)
		if opCtx != nil && opCtx.Operation != nil && opCtx.Operation.Operation == ast.Mutation {
			ip := middleware.RateLimitIPFromContext(ctx)
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

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWs(hub, w, r)
	})
	mux.Handle("/", playground.Handler("GraphQL playground", "/query"))

	var gqlHandler http.Handler = srv
	gqlHandler = middleware.WithRateLimitIP(gqlHandler, a.cfg.TrustedProxies)
	gqlHandler = middleware.MaxBytesHandler(gqlHandler, a.cfg.MaxBodySizeBytes)
	gqlHandler = middleware.RateLimitHTTP(rl, a.cfg.TrustedProxies)(gqlHandler)
	if a.cfg.JWTSecret != "" {
		gqlHandler = middleware.AuthHTTP(a.cfg.JWTSecret)(gqlHandler)
	}
	gqlHandler = corsMiddleware(a.cfg.CORSAllowedOrigins)(gqlHandler)

	mux.Handle("/query", gqlHandler)

	// Admin API.
	adminHandler := admin.NewHandler(ffEngine)
	mux.Handle("/admin/flags", adminHandler)
	mux.Handle("/admin/flags/", adminHandler)

	handler := middleware.RequestID(middleware.AccessLog(mux))

	port := a.cfg.HTTPPort
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	httpSrv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", a.cfg.MetricsPort),
		Handler: metricsMux,
	}

	go func() {
		log.Info("starting gateway", zap.String("addr", "http://localhost:"+port+"/"))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("gateway serve failed", zap.Error(err))
		}
	}()

	go func() {
		log.Info("starting metrics server", zap.Int("port", a.cfg.MetricsPort))
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("metrics server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down gateway")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Error("metrics server shutdown error", zap.Error(err))
	}
	return httpSrv.Shutdown(shutdownCtx)
}
