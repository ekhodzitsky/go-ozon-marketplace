package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CustomClaims extends jwt.RegisteredClaims with a role claim.
type CustomClaims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

type contextKey string

const (
	// ContextKeyUserID carries the authenticated user id in context.
	ContextKeyUserID contextKey = "user_id"
	// ContextKeyRole carries the authenticated user role in context.
	ContextKeyRole contextKey = "role"
	// ContextKeyAuthorizationHeader carries the raw Authorization header for gRPC forwarding.
	ContextKeyAuthorizationHeader contextKey = "authorization_header"
)

// Role represents user roles.
type Role string

const (
	RoleUser    Role = "user"
	RoleAdmin   Role = "admin"
	RoleService Role = "service"
)

// ParseJWT validates a JWT token string and returns parsed claims.
func ParseJWT(tokenStr, secret string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || claims.Subject == "" {
		return nil, fmt.Errorf("missing subject in token")
	}
	if claims.Issuer != "go-ozon-marketplace" {
		return nil, fmt.Errorf("invalid token issuer")
	}
	if !audienceContains(claims.Audience, "api-gateway") {
		return nil, fmt.Errorf("invalid token audience")
	}
	if claims.ID == "" {
		return nil, fmt.Errorf("missing token id")
	}
	if claims.NotBefore != nil && time.Now().Before(claims.NotBefore.Time) {
		return nil, fmt.Errorf("token not valid yet")
	}
	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return nil, fmt.Errorf("token expired")
	}
	if claims.Role == "" {
		claims.Role = string(RoleUser)
	}
	return claims, nil
}

// ParseBearer extracts the token from "Bearer <token>" and parses it.
func ParseBearer(bearer, secret string) (*CustomClaims, error) {
	tokenStr := strings.TrimPrefix(bearer, "Bearer ")
	if tokenStr == bearer {
		return nil, fmt.Errorf("invalid authorization header format")
	}
	return ParseJWT(tokenStr, secret)
}

func audienceContains(aud jwt.ClaimStrings, target string) bool {
	for _, a := range aud {
		if a == target {
			return true
		}
	}
	return false
}
