package main

import (
	"log"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
)

func main() {
	cfg := config.Load()
	if err := app.New(cfg).Run(); err != nil {
		log.Fatalf("gateway error: %v", err)
	}
}
