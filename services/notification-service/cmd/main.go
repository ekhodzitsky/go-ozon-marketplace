package main

import (
	pkgapp "github.com/ekhodzitsky/go-ozon-marketplace/pkg/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/config"
)

func main() {
	pkgapp.RunService("notification-service", config.Load, app.New)
}
