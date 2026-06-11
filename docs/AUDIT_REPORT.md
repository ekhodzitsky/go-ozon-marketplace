# Аудит документации

**Дата:** 2026-06-10
**Версия кода:** v0.2.0
**Объём:** 15 документов, ~1600 строк

---

## Резюме

Документация **существенно расходится** с реальным кодом. ~40% заявленных API-методов, GraphQL-операций, переменных окружения и архитектурных паттернов **не реализованы** или реализованы иначе.

| Файл | OK | WARNING | ERROR |
|------|----|---------|-------|
| README.md | 7 | 2 | 3 |
| ARCHITECTURE.md | 2 | 0 | 9 |
| QUICKSTART.md | 5 | 2 | 5 |
| API.md | 6 | 3 | 18 |
| SECURITY.md | 6 | 4 | 7 |
| DEPLOYMENT.md | 6 | 5 | 7 |
| CONTRIBUTING.md | 10 | 7 | 3 |
| api-gateway/README | 2 | 3 | 10 |
| user-service/README | 2 | 4 | 6 |
| catalog-service/README | 1 | 2 | 10 |
| inventory-service/README | 3 | 2 | 7 |
| order-service/README | 3 | 4 | 8 |
| payment-service/README | 1 | 1 | 10 |
| notification-service/README | 2 | 0 | 9 |
| analytics-service/README | 1 | 1 | 9 |
| **ИТОГО** | **57** | **40** | **121** |

---

## Критические категории (ERROR)

### 1. GraphQL API — полностью нерабочие примеры

**Где:** `docs/QUICKSTART.md`, `docs/API.md`, `services/api-gateway/README.md`

**Проблема:** Документация описывает GraphQL операции, которых нет в схеме:

| Утверждённая операция | Реальная схема | Вердикт |
|-----------------------|----------------|---------|
| `register(input: {...}) { id, email, token }` | `register(email, password, name): ID!` | Нет `input`, нет `token` в ответе |
| `createProduct(input: {...}) { id, name, price }` | `createProduct(name, description, price, categories): ID!` | Нет `input`, нет `categoryId`, `stock` |
| `createOrder(input: { items: [...] })` | **Отсутствует** | Нет в `schema.graphqls` |
| `order(id: "...") { ... items { ... } }` | **Отсутствует** | Нет в `schema.graphqls` |
| `products(filter, pagination) { items { ... } total }` | **Отсутствует** | Есть `searchProducts(query, page, pageSize)` |
| `me` | **Отсутствует** | — |
| `cancelOrder` | **Отсутствует** | — |
| `updateProduct` | **Отсутствует** | — |
| `deleteProduct` | **Отсутствует** | — |
| `orders(userId, status)` | **Отсутствует** | — |
| `inventory(productId)` | **Отсутствует** | — |

**Последствие:** Пользователь, следуя QUICKSTART.md, не сможет создать заказ.

### 2. gRPC методы — ~50% несуществующих

**Где:** `docs/API.md`, README всех сервисов

| Сервис | Утверждённые методы | Реальные методы в proto |
|--------|---------------------|------------------------|
| user-service | `Register, Login, GetUser, ValidateToken, UpdateUser, DeleteUser` | `Register, Login, GetUser` |
| catalog-service | `CreateProduct, GetProduct, UpdateProduct, DeleteProduct, SearchProducts, ListCategories, GetCategory` | `CreateProduct, GetProduct, ListProducts, SearchProducts` |
| order-service | `CreateOrder, GetOrder, CancelOrder, ListOrders, UpdateOrderStatus` | `CreateOrder, GetOrder, ListOrders` |
| inventory-service | `GetStock, ReserveStock, ReleaseStock, UpdateStock, GetLedger` | `Reserve, Release, GetStock` |
| payment-service | `ProcessPayment, RefundPayment, GetPaymentStatus, GetPayment` | `ProcessPayment, Refund` |
| notification-service | `SendNotification, GetNotificationStatus` | `SendEmail` |
| analytics-service | `RecordEvent, GetReport, GetMetrics` | `TrackEvent, GetDailyRevenue` |

### 3. JWT — 5 из 6 полей не реализованы

**Где:** `docs/SECURITY.md`, `docs/API.md`

**В доке:** `sub`, `role`, `iss`, `aud`, `jti`, `nbf`, `exp`
**В коде:** `user_id`, `exp` (только 2 поля)

**Также:**
- `register` возвращает `ID!`, а не `token` — GraphQL пользователь не получает JWT
- `role` вообще не записывается в токен
- Gateway не валидирует JWT, только прокидывает `Authorization` в gRPC metadata

### 4. Переменные окружения — неверные имена и несуществующие

**Неверные имена (дока vs код):**

| В доке | В коде | Где |
|--------|--------|-----|
| `POSTGRES_URL` | `POSTGRES_DSN` | Все сервисы |
| `REDIS_URL` | `REDIS_ADDR` | api-gateway, inventory-service |
| `CLICKHOUSE_URL` | `CLICKHOUSE_DSN` | analytics-service |
| `ELASTICSEARCH_URL` | `ES_URL` | catalog-service |
| `HTTP_PORT` | `PORT` | api-gateway |
| `METRICS_PORT` | не существует | — |
| `ORDER_SERVICE_ADDR` | не существует | api-gateway |
| `INVENTORY_SERVICE_ADDR` | не существует | api-gateway |
| `PAYMENT_SERVICE_ADDR` | не существует | api-gateway |

