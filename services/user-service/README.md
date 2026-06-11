# user-service

Управление пользователями: регистрация, аутентификация, JWT.

## Что делает

- Регистрация новых пользователей (bcrypt хеширование паролей)
- Аутентификация и выдача JWT токенов
- Валидация токенов для других сервисов
- Управление ролями (user, admin, service)
- Хранение профилей в PostgreSQL

## API (gRPC)

| Метод | Описание | Auth |
|-------|----------|------|
| `Register` | Регистрация | Нет |
| `Login` | Вход, получение JWT | Нет |
| `GetUser` | Получить пользователя | user/admin |
| `ValidateToken` | Проверить JWT токен | service |
| `UpdateUser` | Обновить профиль | user (свой) |
| `DeleteUser` | Удалить пользователя | admin |

## Запуск

```bash
cd services/user-service
go run ./cmd/...
```

## Миграции

```bash
make migrate-user USER_DB_URL=postgres://ozon:ozonpass@localhost:5432/marketplace?sslmode=disable
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `GRPC_PORT` | gRPC сервер | `50051` |
| `POSTGRES_URL` | PostgreSQL | — |
| `JWT_SECRET` | Секрет для подписи JWT | — |
| `JWT_EXPIRY` | Время жизни токена | `24h` |
| `BCRYPT_COST` | Стоимость bcrypt | `10` |
| `LOG_LEVEL` | Уровень логов | `info` |
| `LOG_FORMAT` | Формат логов | `json` |

## Модель данных

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## Безопасность

- Пароли хешируются bcrypt с cost ≥ 10
- JWT подписан HS256, секрет минимум 32 символа
- Роли: `user`, `admin`, `service`

## Зависимости

- PostgreSQL
