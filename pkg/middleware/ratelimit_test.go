package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockLimiter struct {
	allowed bool
}

func (m *mockLimiter) Allow(ctx context.Context, key string) bool {
	return m.allowed
}

func TestRateLimitHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		allowed  bool
		wantCode int
	}{
		{"allowed", true, http.StatusOK},
		{"denied", false, http.StatusTooManyRequests},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rl := &mockLimiter{allowed: tt.allowed}
			h := RateLimitHTTP(rl, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.RemoteAddr = "127.0.0.1:1234"
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			assert.Equal(t, tt.wantCode, rr.Code)
		})
	}
}

func TestClientIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		remote  string
		xff     string
		trusted []string
		want    string
	}{
		{"no_proxy", "192.168.1.1:1234", "", nil, "192.168.1.1"},
		{"trusted_xff", "10.0.0.1:1234", "1.2.3.4, 5.6.7.8", []string{"10.0.0.0/8"}, "5.6.7.8"},
		{"untrusted_xff", "192.168.1.1:1234", "1.2.3.4", []string{"10.0.0.0/8"}, "192.168.1.1"},
		{"empty_xff", "10.0.0.1:1234", "", []string{"10.0.0.0/8"}, "10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remote
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			got := ClientIP(req, tt.trusted)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMaxBytesHandler(t *testing.T) {
	t.Parallel()

	handler := MaxBytesHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}), 10)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hello world!!"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
}
