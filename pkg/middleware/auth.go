package middleware

import (
	"context"
	"strings"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthUnaryInterceptor validates JWT bearer token from gRPC metadata.
func AuthUnaryInterceptor(jwtSecret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if isPublicEndpoint(info.FullMethod) {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		tokenStr := strings.TrimPrefix(authHeader[0], "Bearer ")
		claims, err := auth.ParseJWT(tokenStr, jwtSecret)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}

		ctx = context.WithValue(ctx, auth.ContextKeyUserID, claims.Subject)
		ctx = context.WithValue(ctx, auth.ContextKeyRole, auth.Role(claims.Role))
		return handler(ctx, req)
	}
}

// GetUserID extracts user_id from context.
func GetUserID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(auth.ContextKeyUserID).(string)
	return v, ok
}

// GetRole extracts role from context.
func GetRole(ctx context.Context) (auth.Role, bool) {
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

// RequireRole returns PermissionDenied if the context role is not in allowed.
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
