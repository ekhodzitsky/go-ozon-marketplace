package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const ContextKeyUserID contextKey = "user_id"
const ContextKeyRole contextKey = "role"

type Role string

const (
	RoleUser    Role = "user"
	RoleAdmin   Role = "admin"
	RoleService Role = "service"
)

// CustomClaims extends jwt.RegisteredClaims with a role claim.
type CustomClaims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

// AuthUnaryInterceptor validates JWT bearer token from gRPC metadata
func AuthUnaryInterceptor(jwtSecret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Skip auth for public endpoints (register, login, health checks)
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
		if tokenStr == authHeader[0] {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization header format")
		}

		claims := &CustomClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}

		if claims.Subject == "" {
			return nil, status.Error(codes.Unauthenticated, "missing subject in token")
		}
		if claims.Issuer != "go-ozon-marketplace" {
			return nil, status.Error(codes.Unauthenticated, "invalid token issuer")
		}
		if !audienceContains(claims.Audience, "api-gateway") {
			return nil, status.Error(codes.Unauthenticated, "invalid token audience")
		}
		if claims.ID == "" {
			return nil, status.Error(codes.Unauthenticated, "missing token id")
		}
		if claims.NotBefore != nil && time.Now().Before(claims.NotBefore.Time) {
			return nil, status.Error(codes.Unauthenticated, "token not valid yet")
		}

		role := claims.Role
		if role == "" {
			role = string(RoleUser)
		}

		ctx = context.WithValue(ctx, ContextKeyUserID, claims.Subject)
		ctx = context.WithValue(ctx, ContextKeyRole, role)
		return handler(ctx, req)
	}
}

func audienceContains(aud jwt.ClaimStrings, target string) bool {
	for _, a := range aud {
		if a == target {
			return true
		}
	}
	return false
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

// GetUserID extracts user_id from context
func GetUserID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ContextKeyUserID).(string)
	return v, ok
}

// GetRole extracts role from context
func GetRole(ctx context.Context) (Role, bool) {
	v := ctx.Value(ContextKeyRole)
	if v == nil {
		return RoleUser, false
	}
	switch r := v.(type) {
	case Role:
		return r, true
	case string:
		return Role(r), true
	}
	return RoleUser, false
}

// RequireRole returns PermissionDenied if the context role is not in allowed.
func RequireRole(ctx context.Context, allowed ...Role) error {
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
