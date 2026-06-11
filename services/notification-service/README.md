# notification-service

Уведомления: email, push-уведомления по событиям из Kafka.

## Что делает

- Подписка на Kafka events
- Отправка email-уведомлений
- Push-уведомления
- Шаблонизация сообщений
- Повторные попытки при ошибках

## API (gRPC)

| Метод | Описание | Auth |
|-------|----------|------|
| `SendNotification` | Отправить уведомление | service |
| `GetNotificationStatus` | Статус отправки | admin |

## Запуск

```bash
cd services/notification-service
go run ./cmd/...
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `GRPC_PORT` | gRPC сервер | `50056` |
| `KAFKA_BROKERS` | Kafka брокеры | `localhost:19092` |
| `SMTP_HOST` | SMTP сервер | — |
| `SMTP_PORT` | SMTP порт | `587` |
| `SMTP_USER` | SMTP пользователь | — |
| `SMTP_PASSWORD` | SMTP пароль | — |
| `FROM_EMAIL` | Отправитель | `noreply@marketplace.local` |
| `LOG_LEVEL` | Уровень логов | `info` |
| `LOG_FORMAT` | Формат логов | `json` |

## Подписанные события

| Событие | Действие |
|---------|----------|
| `UserRegistered` | Приветственное письмо |
| `OrderConfirmed` | Подтверждение заказа |
| `OrderCancelled` | Уведомление об отмене |
| `PaymentFailed` | Уведомление об ошибке платежа |

## Шаблоны

Шаблоны писем хранятся в `internal/templates/`:
- `welcome.html`
- `order_confirmed.html`
- `order_cancelled.html`
- `payment_failed.html`

## Зависимости

- Kafka (consumer)
- SMTP сервер (опционально)
