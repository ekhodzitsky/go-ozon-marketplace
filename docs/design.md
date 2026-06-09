# go-ozon-marketplace — Design Document

**Date:** 2026-06-09  
**Status:** Approved  
**Scope:** SOTA demo of microservice e-commerce backend for Ozon Go Developer portfolio  

---

## 1. Overview

`go-ozon-marketplace` is a production-grade demonstration of a microservice-based e-commerce marketplace built with Go. It showcases the exact technology stack, architectural patterns, and engineering practices used by Ozon in their high-load systems (3500+ microservices, ~2M RPS peak).

The project is intentionally designed as a **portfolio piece** — every decision is traceable to real Ozon engineering practices documented in public talks, blog posts, and open-source repositories.

---

## 2. Goals & Success Criteria

### Primary Goals
1. Demonstrate mastery of Go in a distributed systems context
2. Showcase Ozon-relevant technologies: gRPC, Kafka, PostgreSQL, Redis, ClickHouse, Kubernetes
3. Implement critical high-load patterns: CQRS, Saga, Outbox, Circuit Breaker, Rate Limiting
4. Provide full observability: metrics, traces, structured logs

### Success Criteria
- [ ] All 8 services start with single `make up` command
- [ ] End-to-end order flow works: create → reserve → pay → notify
- [ ] gRPC load tests show >1000 RPS with P95 < 100ms
- [ ] Unit + integration test coverage > 60%
- [ ] README contains architecture diagram, tech rationale, and benchmark results
- [ ] Kubernetes manifests deployable to any k8s cluster

---

## 3. Architecture

### 3.1. Service Topology

```
┌─────────────────────────────────────────────────────────────┐
│                        Clients                               │
│         (Web App / Mobile / Postman / GraphQL Playground)   │
└──────────────────────┬──────────────────────────────────────┘
                       │ HTTP / GraphQL
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  api-gateway  │ GraphQL Gateway, Rate Limiting, Auth        │
│               │ gRPC client to all downstream services      │
└───────┬───────┴──────────────┬──────────────────────────────┘
        │ gRPC                  │ Kafka (async events)
        ▼                       ▼
┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐
│  user-    │  │  catalog- │  │  order-   │  │  payment- │
│  service  │  │  service  │  │  service  │  │  service  │
│  (PG)     │  │  (PG+ES)  │  │  (PG)     │  │  (PG)     │
└───────────┘  └───────────┘  └─────┬─────┘  └─────┬─────┘
                                    │              │
┌───────────┐  ┌───────────┐        │ Kafka        │ Kafka
│  inventory│  │  analytics│        ▼              ▼
│  -service │  │  -service │  ┌───────────┐  ┌───────────┐
│  (PG+Redis│  │  (CH)     │  │notification│  │   DLQ     │
└───────────┘  └───────────┘  │  -service  │  │  (Kafka)  │
                              └───────────┘  └───────────┘
```

### 3.2. Service Responsibilities

| Service | Domain | Storage | Key Patterns |
|---------|--------|---------|--------------|
| `api-gateway` | API composition, routing | None | API Gateway, Rate Limiter |
| `user-service` | Identity, JWT, profiles | PostgreSQL | — |
| `catalog-service` | Products, categories, search | PostgreSQL + Elasticsearch | CQRS |
| `order-service` | Order lifecycle, orchestration | PostgreSQL | Event Sourcing, Outbox, Saga Orchestrator |
| `inventory-service` | Stock levels, reservation | PostgreSQL + Redis | Optimistic locking, Cache-aside |
| `payment-service` | Payment processing, refunds | PostgreSQL | Saga Participant, DLQ |
| `notification-service` | Email, push notifications | None | Event-driven consumer |
| `analytics-service` | Event aggregation, reports | ClickHouse | Materialized views, Batch insert |

---

## 4. Communication Patterns

### 4.1. Synchronous — gRPC
- Unary RPC for most operations (get user, create product)
- Server streaming for search results (catalog search with pagination)
- gRPC-Gateway exposes REST/GraphQL on top of proto contracts

### 4.2. Asynchronous — Kafka
All services publish domain events to Kafka:

| Event | Producer | Consumers |
|-------|----------|-----------|
| `UserRegistered` | user-service | notification-service, analytics-service |
| `ProductCreated` | catalog-service | analytics-service |
| `OrderCreated` | order-service | inventory-service, analytics-service |
| `InventoryReserved` | inventory-service | order-service, payment-service |
| `InventoryReservationFailed` | inventory-service | order-service (compensation) |
| `PaymentProcessed` | payment-service | order-service, notification-service, analytics-service |
| `PaymentFailed` | payment-service | order-service (compensation), notification-service |
| `OrderConfirmed` | order-service | notification-service, analytics-service |
| `OrderCancelled` | order-service | inventory-service (release), notification-service, analytics-service |

---

## 5. Critical Patterns

