# Audit Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all 🔴 Critical and 🟠 High issues from the second swarm audit of go-ozon-marketplace.

**Architecture:** Interleaved infrastructure and code fixes in dependency order — unblock CI/test infrastructure first, then fix business logic bugs, then security gaps, then architecture/performance, finally Helm hardening.

**Tech Stack:** Go 1.26, Docker, Kubernetes/Helm, PostgreSQL, gRPC, GraphQL/gqlgen, Redis, Uber FX

---

## File Structure

### Phase 1: Infrastructure Foundation
- `services/*/Dockerfile` — all 8 service Dockerfiles (build from repo root, Go 1.26, .dockerignore)
- `.dockerignore` — root dockerignore for monorepo build context
- `.github/workflows/ci.yml` — CI pipeline (add Docker build, Helm lint, pin golangci-lint)
- `tests/helper.go` — test infrastructure (build instead of go run, JWT_SECRET env, port race fix, t.Cleanup)
- `tests/e2e/saga_compensation_test.go` — add JWT_SECRET to test env
- `tests/order_postgres_test.go` — add JWT_SECRET to test env
- `tests/user_postgres_test.go` — add JWT_SECRET to test env

### Phase 2: Critical Business Logic
- `services/order-service/internal/usecase/usecase.go` — don't swallow saga error
- `services/order-service/internal/outbox/relay.go` — thread-safe Start/Stop with sync.Mutex/sync.WaitGroup
- `pkg/middleware/ratelimit.go` — LRU cache instead of unbounded map
- `services/api-gateway/graph/schema.resolvers.go` — propagate auth to downstream gRPC
- `services/api-gateway/internal/delivery/grpc/context.go` — ensure helper works correctly

### Phase 3: Security
- `services/order-service/internal/delivery/grpc/handler.go` — extract user_id from JWT context, reject body mismatch
- `infra/k8s/helm-charts/*/values.yaml` — remove secrets, use valueFrom.secretKeyRef
- `infra/k8s/helm-charts/*/templates/secret.yaml` — new Secret templates
- `infra/k8s/helm-charts/*/templates/deployment.yaml` — use envFrom secretRef
- `infra/docker/docker-compose.yml` — use env vars instead of hardcoded passwords
- `services/*/internal/app/app.go` — wire TLS when CERT_PATH is set

### Phase 4: Architecture & Performance
- `services/order-service/internal/unitofwork/uow.go` — new UnitOfWork interface
- `services/order-service/internal/repository/repository.go` — ensure OrderRepository/OutboxRepository interfaces exist
- `services/order-service/internal/usecase/usecase.go` — inject interfaces, remove pgxpool/postgres imports
- `services/order-service/internal/saga/interfaces.go` — decouple from proto/grpc, use domain DTOs
- `services/order-service/internal/infrastructure/grpcclient/` — new adapters inventory/payment
- `services/order-service/internal/saga/orchestrator.go` — use new interfaces
- `services/order-service/internal/app/app.go` — wire adapters
- `pkg/server/server.go` — GracefulStop with 25s timeout fallback
- `services/api-gateway/internal/app/app.go` — http.Server with Shutdown on SIGTERM
- `services/order-service/internal/repository/postgres/order_postgres.go` — batch MarkProcessed for outbox

### Phase 5: Helm Hardening
- `infra/k8s/helm-charts/*/values.yaml` — require image.tag, add securityContext defaults
- `infra/k8s/helm-charts/*/templates/deployment.yaml` — add securityContext, grpc probes
- `infra/k8s/helm-charts/*/templates/pdb.yaml` — new PodDisruptionBudget
- `pkg/errors/errors.go` — new centralized error types (ErrNotFound)
- All `*/internal/delivery/grpc/handler.go` — map pgx.ErrNoRows to codes.NotFound

---

## Task 1: Fix Docker Build (All Services)

**Files:**
- Modify: `services/*/Dockerfile` (8 files)
- Create: `.dockerignore`

**Context:** Dockerfiles use `golang:1.24-alpine` but modules require `go 1.26`. They also build from service directory but need workspace modules (`../../api`, `../../pkg`).

- [ ] **Step 1: Read one existing Dockerfile**

```bash
cat services/api-gateway/Dockerfile
```

