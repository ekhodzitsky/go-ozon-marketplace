package admin

import (
	"net/http"
	"strings"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/go-chi/chi/v5"
)

// NewRouter returns a chi router with admin endpoints protected by admin JWT auth.
// If verifier is nil, authentication is skipped (useful for tests and disabled setups).
func NewRouter(handler *Handler, verifier auth.Verifier) http.Handler {
	r := chi.NewRouter()
	if verifier != nil {
		r.Use(requireAdminHTTP(verifier))
	}
	r.Get("/flags", handler.ListFlags)
	r.Post("/flags/{name}/enable", handler.EnableFlag)
	r.Post("/flags/{name}/disable", handler.DisableFlag)
	r.Post("/flags/{name}/percentage/{value}", handler.SetPercentage)
	return r
}

func requireAdminHTTP(verifier auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenStr == authHeader {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			identity, err := verifier.Verify(r.Context(), tokenStr)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if identity.Role != auth.RoleAdmin {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
