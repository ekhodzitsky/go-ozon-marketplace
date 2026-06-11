# Changelog

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/),
проект следует [Semantic Versioning](https://semver.org/lang/ru/spec/v2.0.0.html).

## [0.3.0] — 2026-06-10

### Added
- GraphQL gateway: order-service, inventory-service, payment-service clients
- GraphQL queries: me, orders, inventory, order, cancelOrder
- gRPC методы: CancelOrder, UpdateOrderStatus, UpdateProduct, DeleteProduct
- Observability: OpenTelemetry tracing, configurable LOG_LEVEL/LOG_FORMAT, Prometheus metrics
- Kafka consumers: analytics-service, notification-service
- DLQ: payment-service dead letter queue
- Inventory ledger таблица и API
- Payment refunds таблица и API
- Unit и E2E тесты для всех новых функций
- K8s HPA для всех сервисов
- K8s monitoring stack: Prometheus, Grafana, Jaeger
- K8s NetworkPolicies

### Changed
- Документация приведена в соответствие с кодом (15 файлов, 1600+ строк)
- CI/CD: добавлены integration тесты
- Makefile: test-integration, test-e2e targets

### Fixed
- Имена env vars в документации (POSTGRES_DSN, REDIS_ADDR и т.д.)
- GraphQL примеры в QUICKSTART.md

## [0.2.0] — 2026-06-10

### Безопасность
- Харднинг JWT: `WithValidMethods(HS256)`, `WithExpirationRequired`, проверка `iss`/`aud`, убрана утечка raw-ошибок клиенту
- Проверка длины `JWT_SECRET >= 32` на старте (fail-fast)
- mTLS между сервисами (взаимная TLS-аутентификация gateway→downstream)
- Авторизация по ролям (`user`/`admin`/`service`) во всех критичных RPC
- IDOR/BOLA: `GetUser` доступен только себе или админу
- Анти-энумерация: единая ошибка логина, dummy-bcrypt на not-found
- Защита от timing-атак на логин/регистрацию
- GraphQL introspection/playground за env-флагом
- Rate limiter: Redis-backed sliding window, XFF-aware, MaxBytesReader
- gRPC recovery interceptor (паника не валит сервис)
- CORS-политика на gateway

### Надёжность
- Durable Saga с state-machine, persisted state, recovery worker, retry с backoff
- Корректная компенсация: release только зарезервированного, refund при необходимости
- Transactional Outbox (order-service + catalog-service): атомарная запись в БД + outbox
- Реальный Kafka-publisher (sarama) с DLQ и poison-handling
- Реестр резерваций (inventory ledger): идемпотентность, анти-oversell
- Атомарные платежи: tx-manager с `BEGIN ... FOR UPDATE ... COMMIT`
- Фатальный старт-фейл: проверка downstream на старте, нет fallback на моки
- gRPC keepalive, retry policy, round_robin на клиентах
- Default timeouts на всех downstream-вызовах (5s call / 3s query)

### Наблюдаемость
- OTLP exporter вместо deprecated jaeger
- Access-log + request-id + `/metrics` (Prometheus) на gateway
- Конфигурируемый logger (`LOG_LEVEL`, `LOG_FORMAT`) + `Sync()` на shutdown
- pgx-пул с OpenTelemetry tracer + Prometheus PoolCollector
- gRPC histogram с явными latency-бакетами
- `trace_id` в логах через middleware
- Health probes (`grpc.health.v1`) во всех сервисах

### База данных
- CHECK constraints (`>=0`) на склад, количество, цену, сумму
- Индексы: `order_items(order_id)`, `orders(user_id, created_at DESC)`
- `UNIQUE(order_id)` на payments
- `NOT NULL` на timestamps + триггер `set_updated_at`
- Переход от `float64` к `int64` minor units (ценовые поля)
- Разделение `products.stock` → только `inventory-service`
- Партиционирование ClickHouse (`toYYYYMM`), `ZSTD`, `TTL`

### CI / Инфраструктура
- Пиннинг GitHub Actions по SHA
- `govulncheck` в CI
- `buf lint` / `buf breaking` в CI
- Починен coverage gate: обход модулей `go.work`
- Убраны `coverage*.out` из индекса, обновлён `.gitignore`

### Тесты
- Testcontainers (PG, Redis, ES, Kafka, ClickHouse)
- gomock-моки во всех сервисах
- Unit-тесты на все handlers с проверкой gRPC-кодов ошибок
- Saga unit tests (state machine, compensation, recovery)
- E2E сценарии (price tamper, payment fail → refund)
- Builder-паттерн для fluent-запросов

### Исправления
- `uuid.Parse` → `codes.InvalidArgument`
- Убраны payload/PII из логов (outbox, payment, notification)
- GraphQL complexity/depth limit + clamp `pageSize`
- Keyset-пагинация заказов (курсор `created_at,id`)
- ES explicit mapping + защита от битых полей
- Когерентность Redis-кэша (`singleflight`, метрики)
- Типизированные статусы платежей (enum + DB CHECK)

## [0.1.0] — 2026-06-09

### Добавлено
- 8 микросервисов: user, catalog, api-gateway, inventory, payment, order, notification, analytics
- GraphQL gateway с rate limiting
- Saga Orchestrator для распределённых транзакций
- Transactional Outbox pattern
- CQRS для каталога (PostgreSQL + Elasticsearch)
- Redis cache для inventory
- ClickHouse для аналитики
- JWT authentication interceptor
- Prometheus metrics и Grafana dashboards
- Kubernetes Helm charts
- GitHub Actions CI/CD
- Docker multi-stage builds

[0.3.0]: https://github.com/ekhodzitsky/go-ozon-marketplace/releases/tag/v0.3.0
[0.2.0]: https://github.com/ekhodzitsky/go-ozon-marketplace/releases/tag/v0.2.0
[0.1.0]: https://github.com/ekhodzitsky/go-ozon-marketplace/releases/tag/v0.1.0