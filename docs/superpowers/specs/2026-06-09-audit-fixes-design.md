# Design: Post-Audit Critical & High Issue Fixes

## Goal
Fix all 🔴 Critical and 🟠 High issues identified in the second swarm audit of `go-ozon-marketplace`.

## Success Criteria
- `docker build` works for all 8 services
- CI pipeline passes (lint, test, build, helm validate)
- All integration/E2E tests pass
- No hardcoded secrets in code or Helm values
- Auth propagation works end-to-end (Gateway → gRPC)
- Order service uses JWT context for user_id (no IDOR)
- Rate limiter has bounded memory
- Outbox relay is thread-safe and actually publishes (or logs with clear TODO for Kafka)
- Saga returns errors properly (no silent swallowing)

## Approach: Interleaved Infrastructure & Code
Fix in dependency order — unblock CI first, then fix logic, then harden packaging.

### Phase 1: Infrastructure Foundation (Unblocks Everything)
1. **Fix Docker build** — update base image to `golang:1.26-alpine`, build from repo root with workspace support, add `.dockerignore`
2. **Fix CI pipeline** — add Docker build job, Helm lint job, pin golangci-lint version
3. **Fix test infrastructure** — add `JWT_SECRET` to test helpers, replace `go run` with `go build`, fix `GetFreePort` race, use `t.Cleanup` for testcontainers

### Phase 2: Critical Business Logic Bugs
4. **Fix OrderUsecase** — don't swallow Saga errors, return `fmt.Errorf` wrapped
5. **Fix Outbox Relay** — add `sync.Mutex` + `sync.WaitGroup`, prevent double Start/Stop, check for goroutine leak
6. **Fix RateLimiter** — replace unbounded `map` with LRU cache (eviction at 10k entries)
7. **Fix Gateway auth propagation** — call `AppendAuthFromHTTP` in every resolver before gRPC call

### Phase 3: Security Gaps
8. **Fix Order Service IDOR** — extract `user_id` from JWT context, reject mismatches
9. **Remove hardcoded secrets** — use `valueFrom.secretKeyRef` in Helm, env vars in docker-compose
10. **Add TLS wiring option** — allow `CERT_PATH` env to switch gRPC to TLS (keep insecure as dev default)

### Phase 4: Architecture & Performance
11. **Fix Dependency Rule in order-service** — introduce `UnitOfWork` interface, inject interfaces not concrete types
12. **Fix Saga interfaces** — decouple from proto/grpc, use domain DTOs + adapter pattern
13. **Fix Saga N+1 gRPC** — add `ReserveBatch` / `ReleaseBatch` to inventory proto (or document as future optimization)
14. **Fix GracefulStop hang** — add 25s timeout with fallback to `Stop()`
15. **Fix API Gateway graceful shutdown** — `http.Server` with `Shutdown()` on SIGTERM

### Phase 5: Helm & Observability Hardening
16. **Fix Helm defaults** — require `image.tag`, add `securityContext`, `PodDisruptionBudget`, `grpc` probes
17. **Fix pgx.ErrNoRows mapping** — centralize error mapping to gRPC codes (NotFound vs Internal)

## Out of Scope (Documented as Future Work)
- Real Kafka producer integration (outbox currently logs + marks processed)
- mTLS inter-service (TLS helpers exist but not enforced)
- CORS middleware
- Redis cache stampede protection
- ClickHouse async buffered writer

## Risks
- Large change set — may introduce regressions. Mitigation: fix tests first, then code.
- Breaking Helm values API — acceptable for portfolio project.
