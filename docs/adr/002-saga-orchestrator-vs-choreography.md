# ADR-002: Saga — Orchestrator vs Choreography vs Kafka-native

**Статус:** Accepted  
**Дата:** 2026-06-10  
**Автор:** go-ozon-marketplace team  
**Язык:** RU (термины Saga/Orchestrator/Choreography/Kafka-native не переводятся)

---

## Контекст

В `order-service` необходимо реализовать распределённую транзакцию, охватывающую резервирование инвентаря (`inventory-service`), обработку оплаты (`payment-service`) и финальное подтверждение заказа. Выбранный подход должен быть воспроизводимым в демонстрационном проекте и покрывать ключевые паттерны Ozon (3500+ микросервисов).

Рассматривались три альтернативы:

1. **Orchestration (оркестратор в `order-service`)** — центральный координатор явно вызывает шаги саги и управляет компенсациями.
2. **Choreography (событийная хореография)** — сервисы реагируют на доменные события друг друга без центрального координатора.
3. **Kafka-native (Kafka Streams / ksqlDB)** — stateful обработка топологии саги на стороне брокера.

## Решение

Использовать **Orchestration** с явным Saga-оркестратором в `order-service`.

## Trade-offs

| Критерий | Orchestration | Choreography | Kafka-native |
|---|---|---|---|
| **Сложность реализации** | Низкая (явный код, отладка в IDE) | Средняя (неявные связи через топики) | Высокая (инфраструктура Streams, rebalancing) |
| **Наблюдаемость** | Высокая (единый лог оркестратора, явный state machine) | Низкая (логика «размазана» по consumer-группам) | Средняя (Kafka metrics, но business state непрозрачен) |
| **Coupling** | Высокий (оркестратор знает все шаги и их порядок) | Низкий (сервисы знают только события) | Низкий (топология в Streams, а не в коде сервисов) |
| **Durability / Recovery** | Требует persisted saga-state в БД + recovery-воркер | Сложно восстановить глобальное состояние при partial failure | Высокая (log-based state stores, replay) |
| **Масштабируемость** | Ограничена БД оркестратора (row-level locks) | Высокая (независимые consumer-группы) | Очень высокая (partition scaling) |
| **Team autonomy** | Низкая (изменение саги требует правки `order-service`) | Высокая (каждая команда владеет своим consumer) | Высокая (топология вне сервисов) |
| **Совместимость с Outbox** | Высокая (оркестратор может читать outbox как event source) | Средняя (требует согласованности event ordering) | Низкая (Streams предпочитает прямой ingress) |

### Почему не Choreography

- При ошибке на шаге N компенсация должна затронуть шаги 1..N-1. В choreography компенсационные события рассылаются через тот же pub/sub, что приводит к race condition и сложности отслеживания «кто уже скомпенсировал».
- В Ozon-подобной системе с сотнями сервисов choreography быстро превращается в «спагетти-диаграмму» событий, которую невозможно отладить.

### Почему не Kafka-native

- Проект — **portfolio piece**, цель — продемонстрировать Go-код, паттерны и observability, а не операционную экспертизу по Kafka Streams.
- Kafka-native добавляет операционную сложность (RocksDB state stores, exactly-once semantics, rebalancing) без значимого выигрыша на целевом масштабе (<1000 RPS).
- Stateful stream processing скрывает business logic от разработчика, что противоречит цели «чистой архитектуры с явным доменом».

## Последствия

- `order-service` содержит `saga.Orchestrator` с явной state machine, persisted saga-state в PostgreSQL, recovery worker и retry с exponential backoff.
- Компенсация корректна: release только для успешно зарезервированных items, refund при ошибке после оплаты.
- При росте числа шагов >5 или необходимости cross-team autonomy следует рассмотреть choreography или гибрид.

## Связанные задачи

- Durable saga: background-ctx, persisted state, recovery
- Корректная компенсация (release только зарезервированного)
