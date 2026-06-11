# catalog-service

Каталог товаров: CRUD, категории, поиск, CQRS с Elasticsearch.

## Что делает

- CRUD операции с товарами
- Управление категориями
- Полнотекстовый поиск через Elasticsearch (CQRS)
- Публикация событий `ProductCreated/Updated/Deleted` в Kafka
- Transactional Outbox для гарантированной доставки событий

## API (gRPC)

| Метод | Описание | Auth |
|-------|----------|------|
| `CreateProduct` | Создать товар | admin |
| `GetProduct` | Получить товар | Нет |
| `UpdateProduct` | Обновить товар | admin |
| `DeleteProduct` | Удалить товар | admin |
| `SearchProducts` | Поиск в Elasticsearch | Нет |
| `ListCategories` | Список категорий | Нет |
| `GetCategory` | Категория по ID | Нет |

## Запуск

```bash
cd services/catalog-service
go run ./cmd/...
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `GRPC_PORT` | gRPC сервер | `50052` |
| `POSTGRES_URL` | PostgreSQL | — |
| `ELASTICSEARCH_URL` | Elasticsearch | `http://localhost:9200` |
| `KAFKA_BROKERS` | Kafka брокеры | `localhost:19092` |
| `LOG_LEVEL` | Уровень логов | `info` |
| `LOG_FORMAT` | Формат логов | `json` |

## Модель данных

```sql
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(500) NOT NULL,
    description TEXT,
    price_minor INT64 NOT NULL,
    category_id UUID REFERENCES categories(id),
    sku VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    parent_id UUID REFERENCES categories(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## CQRS

```
Write (PostgreSQL)          Read (Elasticsearch)
     │                              ▲
     │ ProductCreated/Updated/Deleted│
     └───────────▶ Kafka ───────────┘
```

## Зависимости

- PostgreSQL
- Elasticsearch
- Kafka (Transactional Outbox)
