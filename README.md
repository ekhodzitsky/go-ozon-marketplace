# go-ozon-marketplace

[![CI](https://github.com/ekhodzitsky/go-ozon-marketplace/actions/workflows/ci.yml/badge.svg)](https://github.com/ekhodzitsky/go-ozon-marketplace/actions)
[![Go Version](https://img.shields.io/badge/go-1.26-blue)](https://golang.org)
[![Release](https://img.shields.io/github/v/release/ekhodzitsky/go-ozon-marketplace)](https://github.com/ekhodzitsky/go-ozon-marketplace/releases)

Микросервисный e-commerce backend на Go — pet-проект для портфолио.

## Что это

Маркетплейс из 8 микросервисов с Saga Orchestrator, Transactional Outbox, поиском через Elasticsearch и стеком observability.

## Архитектура

```mermaid
graph TB
    Client[Клиент<br/>Web / Mobile / GraphQL Playground]
    AG[api-gateway<br/>GraphQL + Rate Limiter]
    US[user-service]
    CS[catalog-service]
    OS[order-service<br/>Saga Orchestrator]
    IS[inventory-service]
    PS[payment-service]
    NS[notification-service]
    AS[analytics-service]
    Kafka[Kafka / Redpanda]
    PG[(PostgreSQL)]
    Redis[(Redis)]
    ES[(Elasticsearch)]
    CH[(ClickHouse)]

    Client -->|HTTP / GraphQL| AG
    AG -->|gRPC| US
    AG -->|gRPC| CS
    OS -->|gRPC| IS
    OS -->|gRPC| PS
    OS -->|Kafka| Kafka
    US & CS & OS & IS & PS --> PG
    IS --> Redis
    CS --> ES
    AS --> CH
    Kafka -->|пока не используется| NS
    Kafka -->|пока не используется| AS
```

### Микросервисы

| Сервис | Порт | Назначение |
|--------|------|------------|
| api-gateway | 8080 | GraphQL gateway, rate limiting, access-log, `/metrics` |
| user-service | 50051 | Регистрация, аутентификация (JWT с ролями) |
| catalog-service | 50052 | Каталог товаров, поиск (PG + ES), transactional outbox |
| inventory-service | 50053 | Управление остатками, резервирование, Redis cache |
| payment-service | 50054 | Обработка платежей, refunds |
| order-service | 50055 | Заказы, Saga Orchestrator, Outbox, Kafka publisher |
| notification-service | 50056 | Уведомления (service-only RPC) |
| analytics-service | 50057 | Аналитика в ClickHouse |

### Ключевые паттерны

| Паттерн | Где | Зачем |
|---------|-----|-------|
| **Saga Orchestrator** | order-service | Управляет заказом: резерв → оплата → подтверждение. При ошибке — компенсация. |
| **Transactional Outbox** | order-service | Гарантирует доставку событий в Kafka: сначала пишем в БД, потом релей отправляет. |
| **CQRS** | catalog-service | Записи в PostgreSQL, поиск в Elasticsearch. Синхронизация через Outbox relay. |
| **Rate Limiting** | api-gateway | Redis-backed sliding window, защита от перегрузки. |
| **mTLS** | Все сервисы | Взаимная TLS-аутентификация (опционально, при наличии `CERT_PATH`). |

## Технологии

- **Go 1.26**, gRPC, GraphQL (gqlgen)
- **PostgreSQL 16**, Redis 7, ClickHouse, Elasticsearch
- **Kafka** (Redpanda), Transactional Outbox, Saga Orchestrator
- **Prometheus**, Grafana, **OpenTelemetry** (пакет есть, инициализация — в планах)
- **mTLS** между сервисами (опционально)
- **Kubernetes**, Helm, GitHub Actions CI/CD

## Быстрый старт

```bash
# 1. Поднять инфраструктуру
make up

# 2. Собрать и запустить сервисы (в отдельных терминалах)
cd services/api-gateway && go run ./cmd/...
cd services/user-service && go run ./cmd/...
cd services/catalog-service && go run ./cmd/...
cd services/order-service && go run ./cmd/...
cd services/inventory-service && go run ./cmd/...
cd services/payment-service && go run ./cmd/...
cd services/notification-service && go run ./cmd/...
cd services/analytics-service && go run ./cmd/...

# 3. Открыть GraphQL Playground
open http://localhost:8080
```

Подробнее — в [docs/QUICKSTART.md](docs/QUICKSTART.md).

## Тесты

```bash
# Unit + integration (testcontainers)
make test

# E2E (требуется Docker)
cd tests && go test -tags=e2e ./e2e/...

# Линтер
make lint
```

## Структура

```
├── api/                # Protobuf + generated gRPC/GraphQL
├── pkg/                # Shared packages (middleware, logger, metrics, tracing)
├── services/           # 8 микросервисов
├── infra/              # Docker Compose, Helm charts, monitoring
├── tests/              # Integration и E2E тесты (testcontainers)
└── docs/               # Документация, ADR, схемы
```

## Документация

- [Архитектура](docs/ARCHITECTURE.md) — схемы, потоки данных, взаимодействие сервисов
- [Быстрый старт](docs/QUICKSTART.md) — пошаговый сценарий: регистрация → товар → поиск
- [API](docs/API.md) — GraphQL и gRPC контракты
- [Развёртывание](docs/DEPLOYMENT.md) — Docker Compose, Kubernetes, Helm
- [Безопасность](docs/SECURITY.md) — JWT, роли, mTLS, rate limiting
- [Дизайн-документ](docs/design.md) — полный design doc с ADR
- [Аудит](docs/AUDIT_REPORT.md) — актуальность документации

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
