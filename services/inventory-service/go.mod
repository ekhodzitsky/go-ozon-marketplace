module github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service

go 1.26

require (
	github.com/ekhodzitsky/go-ozon-marketplace/api v0.0.0
	github.com/ekhodzitsky/go-ozon-marketplace/pkg v0.0.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.6.0
	github.com/redis/go-redis/v9 v9.5.3
	go.uber.org/fx v1.24.0
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.64.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sync v0.6.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/protobuf v1.34.0 // indirect
)

replace (
	github.com/ekhodzitsky/go-ozon-marketplace/api => ../../api
	github.com/ekhodzitsky/go-ozon-marketplace/pkg => ../../pkg
)
