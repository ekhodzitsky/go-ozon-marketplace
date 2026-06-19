# api-gateway

GraphQL шлюз — единая точка входа для всех клиентов.

## Что делает

- Принимает HTTP/GraphQL запросы
- Маршрутизирует на downstream gRPC сервисы
- Прокидывает `Authorization` заголовок в gRPC metadata
- Rate limiting (Redis-backed sliding window)
- WebSocket для real-time обновлений
- Feature flags и A/B testing
- Admin API для управления флагами
- Access-log и метрики Prometheus

## API

- **GraphQL endpoint**: `http://localhost:8080/query`
- **Playground**: `http://localhost:8080` (в dev режиме)
- **WebSocket**: `ws://localhost:8080/ws`
- **Metrics**: `http://localhost:9080/metrics`
- **Admin API**: `http://localhost:8080/admin/flags`

### GraphQL операции

| Тип | Название | Куда маршрутизирует |
|-----|----------|---------------------|
| Mutation | `register` | user-service |
| Mutation | `login` | user-service |
| Mutation | `createProduct` | catalog-service |
| Mutation | `createOrder` | order-service |
| Mutation | `cancelOrder` | order-service |
| Query | `me` | user-service |
| Query | `user` | user-service |
| Query | `product` | catalog-service |
| Query | `searchProducts` | catalog-service |
| Query | `order` | order-service |
| Query | `orders` | order-service |
| Query | `inventory` | inventory-service |
| Query | `featureFlags` | — |
| Query | `abTestAssignments` | — |
| Subscription | `orderStatusChanged` | Redis pub/sub |
| Subscription | `inventoryChanged` | Redis pub/sub |

## Запуск

```bash
cd services/api-gateway
REDIS_ADDR=localhost:6379 go run ./cmd/...
```

Чтобы перегенерировать DI после изменения провайдеров:

```bash
cd services/api-gateway
go generate ./internal/app/...
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `PORT` | HTTP сервер | `8080` |
| `METRICS_PORT` | Metrics сервер | `PORT + 1000` |
| `USER_SERVICE_ADDR` | Адрес user-service | `localhost:50051` |
| `CATALOG_SERVICE_ADDR` | Адрес catalog-service | `localhost:50052` |
| `ORDER_SERVICE_ADDR` | Адрес order-service | `localhost:50055` |
| `INVENTORY_SERVICE_ADDR` | Адрес inventory-service | `localhost:50053` |
| `PAYMENT_SERVICE_ADDR` | Адрес payment-service | `localhost:50054` |
| `ANALYTICS_SERVICE_ADDR` | Адрес analytics-service | `localhost:50056` |
| `REDIS_ADDR` | Redis для rate limiter, pub/sub, feature flags | `localhost:6379` |
| `RATE_LIMIT_USER_RPS` | RPS для обычных пользователей | `100` |
| `RATE_LIMIT_ADMIN_RPS` | RPS для администраторов | `1000` |
| `RATE_LIMIT_WINDOW` | Окно rate limiter | `1s` |
| `TRUSTED_PROXIES` | Доверенные прокси (для `X-Forwarded-For`) | — |
| `MAX_BODY_SIZE_BYTES` | Макс. размер тела запроса | `1MB` |
| `DEFAULT_CALL_TIMEOUT` | Таймаут gRPC вызовов | `5s` |
| `DEFAULT_QUERY_TIMEOUT` | Таймаут gRPC запросов | `3s` |
| `CERT_PATH` | Путь к TLS сертификатам (опционально) | — |
| `INSECURE_SKIP_TLS` | Отключить TLS для gRPC | `false` |
| `JWT_SECRET` | Секрет для проверки JWT | — |
| `CORS_ALLOWED_ORIGINS` | Разрешённые origins для CORS | — |

## Admin API

Требует JWT с ролью `admin`.

```bash
# Список флагов
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:8080/admin/flags

# Включить флаг
curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:8080/admin/flags/fast-search/enable

# Выключить флаг
curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:8080/admin/flags/fast-search/disable

# Установить процент раскатки
curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:8080/admin/flags/fast-search/percentage/50
```

## Архитектура

```
┌─────────┐     ┌──────────────┐     ┌─────────────────┐
│ Client  │────▶│ api-gateway  │────▶│ user-service    │
└─────────┘     └──────────────┘     │ catalog-service │
                                     │ order-service   │
                                     │ inventory-service│
                                     │ payment-service │
                                     │ analytics-service│
                                     └─────────────────┘
```

## Стек

- [gqlgen](https://gqlgen.com/) — GraphQL
- [chi](https://github.com/go-chi/chi) — HTTP роутинг
- [rs/cors](https://github.com/rs/cors) — CORS
- [google/wire](https://github.com/google/wire) — DI
- Prometheus — метрики
- Redis — rate limiting, pub/sub, feature flags
