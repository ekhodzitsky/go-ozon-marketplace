# catalog-service

Каталог товаров: CRUD, поиск через Elasticsearch, Outbox relay.

## Что делает

- Создание и получение товаров
- Полнотекстовый поиск через Elasticsearch
- Outbox relay для синхронизации изменений в Elasticsearch

## API (gRPC)

| Метод | Описание | Auth |
|-------|----------|------|
| `CreateProduct` | Создать товар | Требуется валидный JWT (роль не проверяется) |
| `GetProduct` | Получить товар по ID | Нет |
| `ListProducts` | Список товаров с пагинацией | Нет |
| `SearchProducts` | Поиск в Elasticsearch | Нет |

## Запуск

```bash
cd services/catalog-service
POSTGRES_DSN="postgres://..." JWT_SECRET="..." ES_URL=http://localhost:9200 go run ./cmd/...
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `GRPC_PORT` | gRPC сервер | `50052` |
| `POSTGRES_DSN` | PostgreSQL | **Обязательно** |
| `JWT_SECRET` | Секрет для валидации JWT | **Обязательно** |
| `ES_URL` | Elasticsearch | `http://localhost:9200` |
| `DEFAULT_CALL_TIMEOUT` | Таймаут gRPC вызовов | `5s` |
| `DEFAULT_QUERY_TIMEOUT` | Таймаут gRPC запросов | `3s` |
| `CERT_PATH` | Путь к TLS сертификатам (опционально) | — |

## Модель данных

Таблица `products`:
- `id` (UUID, PK)
- `name`
- `description`
- `price` (BIGINT, копейки)
- `stock`
- `categories` (массив строк)
- `created_at`

## CQRS

```
Write (PostgreSQL)          Read (Elasticsearch)
     │                              ▲
     │ Outbox relay                 │
     └───────────▶ ES ──────────────┘
```

Синхронизация через Outbox relay **напрямую в ES** (не через Kafka).

## Зависимости

- PostgreSQL
- Elasticsearch
