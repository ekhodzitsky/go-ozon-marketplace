package auth

import (
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// CustomClaims — стандартные JWT-claims плюс роль.
type CustomClaims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

// ParseJWT проверяет JWT и возвращает распаршенные claims.
// Использует дефолтный issuer и audience маркетплейса.
func ParseJWT(tokenStr, secret string) (*CustomClaims, error) {
	return parseJWT(tokenStr, secret, "go-ozon-marketplace", "api-gateway")
}

// ParseBearer вытаскивает токен из "Bearer <token>" и парсит его.
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
