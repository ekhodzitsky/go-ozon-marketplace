# api-gateway refactor design

## Goal

Simplify `api-gateway` by replacing hand-rolled solutions with established Go libraries and splitting responsibilities into focused packages.

## Context

Current `internal/app/app.go` is 449 lines and mixes:
- 6 gRPC client initializations (duplicated code),
- Redis, feature flags, A/B experiments, WebSocket hub setup,
- GraphQL server configuration,
- Middleware chain, hand-rolled CORS, admin JWT parsing,
- HTTP and metrics server startup and shutdown.

JWT parsing is duplicated in at least 4 places:
- `pkg/middleware/auth.go` (`AuthHTTP`, `AuthUnaryInterceptor`),
- `services/api-gateway/internal/app/app.go` (`requireAdminHTTP`),
- `services/api-gateway/internal/ws/server.go` (`authenticateUpgrade`),
- `services/api-gateway/graph/resolver.go` helpers (`requireAuth`, `isAdmin`).

Admin routing is implemented via manual string parsing in `internal/admin/admin.go`.

CORS is implemented manually in `app.go`.

## Non-goals

- Replace `gqlgen` (already standard).
- Replace Prometheus client.
- Rewrite business logic in resolvers (price check, A/B tracking) — keep behavior, move only if necessary for clarity.
- Add new features.

## Approach

Use standard libraries:
- `github.com/go-chi/chi/v5` for HTTP routing,
- `github.com/rs/cors` for CORS,
- `github.com/google/wire` for dependency injection,
- a single `pkg/auth` package for JWT parsing,
- `internal/clients.NewGRPCClient` factory to remove duplicated gRPC client setup.

## Target architecture

```
services/api-gateway/
  cmd/main.go              # entrypoint: config, logger, tracer, run app
  internal/
    app/app.go             # wiring only (wire-generated where possible)
    config/config.go       # unchanged
    clients/factory.go     # gRPC client factory
    server/http.go         # chi router, middleware, GraphQL, WS, admin, probes
    server/metrics.go      # metrics server
    admin/handler.go       # chi-based admin routes
    ws/server.go           # uses pkg/auth for JWT
  pkg/
    auth/jwt.go            # unified JWT parsing
```

## Design decisions

1. **Routing.** Replace `http.NewServeMux` with `chi`. Admin endpoints become explicit `chi.Router` routes instead of string parsing.
2. **CORS.** Replace hand-rolled middleware with `rs/cors` configured from `config.Config`.
3. **JWT.** Extract `ParseJWT(token, secret) (*CustomClaims, error)` into `pkg/auth`. HTTP, WebSocket and admin handlers call it.
4. **gRPC clients.** Single factory function `NewGRPCClient(ctx, addr, cfg, opts...)`. All downstream clients created through it.
5. **DI.** Use `google/wire` to generate the `App` constructor. Keep provider functions small and testable.
6. **Server startup.** Split HTTP and metrics server into separate files under `internal/server`.

## Success criteria

- `internal/app/app.go` under 150 lines.
- JWT parsing in exactly one place (`pkg/auth`).
- CORS via `rs/cors`.
- Admin routing via `chi`.
- Adding a new downstream service requires changes only in `config` and the wire provider set.
- Existing tests pass after adaptation.

## Risks

- `wire` adds code generation step (`go generate`).
- Existing integration tests may need updates to match new constructors.

## Dependencies to add

```
github.com/go-chi/chi/v5
github.com/rs/cors
github.com/google/wire
```
