package fxmodules

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/grpcclient"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// GRPCClient provides a typed gRPC client as an fx dependency and manages the
// underlying connection lifecycle. The caller supplies the service address and
// the generated constructor (e.g. userv1.NewUserServiceClient).
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
