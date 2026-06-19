package main

import (
	pkgapp "github.com/ekhodzitsky/go-ozon-marketplace/pkg/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
)

func main() {
	pkgapp.RunService("api-gateway", config.Load, app.New)
}
