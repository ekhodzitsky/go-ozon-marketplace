# api-gateway

GraphQL шлюз — единая точка входа для всех клиентов.

## Что делает

- Принимает HTTP/GraphQL запросы
- Маршрутизирует на downstream gRPC сервисы
- Проверяет JWT и роли
- Rate limiting (Redis-backed sliding window)
- Access-log и метрики Prometheus
- Circuit breaker для защиты от каскадных отказов

## API

- **GraphQL endpoint**: `http://localhost:8080/graphql`
- **Playground**: `http://localhost:8080` (в dev режиме)
- **Metrics**: `http://localhost:8080/metrics`

### GraphQL схема

Основные типы:
- `User`, `Product`, `Order`, `Inventory`
- Мутации: `register`, `login`, `createProduct`, `createOrder`, `cancelOrder`
- Запросы: `me`, `products`, `product`, `order`, `orders`, `searchProducts`

## Запуск

```bash
cd services/api-gateway
go run ./cmd/...
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `HTTP_PORT` | HTTP сервер | `8080` |
| `METRICS_PORT` | Prometheus metrics | `9090` |
| `USER_SERVICE_ADDR` | Адрес user-service | `localhost:50051` |
| `CATALOG_SERVICE_ADDR` | Адрес catalog-service | `localhost:50052` |
| `ORDER_SERVICE_ADDR` | Адрес order-service | `localhost:50055` |
| `INVENTORY_SERVICE_ADDR` | Адрес inventory-service | `localhost:50053` |
| `PAYMENT_SERVICE_ADDR` | Адрес payment-service | `localhost:50054` |
| `REDIS_URL` | Redis для rate limiter | `redis://localhost:6379/0` |
| `JWT_SECRET` | Секрет для JWT проверки | — |
| `RATE_LIMIT_RPS` | Запросов в секунду | `100` |
| `LOG_LEVEL` | Уровень логов | `info` |
| `LOG_FORMAT` | Формат логов | `json` |

## Архитектура

```
┌─────────┐     ┌──────────────┐     ┌─────────────┐
│ Client  │────▶│ api-gateway  │────▶│ gRPC clients│
└─────────┘     └──────────────┘     └──────┬──────┘
                                            │
        ┌───────────────────────────────────┼───┐
        ▼                                   ▼   ▼
   ┌─────────┐  ┌──────────┐  ┌────────┐  ┌────┴──┐
   │user-svc │  │catalog-svc│  │order-svc│  │ others │
   └─────────┘  └──────────┘  └────────┘  └───────┘
```

## Зависимости

- Все downstream сервисы
- Redis (rate limiting)
- Prometheus (метрики)
