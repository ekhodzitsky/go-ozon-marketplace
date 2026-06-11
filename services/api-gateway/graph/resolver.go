package graph

import (
	"time"

	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
)

// Resolver serves as dependency injection for the app.
type Resolver struct {
	UserService    userv1.UserServiceClient
	CatalogService catalogv1.CatalogServiceClient
	CallTimeout    time.Duration
	QueryTimeout   time.Duration
}
