# go-ozon-marketplace

Микросервисный e-commerce backend на Go — pet-проект для портфолио.

## Архитектура

8 микросервисов, GraphQL API Gateway, распределённые транзакции (Saga), CQRS, Transactional Outbox, Kafka и полный observability-стек.

### Микросервисы

| Сервис | Порт | Назначение |
|--------|------|------------|
| api-gateway | 8080 | GraphQL gateway, rate limiting, access-log, `/metrics` |
| user-service | 50051 | Регистрация, аутентификация (JWT с ролями) |
| catalog-service | 50052 | Каталог товаров, поиск (PG + ES), transactional outbox |
| inventory-service | 50053 | Управление остатками, ledger резерваций, Redis cache |
| payment-service | 50054 | Обработка платежей, атомарные транзакции, refunds |
| order-service | 50055 | Заказы, Saga Orchestrator, Outbox, Kafka publisher |
| notification-service | 50056 | Уведомления (service-only RPC) |
| analytics-service | 50057 | Аналитика в ClickHouse (партиционирование, ZSTD, TTL) |

### Технологии

- **Go 1.26**, gRPC, GraphQL (gqlgen)
- **PostgreSQL 16**, Redis 7, ClickHouse, Elasticsearch
- **Kafka** (Redpanda), Transactional Outbox, Saga Orchestrator
- **Prometheus**, Grafana, **OpenTelemetry** → OTLP
- **mTLS** между сервисами
- **Kubernetes**, Helm, GitHub Actions CI/CD

## Запуск

```bash
# Инфраструктура
cd infra/docker && docker compose up -d

# Сборка сервиса
docker build --build-arg SERVICE_NAME=api-gateway -t api-gateway:latest -f Dockerfile .
```

## Тесты

```bash
# Unit + integration
make test

# E2E (требуется Docker)
go test -tags=e2e ./tests/e2e/...

# Сборка всех модулей
make build
```

## Структура

```
├── api/                # Protobuf + generated gRPC/GraphQL
├── pkg/                # Shared packages (middleware, logger, metrics, tracing)
├── services/           # 8 микросервисов
├── infra/              # Docker Compose, Helm charts, monitoring
├── tests/              # Integration и E2E тесты (testcontainers)
└── docs/               # Design docs, ADR, specs
```

## CI/CD

GitHub Actions:
- **lint** (`golangci-lint`)
- **proto** (`buf lint`, `buf breaking`)
- **govulncheck**
- **test** (race, coverage gate 60%, итерация по `go.work` модулям)
- **build** (Docker images для всех сервисов)
- **helm** (lint + template validate)

Все Actions запиннены по SHA.

## Лицензия

MIT