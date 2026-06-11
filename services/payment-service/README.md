# payment-service

Обработка платежей: проведение, возвраты, атомарные транзакции.

## Что делает

- Проведение платежей по заказам
- Возвраты при отмене или компенсации Saga
- Проверка статуса платежа
- Атомарные транзакции (tx-manager)
- DLQ (Dead Letter Queue) для необработанных платежей

## API (gRPC)

| Метод | Описание | Auth |
|-------|----------|------|
| `ProcessPayment` | Провести платёж | service |
| `RefundPayment` | Вернуть платёж | service |
| `GetPaymentStatus` | Статус платежа | user (свой) / admin |
| `GetPayment` | Детали платежа | admin |

## Запуск

```bash
cd services/payment-service
go run ./cmd/...
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `GRPC_PORT` | gRPC сервер | `50054` |
| `POSTGRES_URL` | PostgreSQL | — |
| `KAFKA_BROKERS` | Kafka брокеры | `localhost:19092` |
| `PAYMENT_TIMEOUT` | Таймаут операции | `10s` |
| `LOG_LEVEL` | Уровень логов | `info` |
| `LOG_FORMAT` | Формат логов | `json` |

## Модель данных

```sql
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL,
    amount_minor INT64 NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'RUB',
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    provider VARCHAR(100),
    transaction_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE payment_refunds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID REFERENCES payments(id),
    amount_minor INT64 NOT NULL,
    reason VARCHAR(500),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## Статусы платежа

- `pending` — ожидает проведения
- `processing` — в процессе
- `completed` — успешно проведён
- `failed` — ошибка
- `refunded` — возвращён

## DLQ

Необработанные платежи попадают в DLQ топик Kafka для ручного разбора:

```
payment-dlq
  ├── event: PaymentFailed
  ├── reason: timeout/error
  └── retry_count: N
```

## Зависимости

- PostgreSQL
- Kafka (DLQ)
