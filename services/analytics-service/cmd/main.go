package main

import (
	pkgapp "github.com/ekhodzitsky/go-ozon-marketplace/pkg/app"
	svcapp "github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/config"
	"go.uber.org/fx"
)

func main() {
	pkgapp.RunService("analytics-service", config.Load, func(cfg *config.Config) *fx.App {
		return svcapp.New(cfg)
	})
}
