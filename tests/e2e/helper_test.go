//go:build e2e

package e2e

import (
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// e2eJWTSecret is a long secret that satisfies all service config validations.
const e2eJWTSecret = "this-is-a-very-long-test-secret-for-e2e-tests-only"

// adminToken returns a signed JWT with the admin role for the given user.
func adminToken(userID, secret string) string {
	now := time.Now().UTC()
	claims := middleware.CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
		Role: string(middleware.RoleAdmin),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		panic(err)
	}
	return tokenStr
}
