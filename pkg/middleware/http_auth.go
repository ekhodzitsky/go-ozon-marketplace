package middleware

import (
	"net/http"
	"strings"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
)

// AuthHTTP parses a JWT from the Authorization header and injects an auth.Identity into the request context.
// If an Authorization header is present but invalid, the request is rejected with 401.
func AuthHTTP(verifier auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			identity, err := verifier.Verify(r.Context(), tokenStr)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			identity.AuthorizationHeader = authHeader
			ctx := auth.WithIdentity(r.Context(), identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