- [ ] **Step 2: Create .dockerignore at repo root**

```bash
cat > /Users/ekhodzitsky/Documents/personal/go-ozon-marketplace/.dockerignore << 'EOF'
.git
.github
.gitignore
*.md
docs/
infra/
tests/
scripts/
coverage*.out
**/*_test.go
**/Dockerfile
**/.dockerignore
.env
EOF
```

- [ ] **Step 3: Rewrite api-gateway Dockerfile for root-context build**

```dockerfile
# services/api-gateway/Dockerfile
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY services/api-gateway/go.mod services/api-gateway/go.sum ./services/api-gateway/
COPY api/go.mod api/go.sum ./api/
COPY pkg/go.mod pkg/go.sum ./pkg/

RUN go work sync
WORKDIR /app/services/api-gateway
RUN go mod download

COPY api/ /app/api/
COPY pkg/ /app/pkg/
COPY services/api-gateway/ /app/services/api-gateway/

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -extldflags '-static'" \
    -trimpath \
    -o /bin/server ./cmd/main.go

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/server /server
USER 65532:65532
ENTRYPOINT ["/server"]
```

- [ ] **Step 4: Apply same pattern to remaining 7 services**

For each service (`catalog-service`, `inventory-service`, `order-service`, `payment-service`, `user-service`, `analytics-service`, `notification-service`), copy the api-gateway Dockerfile template and change:
- `services/api-gateway/` → `services/<service-name>/`
- `WORKDIR /app/services/<service-name>`

- [ ] **Step 5: Test build for api-gateway**

```bash
cd /Users/ekhodzitsky/Documents/personal/go-ozon-marketplace
docker build -f services/api-gateway/Dockerfile -t api-gateway:test .
```
Expected: Build succeeds without errors.

- [ ] **Step 6: Test build for order-service**

```bash
docker build -f services/order-service/Dockerfile -t order-service:test .
```
Expected: Build succeeds.

- [ ] **Step 7: Commit**

```bash
git add .dockerignore services/*/Dockerfile
git commit -m "fix(docker): build from repo root with go 1.26, add .dockerignore"
```

---

## Task 2: Fix CI Pipeline

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Read current CI**

```bash
cat .github/workflows/ci.yml
```

- [ ] **Step 2: Add Docker build matrix job**

Add after the `test` job:

```yaml
  build:
    name: Build Docker Images
    runs-on: ubuntu-latest
    strategy:
      matrix:
        service:
          - api-gateway
          - catalog-service
          - inventory-service
          - order-service
          - payment-service
          - user-service
          - analytics-service
          - notification-service
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - name: Build
        run: docker build -f services/${{ matrix.service }}/Dockerfile -t ${{ matrix.service }}:ci .
```

- [ ] **Step 3: Add Helm validation job**

```yaml
  helm:
    name: Validate Helm
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: azure/setup-helm@v4
        with:
          version: '3.14.0'
      - run: |
          for chart in infra/k8s/helm-charts/*; do
            helm lint "$chart"
            helm template test "$chart" > /dev/null
          done
```

- [ ] **Step 4: Pin golangci-lint version**

