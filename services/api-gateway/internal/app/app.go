package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/vektah/gqlparser/v2/ast"
	"google.golang.org/grpc"
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

// Run starts the HTTP server with GraphQL playground and query endpoint.
func (a *App) Run() error {
	userConn, err := grpc.NewClient(a.cfg.UserServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(authClientInterceptor),
	)
	if err != nil {
		return fmt.Errorf("dial user-service: %w", err)
	}
	defer userConn.Close()

	catalogConn, err := grpc.NewClient(a.cfg.CatalogServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(authClientInterceptor),
	)
	if err != nil {
		return fmt.Errorf("dial catalog-service: %w", err)
	}
	defer catalogConn.Close()

	resolver := &graph.Resolver{
		UserService:    userv1.NewUserServiceClient(userConn),
		CatalogService: catalogv1.NewCatalogServiceClient(catalogConn),
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

	mux := http.NewServeMux()
	mux.Handle("/", playground.Handler("GraphQL playground", "/query"))

	rl := middleware.NewRateLimiter(a.cfg.RateLimitRPS)
	mux.Handle("/query", middleware.GraphQLMutationRateLimiter(rl)(srv))

	port := a.cfg.HTTPPort
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	httpSrv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("gateway serve failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down gateway")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpSrv.Shutdown(ctx)
}
