package main

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/app"
)

func main() {
	application := app.New()
	application.Run()
}
