package main

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/app"
)

func main() {
	application := app.New()
	application.Run()
}
