# inventory-service

Управление остатками: склад, резервирование.

## Что делает

- Хранение текущих остатков по товарам
- Резервирование товаров при заказе
- Освобождение резерва при отмене
- Кэширование в Redis (cache-aside)

## API (gRPC)

| Метод | Описание | Auth |
|-------|----------|------|
| `Reserve` | Зарезервировать товар | `service` роль (через `RequireRole`) |
| `Release` | Освободить резерв | `service` роль (через `RequireRole`) |
| `GetStock` | Получить остатки | Нет |

## Запуск

```bash
cd services/inventory-service
POSTGRES_DSN="postgres://..." JWT_SECRET="..." REDIS_ADDR=localhost:6379 go run ./cmd/...
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `GRPC_PORT` | gRPC сервер | `50053` |
| `POSTGRES_DSN` | PostgreSQL | **Обязательно** |
| `JWT_SECRET` | Секрет для валидации JWT | **Обязательно** |
| `REDIS_ADDR` | Redis | `localhost:6379` |
| `DEFAULT_CALL_TIMEOUT` | Таймаут gRPC вызовов | `5s` |
| `DEFAULT_QUERY_TIMEOUT` | Таймаут gRPC запросов | `3s` |
| `CERT_PATH` | Путь к TLS сертификатам (опционально) | — |

## Модель данных

Таблица `inventory`:
- `product_id` (UUID, PK)
- `available` (INT)
- `reserved` (INT)

Таблица `reservations`:
- `id` (UUID, PK)
- `product_id`
- `order_id`
- `quantity`
- `created_at`

## Зависимости

- PostgreSQL
- Redis (кэш)
