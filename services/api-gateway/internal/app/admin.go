package app

import (
	"net/http"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/admin"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
)

func provideAdminHandler(flags *featureflags.FeatureFlags, cfg *config.Config) http.Handler {
	var verifier auth.Verifier
	if cfg.JWTSecret != "" {
		verifier = auth.NewJWTVerifier(cfg.JWTSecret)
	}
	return admin.NewRouter(admin.NewHandler(flags), verifier)
}
