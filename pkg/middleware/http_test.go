package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestAuthHTTP_NoAuth_AllowsPublic(t *testing.T) {
	t.Parallel()

	handler := AuthHTTP("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthHTTP_InvalidAuth_Rejects(t *testing.T) {
	t.Parallel()

	handler := AuthHTTP("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called for invalid auth")
	}))

	tests := []struct {
		name string
		auth string
	}{
		{"malformed", "Basic dXNlcjpwYXNz"},
		{"bad_token", "Bearer not-a-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tt.auth)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}
}

func TestAuthHTTP_ValidAuth_SetsContext(t *testing.T) {
	t.Parallel()

	secret := "secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  "user-1",
			Issuer:   "go-ozon-marketplace",
			Audience: jwt.ClaimStrings{"api-gateway"},
		},
		Role: string(RoleAdmin),
	})
	tokenStr, err := token.SignedString([]byte(secret))
	requireNoError(t, err)

	var userID string
	var role Role
	handler := AuthHTTP(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, _ = GetUserID(r.Context())
		role, _ = GetRole(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "user-1", userID)
	assert.Equal(t, RoleAdmin, role)
}

func TestNewAccessLog_LogsRequest(t *testing.T) {
	t.Parallel()

	observed, logs := observer.New(zap.InfoLevel)
	log := zap.New(observed)

	handler := NewAccessLog(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req = req.WithContext(WithRequestID(req.Context(), "req-42"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	requireEqual(t, 1, logs.Len())
	entry := logs.All()[0]
	assert.Equal(t, "http request", entry.Message)
	assert.Equal(t, "POST", entry.Context[0].String)
	assert.Equal(t, "/test", entry.Context[1].String)
	assert.Equal(t, zap.Int("status", http.StatusCreated), entry.Context[3])
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKeyRequestID, id)
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func requireEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	assert.Equal(t, expected, actual)
}
