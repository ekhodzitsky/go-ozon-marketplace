# Быстрый старт

Пошаговый сценарий: поднять инфраструктуру, создать пользователя, добавить товар и оформить заказ.

## Требования

- Docker и Docker Compose
- Go 1.26
- make
- grpcurl или GraphQL Playground (браузер)

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
- Jaeger (порт 16686)

Проверить:
```bash
docker compose -f infra/docker/docker-compose.yml ps
```

## 2. Запустить сервисы

В отдельных терминалах:

```bash
# Terminal 1 — шлюз
cd services/api-gateway && go run ./cmd/...

# Terminal 2 — пользователи
cd services/user-service && go run ./cmd/...

# Terminal 3 — каталог
cd services/catalog-service && go run ./cmd/...

# Terminal 4 — остатки
cd services/inventory-service && go run ./cmd/...

# Terminal 5 — платежи
cd services/payment-service && go run ./cmd/...

# Terminal 6 — заказы
cd services/order-service && go run ./cmd/...

# Terminal 7 — уведомления
cd services/notification-service && go run ./cmd/...

# Terminal 8 — аналитика
cd services/analytics-service && go run ./cmd/...
```

Или через Docker Compose для каждого сервиса (см. [DEPLOYMENT.md](DEPLOYMENT.md)).

## 3. Открыть GraphQL Playground

```bash
open http://localhost:8080
```

## 4. Сценарий: регистрация → товар → заказ

### Шаг 1. Регистрация пользователя

```graphql
mutation {
  register(input: {
    email: "user@example.com",
    password: "password123",
    name: "Иван"
  }) {
    id
    email
    token
  }
}
```

Сохраните `token` — он понадобится для авторизации.

### Шаг 2. Создать товар (требуется роль admin)

```graphql
mutation {
  createProduct(input: {
    name: "Наушники",
    description: "Беспроводные",
    price: 4999.99,
    categoryId: "1",
    stock: 100
  }) {
    id
    name
    price
  }
}
```

> Для теста можно временно убрать проверку роли в gateway или создать пользователя с ролью `admin` напрямую в БД.

### Шаг 3. Создать заказ

```graphql
mutation {
  createOrder(input: {
    items: [
      { productId: "<product_id>", quantity: 2 }
    ]
  }) {
    id
    status
    totalAmount
  }
}
```

### Шаг 4. Проверить статус заказа

```graphql
query {
  order(id: "<order_id>") {
    id
    status
    items {
      productId
      quantity
      price
    }
  }
}
```

### Шаг 5. Проверить аналитику

```bash
# ClickHouse
curl 'http://localhost:8123/?query=SELECT%20*%20FROM%20orders%20LIMIT%2010'
```

## 5. Посмотреть метрики и трейсы

| Инструмент | URL | Что смотреть |
|------------|-----|--------------|
| **Prometheus** | http://localhost:9090 | Метрики сервисов, RED method |
| **Jaeger** | http://localhost:16686 | Distributed traces по trace_id |
| **GraphQL Playground** | http://localhost:8080 | Интерактивные запросы |

## 6. Остановить всё

```bash
make down
```

Удалит все контейнеры и volumes.
