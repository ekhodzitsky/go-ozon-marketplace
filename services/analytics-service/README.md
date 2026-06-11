# analytics-service

Аналитика: агрегация событий, отчёты, хранение в ClickHouse.

## Что делает

- Подписка на Kafka events
- Batch insert в ClickHouse
- Агрегация метрик: заказы, выручка, конверсия
- Партиционирование по месяцам (toYYYYMM)
- Сжатие ZSTD, TTL для старых данных

## API (gRPC)

| Метод | Описание | Auth |
|-------|----------|------|
| `RecordEvent` | Записать событие | service |
| `GetReport` | Получить отчёт | admin |
| `GetMetrics` | Метрики в real-time | admin |

## Запуск

```bash
cd services/analytics-service
go run ./cmd/...
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `GRPC_PORT` | gRPC сервер | `50057` |
| `KAFKA_BROKERS` | Kafka брокеры | `localhost:19092` |
| `CLICKHOUSE_URL` | ClickHouse | `clickhouse://localhost:9000` |
| `BATCH_SIZE` | Размер batch insert | `1000` |
| `BATCH_TIMEOUT` | Таймаут batch | `5s` |
| `LOG_LEVEL` | Уровень логов | `info` |
| `LOG_FORMAT` | Формат логов | `json` |

## Модель данных (ClickHouse)

```sql
CREATE TABLE events (
    event_id UUID,
    event_type String,
    user_id UUID,
    order_id Nullable(UUID),
    product_id Nullable(UUID),
    amount_minor Nullable(Int64),
    metadata String,
    created_at DateTime64(3)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (event_type, created_at)
TTL created_at + INTERVAL 1 YEAR
SETTINGS index_granularity = 8192;

CREATE TABLE orders_daily (
    date Date,
    orders_count UInt64,
    revenue_minor Int64,
    avg_order_minor Float64
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY date;
```

## Партиционирование

```
2026-01  ──▶  part_202601
2026-02  ──▶  part_202602
...
TTL: автоматическое удаление данных старше 1 года
```

## Зависимости

- Kafka (consumer)
- ClickHouse
