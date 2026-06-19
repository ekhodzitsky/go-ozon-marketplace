# api-gateway-rust

Rust-скелетон API-шлюза для учебного маркетплейса. Дублирует базовый функционал `services/api-gateway/` на Go, используя:

- **axum** — HTTP-сервер и middleware
- **async-graphql** — GraphQL-схема и резолверы
- **tonic** — gRPC-клиенты downstream-сервисов

## Структура

```
src/
  main.rs           # entrypoint, инициализация, запуск сервера
  config.rs         # конфигурация из env
  auth.rs           # JWT verifier и middleware
  clients.rs        # фабрика tonic-клиентов
  admin.rs          # admin endpoints /health, /ready
  ws.rs             # заглушка WebSocket
  error.rs          # базовые ошибки
  proto.rs          # include сгенерированного кода из proto
  graphql/
    mod.rs
    schema.rs       # Schema<Query, Mutation, Subscription>
    resolvers.rs    # GraphQL резолверы
```

## Сборка

Требуется `buf` для экспорта proto-зависимостей (включая `buf/validate/validate.proto`):

```bash
cd services/api-gateway-rust
cargo build
```

## Тесты

```bash
cargo test
```

## Запуск

```bash
# с дефолтными адресами downstream-сервисов
PORT=8080 JWT_SECRET=dev-secret cargo run
```

Переменные окружения (все опциональны, dev-значения совпадают с Go-шлюзом):

| Переменная | Значение по умолчанию |
|---|---|
| `PORT` | `8080` |
| `USER_SERVICE_ADDR` | `localhost:50051` |
| `CATALOG_SERVICE_ADDR` | `localhost:50052` |
| `INVENTORY_SERVICE_ADDR` | `localhost:50053` |
| `PAYMENT_SERVICE_ADDR` | `localhost:50054` |
| `ORDER_SERVICE_ADDR` | `localhost:50055` |
| `ANALYTICS_SERVICE_ADDR` | `localhost:50056` |
| `JWT_SECRET` | `dev-secret` |
| `CORS_ALLOWED_ORIGINS` | `*` (через tower-http `Any`) |
| `RUST_LOG` | `info` |

## GraphQL

Playground доступен по корню `/`. Основные операции:

```graphql
# авторизованный профиль
query Me {
  me { id email name createdAt }
}

# список товаров
query Products {
  products { products { id name price categories } total }
}

# регистрация
mutation Register {
  register(email: "a@b.com", password: "password", name: "User")
}

# вход
mutation Login {
  login(email: "a@b.com", password: "password")
}
```

## Покрытие

Сейчас реализованы скелетоны всех GraphQL-операций, но гарантированно рабочими без наличия downstream-сервисов являются:

- `Query.me` (требует валидный Bearer-токен)
- `Query.products` (список товаров из catalog-service)
- `Query.product`
- `Mutation.register` / `Mutation.login`

WebSocket-подписки и продвинутая авторизация помечены как `TODO`.
