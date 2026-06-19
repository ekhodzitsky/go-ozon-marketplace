# api-gateway Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace hand-rolled routing, CORS, DI, and JWT parsing in `api-gateway` with standard libraries and split responsibilities into focused packages.

**Architecture:** Introduce `pkg/auth` for unified JWT, `internal/clients` for gRPC client factory, `internal/server` for HTTP/metrics servers with `chi`, and use `google/wire` for dependency injection. Keep existing GraphQL schema and resolver behavior.

**Tech Stack:** Go, gRPC, gqlgen, chi, rs/cors, google/wire, zap, redis.

> **Note:** This plan was written before the migration from a custom `pkg/circuitbreaker` to `github.com/sony/gobreaker`. Code examples referencing `pkg/circuitbreaker` should use `github.com/sony/gobreaker` instead.

---

### Task 1: Add dependencies

**Files:**
- Modify: `services/api-gateway/go.mod`
- Modify: `services/api-gateway/go.sum` (via `go mod tidy`)

- [ ] **Step 1: Add chi, rs/cors, wire**

Run:
```bash
cd services/api-gateway
go get github.com/go-chi/chi/v5 github.com/rs/cors github.com/google/wire/cmd/wire@latest
go get github.com/google/wire
```

- [ ] **Step 2: Verify go.mod**

Expected additions:
```
github.com/go-chi/chi/v5 v5.1.0
github.com/rs/cors v1.11.1
github.com/google/wire v0.6.0
```

- [ ] **Step 3: Commit**

```bash
git add services/api-gateway/go.mod services/api-gateway/go.sum
git commit -m "deps(api-gateway): add chi, rs/cors, wire"
```

---

### Task 2: Create unified JWT package

**Files:**
- Create: `pkg/auth/jwt.go`
- Create: `pkg/auth/jwt_test.go`

- [ ] **Step 1: Write the test**

Create `pkg/auth/jwt_test.go`:
```go
package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJWT_Valid(t *testing.T) {
	secret := "test-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        "tok-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Role: "user",
	})
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	claims, err := ParseJWT(signed, secret)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.Subject)
	assert.Equal(t, "user", claims.Role)
}

func TestParseJWT_InvalidIssuer(t *testing.T) {
	secret := "test-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "evil",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        "tok-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	_, err = ParseJWT(signed, secret)
	assert.Error(t, err)
}

func TestParseJWT_MissingSubject(t *testing.T) {
	secret := "test-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        "tok-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	_, err = ParseJWT(signed, secret)
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:
```bash
cd /Users/ekhodzitsky/Documents/personal/go-ozon-marketplace
go test ./pkg/auth/...
```
Expected: FAIL — package not found or `ParseJWT` undefined.

- [ ] **Step 3: Implement `pkg/auth/jwt.go`**

Create `pkg/auth/jwt.go`:
```go
package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CustomClaims extends jwt.RegisteredClaims with a role claim.
type CustomClaims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

type contextKey string

const (
	ContextKeyUserID contextKey = "user_id"
	ContextKeyRole   contextKey = "role"
)

// Role represents user roles.
type Role string

const (
	RoleUser    Role = "user"
	RoleAdmin   Role = "admin"
	RoleService Role = "service"
)

// ParseJWT validates a bearer token string and returns parsed claims.
func ParseJWT(tokenStr, secret string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || claims.Subject == "" {
		return nil, fmt.Errorf("missing subject in token")
	}
	if claims.Issuer != "go-ozon-marketplace" {
		return nil, fmt.Errorf("invalid token issuer")
	}
	if !audienceContains(claims.Audience, "api-gateway") {
		return nil, fmt.Errorf("invalid token audience")
	}
	if claims.ID == "" {
		return nil, fmt.Errorf("missing token id")
	}
	if claims.NotBefore != nil && time.Now().Before(claims.NotBefore.Time) {
		return nil, fmt.Errorf("token not valid yet")
	}
	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return nil, fmt.Errorf("token expired")
	}
	if claims.Role == "" {
		claims.Role = string(RoleUser)
	}
	return claims, nil
}

