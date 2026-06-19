package fxmodules

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// ServiceConfig combines the common config interfaces needed by a gRPC service app.
type ServiceConfig interface {
	LoggerConfig
	GRPCServerConfig
}

// GRPCService wires a standard gRPC service fx application. newHandler must be a
// constructor function returning the generated server interface (Handler).
func GRPCService[Cfg ServiceConfig, Handler any](
	serviceName string,
	cfg Cfg,
	register func(grpc.ServiceRegistrar, Handler),
	newHandler any,
	opts ...fx.Option,
) *fx.App {
	return fx.New(
		Logger(cfg),
		Config(cfg),
		GRPCServer(cfg),
		fx.Provide(newHandler),
		fx.Provide(func(h Handler) server.RegisterFn {
			return func(s grpc.ServiceRegistrar) { register(s, h) }
		}),
		fx.Options(opts...),
	)
}
