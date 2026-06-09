module github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service

go 1.26

require (
	github.com/ekhodzitsky/go-ozon-marketplace/api v0.0.0
	github.com/ekhodzitsky/go-ozon-marketplace/pkg v0.0.0
	go.uber.org/fx v1.24.0
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.64.0
)

require (
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/protobuf v1.34.0 // indirect
)

replace (
	github.com/ekhodzitsky/go-ozon-marketplace/api => ../../api
	github.com/ekhodzitsky/go-ozon-marketplace/pkg => ../../pkg
)
