package fxmodules

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/grpcclient"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// GRPCClient отдаёт типизированный gRPC-клиент как fx-зависимость и управляет
// жизнью соединения. Вызывающий передаёт адрес сервиса и сгенерированный
// конструктор (например, userv1.NewUserServiceClient).
func GRPCClient[T any](addr string, newClient func(grpc.ClientConnInterface) T) fx.Option {
	return fx.Provide(func(lc fx.Lifecycle, factory *grpcclient.Factory) (T, error) {
		var zero T
		conn, err := factory.NewClient(context.Background(), addr)
		if err != nil {
			return zero, err
		}
		lc.Append(fx.Hook{
			OnStop: func(context.Context) error {
				return conn.Close()
			},
		})
		return newClient(conn), nil
	})
}