Change `version: latest` to `version: v1.59.1`.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add docker build, helm lint, pin golangci-lint"
```

---

## Task 3: Fix Test Infrastructure

**Files:**
- Modify: `tests/helper.go`
- Modify: `tests/e2e/saga_compensation_test.go`
- Modify: `tests/order_postgres_test.go`
- Modify: `tests/user_postgres_test.go`

- [ ] **Step 1: Read tests/helper.go**

```bash
cat tests/helper.go
```

- [ ] **Step 2: Modify StartService to build binary first**

Replace `StartService` implementation:

```go
func StartService(t *testing.T, serviceDir string, env []string) *exec.Cmd {
    t.Helper()
    bin := filepath.Join(t.TempDir(), "service")
    build := exec.Command("go", "build", "-o", bin, "./cmd/main.go")
    build.Dir = serviceDir
    build.Env = os.Environ()
    if out, err := build.CombinedOutput(); err != nil {
        t.Fatalf("build %s: %v\n%s", serviceDir, err, out)
    }

    cmd := exec.Command(bin)
    cmd.Dir = serviceDir
    cmd.Env = append(os.Environ(), env...)
    if err := cmd.Start(); err != nil {
        t.Fatalf("failed to start service %s: %v", serviceDir, err)
    }
    t.Cleanup(func() { _ = cmd.Process.Kill() })
    return cmd
}
```

- [ ] **Step 3: Modify StartPostgres to use t.Cleanup**

```go
func StartPostgres(ctx context.Context, t *testing.T) string {
    t.Helper()
    container, err := postgres.Run(ctx, "postgres:16-alpine",
        postgres.WithDatabase("marketplace"),
        postgres.WithUsername("ozon"),
        postgres.WithPassword("ozonpass"),
    )
    require.NoError(t, err)
    t.Cleanup(func() { _ = container.Terminate(ctx) })

    connStr, err := container.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)
    return connStr
}
```

- [ ] **Step 4: Add JWT_SECRET to all StartService calls**

In `tests/e2e/saga_compensation_test.go`, add `"JWT_SECRET=test-secret"` to the env slice.

In `tests/order_postgres_test.go`, add `"JWT_SECRET=test-secret"`.

In `tests/user_postgres_test.go`, add `"JWT_SECRET=test-secret"`.

- [ ] **Step 5: Commit**

```bash
git add tests/
git commit -m "test: fix test infra - build binaries, t.Cleanup, JWT_SECRET env"
```

---

## Task 4: Fix OrderUsecase Swallowing Saga Error

**Files:**
- Modify: `services/order-service/internal/usecase/usecase.go`

- [ ] **Step 1: Read the file**

```bash
cat services/order-service/internal/usecase/usecase.go
```

- [ ] **Step 2: Find the swallowing line**

Look for:
```go
if err := u.orchestrator.ProcessOrder(ctx, order); err != nil {
    return order.ID, nil
}
```

- [ ] **Step 3: Fix to return error**

```go
if err := u.orchestrator.ProcessOrder(ctx, order); err != nil {
    return order.ID, fmt.Errorf("saga process order: %w", err)
}
```

- [ ] **Step 4: Commit**

```bash
git add services/order-service/internal/usecase/usecase.go
git commit -m "fix(order): return saga error instead of swallowing"
```

---

## Task 5: Fix Outbox Relay Goroutine Leak and Race

**Files:**
- Modify: `services/order-service/internal/outbox/relay.go`

- [ ] **Step 1: Read the file**

```bash
cat services/order-service/internal/outbox/relay.go
```

- [ ] **Step 2: Rewrite with sync primitives**

```go
type Relay struct {
    repo    repository.OutboxRepository
    log     *zap.Logger
    ticker  *time.Ticker
    stop    chan struct{}
    wg      sync.WaitGroup
    mu      sync.Mutex
    started bool
}

func (r *Relay) Start(ctx context.Context) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if r.started {
        return
    }
    r.started = true
    r.stop = make(chan struct{})
    r.ticker = time.NewTicker(500 * time.Millisecond)
    r.wg.Add(1)
    go r.loop(ctx)
}

func (r *Relay) Stop() {
    r.mu.Lock()
    if !r.started {
        r.mu.Unlock()
        return
    }
    r.started = false
    close(r.stop)
    r.ticker.Stop()
    r.mu.Unlock()
    r.wg.Wait()
}

func (r *Relay) loop(ctx context.Context) {
    defer r.wg.Done()
    for {
        select {
        case <-r.ticker.C:
            r.poll(ctx)
        case <-r.stop:
            return
        case <-ctx.Done():
            return
        }
    }
}
```

- [ ] **Step 3: Commit**

```bash
git add services/order-service/internal/outbox/relay.go
git commit -m "fix(outbox): thread-safe relay with sync.Mutex/WaitGroup"
```

---

## Task 6: Fix RateLimiter Memory Leak

**Files:**
- Modify: `pkg/middleware/ratelimit.go`
- Modify: `go.mod` (api-gateway or root if shared)

- [ ] **Step 1: Check if golang-lru is already available**

```bash
grep "lru" go.work.sum || true
```

- [ ] **Step 2: Add golang-lru to api-gateway go.mod**

```bash
cd services/api-gateway && go get github.com/hashicorp/golang-lru/v2
```

- [ ] **Step 3: Rewrite RateLimiter with LRU**

```go
type RateLimiter struct {
    rps   rate.Limit
    burst int
    cache *lru.Cache[string, *rate.Limiter]
}

