package main

import (
	pkgapp "github.com/ekhodzitsky/go-ozon-marketplace/pkg/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/config"
)

func main() {
	pkgapp.RunService("payment-service", config.Load, app.New)
}
