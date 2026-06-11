# Быстрый старт

Пошаговый сценарий: поднять инфраструктуру, создать пользователя, добавить товар и поискать.

## Требования

- Docker и Docker Compose
- Go 1.26
- make
- grpcurl (опционально)

## 1. Поднять инфраструктуру

```bash
make up
```

Поднимается:
- PostgreSQL 16 (порт 5432)
- Redis 7 (порт 6379)
- Redpanda / Kafka (порт 19092)
- ClickHouse (порт 8123)
- Elasticsearch (порт 9200)
- Prometheus (порт 9090)
- Grafana (порт 3000)
- Jaeger (порт 16686)

Проверить:
```bash
docker compose -f infra/docker/docker-compose.yml ps
```

## 2. Запустить сервисы

В отдельных терминалах (каждый сервис требует env vars):

```bash
export POSTGRES_DSN="postgres://ozon:ozonpass@localhost:5432/marketplace?sslmode=disable"
export JWT_SECRET="min-32-chars-secret-key-here!!!"

# Terminal 1 — шлюз
cd services/api-gateway && REDIS_ADDR=localhost:6379 go run ./cmd/...

# Terminal 2 — пользователи
cd services/user-service && go run ./cmd/...

# Terminal 3 — каталог
cd services/catalog-service && ES_URL=http://localhost:9200 go run ./cmd/...

# Terminal 4 — остатки
cd services/inventory-service && REDIS_ADDR=localhost:6379 go run ./cmd/...

# Terminal 5 — платежи
cd services/payment-service && go run ./cmd/...

# Terminal 6 — заказы
cd services/order-service && INVENTORY_ADDR=localhost:50053 PAYMENT_ADDR=localhost:50054 go run ./cmd/...

# Terminal 7 — уведомления
cd services/notification-service && go run ./cmd/...

# Terminal 8 — аналитика
cd services/analytics-service && CLICKHOUSE_DSN=localhost:9000 go run ./cmd/...
```

## 3. Открыть GraphQL Playground

```bash
open http://localhost:8080
```

## 4. Сценарий: регистрация → товар → поиск

### Шаг 1. Регистрация пользователя

```graphql
mutation {
  register(email: "user@example.com", password: "password123", name: "Иван")
}
```

Вернёт ID пользователя. **Токен не возвращается** — получаем через `login`:

```graphql
mutation {
  login(email: "user@example.com", password: "password123")
}
```

Вернёт JWT токен. Сохраните его для последующих запросов (передавайте в заголовке `Authorization: Bearer <token>`).

### Шаг 2. Создать товар

```graphql
mutation {
  createProduct(
    name: "Наушники"
    description: "Беспроводные"
    price: 4999.99
    categories: ["Электроника"]
  )
}
```

Вернёт ID товара.

### Шаг 3. Получить товар

```graphql
query {
  product(id: "<product_id>") {
    id
    name
    price
    categories
  }
}
```

### Шаг 4. Поиск товаров

```graphql
query {
  searchProducts(query: "наушники", page: 1, pageSize: 10) {
    products {
      id
      name
      price
    }
    total
  }
}
```

### Шаг 5. Получить пользователя

```graphql
query {
  user(id: "<user_id>") {
    id
    email
    name
  }
}
```

## 5. Order-service (только gRPC)

`order-service` не подключён к GraphQL gateway. Работайте напрямую через gRPC:

```bash
# Создать заказ
grpcurl -plaintext -d '{
  "user_id": "<user_id>",
  "items": [
    {"product_id": "<product_id>", "quantity": 2, "price": 4999.99}
  ]
}' localhost:50055 order.v1.OrderService/CreateOrder

# Получить заказ
grpcurl -plaintext -d '{"order_id": "<order_id>"}' localhost:50055 order.v1.OrderService/GetOrder

# Список заказов
grpcurl -plaintext -d '{"user_id": "<user_id>", "page": 1, "page_size": 10}' localhost:50055 order.v1.OrderService/ListOrders
```

## 6. Посмотреть метрики и трейсы

| Инструмент | URL | Что смотреть |
|------------|-----|--------------|
| **Prometheus** | http://localhost:9090 | Метрики сервисов |
| **Grafana** | http://localhost:3000 | Dashboards |
| **Jaeger** | http://localhost:16686 | Distributed traces |
| **GraphQL Playground** | http://localhost:8080 | Интерактивные запросы |

## 7. Остановить всё

```bash
make down
```

Удалит все контейнеры и volumes.
