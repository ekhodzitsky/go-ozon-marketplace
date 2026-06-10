module github.com/ekhodzitsky/go-ozon-marketplace/services/user-service

go 1.26

require (
	github.com/ekhodzitsky/go-ozon-marketplace/api v0.0.0
	github.com/ekhodzitsky/go-ozon-marketplace/pkg v0.0.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.6.0
	go.uber.org/fx v1.24.0
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.48.0
	google.golang.org/grpc v1.81.1
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace (
	github.com/ekhodzitsky/go-ozon-marketplace/api => ../../api
	github.com/ekhodzitsky/go-ozon-marketplace/pkg => ../../pkg
)
