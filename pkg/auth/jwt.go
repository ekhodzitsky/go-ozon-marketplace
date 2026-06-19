package auth

import (
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// CustomClaims extends jwt.RegisteredClaims with a role claim.
type CustomClaims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

// ParseJWT validates a JWT token string and returns parsed claims.
// It uses the default issuer and audience expected by the marketplace.
func ParseJWT(tokenStr, secret string) (*CustomClaims, error) {
	return parseJWT(tokenStr, secret, "go-ozon-marketplace", "api-gateway")
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