### 5.1. Transactional Outbox (order-service)
1. Business transaction writes to `orders` table AND `outbox` table in same DB transaction
2. Separate relay process polls `outbox` and publishes to Kafka
3. Marks outbox record as `processed`

**Why:** Guarantees at-least-once delivery without 2PC.

### 5.2. Saga — Order Processing
```
OrderCreated → InventoryReserve → PaymentProcess → OrderConfirm
                    ↓                      ↓
            ReserveFailed ──────────→ PaymentFailed
                    ↓                      ↓
              CancelOrder ←────────── Compensate
```

**Orchestrator:** order-service manages state machine.
**Compensation:** If any step fails, previous steps are compensated (release inventory, refund payment).

### 5.3. CQRS — Catalog Search
- **Write model:** PostgreSQL (normalized schema for product CRUD)
- **Read model:** Elasticsearch (denormalized product documents for search/filter)
- catalog-service publishes `ProductCreated/Updated/Deleted` → Kafka → analytics-service or sync worker updates ES

### 5.4. Circuit Breaker
- Implemented in gRPC client middleware (api-gateway → downstream)
- States: Closed → Open → Half-Open
- Prevents cascade failures during service degradation

### 5.5. Rate Limiting
- Token Bucket algorithm in api-gateway
- Redis-backed for distributed rate limiting across gateway replicas

---

## 6. Technology Stack

### 6.1. Core
| Component | Technology | Version |
|-----------|------------|---------|
| Language | Go | 1.23+ |
| RPC | gRPC + Protocol Buffers | v2 |
| Gateway | gRPC-Gateway + gqlgen | latest |
| DI | uber-go/fx | latest |

### 6.2. Data & Messaging
| Component | Technology | Notes |
|-----------|------------|-------|
| Primary DB | PostgreSQL 16 | pgx/pgxpool driver |
| Cache | Redis 7 | go-redis/v9 |
| Search | Elasticsearch 8 | olivere/elastic |
| Analytics | ClickHouse 24 | clickhouse-go/v2 |
| Message Broker | Kafka (Redpanda local) | segmentio/kafka-go |

### 6.3. Observability
| Component | Technology |
|-----------|------------|
| Metrics | Prometheus + Grafana |
| Tracing | OpenTelemetry → Jaeger |
| Logging | Zap (structured JSON) |

### 6.4. Testing
| Level | Tool |
|-------|------|
| Unit | testify + mockery |
| Integration | testcontainers-go |
| E2E | cute (ozontech) |
| Load | framer (ozontech) / ghz |

### 6.5. Infrastructure
| Component | Technology |
|-----------|------------|
| Local Orchestration | Docker Compose |
| Containerization | Docker (multi-stage) |
| Orchestration | Kubernetes + Helm |
| CI/CD | GitHub Actions |

---

## 7. Repository Structure

```
go-ozon-marketplace/
├── api/                          # Proto contracts (separate go.mod)
│   ├── proto/
│   │   ├── user/v1/user.proto
│   │   ├── catalog/v1/catalog.proto
│   │   ├── order/v1/order.proto
│   │   ├── inventory/v1/inventory.proto
│   │   ├── payment/v1/payment.proto
│   │   └── notification/v1/notification.proto
│   ├── buf.yaml
│   └── buf.gen.yaml
├── pkg/                          # Shared libraries (separate go.mod)
│   ├── logger/                   # Zap wrapper with OTel trace context
│   ├── errors/                   # Domain errors + gRPC code mapping
│   ├── middleware/               # gRPC interceptors (logging, recovery, metrics, auth)
│   ├── tracing/                  # OTel tracer provider setup
│   ├── kafka/                    # Producer & Consumer abstractions
│   ├── postgres/                 # Pgx pool + migration runner
│   ├── redis/                    # Redis client wrapper
│   └── server/                   # Graceful shutdown HTTP/gRPC server
├── services/                     # Each service = independent go.mod
│   ├── api-gateway/
│   │   ├── cmd/
│   │   ├── internal/
│   │   │   ├── app/              # fx app composition
│   │   │   ├── delivery/         # HTTP/GraphQL handlers
│   │   │   └── config/
│   │   ├── Dockerfile
│   │   └── go.mod
│   ├── user-service/
│   │   ├── cmd/
│   │   ├── internal/
│   │   │   ├── app/
│   │   │   ├── domain/           # Entities, value objects
│   │   │   ├── usecase/          # Business logic interfaces + impl
│   │   │   ├── repository/       # PG implementations
│   │   │   └── delivery/         # gRPC handlers
│   │   ├── migrations/
│   │   ├── Dockerfile
│   │   └── go.mod
│   ├── catalog-service/
│   ├── order-service/
│   ├── inventory-service/
│   ├── payment-service/
│   ├── notification-service/
│   └── analytics-service/
├── infra/
│   ├── docker/
│   │   └── docker-compose.yml
│   ├── k8s/
│   │   └── base/ + helm-charts/
│   └── monitoring/
│       ├── prometheus/
│       ├── grafana/dashboards/
│       └── jaeger/
├── tests/
│   └── e2e/                      # End-to-end test suites
├── scripts/
│   ├── migrate.sh
│   ├── proto-gen.sh
│   └── seed.sh
├── Makefile
├── go.work
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── cd.yml
└── README.md
```

