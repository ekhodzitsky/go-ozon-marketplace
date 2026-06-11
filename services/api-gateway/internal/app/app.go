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
	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/redis"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
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

	interceptorChain := grpc.WithChainUnaryInterceptor(authClientInterceptor, tracing.UnaryClientInterceptor())

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

	redisClient, err := redis.NewClient(context.Background(), a.cfg.RedisAddr)
	if err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	defer redisClient.Close()

	rl := middleware.NewRedisRateLimiter(redisClient, a.cfg.RateLimitRPS, a.cfg.RateLimitWindow)

	resolver := &graph.Resolver{
		UserService:      userv1.NewUserServiceClient(userConn),
		CatalogService:   catalogv1.NewCatalogServiceClient(catalogConn),
		OrderService:     orderv1.NewOrderServiceClient(orderConn),
		InventoryService: inventoryv1.NewInventoryServiceClient(inventoryConn),
		PaymentService:   paymentv1.NewPaymentServiceClient(paymentConn),
		CallTimeout:      a.cfg.DefaultCallTimeout,
		QueryTimeout:     a.cfg.DefaultQueryTimeout,
	}

	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

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
	mux.Handle("/", playground.Handler("GraphQL playground", "/query"))

	var gqlHandler http.Handler = srv
	gqlHandler = middleware.WithRateLimitIP(gqlHandler, a.cfg.TrustedProxies)
	gqlHandler = middleware.MaxBytesHandler(gqlHandler, a.cfg.MaxBodySizeBytes)
	gqlHandler = middleware.RateLimitHTTP(rl, a.cfg.TrustedProxies)(gqlHandler)

	mux.Handle("/query", gqlHandler)

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