// ParseBearer extracts the token from "Bearer <token>" and parses it.
func ParseBearer(bearer, secret string) (*CustomClaims, error) {
	tokenStr := strings.TrimPrefix(bearer, "Bearer ")
	if tokenStr == bearer {
		return nil, fmt.Errorf("invalid authorization header format")
	}
	return ParseJWT(tokenStr, secret)
}

func audienceContains(aud jwt.ClaimStrings, target string) bool {
	for _, a := range aud {
		if a == target {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./pkg/auth/... -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/auth/jwt.go pkg/auth/jwt_test.go
git commit -m "feat(pkg/auth): unified JWT parsing"
```

---

### Task 3: Update `pkg/middleware/auth.go` to use `pkg/auth`

**Files:**
- Modify: `pkg/middleware/auth.go`

- [ ] **Step 1: Refactor gRPC interceptor**

Replace JWT parsing logic in `AuthUnaryInterceptor` with `auth.ParseJWT`:

```go
package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthUnaryInterceptor validates JWT bearer token from gRPC metadata.
func AuthUnaryInterceptor(jwtSecret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if isPublicEndpoint(info.FullMethod) {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		tokenStr := strings.TrimPrefix(authHeader[0], "Bearer ")
		claims, err := auth.ParseJWT(tokenStr, jwtSecret)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}

		ctx = context.WithValue(ctx, auth.ContextKeyUserID, claims.Subject)
		ctx = context.WithValue(ctx, auth.ContextKeyRole, claims.Role)
		return handler(ctx, req)
	}
}
```

- [ ] **Step 2: Remove duplicated types and helpers**

Delete `CustomClaims`, `Role`, `RoleUser`, `RoleAdmin`, `RoleService`, `audienceContains`, and `GetUserID`/`GetRole` if they conflict with `pkg/auth`.

Keep convenience wrappers in `pkg/middleware`:
```go
// GetUserID extracts user_id from context.
func GetUserID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(auth.ContextKeyUserID).(string)
	return v, ok
}

// GetRole extracts role from context.
func GetRole(ctx context.Context) (auth.Role, bool) {
	v := ctx.Value(auth.ContextKeyRole)
	if v == nil {
		return auth.RoleUser, false
	}
	switch r := v.(type) {
	case auth.Role:
		return r, true
	case string:
		return auth.Role(r), true
	}
	return auth.RoleUser, false
}

// RequireRole returns PermissionDenied if the context role is not in allowed.
func RequireRole(ctx context.Context, allowed ...auth.Role) error {
	role, ok := GetRole(ctx)
	if !ok {
		return status.Error(codes.PermissionDenied, "missing role")
	}
	for _, a := range allowed {
		if role == a {
			return nil
		}
	}
	return status.Errorf(codes.PermissionDenied, "role %s not allowed", role)
}
```

- [ ] **Step 3: Run tests**

Run:
```bash
go test ./pkg/middleware/... -v
```
Expected: PASS after fixing any broken references.

- [ ] **Step 4: Commit**

```bash
git add pkg/middleware/auth.go
git commit -m "refactor(pkg/middleware): use pkg/auth for JWT"
```

---

### Task 4: Update `pkg/middleware/http.go` to use `pkg/auth`

**Files:**
- Modify: `pkg/middleware/http.go`

- [ ] **Step 1: Refactor AuthHTTP**

```go
// AuthHTTP parses JWT from Authorization header and injects user_id/role into request context.
func AuthHTTP(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := auth.ParseBearer(authHeader, jwtSecret)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), auth.ContextKeyUserID, claims.Subject)
			ctx = context.WithValue(ctx, auth.ContextKeyRole, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

- [ ] **Step 2: Remove duplicated imports and types**

Remove `jwt/v5` import and `CustomClaims` if still present.

- [ ] **Step 3: Run tests**

```bash
go test ./pkg/middleware/... -v
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add pkg/middleware/http.go
git commit -m "refactor(pkg/middleware): AuthHTTP uses pkg/auth"
```

---

### Task 5: Create gRPC client factory

**Files:**
- Create: `services/api-gateway/internal/clients/factory.go`
- Create: `services/api-gateway/internal/clients/factory_test.go` (optional)

- [ ] **Step 1: Implement factory**

Create `services/api-gateway/internal/clients/factory.go`:
```go
package clients

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/circuitbreaker"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Factory creates gRPC clients with common interceptors and TLS.
type Factory struct {
	cfg *config.Config
	cb  *circuitbreaker.CircuitBreaker
}

// NewFactory creates a new gRPC client factory.
func NewFactory(cfg *config.Config, cb *circuitbreaker.CircuitBreaker) *Factory {
	return &Factory{cfg: cfg, cb: cb}
}

// NewClient creates a gRPC client connection for the given address.
func (f *Factory) NewClient(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	creds, err := f.clientCreds(addr)
	if err != nil {
		return nil, fmt.Errorf("tls credentials: %w", err)
	}

	cb := circuitBreakerClientInterceptor(f.cb)
	return grpc.NewClient(addr,
		grpc.WithTransportCredentials(creds),
		grpc.WithChainUnaryInterceptor(cb, tracing.UnaryClientInterceptor()),
	)
}

func (f *Factory) clientCreds(addr string) (credentials.TransportCredentials, error) {
	if f.cfg.CertPath != "" {
		return server.LoadClientMTLSCredentials(
			filepath.Join(f.cfg.CertPath, "server-cert.pem"),
			filepath.Join(f.cfg.CertPath, "server-key.pem"),
			filepath.Join(f.cfg.CertPath, "ca-cert.pem"),
			serverNameFromAddr(addr),
		)
	}
	if f.cfg.InsecureSkipTLS {
		return insecure.NewCredentials(), nil
	}
	return nil, fmt.Errorf("no CERT_PATH configured and INSECURE_SKIP_TLS is false")
}

func serverNameFromAddr(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

func circuitBreakerClientInterceptor(cb *circuitbreaker.CircuitBreaker) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return cb.Call(func() error {
			return invoker(ctx, method, req, reply, cc, opts...)
		})
	}
}
```

Note: `authClientInterceptor` was only needed to forward `Authorization` from GraphQL context. With gqlgen, the context carries HTTP headers, but the interceptor approach is awkward. Better to attach metadata in resolver via `metadata.AppendToOutgoingContext`. Keep this decision for Task 7.

- [ ] **Step 2: Run build**

```bash
cd services/api-gateway
go build ./internal/clients/...
```
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add services/api-gateway/internal/clients/factory.go
git commit -m "feat(api-gateway): gRPC client factory"
```

---

### Task 6: Update WebSocket auth to use `pkg/auth`

**Files:**
- Modify: `services/api-gateway/internal/ws/server.go`

- [ ] **Step 1: Replace JWT parsing**

Replace `authenticateUpgrade` to use `auth.ParseJWT`:

```go
import "github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"

func authenticateUpgrade(r *http.Request, jwtSecret string) (string, error) {
	if jwtSecret == "" {
		return "", nil
	}
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		authHeader := r.Header.Get("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenStr = authHeader[7:]
		}
	}
	if tokenStr == "" {
		return "", fmt.Errorf("missing token")
	}
	claims, err := auth.ParseJWT(tokenStr, jwtSecret)
	if err != nil {
		return "", err
	}
	return claims.Subject, nil
}
```

- [ ] **Step 2: Remove local `customClaims` type**

Delete the local `customClaims` struct.

- [ ] **Step 3: Run tests**

```bash
cd services/api-gateway
go test ./internal/ws/... -v
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add services/api-gateway/internal/ws/server.go
git commit -m "refactor(api-gateway/ws): use pkg/auth for JWT"
```

---

### Task 7: Create HTTP server with chi

**Files:**
- Create: `services/api-gateway/internal/server/http.go`

- [ ] **Step 1: Implement server**

Create `services/api-gateway/internal/server/http.go`:
```go
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"

	pkgmiddleware "github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/admin"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/ws"
)

