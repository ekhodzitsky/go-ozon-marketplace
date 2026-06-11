# api-gateway

GraphQL шлюз — единая точка входа для всех клиентов.

## Что делает

- Принимает HTTP/GraphQL запросы
- Маршрутизирует на downstream gRPC сервисы (`user-service`, `catalog-service`)
- Прокидывает `Authorization` заголовок в gRPC metadata (валидация JWT — на уровне сервисов)
- Rate limiting (Redis-backed sliding window)
- Access-log и метрики Prometheus

## API

- **GraphQL endpoint**: `http://localhost:8080/query`
- **Playground**: `http://localhost:8080` (в dev режиме)
- **Metrics**: `http://localhost:8080/metrics`

### GraphQL операции

| Тип | Название | Сигнатура | Куда маршрутизирует |
|-----|----------|-----------|---------------------|
| Mutation | `register` | `register(email, password, name): ID!` | user-service:Register |
| Mutation | `login` | `login(email, password): String!` | user-service:Login |
| Mutation | `createProduct` | `createProduct(name, description, price, categories): ID!` | catalog-service:CreateProduct |
| Query | `user` | `user(id): User` | user-service:GetUser |
| Query | `product` | `product(id): Product` | catalog-service:GetProduct |
| Query | `searchProducts` | `searchProducts(query, page, pageSize): ProductConnection` | catalog-service:SearchProducts |

> **Важно:** Gateway не подключён к `order-service`, `inventory-service`, `payment-service`. Для работы с ними используйте прямые gRPC вызовы.

## Запуск

```bash
cd services/api-gateway
REDIS_ADDR=localhost:6379 go run ./cmd/...
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `PORT` | HTTP сервер | `8080` |
| `USER_SERVICE_ADDR` | Адрес user-service | `localhost:50051` |
| `CATALOG_SERVICE_ADDR` | Адрес catalog-service | `localhost:50052` |
| `REDIS_ADDR` | Redis для rate limiter | `localhost:6379` |
| `RATE_LIMIT_RPS` | Запросов в секунду | `10` |
| `RATE_LIMIT_WINDOW` | Окно rate limiter | `1s` |
| `TRUSTED_PROXIES` | Доверенные прокси (для `X-Forwarded-For`) | — |
| `MAX_BODY_SIZE_BYTES` | Макс. размер тела запроса | `1MB` |
| `DEFAULT_CALL_TIMEOUT` | Таймаут gRPC вызовов | `5s` |
| `DEFAULT_QUERY_TIMEOUT` | Таймаут gRPC запросов | `3s` |
| `CERT_PATH` | Путь к TLS сертификатам (опционально) | — |

## Архитектура

```
┌─────────┐     ┌──────────────┐     ┌─────────────────┐
│ Client  │────▶│ api-gateway  │────▶│ user-service    │
└─────────┘     └──────────────┘     │ catalog-service │
                                     └─────────────────┘
```

## Зависимости

- user-service
- catalog-service
- Redis (rate limiting)
