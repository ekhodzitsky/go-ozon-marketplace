package main

import (
	pkgapp "github.com/ekhodzitsky/go-ozon-marketplace/pkg/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/config"
)

func main() {
	pkgapp.RunService("inventory-service", config.Load, app.New)
}