// HTTP holds the gateway HTTP server and dependencies.
type HTTP struct {
	server *http.Server
}

// NewHTTP creates the gateway HTTP server.
func NewHTTP(
	cfg *config.Config,
	resolver *graph.Resolver,
	hub *ws.Hub,
	rl pkgmiddleware.RateLimiter,
	adminHandler http.Handler,
) *HTTP {
	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))
	setupGraphQLServer(srv, rl)

	r := chi.NewRouter()
	r.Use(pkgmiddleware.RequestID)
	r.Use(pkgmiddleware.AccessLog)

	c := cors.New(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})
	r.Use(c.Handler)

	if cfg.JWTSecret != "" {
		r.Use(pkgmiddleware.AuthHTTP(cfg.JWTSecret))
	}
	r.Use(pkgmiddleware.MaxBytesHandler(???, cfg.MaxBodySizeBytes)) // fix import
	r.Use(pkgmiddleware.RateLimitHTTP(rl, cfg.TrustedProxies))
	r.Use(pkgmiddleware.WithRateLimitIP(cfg.TrustedProxies))

	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWs(hub, w, r, ws.Config{
			AllowedOrigins: cfg.CORSAllowedOrigins,
			JWTSecret:      cfg.JWTSecret,
		})
	})
	r.Get("/", playground.Handler("GraphQL playground", "/query"))
	r.Handle("/query", srv)

	r.Mount("/admin", adminHandler)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	return &HTTP{
		server: &http.Server{
			Addr:         ":" + cfg.HTTPPort,
			Handler:      r,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
	}
}

