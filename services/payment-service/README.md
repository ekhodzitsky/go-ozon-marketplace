# payment-service

Обработка платежей: проведение, возвраты.

## Что делает

- Проведение платежей по заказам
- Возвраты при отмене или компенсации Saga

## API (gRPC)

| Метод | Описание | Auth |
|-------|----------|------|
| `ProcessPayment` | Провести платёж | Проверяет `authUserID` |
| `Refund` | Вернуть платёж | Проверяет `authUserID` |

## Запуск

```bash
cd services/payment-service
POSTGRES_DSN="postgres://..." JWT_SECRET="..." go run ./cmd/...
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `GRPC_PORT` | gRPC сервер | `50054` |
| `POSTGRES_DSN` | PostgreSQL | **Обязательно** |
| `JWT_SECRET` | Секрет для валидации JWT | **Обязательно** |
| `DEFAULT_CALL_TIMEOUT` | Таймаут gRPC вызовов | `5s` |
| `DEFAULT_QUERY_TIMEOUT` | Таймаут gRPC запросов | `3s` |
| `CERT_PATH` | Путь к TLS сертификатам (опционально) | — |

## Модель данных

Таблица `payments`:
- `id` (UUID, PK)
- `order_id`
- `amount` (BIGINT, копейки)
- `status` (`pending`, `success`, `failed`, `refunded`)
- `created_at`

## Зависимости

- PostgreSQL
