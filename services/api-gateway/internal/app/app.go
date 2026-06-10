package app

import (
	"fmt"
	"log"
	"net/http"
	"os"

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
)

// App encapsulates the gateway application.
type App struct {
	cfg *config.Config
}

// New creates a new App.
func New(cfg *config.Config) *App {
	return &App{cfg: cfg}
}

// Run starts the HTTP server with GraphQL playground and query endpoint.
func (a *App) Run() error {
	userConn, err := grpc.NewClient(a.cfg.UserServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial user-service: %w", err)
	}
	defer userConn.Close()

	catalogConn, err := grpc.NewClient(a.cfg.CatalogServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))

	rl := middleware.NewRateLimiter(a.cfg.RateLimitRPS)
	http.Handle("/query", middleware.GraphQLMutationRateLimiter(rl)(srv))

	port := a.cfg.HTTPPort
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	return http.ListenAndServe(":"+port, nil)
}
