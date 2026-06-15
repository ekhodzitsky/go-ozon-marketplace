package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/redis/go-redis/v9"
)

// RateLimiter is the generic rate-limiter interface.
type RateLimiter interface {
	Allow(ctx context.Context, key string) bool
}

// RedisRateLimiter implements a sliding-window rate limiter backed by Redis.
type RedisRateLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
	script *redis.Script
}

// NewRedisRateLimiter creates a Redis-backed sliding-window rate limiter.
func NewRedisRateLimiter(client *redis.Client, limit int, window time.Duration) *RedisRateLimiter {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = time.Second
	}
	lua := `
		local key = KEYS[1]
		local window = tonumber(ARGV[1])
		local now = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local expire = tonumber(ARGV[4])
		redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
		local count = redis.call('ZCARD', key)
		if count < limit then
			redis.call('ZADD', key, now, now)
			redis.call('EXPIRE', key, expire)
			return 1
		end
		return 0
	`
	return &RedisRateLimiter{
		client: client,
		limit:  limit,
		window: window,
		script: redis.NewScript(lua),
	}
}

// Allow reports whether one request from key is allowed.
// Redis errors are treated as fail-closed to prevent abuse when the rate-limiting backend is unavailable.
func (rl *RedisRateLimiter) Allow(ctx context.Context, key string) bool {
	now := time.Now().UnixMilli()
	windowMs := rl.window.Milliseconds()
	expireSec := int64(rl.window.Seconds())
	if expireSec < 1 {
		expireSec = 1
	}
	res, err := rl.script.Run(ctx, rl.client, []string{key}, windowMs, now, rl.limit, expireSec).Int()
	if err != nil {
		return false
	}
	return res == 1
}

// ClientIP returns the client IP. It respects X-Forwarded-For only when the
// immediate peer (RemoteAddr) is within one of the trusted CIDRs.
func ClientIP(r *http.Request, trusted []string) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if len(trusted) == 0 {
		return host
	}

	var cidrs []*net.IPNet
	for _, c := range trusted {
		_, ipNet, err := net.ParseCIDR(c)
		if err != nil {
			continue
		}
		cidrs = append(cidrs, ipNet)
	}

	peerIP := net.ParseIP(host)
	trustedPeer := false
	for _, cidr := range cidrs {
		if cidr.Contains(peerIP) {
			trustedPeer = true
			break
		}
	}
	if !trustedPeer {
		return host
	}

	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return host
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if ip != "" {
			return ip
		}
	}
	return host
}

// MaxBytesHandler wraps the next handler with http.MaxBytesReader.
func MaxBytesHandler(next http.Handler, maxBytes int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if maxBytes > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// RateLimitHTTP returns middleware that rate-limits all requests by IP.
func RateLimitHTTP(rl RateLimiter, trusted []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIP(r, trusted)
			if !rl.Allow(r.Context(), ip) {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type ctxKeyRateLimitIP struct{}

// WithRateLimitIP puts the client IP into the request context.
func WithRateLimitIP(next http.Handler, trusted []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), ctxKeyRateLimitIP{}, ClientIP(r, trusted))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RateLimitIPFromContext extracts the client IP set by WithRateLimitIP.
func RateLimitIPFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRateLimitIP{}).(string)
	return v
}

type noopRateLimiter struct{}

func (n *noopRateLimiter) Allow(ctx context.Context, key string) bool { return true }

// RoleRateLimiter selects a rate limiter based on the role in context.
type RoleRateLimiter struct {
	user    RateLimiter
	admin   RateLimiter
	service RateLimiter
}

// NewRoleRateLimiter creates a role-based rate limiter.
// User and admin limits use Redis; service role has no limit.
func NewRoleRateLimiter(client *redis.Client, userLimit, adminLimit int, window time.Duration) *RoleRateLimiter {
	return &RoleRateLimiter{
		user:    NewRedisRateLimiter(client, userLimit, window),
		admin:   NewRedisRateLimiter(client, adminLimit, window),
		service: &noopRateLimiter{},
	}
}

// Allow delegates to the appropriate limiter based on role.
func (rl *RoleRateLimiter) Allow(ctx context.Context, key string) bool {
	role, _ := GetRole(ctx)
	switch role {
	case auth.RoleAdmin:
		return rl.admin.Allow(ctx, key)
	case auth.RoleService:
		return rl.service.Allow(ctx, key)
	default:
		return rl.user.Allow(ctx, key)
	}
}
