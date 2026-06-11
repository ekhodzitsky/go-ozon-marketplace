# analytics-service

Аналитика: запись событий, выручка за день в ClickHouse.

## Что делает

- Запись событий через gRPC (service-only)
- Получение выручки за день
- Batch insert в ClickHouse

## API (gRPC)

| Метод | Описание | Auth |
|-------|----------|------|
| `TrackEvent` | Записать событие | Нет проверки роли |
| `GetDailyRevenue` | Выручка за дату | Нет проверки роли |

## Запуск

```bash
cd services/analytics-service
JWT_SECRET="..." CLICKHOUSE_DSN=localhost:9000 go run ./cmd/...
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `GRPC_PORT` | gRPC сервер | `50057` |
| `JWT_SECRET` | Секрет для валидации JWT | **Обязательно** |
| `CLICKHOUSE_DSN` | Адрес ClickHouse | `localhost:9000` |
| `DEFAULT_CALL_TIMEOUT` | Таймаут gRPC вызовов | `5s` |
| `DEFAULT_QUERY_TIMEOUT` | Таймаут gRPC запросов | `3s` |
| `CERT_PATH` | Путь к TLS сертификатам (опционально) | — |

## Модель данных (ClickHouse)

Таблица `events`:
- `event_type` (String)
- `aggregate_id` (String)
- `payload` (String)
- `created_at` (DateTime)
- `aggregation_key` (String)

## Что ещё не реализовано

- Kafka consumer для автоматической записи событий
- Партиционирование по месяцам
- ZSTD сжатие и TTL
- Materialized views

## Зависимости

- ClickHouse