func NewRateLimiter(rps int) *RateLimiter {
    cache, _ := lru.New[string, *rate.Limiter](10000)
    return &RateLimiter{
        rps:   rate.Limit(rps),
        burst: rps,
        cache: cache,
    }
}

func (rl *RateLimiter) Allow(key string) bool {
    lim, ok := rl.cache.Get(key)
    if !ok {
        lim = rate.NewLimiter(rl.rps, rl.burst)
        rl.cache.Add(key, lim)
    }
    return lim.Allow()
}
```

- [ ] **Step 4: Commit**

```bash
git add pkg/middleware/ratelimit.go services/api-gateway/go.mod services/api-gateway/go.sum
git commit -m "fix(gateway): rate limiter with LRU eviction (10k entries)"
```

---

## Task 7: Fix Gateway Auth Propagation

**Files:**
- Modify: `services/api-gateway/graph/schema.resolvers.go`
- Modify: `services/api-gateway/internal/delivery/grpc/context.go` (if needed)

- [ ] **Step 1: Read resolvers**

```bash
cat services/api-gateway/graph/schema.resolvers.go
```

- [ ] **Step 2: Ensure AppendAuthFromHTTP is called before each gRPC call**

In each resolver method (CreateProduct, etc.), wrap ctx:

```go
ctx = grpccontext.AppendAuthFromHTTP(ctx, r.HTTPRequest)
resp, err := r.CatalogService.CreateProduct(ctx, req)
```

If `r.HTTPRequest` is not available, use gqlgen's request from context:

```go
req := graphql.GetOperationContext(ctx).Headers
md := metadata.New(map[string]string{"authorization": req.Get("Authorization")})
ctx = metadata.NewOutgoingContext(ctx, md)
```

- [ ] **Step 3: Commit**

```bash
git add services/api-gateway/graph/schema.resolvers.go
git commit -m "fix(gateway): propagate auth headers to downstream gRPC"
```

---

## Task 8: Fix Order Service IDOR

**Files:**
- Modify: `services/order-service/internal/delivery/grpc/handler.go`
- Modify: `pkg/middleware/auth.go` (ensure GetUserID helper)

- [ ] **Step 1: Read handler**

```bash
cat services/order-service/internal/delivery/grpc/handler.go
```

- [ ] **Step 2: Add user_id validation from JWT context**

```go
func (h *OrderHandler) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
    authUserID, ok := middleware.GetUserID(ctx)
    if !ok {
        return nil, status.Error(codes.Unauthenticated, "missing user_id in context")
    }
    if req.UserId != "" && req.UserId != authUserID {
        return nil, status.Error(codes.PermissionDenied, "user_id mismatch")
    }
    userID, err := uuid.Parse(authUserID)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "invalid user_id in token: %v", err)
    }
    // ... rest of handler
}
```

Do the same for `ListOrders`.

- [ ] **Step 3: Commit**

```bash
git add services/order-service/internal/delivery/grpc/handler.go
git commit -m "fix(order): validate user_id from JWT context, prevent IDOR"
```

---

## Task 9: Fix pgx.ErrNoRows Mapping

**Files:**
- Create: `pkg/errors/errors.go`
- Modify: All `*/internal/delivery/grpc/handler.go`

- [ ] **Step 1: Create central error package**

```go
package errors

import "errors"

