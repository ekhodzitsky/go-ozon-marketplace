package main

import (
	pkgapp "github.com/ekhodzitsky/go-ozon-marketplace/pkg/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/config"
)

func main() {
	pkgapp.RunService("catalog-service", config.Load, app.New)
}