// ListenAndServe starts the HTTP server.
func (h *HTTP) ListenAndServe() error {
	if err := h.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("gateway serve failed: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server.
func (h *HTTP) Shutdown(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}
```

Note: Fix `pkgmiddleware.MaxBytesHandler` signature if it doesn't return `func(http.Handler) http.Handler`.

- [ ] **Step 2: Fix imports and middleware signatures**

If `pkgmiddleware.MaxBytesHandler` returns `http.Handler` instead of middleware, wrap accordingly.

- [ ] **Step 3: Run build**

```bash
cd services/api-gateway
go build ./internal/server/...
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add services/api-gateway/internal/server/http.go
git commit -m "feat(api-gateway): chi-based HTTP server"
```

---

### Task 8: Create metrics server

**Files:**
- Create: `services/api-gateway/internal/server/metrics.go`

- [ ] **Step 1: Implement metrics server**

```go
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the metrics HTTP server.
type Metrics struct {
	server *http.Server
}

// NewMetrics creates a metrics server on the given address.
func NewMetrics(addr string) *Metrics {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return &Metrics{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
}

// ListenAndServe starts the metrics server.
func (m *Metrics) ListenAndServe() error {
	if err := m.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("metrics server failed: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the metrics server.
func (m *Metrics) Shutdown(ctx context.Context) error {
	return m.server.Shutdown(ctx)
}
```

- [ ] **Step 2: Commit**

```bash
git add services/api-gateway/internal/server/metrics.go
git commit -m "feat(api-gateway): metrics server"
```

---

### Task 9: Rewrite admin handler with chi

**Files:**
- Modify: `services/api-gateway/internal/admin/admin.go`
- Create/Modify: `services/api-gateway/internal/admin/routes.go`

- [ ] **Step 1: Create chi router**

Create `services/api-gateway/internal/admin/routes.go`:
```go
package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter returns a chi router with admin endpoints.
func NewRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.StripSlashes)
	r.Get("/flags", h.ListFlags)
	r.Post("/flags/{name}/enable", h.EnableFlag)
	r.Post("/flags/{name}/disable", h.DisableFlag)
	r.Post("/flags/{name}/percentage/{value}", h.SetPercentage)
	return r
}
```

- [ ] **Step 2: Refactor Handler methods**

Modify `services/api-gateway/internal/admin/admin.go`:
```go
package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
)

type Handler struct {
	engine *featureflags.Engine
}

func NewHandler(engine *featureflags.Engine) *Handler {
	return &Handler{engine: engine}
}

func (h *Handler) ListFlags(w http.ResponseWriter, r *http.Request) {
	flags := h.engine.List()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(flags)
}

func (h *Handler) EnableFlag(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := h.engine.SetEnabled(name, true); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "enabled"})
}