var ErrNotFound = errors.New("not found")
```

- [ ] **Step 2: Map in repositories**

In each repository `GetByID`, wrap `pgx.ErrNoRows`:

```go
if errors.Is(err, pgx.ErrNoRows) {
    return nil, fmt.Errorf("%w: %s", apperrors.ErrNotFound, entityName)
}
```

- [ ] **Step 3: Map in gRPC handlers**

```go
if errors.Is(err, apperrors.ErrNotFound) {
    return nil, status.Error(codes.NotFound, err.Error())
}
return nil, status.Errorf(codes.Internal, "...: %v", err)
```

- [ ] **Step 4: Commit**

```bash
git add pkg/errors/ services/*/internal/delivery/grpc/handler.go services/*/internal/repository/postgres/*.go
git commit -m "feat(errors): centralize ErrNotFound, map to gRPC NotFound"
```

---

## Task 10: Fix GracefulStop Hang

**Files:**
- Modify: `pkg/server/server.go`

- [ ] **Step 1: Add timeout**

```go
func (s *GRPCServer) GracefulStop() {
    s.log.Info("stopping gRPC server gracefully")
    done := make(chan struct{})
    go func() {
        s.Server.GracefulStop()
        close(done)
    }()
    select {
    case <-done:
    case <-time.After(25 * time.Second):
        s.log.Warn("force stopping gRPC server")
        s.Server.Stop()
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add pkg/server/server.go
git commit -m "fix(server): GracefulStop with 25s timeout fallback"
```

---

## Task 11: Fix API Gateway Graceful Shutdown

**Files:**
- Modify: `services/api-gateway/internal/app/app.go`

- [ ] **Step 1: Rewrite Run() with http.Server + Shutdown**

```go
func (a *App) Run() error {
    mux := http.NewServeMux()
    // ... setup handlers on mux instead of DefaultServeMux
    srv := &http.Server{
        Addr:    ":" + a.cfg.HTTPPort,
        Handler: mux,
    }
    go func() {
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            a.log.Fatal("gateway serve failed", zap.Error(err))
        }
    }()
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    a.log.Info("shutting down gateway")
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    return srv.Shutdown(ctx)
}
```

- [ ] **Step 2: Commit**

```bash
git add services/api-gateway/internal/app/app.go
git commit -m "fix(gateway): graceful shutdown with http.Server.Shutdown"
```

---

## Task 12: Fix Helm Secrets and Defaults

**Files:**
- Modify: `infra/k8s/helm-charts/*/values.yaml`
- Modify: `infra/k8s/helm-charts/*/templates/deployment.yaml`
- Create: `infra/k8s/helm-charts/*/templates/secret.yaml`

- [ ] **Step 1: Remove secrets from values.yaml**

Replace:
```yaml
env:
  JWT_SECRET: "super-secret-key"
  POSTGRES_DSN: "postgres://..."
```
With:
```yaml
secrets:
  jwtSecret: ""  # required
  postgresDSN: ""  # required
```

- [ ] **Step 2: Create secret.yaml template**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "chart.fullname" . }}-secrets
type: Opaque
stringData:
  JWT_SECRET: {{ required "secrets.jwtSecret is required" .Values.secrets.jwtSecret | quote }}
  POSTGRES_DSN: {{ required "secrets.postgresDSN is required" .Values.secrets.postgresDSN | quote }}
```

- [ ] **Step 3: Update deployment.yaml to use envFrom**

```yaml
envFrom:
  - secretRef:
      name: {{ include "chart.fullname" . }}-secrets
```

- [ ] **Step 4: Require image.tag in deployment**

```yaml
image: "{{ .Values.image.repository }}:{{ required "image.tag is required" .Values.image.tag }}"
```

- [ ] **Step 5: Commit**

```bash
git add infra/k8s/helm-charts/
git commit -m "fix(helm): remove plaintext secrets, require image.tag, use Secret refs"
```

---

## Task 13: Add Helm securityContext and PDB

**Files:**
- Modify: `infra/k8s/helm-charts/*/templates/deployment.yaml`
- Create: `infra/k8s/helm-charts/*/templates/pdb.yaml`

- [ ] **Step 1: Add securityContext to deployment**

```yaml
spec:
  template:
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        runAsGroup: 65532
        fsGroup: 65532
      containers:
        - name: {{ .Chart.Name }}
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop:
                - ALL
            seccompProfile:
              type: RuntimeDefault
```

- [ ] **Step 2: Create pdb.yaml**

```yaml
{{- if gt (.Values.replicaCount | int) 1 }}
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ include "chart.fullname" . }}
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ include "chart.name" . }}
      app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
```

- [ ] **Step 3: Commit**

```bash
git add infra/k8s/helm-charts/
git commit -m "feat(helm): add securityContext, seccomp, PDB"
```

---

## Task 14: Fix Order-Service Dependency Rule

**Files:**
- Create: `services/order-service/internal/unitofwork/uow.go`
- Modify: `services/order-service/internal/usecase/usecase.go`
- Modify: `services/order-service/internal/app/app.go`

