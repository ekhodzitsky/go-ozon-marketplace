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

    AG --> US & CS & OS
    OS --> IS & PS
    US & CS & OS & IS & PS --> PG
    IS --> Redis
    CS --> ES
    OS & IS & PS & CS & US --> Kafka
    Kafka --> NS & AS
    AS --> CH
```

| Сервис | Что делает | Хранилище | Ключевой паттерн |
|--------|------------|-----------|-----------------|
| **api-gateway** | Принимает GraphQL, маршрутизирует на gRPC, rate limiting, access-log | — | API Gateway, Rate Limiter |
| **user-service** | Регистрация, аутентификация, JWT с ролями | PostgreSQL | — |
| **catalog-service** | CRUD товаров, категории, публикация событий | PostgreSQL + Elasticsearch | CQRS, Outbox |
| **order-service** | Жизненный цикл заказа, оркестрация Saga | PostgreSQL | Saga Orchestrator, Outbox |
| **inventory-service** | Остатки, резервирование, ledger | PostgreSQL + Redis | Optimistic locking, Cache-aside |
| **payment-service** | Проведение платежей, возвраты | PostgreSQL | Saga Participant, DLQ |
| **notification-service** | Email, push-уведомления по событиям | — | Event-driven consumer |
| **analytics-service** | Агрегация событий, отчёты | ClickHouse | Materialized views, Batch insert |

## Поток данных: создание заказа

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
    participant NS as notification-service

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
```

## Saga: компенсация при ошибке

```mermaid
flowchart TD
    A[OrderCreated] --> B[InventoryReserve]
    B -->|Успех| C[PaymentProcess]
    B -->|Ошибка| D[CancelOrder]
    C -->|Успех| E[OrderConfirm]
    C -->|Ошибка| F[RefundPayment]
    F --> D
    D --> G[ReleaseInventory]
```

Если платёж не прошёл — деньги возвращаются, заказ отменяется, резерв снимается.

## CQRS: каталог

```mermaid
graph LR
    A[Admin Panel] -->|Write| PG[(PostgreSQL)]
    PG -->|Events| Kafka
    Kafka -->|Sync| ES[(Elasticsearch)]
    B[Search API] -->|Read| ES
```

- **Записи** — нормализованная схема в PostgreSQL
- **Чтение** — денормализованные документы в Elasticsearch
- Синхронизация через Kafka events

## Связи между сервисами

### Синхронные (gRPC)

| Вызов | От | К | Зачем |
|-------|----|---|-------|
| CreateOrder | api-gateway | order-service | Создать заказ |
| ReserveInventory | order-service | inventory-service | Зарезервировать товар |
| ProcessPayment | order-service | payment-service | Провести платёж |
| GetUser | api-gateway | user-service | Получить профиль |
| SearchProducts | api-gateway | catalog-service | Поиск товаров |

### Асинхронные (Kafka)

| Событие | Продюсер | Консумеры |
|---------|----------|-----------|
| `UserRegistered` | user-service | notification-service, analytics-service |
| `ProductCreated` | catalog-service | analytics-service |
| `OrderCreated` | order-service | inventory-service, analytics-service |
| `InventoryReserved` | inventory-service | order-service, payment-service |
| `InventoryReservationFailed` | inventory-service | order-service (компенсация) |
| `PaymentProcessed` | payment-service | order-service, notification-service, analytics-service |
| `PaymentFailed` | payment-service | order-service (компенсация), notification-service |
| `OrderConfirmed` | order-service | notification-service, analytics-service |
| `OrderCancelled` | order-service | inventory-service (освобождение), notification-service, analytics-service |

## Масштабирование

- **api-gateway** — stateless, масштабируется горизонтально, rate limiter через Redis
- **catalog-service** — read-heavy, кэш в Redis, поиск в Elasticsearch
- **order-service** — Saga state machine в БД, можно шардировать по `user_id`
- **inventory-service** — ledger + optimistic locking, шардирование по `product_id`
- **analytics-service** — batch insert в ClickHouse, партиционирование по месяцам
