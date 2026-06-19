package clients

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/grpcclient"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/sony/gobreaker"
)

// NewFactory adapts the api-gateway configuration to the shared gRPC client factory.
// It adds user-token forwarding so that authenticated caller context reaches downstream services.
func NewFactory(cfg *config.Config, cb *gobreaker.CircuitBreaker) *grpcclient.Factory {
	return grpcclient.NewFactory(grpcclient.Config{
		CertPath: cfg.CertPath,
	}, cb,
		grpcclient.WithUnaryInterceptor(authForwardingInterceptor),
		grpcclient.WithInsecureAllowed(cfg.InsecureSkipTLS),
	)
}
