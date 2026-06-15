package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	pkgredis "github.com/ekhodzitsky/go-ozon-marketplace/pkg/redis"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/admin"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/clients"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/ws"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"github.com/redis/go-redis/v9"
)

// App encapsulates the gateway application.
type App struct {
	log     *zap.Logger
	http    *server.HTTP
	metrics *server.Metrics
}

// New creates a new App via wire-generated dependency injection.
func New(cfg *config.Config) (*App, func(), error) {
	return InitializeApp(cfg)
}

// Run starts servers and waits for shutdown signal.
func (a *App) Run() error {
	go func() {
		a.log.Info("starting gateway")
		if err := a.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.log.Fatal("gateway serve failed", zap.Error(err))
		}
	}()

	go func() {
		a.log.Info("starting metrics server")
		if err := a.metrics.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.log.Fatal("metrics server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	a.log.Info("shutting down gateway")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.metrics.Shutdown(shutdownCtx); err != nil {
		a.log.Error("metrics server shutdown error", zap.Error(err))
	}
	return a.http.Shutdown(shutdownCtx)
}

func provideContext() context.Context {
	return context.Background()
}

func provideLogger(cfg *config.Config) (*zap.Logger, error) {
	return logger.New(cfg.LogLevel, cfg.LogFormat)
}

func provideCircuitBreaker() *circuitbreaker.CircuitBreaker {
	return circuitbreaker.New(5, 2, 30*time.Second)
}

func provideClientFactory(cfg *config.Config, cb *circuitbreaker.CircuitBreaker) *clients.Factory {
	return clients.NewFactory(cfg, cb)
}

func provideRedis(ctx context.Context, cfg *config.Config) (*redis.Client, func(), error) {
	client, err := pkgredis.NewClient(ctx, cfg.RedisAddr)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		_ = client.Close()
	}
	return client, cleanup, nil
}

func provideFeatureFlags(ctx context.Context, redisClient *redis.Client) (*featureflags.Engine, error) {
	engine := featureflags.NewEngine(redisClient)
	_ = engine.LoadFromRedis()
	engine.Register(&featureflags.Flag{Name: "new-checkout-flow", Enabled: false, Strategy: "default"})
	engine.Register(&featureflags.Flag{Name: "fast-search", Enabled: false, Strategy: "default"})
	engine.Register(&featureflags.Flag{Name: "discount-system", Enabled: false, Strategy: "default"})
	engine.Register(&featureflags.Flag{Name: "real-time-updates", Enabled: false, Strategy: "default"})
	_ = engine.SaveToRedis()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = engine.LoadFromRedis()
			case <-engine.Done():
				return
			}
		}
	}()
	return engine, nil
}

func provideRateLimiter(redisClient *redis.Client, cfg *config.Config) middleware.RateLimiter {
	return middleware.NewRoleRateLimiter(redisClient, cfg.RateLimitUserRPS, cfg.RateLimitAdminRPS, cfg.RateLimitWindow)
}

func provideHub(redisClient *redis.Client) *ws.Hub {
	hub := ws.NewHub()
	go hub.Run()
	go func() {
		ws.StartRedisPubSub(context.Background(), redisClient, hub)
	}()
	return hub
}

func provideAdminHandler(ffEngine *featureflags.Engine, cfg *config.Config) http.Handler {
	return admin.NewRouter(admin.NewHandler(ffEngine), cfg.JWTSecret)
}

type serviceConnections struct {
	user      *grpc.ClientConn
	catalog   *grpc.ClientConn
	order     *grpc.ClientConn
	inventory *grpc.ClientConn
	payment   *grpc.ClientConn
	analytics *grpc.ClientConn
}

func (c *serviceConnections) close() {
	if c.user != nil {
		_ = c.user.Close()
	}
	if c.catalog != nil {
		_ = c.catalog.Close()
	}
	if c.order != nil {
		_ = c.order.Close()
	}
	if c.inventory != nil {
		_ = c.inventory.Close()
	}
	if c.payment != nil {
		_ = c.payment.Close()
	}
	if c.analytics != nil {
		_ = c.analytics.Close()
	}
}

func provideResolver(
	ctx context.Context,
	factory *clients.Factory,
	ffEngine *featureflags.Engine,
	hub *ws.Hub,
	redisClient *redis.Client,
	cfg *config.Config,
) (*graph.Resolver, func(), error) {
	conns := &serviceConnections{}
	cleanup := func() {
		conns.close()
	}

	userConn, err := factory.NewClient(ctx, cfg.UserServiceAddr)
	if err != nil {
		return nil, cleanup, fmt.Errorf("user client: %w", err)
	}
	conns.user = userConn

	catalogConn, err := factory.NewClient(ctx, cfg.CatalogServiceAddr)
	if err != nil {
		return nil, cleanup, fmt.Errorf("catalog client: %w", err)
	}
	conns.catalog = catalogConn

	orderConn, err := factory.NewClient(ctx, cfg.OrderServiceAddr)
	if err != nil {
		return nil, cleanup, fmt.Errorf("order client: %w", err)
	}
	conns.order = orderConn

	inventoryConn, err := factory.NewClient(ctx, cfg.InventoryServiceAddr)
	if err != nil {
		return nil, cleanup, fmt.Errorf("inventory client: %w", err)
	}
	conns.inventory = inventoryConn

	paymentConn, err := factory.NewClient(ctx, cfg.PaymentServiceAddr)
	if err != nil {
		return nil, cleanup, fmt.Errorf("payment client: %w", err)
	}
	conns.payment = paymentConn

	analyticsConn, err := factory.NewClient(ctx, cfg.AnalyticsServiceAddr)
	if err != nil {
		return nil, cleanup, fmt.Errorf("analytics client: %w", err)
	}
	conns.analytics = analyticsConn

	resolver := &graph.Resolver{
		UserService:        userv1.NewUserServiceClient(userConn),
		CatalogService:     catalogv1.NewCatalogServiceClient(catalogConn),
		OrderService:       orderv1.NewOrderServiceClient(orderConn),
		InventoryService:   inventoryv1.NewInventoryServiceClient(inventoryConn),
		PaymentService:     paymentv1.NewPaymentServiceClient(paymentConn),
		AnalyticsService:   analyticsv1.NewAnalyticsServiceClient(analyticsConn),
		FeatureFlagsEngine: ffEngine,
		ABExperiments:      defaultExperiments(),
		Hub:                hub,
		Redis:              redisClient,
		CallTimeout:        cfg.DefaultCallTimeout,
		QueryTimeout:       cfg.DefaultQueryTimeout,
	}
	return resolver, cleanup, nil
}

func defaultExperiments() []*abtesting.Experiment {
	return []*abtesting.Experiment{
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
