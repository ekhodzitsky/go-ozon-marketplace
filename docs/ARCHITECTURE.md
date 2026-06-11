# Архитектура

Как устроен маркетплейс изнутри: сервисы, потоки данных, паттерны.

## Сервисы и их зоны ответственности

```mermaid
graph LR
    subgraph "Публичный слой"
        AG[api-gateway]
    end

    subgraph "Бизнес-логика"
        US[user-service]
        CS[catalog-service]
        OS[order-service]
        IS[inventory-service]
        PS[payment-service]
    end

    subgraph "Слушатели событий"
        NS[notification-service]
        AS[analytics-service]
    end

    subgraph "Хранилища"
        PG[(PostgreSQL)]
        Redis[(Redis)]
        ES[(Elasticsearch)]
        CH[(ClickHouse)]
        Kafka[Kafka / Redpanda]
    end

    AG --> US & CS
    OS --> IS & PS
    US & CS & OS & IS & PS --> PG
    IS --> Redis
    CS --> ES
    OS --> Kafka
    AS --> CH
```

| Сервис | Что делает | Хранилище | Ключевой паттерн |
|--------|------------|-----------|-----------------|
| **api-gateway** | Принимает GraphQL, маршрутизирует на gRPC, rate limiting, access-log | — | API Gateway, Rate Limiter |
| **user-service** | Регистрация, аутентификация, JWT с ролями | PostgreSQL | — |
| **catalog-service** | CRUD товаров, поиск (ES), Outbox relay в Elasticsearch | PostgreSQL + Elasticsearch | CQRS, Outbox |
| **order-service** | Жизненный цикл заказа, Saga Orchestrator | PostgreSQL | Saga Orchestrator, Outbox |
| **inventory-service** | Остатки, резервирование | PostgreSQL + Redis | Pessimistic locking (FOR UPDATE), Cache-aside |
| **payment-service** | Проведение платежей, возвраты | PostgreSQL | Saga Participant |
| **notification-service** | Отправка email по gRPC (service-only) | — | — |
| **analytics-service** | Запись событий, выручка за день | ClickHouse | Batch insert |

## Поток данных: создание заказа (Saga)

```mermaid
sequenceDiagram
    participant C as Клиент
    participant AG as api-gateway
    participant OS as order-service
    participant ODB as order-db
    participant OB as outbox
    participant K as Kafka
    participant IS as inventory-service
    participant PS as payment-service

    C->>AG: GraphQL: login → получить JWT
    C->>AG: gRPC CreateOrder (через CLI или напрямую)
    AG->>OS: gRPC CreateOrder
    OS->>ODB: BEGIN TX
    OS->>ODB: INSERT order (status=pending)
    OS->>OB: INSERT outbox (event=OrderCreated)
    ODB-->>OS: COMMIT
    OS-->>AG: order_id
    AG-->>C: order

    OS->>IS: gRPC Reserve (sync)
    IS->>IS: UPDATE inventory (FOR UPDATE)
    IS-->>OS: success / fail

    alt Reserve failed
        OS->>ODB: UPDATE status=cancelled
    else Reserve success
        OS->>PS: gRPC ProcessPayment (sync)
        PS->>PS: INSERT payment
        PS-->>OS: success / fail

        alt Payment failed
            OS->>PS: gRPC Refund (compensation)
            OS->>IS: gRPC Release (compensation)
            OS->>ODB: UPDATE status=cancelled
        else Payment success
            OS->>ODB: UPDATE status=confirmed
        end
    end
```

**Важно:** Saga работает через **прямые gRPC вызовы**, не через Kafka events. Kafka используется только для Outbox релея (публикация событий, пока без consumers).

## Saga: компенсация при ошибке

```mermaid
flowchart TD
    A[CreateOrder] --> B[Reserve inventory]
    B -->|Успех| C[Process payment]
    B -->|Ошибка| D[CancelOrder]
    C -->|Успех| E[ConfirmOrder]
    C -->|Ошибка| F[RefundPayment]
    F --> D
    D --> G[Release inventory]
```

## CQRS: каталог

```mermaid
graph LR
    A[Admin / API] -->|Write| PG[(PostgreSQL)]
    PG -->|Outbox relay| ES[(Elasticsearch)]
    B[Search API] -->|Read| ES
```

- **Записи** — нормализованная схема в PostgreSQL
- **Чтение** — денормализованные документы в Elasticsearch
- Синхронизация через **Outbox relay напрямую в ES** (не через Kafka)

## Связи между сервисами

### Синхронные (gRPC)

| Вызов | От | К | Зачем |
|-------|----|---|-------|
| Register / Login | api-gateway | user-service | Аутентификация |
| GetUser | api-gateway | user-service | Получить профиль |
| CreateProduct / GetProduct / SearchProducts | api-gateway | catalog-service | Каталог |
| CreateOrder / GetOrder / ListOrders | напрямую | order-service | Заказы |
| Reserve / Release / GetStock | order-service | inventory-service | Резервирование |
| ProcessPayment / Refund | order-service | payment-service | Платежи |
| SendEmail | напрямую | notification-service | Email (service-only) |
| TrackEvent / GetDailyRevenue | напрямую | analytics-service | Аналитика (service-only) |

### Асинхронные (Kafka)

Пока используется только в **order-service** для Outbox релея. Consumers ещё не реализованы.

| Событие | Продюсер | Консумеры |
|---------|----------|-----------|
| `OrderCreated` | order-service | *(пока нет)* |

## Масштабирование

- **api-gateway** — stateless, масштабируется горизонтально, rate limiter через Redis
- **catalog-service** — read-heavy, кэш в Redis, поиск в Elasticsearch
- **order-service** — Saga state machine в БД
- **inventory-service** — `FOR UPDATE` + Redis cache
- **analytics-service** — batch insert в ClickHouse

## Что ещё не реализовано

- Kafka consumers в notification-service и analytics-service
- Circuit breaker
- DLQ в payment-service (есть только в order-service outbox)
- Optimistic locking (используется pessimistic)
- Materialized views в ClickHouse
- ZSTD сжатие и TTL в ClickHouse
