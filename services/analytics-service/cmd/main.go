package main

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/app"
)

func main() {
	application := app.New()
	application.Run()
}
