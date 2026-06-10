package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

// RateLimiter provides a simple in-memory token-bucket rate limiter per client IP.
type RateLimiter struct {
	rps    rate.Limit
	burst  int
	mu     sync.Mutex
	limits map[string]*rate.Limiter
}

// NewRateLimiter creates a RateLimiter with the given requests-per-second.
func NewRateLimiter(rps int) *RateLimiter {
	if rps <= 0 {
		rps = 10
	}
	return &RateLimiter{
		rps:    rate.Limit(rps),
		burst:  rps,
		limits: make(map[string]*rate.Limiter),
	}
}

// Allow reports whether one request from key is allowed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	lim, ok := rl.limits[key]
	if !ok {
		lim = rate.NewLimiter(rl.rps, rl.burst)
		rl.limits[key] = lim
	}
	rl.mu.Unlock()
	return lim.Allow()
}

type graphQLRequest struct {
	Query string `json:"query"`
}

// GraphQLMutationRateLimiter returns HTTP middleware that applies rate limiting
// to GraphQL mutations: register, login, createProduct.
func GraphQLMutationRateLimiter(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			r.Body.Close()

			var gqlReq graphQLRequest
			if err := json.Unmarshal(body, &gqlReq); err != nil {
				r.Body = io.NopCloser(bytes.NewReader(body))
				next.ServeHTTP(w, r)
				return
			}

			if isTargetedMutation(gqlReq.Query) {
				ip := clientIP(r)
				if !limiter.Allow(ip) {
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
			}

			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
		})
	}
}

func isTargetedMutation(query string) bool {
	q := strings.ToLower(query)
	if !strings.Contains(q, "mutation") {
		return false
	}
	for _, field := range []string{"register", "login", "createproduct"} {
		if strings.Contains(q, field) {
			return true
		}
	}
	return false
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
