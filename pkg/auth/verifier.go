package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Verifier validates a raw token and returns an authenticated Identity.
type Verifier interface {
	Verify(ctx context.Context, token string) (Identity, error)
}

// JWTVerifier validates HS256 JWTs against a secret, issuer and audience.
type JWTVerifier struct {
	secret   string
	issuer   string
	audience string
}

// VerifyOption customizes a JWTVerifier.
type VerifyOption func(*JWTVerifier)

// WithIssuer overrides the default issuer.
func WithIssuer(issuer string) VerifyOption {
	return func(v *JWTVerifier) { v.issuer = issuer }
}

// WithAudience overrides the default audience.
func WithAudience(audience string) VerifyOption {
	return func(v *JWTVerifier) { v.audience = audience }
}

// NewJWTVerifier creates a verifier for the given HS256 secret.
func NewJWTVerifier(secret string, opts ...VerifyOption) *JWTVerifier {
	v := &JWTVerifier{
		secret:   secret,
		issuer:   "go-ozon-marketplace",
		audience: "api-gateway",
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// Verify parses and validates a JWT token string and returns the caller Identity.
func (v *JWTVerifier) Verify(ctx context.Context, token string) (Identity, error) {
	claims, err := parseJWT(token, v.secret, v.issuer, v.audience)
	if err != nil {
		return Identity{}, err
	}
	return Identity{
		UserID: claims.Subject,
		Role:   Role(claims.Role),
	}, nil
}

func parseJWT(tokenStr, secret, issuer, audience string) (*CustomClaims, error) {
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
	if claims.Issuer != issuer {
		return nil, fmt.Errorf("invalid token issuer")
	}
	if !audienceContains(claims.Audience, audience) {
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