func (h *Handler) DisableFlag(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := h.engine.SetEnabled(name, false); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "disabled"})
}

func (h *Handler) SetPercentage(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	value, err := strconv.Atoi(chi.URLParam(r, "value"))
	if err != nil {
		http.Error(w, "invalid percentage", http.StatusBadRequest)
		return
	}
	if err := h.engine.SetPercentage(name, value); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, map[string]interface{}{"status": "percentage_set", "percentage": value})
}

func respondJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 3: Run tests**

```bash
cd services/api-gateway
go test ./internal/admin/... -v
```
Expected: PASS after updating tests to use chi router or httptest.

- [ ] **Step 4: Commit**

```bash
git add services/api-gateway/internal/admin/admin.go services/api-gateway/internal/admin/routes.go
git commit -m "refactor(api-gateway/admin): use chi router"
```

---

### Task 10: Wire everything together in `internal/app`

**Files:**
- Modify: `services/api-gateway/internal/app/app.go`
- Create: `services/api-gateway/internal/app/wire.go`
- Create: `services/api-gateway/internal/app/wire_gen.go` (generated)

- [ ] **Step 1: Define providers in wire.go**

Create `services/api-gateway/internal/app/wire.go`:
```go
//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/circuitbreaker"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/redis"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/admin"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/clients"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/ws"
)

// InitializeApp constructs App from config.
func InitializeApp(cfg *config.Config) (*App, func(), error) {
	wire.Build(
		wire.Struct(new(App), "*"),
		provideLogger,
		provideCircuitBreaker,
		provideClientFactory,
		provideRedis,
		provideFeatureFlags,
		provideRateLimiter,
		provideHub,
		provideAdminHandler,
		provideResolver,
		provideHTTPServer,
		provideMetricsServer,
	)
	return nil, nil, nil
}
```

- [ ] **Step 2: Implement providers in app.go**

