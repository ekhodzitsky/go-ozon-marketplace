package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTVerifier_Verify(t *testing.T) {
	secret := "test-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        "tok-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Role: "admin",
	})
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	verifier := NewJWTVerifier(secret)
	id, err := verifier.Verify(context.Background(), signed)
	require.NoError(t, err)
	assert.Equal(t, "user-1", id.UserID)
	assert.Equal(t, RoleAdmin, id.Role)
}

func TestJWTVerifier_VerifyInvalidIssuer(t *testing.T) {
	secret := "test-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "evil",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        "tok-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	verifier := NewJWTVerifier(secret)
	_, err = verifier.Verify(context.Background(), signed)
	assert.Error(t, err)
}

func TestJWTVerifier_VerifyCustomAudience(t *testing.T) {
	secret := "test-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"order-service"},
			ID:        "tok-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	verifier := NewJWTVerifier(secret, WithAudience("order-service"))
	id, err := verifier.Verify(context.Background(), signed)
	require.NoError(t, err)
	assert.Equal(t, "user-1", id.UserID)
}

func TestJWTVerifier_EmptySecret(t *testing.T) {
	secret := "test-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        "tok-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	verifier := NewJWTVerifier("")
	_, err = verifier.Verify(context.Background(), signed)
	assert.Error(t, err)
}
