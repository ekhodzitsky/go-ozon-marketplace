package main

import (
	pkgapp "github.com/ekhodzitsky/go-ozon-marketplace/pkg/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/config"
)

func main() {
	pkgapp.RunService("user-service", config.Load, app.New)
}
