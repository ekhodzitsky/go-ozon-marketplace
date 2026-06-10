# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-06-09

### Added
- 8 microservices: user, catalog, api-gateway, inventory, payment, order, notification, analytics
- GraphQL gateway with rate limiting (Token Bucket)
- Saga Orchestrator for distributed transactions
- Transactional Outbox pattern
- CQRS for catalog (PostgreSQL + Elasticsearch)
- Redis cache for inventory
- ClickHouse for analytics events
- JWT authentication interceptor for all gRPC services
- Prometheus metrics and Grafana dashboards
- Jaeger distributed tracing
- Kubernetes Helm charts with HPA, PDB, security contexts
- GitHub Actions CI/CD (lint, test, build, helm validate)
- Docker multi-stage builds (distroless)

### Security
- Fixed IDOR in order-service (validate user_id from JWT context)
- Fixed gateway auth propagation to downstream gRPC
- Removed hardcoded secrets from Helm values and docker-compose
- Added TLS helpers for gRPC

### Fixed
- Docker build (Go workspace + root-context Dockerfile)
- CI pipeline (Go 1.26, pinned tools, docker/helm validation)
- Test infrastructure (binary builds, JWT_SECRET env, t.Cleanup)
- Order usecase swallowing saga errors
- Outbox relay goroutine leak and data race
- Rate limiter unbounded memory growth (LRU cache)
- GracefulStop hang (25s timeout)
- API Gateway graceful shutdown
- pgx.ErrNoRows mapping to gRPC NotFound
- Saga interfaces decoupled from proto/grpc
- Outbox batch MarkProcessed (N+1 eliminated)

[0.1.0]: https://github.com/ekhodzitsky/go-ozon-marketplace/releases/tag/v0.1.0