**Переменные, описанные в README сервисов, но отсутствующие в `config.go`:**
- `JWT_EXPIRY`, `BCRYPT_COST` (user-service)
- `SAGA_TIMEOUT`, `OUTBOX_POLL_INTERVAL` (order-service)
- `PAYMENT_TIMEOUT` (payment-service)
- `BATCH_SIZE`, `BATCH_TIMEOUT` (analytics-service)
- `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `FROM_EMAIL` (notification-service)
- `LOG_LEVEL`, `LOG_FORMAT`, `OTEL_EXPORTER_OTLP_ENDPOINT` (все сервисы)

### 5. SQL схемы — таблицы и колонки не существуют

| Сервис | Утверждённая схема | Реальность |
|--------|-------------------|------------|
| user-service | `users.updated_at` | Отсутствует в миграциях |
| catalog-service | Таблица `categories`, поля `sku`, `category_id` | Отсутствуют |
| inventory-service | `inventory.quantity`, `version`, `updated_at` | Есть `available`, `reserved`, нет `version`, `updated_at` |
| inventory-service | Таблица `inventory_ledger` | Отсутствует |
| order-service | `outbox.processed BOOLEAN` | Есть `processed_at TIMESTAMP` |
| order-service | `saga_state.started_at`, `completed_at` | Есть `sagas.created_at`, `updated_at`, `error_message` |
| payment-service | `payments.currency`, `provider`, `transaction_id`, `updated_at` | Отсутствуют |
| payment-service | Таблица `payment_refunds` | Отсутствует |
| analytics-service | Таблица `events` с 7+ полями | Есть `event_type`, `aggregate_id`, `payload`, `created_at`, `aggregation_key` |
| analytics-service | Таблица `orders_daily` | Отсутствует |

### 6. Архитектурные паттерны — заявлены, но не реализованы

| Паттерн | Заявлено в | Реальность |
|---------|-----------|------------|
| **Circuit Breaker** | README.md, ARCHITECTURE.md, SECURITY.md, api-gateway/README | Не реализован |
| **CQRS через Kafka** | ARCHITECTURE.md, catalog-service/README | Outbox relay пишет напрямую в ES, Kafka не используется |
| **Event-driven consumers** | ARCHITECTURE.md | notification-service, analytics-service не имеют Kafka consumer |
| **DLQ** | ARCHITECTURE.md, payment-service/README | Есть только в order-service outbox relay |
| **Optimistic locking** | ARCHITECTURE.md, inventory-service/README | Используется `FOR UPDATE` (pessimistic) |
| **Materialized views** | ARCHITECTURE.md | Не реализованы |
| **ZSTD, TTL** | ARCHITECTURE.md, analytics-service/README | Не реализованы |

### 7. Rate limiting — неверные значения

**В SECURITY.md:** user 100/мин, admin 1000/мин, service без лимита
**В коде:** один лимитер для всех, default `10 RPS`

### 8. Другие критические расхождения

- **LICENSE badge** → файл отсутствует
- **Makefile** → нет таргета `build`
- **mTLS** → описана как обязательная, реально опциональная (`CertPath`)
- **OpenTelemetry** → пакет есть, но ни один сервис не вызывает `InitTracer`
- **Kafka в сервисах** → заявлена в 5 сервисах, используется только в order-service
- **HPA в Helm** → упоминается, но файлы `hpa.yaml` отсутствуют
- **Integration тесты в CI** → не запускаются (нет `-tags=integration`)
- **Docker compose** → нет сервисов приложений, только инфраструктура

---

## WARNING (предупреждения)

| Проблема | Файл | Пояснение |
|----------|------|-----------|
| mTLS опциональна | SECURITY.md | Используется только при `CertPath != ""` |
| OpenTelemetry не инициализирован | README.md | Пакет `pkg/tracing` есть, но не используется |
| Grafana не упомянута | QUICKSTART.md | Поднимается на 3000, но не описана |
| Нет инструкций по env vars | QUICKSTART.md | Сервисы падают без `POSTGRES_DSN`, `JWT_SECRET` |
| `traceparent` не найден | API.md | Не реализовано в коде |
| Деньги в proto — `double` | SECURITY.md, CONTRIBUTING.md | В БД `BIGINT`, но в proto и GraphQL `float64` |
| `searchProducts` параметры | API.md | В доке опущены `page`/`pageSize` |
| `make migrate-user` vs `DB_URL` | CONTRIBUTING.md | Несоответствие имён переменных |
| godoc не везде | CONTRIBUTING.md | Многие экспортируемые типы без комментариев |
| Saga статусы | order-service/README | Промежуточные `reserved`, `paid` не описаны |

---

## Рекомендации

### Вариант А: Привести документацию к коду (быстрее)
Удалить несуществующие методы, API, переменные. Описать только то, что реально работает.

### Вариант Б: Привести код к документации (дольше)
Реализовать заявленные методы, GraphQL операции, JWT поля, Kafka consumers и т.д.

### Вариант В: Гибрид
- Исправить очевидные ошибки в документации (имена env vars, имена методов)
- Добавить раздел "Что ещё не реализовано" в каждом документе
- Постепенно реализовывать фичи по приоритету
