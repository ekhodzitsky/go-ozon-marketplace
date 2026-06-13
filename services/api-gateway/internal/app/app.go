package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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
	"github.com/golang-jwt/jwt/v5"
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
	if graphql.HasOperationContext(ctx) {
		if auth := graphql.GetOperationContext(ctx).Headers.Get("Authorization"); auth != "" {
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
				if o == origin {
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

// serverNameFromAddr strips a port from a gRPC address to derive the TLS SNI hostname.
func serverNameFromAddr(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

func clientCreds(cfg *config.Config, addr string) (credentials.TransportCredentials, error) {
	if cfg.CertPath != "" {
		return server.LoadClientMTLSCredentials(
			filepath.Join(cfg.CertPath, "server-cert.pem"),
			filepath.Join(cfg.CertPath, "server-key.pem"),
			filepath.Join(cfg.CertPath, "ca-cert.pem"),
			serverNameFromAddr(addr),
		)
	}
	if cfg.InsecureSkipTLS {
		return insecure.NewCredentials(), nil
	}
	return nil, fmt.Errorf("no CERT_PATH configured and INSECURE_SKIP_TLS is false")
}

// Run starts the HTTP server with GraphQL playground and query endpoint.
func (a *App) Run() error {
	log, err := logger.New(a.cfg.LogLevel, a.cfg.LogFormat)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	closeLogged := func(name string, closer interface{ Close() error }) {
		if err := closer.Close(); err != nil {
			log.Error("failed to close "+name, zap.Error(err))
		}
	}

	cb := circuitbreaker.New(5, 2, 30*time.Second)
	interceptorChain := grpc.WithChainUnaryInterceptor(authClientInterceptor, circuitBreakerClientInterceptor(cb), tracing.UnaryClientInterceptor())

	userCreds, err := clientCreds(a.cfg, a.cfg.UserServiceAddr)
	if err != nil {
		return fmt.Errorf("user service tls: %w", err)
	}
	userConn, err := grpc.NewClient(a.cfg.UserServiceAddr,
		grpc.WithTransportCredentials(userCreds),
		interceptorChain,
	)
	if err != nil {
		return fmt.Errorf("dial user-service: %w", err)
	}
	defer func() { closeLogged("user service connection", userConn) }()

	catalogCreds, err := clientCreds(a.cfg, a.cfg.CatalogServiceAddr)
	if err != nil {
		return fmt.Errorf("catalog service tls: %w", err)
	}
	catalogConn, err := grpc.NewClient(a.cfg.CatalogServiceAddr,
		grpc.WithTransportCredentials(catalogCreds),
		interceptorChain,
	)
	if err != nil {
		return fmt.Errorf("dial catalog-service: %w", err)
	}
	defer func() { closeLogged("catalog service connection", catalogConn) }()

	orderCreds, err := clientCreds(a.cfg, a.cfg.OrderServiceAddr)
	if err != nil {
		return fmt.Errorf("order service tls: %w", err)
	}
	orderConn, err := grpc.NewClient(a.cfg.OrderServiceAddr,
		grpc.WithTransportCredentials(orderCreds),
		interceptorChain,
	)
	if err != nil {
		return fmt.Errorf("dial order-service: %w", err)
	}
	defer func() { closeLogged("order service connection", orderConn) }()

	inventoryCreds, err := clientCreds(a.cfg, a.cfg.InventoryServiceAddr)
	if err != nil {
		return fmt.Errorf("inventory service tls: %w", err)
	}
	inventoryConn, err := grpc.NewClient(a.cfg.InventoryServiceAddr,
		grpc.WithTransportCredentials(inventoryCreds),
		interceptorChain,
	)
	if err != nil {
		return fmt.Errorf("dial inventory-service: %w", err)
	}
	defer func() { closeLogged("inventory service connection", inventoryConn) }()

	paymentCreds, err := clientCreds(a.cfg, a.cfg.PaymentServiceAddr)
	if err != nil {
		return fmt.Errorf("payment service tls: %w", err)
	}
	paymentConn, err := grpc.NewClient(a.cfg.PaymentServiceAddr,
		grpc.WithTransportCredentials(paymentCreds),
		interceptorChain,
	)
	if err != nil {
		return fmt.Errorf("dial payment-service: %w", err)
	}
	defer func() { closeLogged("payment service connection", paymentConn) }()

	analyticsCreds, err := clientCreds(a.cfg, a.cfg.AnalyticsServiceAddr)
	if err != nil {
		return fmt.Errorf("analytics service tls: %w", err)
	}
	analyticsConn, err := grpc.NewClient(a.cfg.AnalyticsServiceAddr,
		grpc.WithTransportCredentials(analyticsCreds),
		interceptorChain,
	)
	if err != nil {
		return fmt.Errorf("dial analytics-service: %w", err)
	}
	defer func() { closeLogged("analytics service connection", analyticsConn) }()

	redisClient, err := redis.NewClient(context.Background(), a.cfg.RedisAddr)
	if err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	defer func() { closeLogged("redis client", redisClient) }()

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
		ws.ServeWs(hub, w, r, ws.Config{
			AllowedOrigins: a.cfg.CORSAllowedOrigins,
			JWTSecret:      a.cfg.JWTSecret,
		})
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
	adminHandler := http.Handler(admin.NewHandler(ffEngine))
	adminHandler = requireAdminHTTP(a.cfg.JWTSecret)(adminHandler)
	mux.Handle("/admin/flags", adminHandler)
	mux.Handle("/admin/flags/", adminHandler)

	// Health/readiness probes.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// Connectivity is established lazily via grpc.NewClient;
		// readiness can be extended with dependency checks later.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	handler := middleware.RequestID(middleware.AccessLog(mux))

	port := a.cfg.HTTPPort
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	httpSrv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
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

func requireAdminHTTP(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(auth, "Bearer ")
			if tokenStr == auth {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			token, err := jwt.ParseWithClaims(tokenStr, &middleware.CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method")
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			claims, ok := token.Claims.(*middleware.CustomClaims)
			if !ok || claims.Subject == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			role := claims.Role
			if role == "" {
				role = string(middleware.RoleUser)
			}
			if middleware.Role(role) != middleware.RoleAdmin {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
