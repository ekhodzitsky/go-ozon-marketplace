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
func AuthHTTP(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth != "" {
				tokenStr := strings.TrimPrefix(auth, "Bearer ")
				if tokenStr != auth {
					token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
						if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
							return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
						}
						return []byte(jwtSecret), nil
					})
					if err == nil && token.Valid {
						if claims, ok := token.Claims.(jwt.MapClaims); ok {
							if userID, ok := claims["user_id"].(string); ok && userID != "" {
								ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
								role, _ := claims["role"].(string)
								if role == "" {
									role = string(RoleUser)
								}
								ctx = context.WithValue(ctx, ContextKeyRole, role)
								r = r.WithContext(ctx)
							}
						}
					}
				}
			}
			next.ServeHTTP(w, r)
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

// AccessLog logs every HTTP request using zap.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log, _ := logger.New("info", "json")
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