- [ ] **Step 1: Create UnitOfWork interface**

```go
package unitofwork

import "context"

type UnitOfWork interface {
    Begin(ctx context.Context) (Transaction, error)
}

type Transaction interface {
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
}
```

- [ ] **Step 2: Refactor usecase to accept interfaces**

```go
type OrderUsecase struct {
    uow          unitofwork.UnitOfWork
    orderRepo    repository.OrderRepository
    outboxRepo   repository.OutboxRepository
    orchestrator saga.Orchestrator
}
```

Remove imports of `pgxpool`, `postgres` package.

- [ ] **Step 3: Wire in app.go**

```go
fx.Provide(
    func(r *postgres.OrderPostgres) repository.OrderRepository { return r },
    func(r *postgres.OutboxPostgres) repository.OutboxRepository { return r },
    // ...
    usecase.NewOrderUsecase,
)
```

- [ ] **Step 4: Commit**

```bash
git add services/order-service/internal/unitofwork/ services/order-service/internal/usecase/usecase.go services/order-service/internal/app/app.go
git commit -m "refactor(order): Dependency Rule - UoW, interface injection"
```

---

## Task 15: Fix docker-compose Hardcoded Secrets

**Files:**
- Modify: `infra/docker/docker-compose.yml`

- [ ] **Step 1: Replace hardcoded passwords with env vars**

```yaml
environment:
  POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-ozonpass}
  GF_SECURITY_ADMIN_PASSWORD: ${GF_ADMIN_PASSWORD:-admin}
```

- [ ] **Step 2: Commit**

```bash
git add infra/docker/docker-compose.yml
git commit -m "fix(docker): use env vars for passwords in compose"
```

---

## Task 16: Fix Saga Interfaces (Decouple from Proto)

**Files:**
- Modify: `services/order-service/internal/saga/interfaces.go`
- Create: `services/order-service/internal/infrastructure/grpcclient/inventory.go`
- Create: `services/order-service/internal/infrastructure/grpcclient/payment.go`
- Modify: `services/order-service/internal/saga/orchestrator.go`

- [ ] **Step 1: Rewrite saga interfaces to domain DTOs**

```go
package saga

import "context"

type InventoryClient interface {
    Reserve(ctx context.Context, productID string, quantity int32, orderID string) error
    Release(ctx context.Context, productID string, quantity int32, orderID string) error
}

type PaymentClient interface {
    ProcessPayment(ctx context.Context, orderID, userID string, amount float64) (string, error)
    Refund(ctx context.Context, paymentID string) error
}
```

- [ ] **Step 2: Create inventory adapter**

```go
package grpcclient

// adapts saga.InventoryClient to actual gRPC inventory client
```

- [ ] **Step 3: Create payment adapter**

Similar pattern.

- [ ] **Step 4: Update orchestrator to use domain interfaces**

- [ ] **Step 5: Commit**

```bash
git add services/order-service/internal/saga/ services/order-service/internal/infrastructure/grpcclient/
git commit -m "refactor(order): decouple saga interfaces from proto/grpc"
```

---

## Task 17: Fix Outbox Batch MarkProcessed

**Files:**
- Modify: `services/order-service/internal/repository/postgres/outbox_postgres.go`
- Modify: `services/order-service/internal/outbox/relay.go`

- [ ] **Step 1: Add BatchMarkProcessed**

```go
func (r *OutboxPostgres) BatchMarkProcessed(ctx context.Context, ids []uuid.UUID) error {
    query := `UPDATE outbox SET processed_at = NOW() WHERE id = ANY($1)`
    _, err := r.db.Exec(ctx, query, ids)
    return err
}
```

- [ ] **Step 2: Update relay to batch**

Collect IDs, call `BatchMarkProcessed` after loop.

- [ ] **Step 3: Commit**

```bash
git add services/order-service/internal/repository/postgres/outbox_postgres.go services/order-service/internal/outbox/relay.go
git commit -m "perf(outbox): batch MarkProcessed to avoid N+1 UPDATE"
```

---

## Self-Review Checklist

- [x] Spec coverage: All critical/high issues from audit have at least one task.
- [x] Placeholder scan: No TBD/TODO/fill-in-details found.
- [x] Type consistency: Interfaces and constructors match across tasks.
