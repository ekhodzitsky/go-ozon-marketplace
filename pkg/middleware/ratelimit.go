package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/redis/go-redis/v9"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	redisstore "github.com/ulule/limiter/v3/drivers/store/redis"
)

// RateLimiter — общий интерфейс rate limiter.
type RateLimiter interface {
	Allow(ctx context.Context, key string) bool
}

// RedisRateLimiter — sliding-window rate limiter поверх Redis.
type RedisRateLimiter struct {
	limiter *limiter.Limiter
}

// NewRedisRateLimiter создаёт Redis-backed sliding-window rate limiter.
// Если Redis недоступен, падает на in-memory store.
func NewRedisRateLimiter(client *redis.Client, limit int, window time.Duration) *RedisRateLimiter {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = time.Second
	}

	rate := limiter.Rate{Period: window, Limit: int64(limit)}
	store := newLimiterStore(client)
	return &RedisRateLimiter{limiter: limiter.New(store, rate)}
}

func newLimiterStore(client *redis.Client) limiter.Store {
	if client == nil {
		return memory.NewStore()
	}
	store, err := redisstore.NewStore(client)
	if err != nil {
		return memory.NewStore()
	}
	return store
}

// Allow говорит, пропускать ли один запрос от key.
// Ошибки Redis трактуются как fail-closed, чтобы не дать абьюзу, когда бэкенд rate limiting недоступен.
func (rl *RedisRateLimiter) Allow(ctx context.Context, key string) bool {
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	limitCtx, err := rl.limiter.Get(ctx, key)
	if err != nil {
		return false
	}
	return !limitCtx.Reached
}

// ClientIP возвращает IP клиента. Учитывает X-Forwarded-For только если
// ближайший пир (RemoteAddr) попадает в один из доверенных CIDR.
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

// MaxBytesHandler оборачивает handler в http.MaxBytesReader.
func MaxBytesHandler(next http.Handler, maxBytes int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if maxBytes > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// RateLimitHTTP возвращает middleware, которое rate-limitит все запросы по IP.
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

// WithRateLimitIP кладёт IP клиента в контекст запроса.
func WithRateLimitIP(next http.Handler, trusted []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), ctxKeyRateLimitIP{}, ClientIP(r, trusted))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RateLimitIPFromContext достаёт IP клиента, установленный WithRateLimitIP.
func RateLimitIPFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRateLimitIP{}).(string)
	return v
}

type noopRateLimiter struct{}

func (n *noopRateLimiter) Allow(ctx context.Context, key string) bool { return true }

// RoleRateLimiter выбирает rate limiter по роли из контекста.
type RoleRateLimiter struct {
	user    RateLimiter
	admin   RateLimiter
	service RateLimiter
}

// NewRoleRateLimiter создаёт rate limiter, зависящий от роли.
// Для user и admin используется Redis; service-роль не ограничивается.
func NewRoleRateLimiter(client *redis.Client, userLimit, adminLimit int, window time.Duration) *RoleRateLimiter {
	return &RoleRateLimiter{
		user:    NewRedisRateLimiter(client, userLimit, window),
		admin:   NewRedisRateLimiter(client, adminLimit, window),
		service: &noopRateLimiter{},
	}
}

// Allow делегирует в подходящий limiter в зависимости от роли.
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
