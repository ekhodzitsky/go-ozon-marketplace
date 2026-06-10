package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_Allow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		rps       int
		requests  int
		wantAllow bool
	}{
		{
			name:      "under_limit",
			rps:       10,
			requests:  5,
			wantAllow: true,
		},
		{
			name:      "at_limit",
			rps:       10,
			requests:  10,
			wantAllow: true,
		},
		{
			name:      "over_limit",
			rps:       2,
			requests:  3,
			wantAllow: false,
		},
		{
			name:      "zero_rps_defaults_to_ten",
			rps:       0,
			requests:  5,
			wantAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rl := NewRateLimiter(tt.rps)

			var last bool
			for i := 0; i < tt.requests; i++ {
				last = rl.Allow("127.0.0.1")
			}
			assert.Equal(t, tt.wantAllow, last)
		})
	}
}

func TestGraphQLMutationRateLimiter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		requests   int
		wantStatus int
	}{
		{
			name:       "mutation_register_allowed",
			body:       `{"query":"mutation { register(email:\"a\",password:\"b\",name:\"c\") }"}`,
			requests:   1,
			wantStatus: http.StatusOK,
		},
		{
			name:       "mutation_register_rate_limited",
			body:       `{"query":"mutation { register(email:\"a\",password:\"b\",name:\"c\") }"}`,
			requests:   11,
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "mutation_login_rate_limited",
			body:       `{"query":"mutation { login(email:\"a\",password:\"b\") }"}`,
			requests:   11,
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "mutation_createProduct_rate_limited",
			body:       `{"query":"mutation { createProduct(name:\"a\",description:\"b\",price:1.0,stock:1,categories:[\"c\"]) }"}`,
			requests:   11,
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "query_not_limited",
			body:       `{"query":"query { user(id:\"1\") { id } }"}`,
			requests:   20,
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid_json_not_limited",
			body:       `not-json`,
			requests:   20,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rl := NewRateLimiter(10)
			handler := GraphQLMutationRateLimiter(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			var lastStatus int
			for i := 0; i < tt.requests; i++ {
				req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
				req.RemoteAddr = "127.0.0.1:1234"
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				lastStatus = rr.Code
			}
			assert.Equal(t, tt.wantStatus, lastStatus)
		})
	}
}

func TestGraphQLMutationRateLimiter_NonPost(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter(10)
	handler := GraphQLMutationRateLimiter(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/query", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}
