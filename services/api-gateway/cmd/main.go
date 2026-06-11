package main

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()
	if err := app.New(cfg).Run(); err != nil {
		logger.New().Fatal("gateway error", zap.Error(err))
	}
}