Rewrite `services/api-gateway/internal/app/app.go`:
```go
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/abtesting"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/circuitbreaker"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/redis"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/admin"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/clients"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/ws"
	"go.uber.org/zap"
)

// App encapsulates the gateway application.
type App struct {
	log     *zap.Logger
	http    *server.HTTP
	metrics *server.Metrics
}

// New creates a new App (kept for backward compat; prefer InitializeApp).
func New(cfg *config.Config) (*App, func(), error) {
	return InitializeApp(cfg)
}

// Run starts servers and waits for shutdown signal.
func (a *App) Run() error {
	go func() {
		a.log.Info("starting gateway")
		if err := a.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.log.Fatal("gateway serve failed", zap.Error(err))
		}
	}()

	go func() {
		a.log.Info("starting metrics server")
		if err := a.metrics.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.log.Fatal("metrics server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	a.log.Info("shutting down gateway")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.metrics.Shutdown(shutdownCtx); err != nil {
		a.log.Error("metrics server shutdown error", zap.Error(err))
	}
	return a.http.Shutdown(shutdownCtx)
}

func provideLogger(cfg *config.Config) (*zap.Logger, error) {
	return logger.New(cfg.LogLevel, cfg.LogFormat)
}

func provideCircuitBreaker() *circuitbreaker.CircuitBreaker {
	return circuitbreaker.New(5, 2, 30*time.Second)
}

func provideClientFactory(cfg *config.Config, cb *circuitbreaker.CircuitBreaker) *clients.Factory {
	return clients.NewFactory(cfg, cb)
}

func provideRedis(ctx context.Context, cfg *config.Config) (*redis.Client, error) {
	return redis.NewClient(ctx, cfg.RedisAddr)
}

func provideFeatureFlags(ctx context.Context, redisClient *redis.Client) (*featureflags.Engine, error) {
	engine := featureflags.NewEngine(redisClient)
	_ = engine.LoadFromRedis()
	engine.Register(&featureflags.Flag{Name: "new-checkout-flow", Enabled: false, Strategy: "default"})
	engine.Register(&featureflags.Flag{Name: "fast-search", Enabled: false, Strategy: "default"})
	engine.Register(&featureflags.Flag{Name: "discount-system", Enabled: false, Strategy: "default"})
	engine.Register(&featureflags.Flag{Name: "real-time-updates", Enabled: false, Strategy: "default"})
	_ = engine.SaveToRedis()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = engine.LoadFromRedis()
			case <-engine.Done():
				return
			}
		}
	}()
	return engine, nil
}

func provideRateLimiter(redisClient *redis.Client, cfg *config.Config) middleware.RateLimiter {
	return middleware.NewRoleRateLimiter(redisClient, cfg.RateLimitUserRPS, cfg.RateLimitAdminRPS, cfg.RateLimitWindow)
}

func provideHub() *ws.Hub {
	hub := ws.NewHub()
	go hub.Run()
	return hub
}

func provideAdminHandler(ffEngine *featureflags.Engine) http.Handler {
	return admin.NewRouter(admin.NewHandler(ffEngine))
}

func provideResolver(
	factory *clients.Factory,
	ffEngine *featureflags.Engine,
	hub *ws.Hub,
	redisClient *redis.Client,
	cfg *config.Config,
) (*graph.Resolver, error) {
	ctx := context.Background()
	userConn, err := factory.NewClient(ctx, cfg.UserServiceAddr)
	if err != nil {
		return nil, fmt.Errorf("user client: %w", err)
	}
	catalogConn, err := factory.NewClient(ctx, cfg.CatalogServiceAddr)
	if err != nil {
		return nil, fmt.Errorf("catalog client: %w", err)
	}
	orderConn, err := factory.NewClient(ctx, cfg.OrderServiceAddr)
	if err != nil {
		return nil, fmt.Errorf("order client: %w", err)
	}
	inventoryConn, err := factory.NewClient(ctx, cfg.InventoryServiceAddr)
	if err != nil {
		return nil, fmt.Errorf("inventory client: %w", err)
	}
	paymentConn, err := factory.NewClient(ctx, cfg.PaymentServiceAddr)
	if err != nil {
		return nil, fmt.Errorf("payment client: %w", err)
	}
	analyticsConn, err := factory.NewClient(ctx, cfg.AnalyticsServiceAddr)
	if err != nil {
		return nil, fmt.Errorf("analytics client: %w", err)
	}

	return &graph.Resolver{
		UserService:        userv1.NewUserServiceClient(userConn),
		CatalogService:     catalogv1.NewCatalogServiceClient(catalogConn),
		OrderService:       orderv1.NewOrderServiceClient(orderConn),
		InventoryService:   inventoryv1.NewInventoryServiceClient(inventoryConn),
		PaymentService:     paymentv1.NewPaymentServiceClient(paymentConn),
		AnalyticsService:   analyticsv1.NewAnalyticsServiceClient(analyticsConn),
		FeatureFlagsEngine: ffEngine,
		ABExperiments:      defaultExperiments(),
		Hub:                hub,
		Redis:              redisClient,
		CallTimeout:        cfg.DefaultCallTimeout,
		QueryTimeout:       cfg.DefaultQueryTimeout,
	}, nil
}

func defaultExperiments() []*abtesting.Experiment {
	return []*abtesting.Experiment{
		{
			Name: "checkout-button-color",
			Variations: []abtesting.Variation{
				{Name: "control", Weight: 50},
				{Name: "green", Weight: 50},
			},
		},
		{
			Name: "search-algorithm",
			Variations: []abtesting.Variation{
				{Name: "v1", Weight: 70},
				{Name: "v2", Weight: 30},
			},
		},
	}
}

func provideHTTPServer(
	cfg *config.Config,
	resolver *graph.Resolver,
	hub *ws.Hub,
	rl middleware.RateLimiter,
	adminHandler http.Handler,
) *server.HTTP {
	return server.NewHTTP(cfg, resolver, hub, rl, adminHandler)
}

func provideMetricsServer(cfg *config.Config) *server.Metrics {
	return server.NewMetrics(fmt.Sprintf(":%d", cfg.MetricsPort))
}
```

