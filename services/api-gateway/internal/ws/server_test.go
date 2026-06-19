package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOriginAllowed_NoAllowedOrigins(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	assert.True(t, originAllowed(req, nil))
	assert.True(t, originAllowed(req, []string{}))
}

func TestOriginAllowed_MatchingOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "https://app.example.com")

	assert.True(t, originAllowed(req, []string{"https://app.example.com"}))
}

func TestOriginAllowed_NonMatchingOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "https://evil.example.com")

	assert.False(t, originAllowed(req, []string{"https://app.example.com"}))
}

func TestOriginAllowed_EmptyOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	assert.True(t, originAllowed(req, []string{"https://app.example.com"}))
}

func TestAuthenticateUpgrade_NoVerifier(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	userID, err := authenticateUpgrade(req, nil)
	assert.NoError(t, err)
	assert.Empty(t, userID)
}

func TestAuthenticateUpgrade_MissingToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	_, err := authenticateUpgrade(req, auth.NewJWTVerifier("secret"))
	assert.Error(t, err)
}

func TestAuthenticateUpgrade_ValidQueryToken(t *testing.T) {
	secret := "test-secret-must-be-long-enough"
	token := buildToken(t, secret, "user-1")
	req := httptest.NewRequest(http.MethodGet, "/ws?token="+token, nil)

	userID, err := authenticateUpgrade(req, auth.NewJWTVerifier(secret))
	assert.NoError(t, err)
	assert.Equal(t, "user-1", userID)
}

func TestAuthenticateUpgrade_ValidHeaderToken(t *testing.T) {
	secret := "test-secret-must-be-long-enough"
	token := buildToken(t, secret, "user-1")
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	userID, err := authenticateUpgrade(req, auth.NewJWTVerifier(secret))
	assert.NoError(t, err)
	assert.Equal(t, "user-1", userID)
}

func TestAuthenticateUpgrade_InvalidToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws?token=invalid-token", nil)
	_, err := authenticateUpgrade(req, auth.NewJWTVerifier("secret"))
	assert.Error(t, err)
}

func TestNewHub(t *testing.T) {
	hub := NewHub()
	assert.NotNil(t, hub)
}

func TestBroadcast_NoSubscribers(t *testing.T) {
	hub := NewHub()

	err := Broadcast(hub, []byte(`{"topic":"orders","payload":{}}`))
	require.NoError(t, err)
	assert.NotNil(t, hub)
}

func buildToken(t *testing.T, secret, subject string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   subject,
		Issuer:    "go-ozon-marketplace",
		Audience:  jwt.ClaimStrings{"api-gateway"},
		ID:        "tok-1",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	s, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}
