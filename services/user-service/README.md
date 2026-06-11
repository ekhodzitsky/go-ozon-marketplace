# user-service

Управление пользователями: регистрация, аутентификация, JWT.

## Что делает

- Регистрация новых пользователей (bcrypt хеширование паролей)
- Аутентификация и выдача JWT токенов
- Получение профиля по ID

## API (gRPC)

| Метод | Описание | Auth |
|-------|----------|------|
| `Register` | Регистрация | Нет |
| `Login` | Вход, получение JWT | Нет |
| `GetUser` | Получить пользователя | Требуется валидный JWT |

## Запуск

```bash
cd services/user-service
POSTGRES_DSN="postgres://..." JWT_SECRET="..." go run ./cmd/...
```

## Миграции

```bash
make migrate-user USER_DB_URL=postgres://user:pass@localhost:5432/marketplace?sslmode=disable
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `GRPC_PORT` | gRPC сервер | `50051` |
| `HTTP_PORT` | HTTP сервер | `8080` (пока не используется) |
| `POSTGRES_DSN` | PostgreSQL | **Обязательно** |
| `JWT_SECRET` | Секрет для подписи JWT | **Обязательно** |
| `DEFAULT_CALL_TIMEOUT` | Таймаут gRPC вызовов | `5s` |
| `DEFAULT_QUERY_TIMEOUT` | Таймаут gRPC запросов | `3s` |
| `CERT_PATH` | Путь к TLS сертификатам (опционально) | — |

## Модель данных

Таблица `users`:
- `id` (UUID, PK)
- `email` (unique, not null)
- `password_hash` (not null)
- `name` (not null)
- `role` (default 'user')
- `created_at`

## Безопасность

- Пароли хешируются bcrypt с `DefaultCost = 10`
- JWT подписан HS256, содержит `user_id`, `exp` (24ч), опционально `role`

## Зависимости

- PostgreSQL
