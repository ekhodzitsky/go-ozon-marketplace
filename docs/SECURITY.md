# Безопасность

Как защищен маркетплейс: аутентификация, авторизация, шифрование, rate limiting.

## Аутентификация

### JWT

Все клиентские запросы проходят через `api-gateway`. Gateway прокидывает `Authorization` в gRPC metadata, а downstream сервисы валидируют JWT через `AuthUnaryInterceptor`.

**Реализовано:**
- Алгоритм: **HS256**
- Поля: `user_id`, `exp`, `role` (если передан)
- `exp` — 24 часа (захардкожено)

**Не реализовано:**
- `iss`, `aud`, `jti`, `nbf`, `sub` — отсутствуют в токене
- Валидация минимальной длины секрета

### Пример токена

```json
{
  "user_id": "user-123",
  "role": "user",
  "exp": 1718086400
}
```

## Авторизация (RBAC)

Три роли:

| Роль | Описание | Доступ |
|------|----------|--------|
| `user` | Обычный покупатель | Свои заказы, профиль, каталог |
| `admin` | Администратор | CRUD товаров, все заказы, все пользователи |
| `service` | Другой микросервис | Внутренние RPC (`inventory-service`, `notification-service`) |

### Проверка ролей

В коде gRPC handler:

```go
role, ok := middleware.GetRole(ctx)
if !ok || role != middleware.RoleAdmin {
    return nil, status.Error(codes.PermissionDenied, "admin required")
}
```

> **Примечание:** GraphQL gateway **не проверяет роли** — проверка происходит только на уровне gRPC сервисов.

## mTLS между сервисами

Все gRPC вызовы между сервисами могут использовать mTLS:

- Если задан `CERT_PATH` — используется `LoadClientMTLSCredentials` / `LoadServerMTLSCredentials`
- Если `CERT_PATH` пуст — используется `insecure.NewCredentials()`

Генерация сертификатов:

```bash
./scripts/generate-certs.sh
```

> mTLS **опционален**, не обязателен для запуска.

## Rate Limiting

Sliding window rate limiter в `api-gateway`:

- Хранение счётчиков в **Redis**
- Один лимит для всех: **10 RPS** по умолчанию (настраивается через `RATE_LIMIT_RPS`)
- Учёт `X-Forwarded-For` при наличии доверенных прокси (`TRUSTED_PROXIES`)

При превышении лимита:

```json
{
  "errors": [
    {
      "message": "rate limit exceeded"
    }
  ]
}
```

> **Не реализовано:** разделение лимитов по ролям (user/admin/service).

## Circuit Breaker

> **Не реализован.** Заявлен в дизайн-документе, но отсутствует в коде.

## Защита данных

### Пароли

- Хеширование через **bcrypt** с `DefaultCost = 10`
- Никогда не хранятся в открытом виде

### Деньги

- В БД — `BIGINT` (копейки, `price * 100`)
- В proto и GraphQL — `double` / `Float` (доллары)

### SQL-инъекции

- Использование **pgx** с параметризованными запросами
- Никакого конкатенации SQL

### Чувствительные данные

- JWT secret — `JWT_SECRET` env var
- Пароли БД — `POSTGRES_DSN` env var
- TLS ключи — `CERT_PATH` env var

## Аудит

Логи содержат:
- `trace_id` — для связывания запросов (через OTel context propagation)
- `user_id` — кто делал запрос
- `method` — какой метод вызван
- `duration` — сколько заняло

Все логи — structured JSON через Zap.

## Что ещё не реализовано

- Поля `iss`, `aud`, `jti`, `nbf` в JWT
- Circuit breaker
- Разделение rate limit по ролям
- Валидация минимальной длины JWT секрета
