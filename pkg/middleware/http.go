package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const contextKeyRequestID contextKey = "request_id"

// RequestID adds X-Request-ID header to every request.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", reqID)
		ctx := context.WithValue(r.Context(), contextKeyRequestID, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts request-id from context.
func GetRequestID(ctx context.Context) string {
	v, _ := ctx.Value(contextKeyRequestID).(string)
	return v
}

// AuthHTTP parses JWT from Authorization header and injects user_id/role into request context.
// If an Authorization header is present but invalid, the request is rejected with 401.
func AuthHTTP(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				next.ServeHTTP(w, r)
				return
			}

			tokenStr := strings.TrimPrefix(auth, "Bearer ")
			if tokenStr == auth {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(*CustomClaims)
			if !ok || claims.Subject == "" || claims.Issuer != "go-ozon-marketplace" || !audienceContains(claims.Audience, "api-gateway") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.Subject)
			role := claims.Role
			if role == "" {
				role = string(RoleUser)
			}
			ctx = context.WithValue(ctx, ContextKeyRole, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// AccessLog logs every HTTP request using the default zap logger.
func AccessLog(next http.Handler) http.Handler {
	return NewAccessLog(defaultHTTPLog())(next)
}

// NewAccessLog returns HTTP access-log middleware that uses the provided logger.
func NewAccessLog(log *zap.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = defaultHTTPLog()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)
			log.Info("http request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
				zap.Int("status", rw.status),
				zap.Duration("duration", time.Since(start)),
				zap.String("request_id", GetRequestID(r.Context())),
			)
		})
	}
}

func defaultHTTPLog() *zap.Logger {
	log, _ := logger.New("info", "json")
	return log
}
