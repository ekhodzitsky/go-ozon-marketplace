# notification-service

Уведомления: отправка email по gRPC.

## Что делает

- Отправка email через gRPC (service-only)
- Пока не подписан на Kafka events

## API (gRPC)

| Метод | Описание | Auth |
|-------|----------|------|
| `SendEmail` | Отправить email | `service` роль (через `RequireRole`) |

## Запуск

```bash
cd services/notification-service
JWT_SECRET="..." go run ./cmd/...
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `GRPC_PORT` | gRPC сервер | `50056` |
| `JWT_SECRET` | Секрет для валидации JWT | **Обязательно** |
| `DEFAULT_CALL_TIMEOUT` | Таймаут gRPC вызовов | `5s` |
| `DEFAULT_QUERY_TIMEOUT` | Таймаут gRPC запросов | `3s` |
| `CERT_PATH` | Путь к TLS сертификатам (опционально) | — |

## Что ещё не реализовано

- Подписка на Kafka events
- SMTP интеграция
- Шаблоны писем

## Зависимости

- Нет внешних зависимостей (только gRPC)
