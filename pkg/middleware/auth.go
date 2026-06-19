package middleware

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetUserID достаёт user_id из контекста.
func GetUserID(ctx context.Context) (string, bool) {
	if id, ok := auth.IdentityFromContext(ctx); ok {
		return id.UserID, true
	}
	v, ok := ctx.Value(auth.ContextKeyUserID).(string)
	return v, ok
}

// GetRole достаёт роль из контекста.
func GetRole(ctx context.Context) (auth.Role, bool) {
	if id, ok := auth.IdentityFromContext(ctx); ok {
		return id.Role, true
	}
	v := ctx.Value(auth.ContextKeyRole)
	if v == nil {
		return auth.RoleUser, false
	}
	switch r := v.(type) {
	case auth.Role:
		return r, true
	case string:
		return auth.Role(r), true
	}
	return auth.RoleUser, false
}

// RequireRole возвращает PermissionDenied, если роль в контексте не входит в allowed.
func RequireRole(ctx context.Context, allowed ...auth.Role) error {
	role, ok := GetRole(ctx)
	if !ok {
		return status.Error(codes.PermissionDenied, "missing role")
	}
	for _, a := range allowed {
		if role == a {
			return nil
		}
	}
	return status.Errorf(codes.PermissionDenied, "role %s not allowed", role)
}

func isPublicEndpoint(method string) bool {
	public := []string{
		"/user.v1.UserService/Register",
		"/user.v1.UserService/Login",
		"/grpc.health.v1.Health/Check",
		"/catalog.v1.CatalogService/GetProduct",
		"/catalog.v1.CatalogService/ListProducts",
		"/catalog.v1.CatalogService/SearchProducts",
	}
	for _, p := range public {
		if method == p {
			return true
		}
	}
	return false
}