Note: Add missing proto imports (`analyticsv1`, `catalogv1`, etc.).

- [ ] **Step 3: Generate wire code**

```bash
cd services/api-gateway
go generate ./internal/app/...
```

If wire not installed:
```bash
go install github.com/google/wire/cmd/wire@latest
wire ./internal/app
```

- [ ] **Step 4: Run build**

```bash
cd services/api-gateway
go build ./...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api-gateway/internal/app/app.go services/api-gateway/internal/app/wire.go services/api-gateway/internal/app/wire_gen.go
git commit -m "refactor(api-gateway/app): wire-based DI and slim app.go"
```

---

### Task 11: Update `cmd/main.go`

**Files:**
- Modify: `services/api-gateway/cmd/main.go`

- [ ] **Step 1: Handle cleanup from InitializeApp**

```go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()
	log, err := logger.New(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		panic(err)
	}

	tp, err := tracing.InitTracer("api-gateway", cfg.OTELExporterOTLPEndpoint)
	if err != nil {
		log.Fatal("init tracer", zap.Error(err))
	}
	defer func() {
		if err := tracing.ShutdownTracer(tp, context.Background()); err != nil {
			log.Error("shutdown tracer", zap.Error(err))
		}
	}()

	application, cleanup, err := app.New(cfg)
	if err != nil {
		log.Fatal("init app", zap.Error(err))
	}
	defer cleanup()

	if err := application.Run(); err != nil {
		log.Fatal("gateway error", zap.Error(err))
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}
```

- [ ] **Step 2: Commit**

```bash
git add services/api-gateway/cmd/main.go
git commit -m "refactor(api-gateway): main uses wire-generated app constructor"
```

---

### Task 12: Adapt and run tests

**Files:**
- Modify: `services/api-gateway/internal/app/*_test.go`
- Modify: `services/api-gateway/internal/admin/*_test.go`
- Modify: `services/api-gateway/internal/ws/*_test.go`

- [ ] **Step 1: Run full test suite**

```bash
cd services/api-gateway
go test ./... -count=1
```

- [ ] **Step 2: Fix broken tests**

Likely issues:
- `app.New` now returns `(*App, func(), error)` instead of `*App`.
- Admin tests may need to use chi router to extract URL params.
- WS tests may need updated JWT claims import.

- [ ] **Step 3: Commit fixes**

```bash
git add services/api-gateway
git commit -m "test(api-gateway): adapt tests to refactored constructors"
```

---

### Task 13: Update documentation

**Files:**
- Modify: `services/api-gateway/README.md`

- [ ] **Step 1: Fix README inaccuracies**

- Mention all 6 downstream services, not just user/catalog.
- Document that metrics run on a separate port (`METRICS_PORT = PORT + 1000`).
- Document WebSocket, admin API, feature flags.

- [ ] **Step 2: Commit**

```bash
git add services/api-gateway/README.md
git commit -m "docs(api-gateway): update README to match implementation"
```

---

## Self-review checklist

- [ ] Spec coverage: all design decisions have a task.
- [ ] Placeholder scan: no TBD/TODO in steps.
- [ ] Type consistency: `auth.Role`, `auth.CustomClaims`, `middleware.RateLimiter` used consistently.
- [ ] Import paths verified against actual module name.