---

## 8. Data Flow — Order Creation (Detailed)

```mermaid
sequenceDiagram
    participant C as Client
    participant AG as API Gateway
    participant OS as order-service
    participant ODB as order-db
    participant OB as outbox
    participant K as Kafka
    participant IS as inventory-service
    participant PS as payment-service
    participant NS as notification-service
    participant AS as analytics-service

    C->>AG: GraphQL: createOrder
    AG->>OS: gRPC CreateOrder
    OS->>ODB: BEGIN TX
    OS->>ODB: INSERT order (status=pending)
    OS->>OB: INSERT outbox (event=OrderCreated)
    ODB-->>OS: COMMIT
    OS-->>AG: order_id
    AG-->>C: order

    loop Outbox Relay
        OS->>OB: SELECT * WHERE processed=false
        OS->>K: Publish OrderCreated
        OS->>OB: UPDATE processed=true
    end

    K->>IS: OrderCreated
    IS->>IS: Reserve inventory
    IS->>K: Publish InventoryReserved

    K->>OS: InventoryReserved
    OS->>ODB: UPDATE status=awaiting_payment

    K->>PS: InventoryReserved
    PS->>PS: Process payment
    PS->>K: Publish PaymentProcessed

    K->>OS: PaymentProcessed
    OS->>ODB: UPDATE status=confirmed

    K->>NS: OrderConfirmed
    NS->>NS: Send email

    K->>AS: OrderConfirmed
    AS->>AS: INSERT ClickHouse
```

---

## 9. Observability Strategy

### 9.1. Logs
- Structured JSON via Zap
- Every log entry contains: `trace_id`, `span_id`, `service`, `timestamp`, `level`, `message`
- Correlation via `context.Context` propagation

### 9.2. Metrics
- **RED method:** Request rate, Error rate, Duration (histogram)
- **Business:** orders_created_total, orders_confirmed_total, revenue_rub
- **Infrastructure:** db_connections_active, kafka_consumer_lag, redis_hit_ratio

### 9.3. Traces
- OpenTelemetry auto-instrumentation for gRPC, HTTP, PostgreSQL, Redis, Kafka
- Trace context propagated via gRPC metadata and Kafka headers
- Jaeger for trace storage and UI

---

## 10. Testing Strategy

### 10.1. Unit Tests
- Table-driven tests (Go idiom)
- testify/assert + mockery for mocks
- Target: domain and usecase layers

### 10.2. Integration Tests
- testcontainers-go for PostgreSQL, Redis, Kafka, ClickHouse, Elasticsearch
- Each service has `*_integration_test.go`
- Run with `go test -tags=integration`

### 10.3. E2E Tests
- cute (ozontech) for HTTP/GraphQL API testing
- Allure reports
- Run against fully deployed Docker Compose stack

### 10.4. Load Tests
- framer for gRPC load generation
- ghz for alternative gRPC benchmarking
- Scenarios: create order, search catalog, get user

---

## 11. Deployment

### 11.1. Local Development
```bash
make up          # docker compose up --build
make test        # unit + integration tests
make e2e         # end-to-end tests
make bench       # load tests with framer
make down        # docker compose down -v
```

### 11.2. Kubernetes
- Helm chart per service (`infra/k8s/helm-charts/<service>/`)
- Base manifests: Deployment, Service, ConfigMap, Secret, Ingress
- HPA based on CPU/memory custom metrics
- PodDisruptionBudget for availability

---

## 12. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Scope too large | High | MVP first: gateway + order + inventory + payment. Add others incrementally |
| Kafka complexity | Medium | Use Redpanda for local dev (Kafka-compatible, single binary) |
| ClickHouse setup | Low | Start with single-node ClickHouse, omit for MVP if needed |
| Test flakiness | Medium | Use testcontainers with fixed versions, retry logic |

---

## 13. References

- [Ozon Tech — "Одна платформа, чтобы править всеми"](https://habr.com/ru/companies/ozontech/articles/708274/)
- [Ozon Tech — Kafka at 5M RPS](https://habr.com/ru/companies/ozontech/articles/749328/)
- [Ozon Tech GitHub](https://github.com/ozontech)
- [Route 256 — Go Middle](https://route256.ozon.ru/go-middle)
- [go-ecommerce-microservices reference](https://github.com/Solymani-Hossein/go-ecommerce-microservices)
- [Watermill — Go event-driven library](https://github.com/ThreeDotsLabs/watermill)

---

## 14. Changelog

| Date | Author | Change |
|------|--------|--------|
| 2026-06-09 | — | Initial design document |
