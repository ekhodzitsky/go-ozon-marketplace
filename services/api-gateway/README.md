# api-gateway

API-шлюз для учебного маркетплейса. Единая точка входа для клиентов: GraphQL + WebSocket-подписки, JWT-аутентификация, rate limiting, feature flags и вызовы downstream-сервисов по gRPC.

- **axum** — HTTP-сервер и middleware
- **async-graphql** — GraphQL-схема, резолверы и подписки
- **tonic** — gRPC-клиенты downstream-сервисов
- **redis** — rate limiting, pub/sub подписок и feature flags
- **metrics-exporter-prometheus** — метрики

## Структура

```
src/
  main.rs              # entrypoint, инициализация, запуск сервера
  config.rs            # конфигурация из env
  auth.rs              # JWT verifier и middleware
  clients.rs           # фабрика tonic-клиентов с circuit breaker и метриками
  admin.rs             # admin endpoints /health, /ready, /metrics, /flags
  ws.rs                # WebSocket-эндпоинт для GraphQL subscriptions
  error.rs             # базовые ошибки
  ratelimit.rs         # Redis-backed sliding window rate limiter
  circuit_breaker.rs   # простой circuit breaker для downstream-вызовов
  feature_flags.rs     # хранилище и admin API фича-флагов
  metrics.rs           # Prometheus-метрики и middleware
  validation.rs        # валидация email, uuid, цен и количеств
  proto.rs             # include сгенерированного кода из proto
  graphql/
    mod.rs
    schema.rs          # Schema<Query, Mutation, Subscription>
    resolvers.rs       # GraphQL резолверы
    subscription.rs    # GraphQL subscriptions через Redis pub/sub
```

## Сборка

Требуется `buf` для экспорта proto-зависимостей (включая `buf/validate/validate.proto`):

```bash
cd services/api-gateway
cargo build
```

## Тесты

```bash
cd services/api-gateway
cargo test
```

## Запуск

```bash
# с дефолтными адресами downstream-сервисов и Redis на localhost
PORT=8080 JWT_SECRET=dev-secret REDIS_ADDR=redis://localhost:6379 cargo run
```

Переменные окружения (все опциональны, dev-значения совпадают с остальными сервисами):

| Переменная | Значение по умолчанию | Описание |
|---|---|---|
| `PORT` | `8080` | HTTP-порт шлюза |
| `USER_SERVICE_ADDR` | `localhost:50051` | user-service |
| `CATALOG_SERVICE_ADDR` | `localhost:50052` | catalog-service |
| `INVENTORY_SERVICE_ADDR` | `localhost:50053` | inventory-service |
| `PAYMENT_SERVICE_ADDR` | `localhost:50054` | payment-service |
| `ORDER_SERVICE_ADDR` | `localhost:50055` | order-service |
| `ANALYTICS_SERVICE_ADDR` | `localhost:50056` | analytics-service |
| `REDIS_ADDR` | `redis://localhost:6379` | Redis для rate limit, pub/sub, feature flags |
| `JWT_SECRET` | `dev-secret` | Секрет для проверки JWT |
| `CORS_ALLOWED_ORIGINS` | `*` (через tower-http `Any`) | Разрешённые origins |
| `RUST_LOG` | `info` | Уровень логирования |
| `RATE_LIMIT_REQUESTS` | `100` | Максимум запросов в окно |
| `RATE_LIMIT_WINDOW_SECONDS` | `60` | Окно rate limiter в секундах |
| `TLS_ENABLED` | `false` | Включить TLS для gRPC |
| `MTLS_ENABLED` | `false` | Включить mTLS для gRPC |
| `CERT_PATH` | `""` | Путь к TLS-сертификату/CA |
| `KEY_PATH` | `""` | Путь к TLS-ключу (для mTLS) |
| `INSECURE_SKIP_TLS` | `false` | Пропустить TLS для gRPC |

## Endpoints

| Endpoint | Метод | Описание |
|---|---|---|
| `/` | GET | GraphiQL playground |
| `/query` | GET/POST | GraphQL endpoint |
| `/ws` | GET | WebSocket для GraphQL subscriptions |
| `/admin/health` | GET | Health check |
| `/admin/ready` | GET | Readiness probe |
| `/admin/metrics` | GET | Prometheus-метрики |
| `/admin/flags` | GET | Список feature flags (требует admin JWT) |
| `/admin/flags/:flag/enable` | POST | Включить feature flag (требует admin JWT) |
| `/admin/flags/:flag/disable` | POST | Выключить feature flag (требует admin JWT) |

## GraphQL

Playground доступен по корню `/`. Основные операции:

```graphql
# авторизованный профиль
query Me {
  me { id email name createdAt }
}

# пользователь по id
query User($id: ID!) {
  user(id: $id) { id email name createdAt }
}

# список товаров с поиском и пагинацией
query Products($query: String, $page: Int, $pageSize: Int) {
  products(query: $query, page: $page, pageSize: $pageSize) {
    products { id name description price categories createdAt }
    total
  }
}

# товар по id
query Product($id: ID!) {
  product(id: $id) { id name description price categories createdAt }
}

# остатки товара
query Inventory($productId: ID!) {
  inventory(productId: $productId) { productId available reserved }
}

# заказ по id
query Order($id: ID!) {
  order(id: $id) { id userId items { productId quantity price } totalAmount status createdAt updatedAt }
}

# список заказов пользователя
query Orders($userId: ID!, $page: Int, $pageSize: Int) {
  orders(userId: $userId, page: $page, pageSize: $pageSize) {
    orders { id userId items { productId quantity price } totalAmount status createdAt updatedAt }
    total
  }
}

# регистрация
mutation Register($email: String!, $password: String!, $name: String!) {
  register(email: $email, password: $password, name: $name)
}

# вход
mutation Login($email: String!, $password: String!) {
  login(email: $email, password: $password)
}

# создание товара (admin)
mutation CreateProduct($name: String!, $description: String!, $price: Float!, $categories: [String!]!) {
  createProduct(name: $name, description: $description, price: $price, categories: $categories)
}

# создание заказа
mutation CreateOrder($items: [OrderItemInput!]!) {
  createOrder(items: $items)
}

# отмена заказа
mutation CancelOrder($orderId: ID!) {
  cancelOrder(orderId: $orderId)
}

# подписка на статус заказа (WebSocket)
subscription OrderStatus($orderId: ID!) {
  orderStatus(orderId: $orderId) { id status }
}

# подписка на изменения остатков (WebSocket)
subscription InventoryChanged($productId: ID!) {
  inventoryChanged(productId: $productId) { productId available reserved }
}
```

## Фичи

- **JWT-аутентификация** через `Authorization: Bearer <token>`.
- **Rate limiting** — sliding window по IP и по `user_id` на основе Redis.
- **Circuit breaker** — для каждого downstream-сервиса; при открытом breaker возвращается 503.
- **Feature flags** — хранятся в Redis, управляются через `/admin/flags` с проверкой admin-роли.
- **Metrics** — `http_requests_total`, `http_request_duration_seconds`, `grpc_requests_total`, `grpc_request_duration_seconds` в формате Prometheus.
- **Input validation** — email, uuid, положительные цена и количество; ошибки возвращаются как GraphQL errors.
- **TLS/mTLS** — опциональная настройка для gRPC downstream.

## Покрытие

Unit-тесты покрывают:

- JWT verifier (`auth`)
- circuit breaker
- rate limiter (in-memory backend)
- feature flags (in-memory backend)
- input validation
- admin health endpoint
