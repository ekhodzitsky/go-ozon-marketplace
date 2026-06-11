# ADR-003: Transactional Outbox vs Прямая публикация в Kafka vs CDC

**Статус:** Accepted (с оговоркой)  
**Дата:** 2026-06-10  
**Автор:** go-ozon-marketplace team  
**Язык:** RU (термины Outbox/Kafka/CDC не переводятся)

---

## Контекст

`order-service` должен публиковать доменные события (`OrderCreated`, `OrderConfirmed`, `OrderCancelled`) в Kafka/Redpanda так, чтобы гарантировать консистентность с состоянием заказа в PostgreSQL. Рассматривались три альтернативы:

1. **Transactional Outbox** — события пишутся в таблицу `outbox` в той же БД-транзакции, что и бизнес-данные; отдельный relay процесс читает и публикует в брокер.
2. **Прямая публикация в Kafka (Kafka-native)** — `order-service` вызывает Kafka producer напрямую из бизнес-логики, возможно — в рамках Kafka Transactions (exactly-once).
3. **Change Data Capture (Debezium)** — чтение PostgreSQL WAL и публикация в Kafka без application-level outbox.

## Решение

Использовать **Transactional Outbox** с ручным relay в `order-service`.

> **Оговорка:** На момент принятия решения реальный Kafka producer **не реализован** — relay только логирует события через Zap и помечает их `processed`. Брокер Redpanda поднимается в Docker Compose, но producer/consumer отсутствуют. Это задокументировано как честный TODO ([ORD-OBX-01](../AUDIT_BACKLOG.md), [DOC-00](../AUDIT_BACKLOG.md)).

## Trade-offs

| Критерий | Transactional Outbox | Прямая публикация | CDC (Debezium) |
|---|---|---|---|
| **Exactly-once / At-least-once** | At-least-once (идемпотентность на стороне consumer) | At-most-once без TX; Exactly-once возможно через Kafka Transactions, но сложно | At-least-once |
| **Консистентность с БД** | Гарантирована (таблица `outbox` в той же TX) | Требует distributed transaction (2PC) или рискует потерять событие при rollback БД | Гарантирована на уровне WAL |
| **Зависимость от Kafka** | Низкая (relay может быть отложен или заменён) | Высокая (блокировка на latency producer) | Низкая |
| **Сложность инфраструктуры** | Низкая (таблица + ticker/воркер) | Низкая | Высокая (Debezium Connector, Kafka Connect cluster) |
| **Latency (event → broker)** | Средняя (зависит от poll interval relay) | Низкая (прямой send) | Низкая (near-real-time из WAL) |
| **Наблюдаемость** | Высокая (таблица `outbox` = явный backlog; retry_count/last_error) | Низкая (потерянное событие неотличимо от успеха) | Средняя (метрики коннектора, но business payload скрыт в WAL) |
| **Coupling со схемой БД** | Низкое (приложение контролирует payload и версионирование) | Низкое | Высокое (изменение столбцов влияет на topic schema) |
| **Возможность replay** | Да (по таблице `outbox`) | Нет | Да (snapshot + WAL) |

### Почему не прямая публикация

- Прямой `producer.Send()` внутри бизнес-транзакции создаёт **distributed transaction** (PostgreSQL + Kafka) или рискует опубликовать событие, которое впоследствии откатится по `ROLLBACK`.
- Kafka Transactions API (exactly-once) требует `transactional.id`, настройки `isolation.level`, координации с consumer-группами — операционная сложность неоправдана для демонстрационного проекта.
- При недоступности Kafka бизнес-транзакция вынуждена фейлить или накапливать события в памяти → OOM.

### Почему не CDC

- Debezium требует отдельного коннектора и операционного сопровождения (Kafka Connect, schema registry, dead-letter topics).
- Схема WAL напрямую влияет на формат сообщений — сложно эволюционировать контракт независимо от DDL.
- В рамках **portfolio piece** outbox проще для чтения, тестирования и демонстрации паттерна.

## Последствия

- В `order-db` и `catalog-db` созданы таблицы `outbox`; relay — фоновый процесс с реальным Kafka producer (sarama).
- Реализован poison-handling: `retry_count`, `last_error`, `next_retry_at`, DLQ, `SELECT ... FOR UPDATE SKIP LOCKED`.
- Все downstream consumer-ы идемпотентны.
- При переходе на >10K RPS следует рассмотреть CDC (Debezium) как следующий этап.

## Связанные задачи

- Реальный Kafka publisher
- Poison-handling + конкурентный забор `SKIP LOCKED`
- Честный статус Kafka/Outbox в документации
