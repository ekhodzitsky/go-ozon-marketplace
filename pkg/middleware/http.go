package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type contextKey string

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
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := auth.ParseBearer(authHeader, jwtSecret)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), auth.ContextKeyUserID, claims.Subject)
			ctx = context.WithValue(ctx, auth.ContextKeyRole, auth.Role(claims.Role))
			ctx = context.WithValue(ctx, auth.ContextKeyAuthorizationHeader, authHeader)
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
