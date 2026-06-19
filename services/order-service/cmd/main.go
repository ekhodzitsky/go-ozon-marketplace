package main

import (
	"net/http"
	_ "net/http/pprof"

	pkgapp "github.com/ekhodzitsky/go-ozon-marketplace/pkg/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/config"
)

func main() {
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()
	pkgapp.RunService("order-service", config.Load, app.New)
}
