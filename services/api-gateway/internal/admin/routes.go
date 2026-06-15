package admin

import (
	"net/http"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/go-chi/chi/v5"
)

// NewRouter returns a chi router with admin endpoints protected by admin JWT auth.
// If jwtSecret is empty, authentication is skipped (useful for tests and disabled setups).
func NewRouter(handler *Handler, jwtSecret string) http.Handler {
	r := chi.NewRouter()
	if jwtSecret != "" {
		r.Use(requireAdminHTTP(jwtSecret))
	}
	r.Get("/flags", handler.ListFlags)
	r.Post("/flags/{name}/enable", handler.EnableFlag)
	r.Post("/flags/{name}/disable", handler.DisableFlag)
	r.Post("/flags/{name}/percentage/{value}", handler.SetPercentage)
	return r
}

func requireAdminHTTP(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			claims, err := auth.ParseBearer(authHeader, jwtSecret)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if auth.Role(claims.Role) != auth.RoleAdmin {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
