# inventory-service

Управление остатками: склад, резервирование, ledger.

## Что делает

- Хранение текущих остатков по товарам
- Резервирование товаров при заказе (идемпотентно)
- Освобождение резерва при отмене
- Ledger — история всех изменений остатков
- Кэширование в Redis (cache-aside)
- Optimistic locking для конкурентных обновлений

## API (gRPC)

| Метод | Описание | Auth |
|-------|----------|------|
| `GetStock` | Получить остатки | Нет |
| `ReserveStock` | Зарезервировать | service |
| `ReleaseStock` | Освободить резерв | service |
| `UpdateStock` | Обновить остатки | admin |
| `GetLedger` | История изменений | admin |

## Запуск

```bash
cd services/inventory-service
go run ./cmd/...
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `GRPC_PORT` | gRPC сервер | `50053` |
| `POSTGRES_URL` | PostgreSQL | — |
| `REDIS_URL` | Redis | `redis://localhost:6379/0` |
| `LOG_LEVEL` | Уровень логов | `info` |
| `LOG_FORMAT` | Формат логов | `json` |

## Модель данных

```sql
CREATE TABLE inventory (
    product_id UUID PRIMARY KEY REFERENCES products(id),
    quantity INT NOT NULL DEFAULT 0,
    reserved INT NOT NULL DEFAULT 0,
    version INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE inventory_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL,
    quantity_change INT NOT NULL,
    operation_type VARCHAR(50) NOT NULL,
    order_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## Резервирование

```sql
-- Optimistic locking
UPDATE inventory 
SET quantity = quantity - ?, reserved = reserved + ?, version = version + 1
WHERE product_id = ? AND version = ? AND quantity >= ?;
```

## Зависимости

- PostgreSQL
- Redis (кэш)
