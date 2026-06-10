# go-ozon-marketplace

Микросервисный e-commerce backend на Go — pet-проект для портфолио.

## Архитектура

8 микросервисов, GraphQL API Gateway, распределённые транзакции (Saga), CQRS и полный observability стек.

### Микросервисы

| Сервис | Порт | Назначение |
|--------|------|------------|
| api-gateway | 8080 | GraphQL gateway, rate limiting |
| user-service | 50051 | Регистрация, аутентификация (JWT) |
| catalog-service | 50052 | Каталог товаров, поиск (PG + ES) |
| inventory-service | 50053 | Управление остатками, Redis cache |
| payment-service | 50054 | Обработка платежей |
| order-service | 50055 | Заказы, Saga Orchestrator, Outbox |
| notification-service | 50056 | Уведомления |
| analytics-service | 50057 | Аналитика в ClickHouse |

### Технологии

- **Go 1.26**, gRPC, GraphQL (gqlgen)
- **PostgreSQL 16**, Redis 7, ClickHouse, Elasticsearch
- **Kafka** (Redpanda), Outbox pattern
- **Prometheus**, Grafana, Jaeger tracing
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
go test -race ./...

# E2E
go test ./tests/e2e/...
```

## Структура

```
├── api/                # Protobuf + generated gRPC/GraphQL
├── pkg/                # Shared packages (middleware, logger, metrics)
├── services/           # 8 микросервисов
├── infra/              # Docker Compose, Helm charts, monitoring
├── tests/              # Integration и E2E тесты
└── docs/               # Design docs, specs, plans
```

## CI/CD

GitHub Actions: lint → test (60% coverage gate) → Docker build → Helm lint

## Лицензия

MIT
