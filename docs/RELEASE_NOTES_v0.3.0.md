# Release Notes v0.3.0

## Краткое описание

Релиз v0.3.0 расширяет функциональность маркетплейса полноценными GraphQL-запросами для заказов, инвентаря и платежей, добавляет новые gRPC-методы для управления жизненным циклом заказов и каталога, а также внедряет комплексную observability-платформу на базе OpenTelemetry, Prometheus и Grafana. Все сервисы теперь покрыты K8s HPA, NetworkPolicies и интегрированы с мониторинговым стеком. Kafka consumers подключены к analytics-service и notification-service, а payment-service получил dead letter queue для обработки ошибочных сообщений.

## Ключевые фичи

- **GraphQL Gateway**: клиенты для order-service, inventory-service и payment-service; запросы `me`, `orders`, `inventory`, `order`, `cancelOrder`.
- **Новые gRPC методы**: `CancelOrder`, `UpdateOrderStatus`, `UpdateProduct`, `DeleteProduct`.
- **Observability**: OpenTelemetry tracing, configurable `LOG_LEVEL`/`LOG_FORMAT`, Prometheus metrics на всех сервисах.
- **Kafka consumers**: analytics-service и notification-service читают события из Kafka.
- **DLQ**: payment-service dead letter queue для poison messages.
- **Inventory ledger**: таблица и API для учёта резерваций и движения товаров.
- **Payment refunds**: таблица и API для возвратов платежей.
- **Тестирование**: unit и E2E тесты для всех новых функций.
- **Kubernetes**: HPA для всех сервисов, monitoring stack (Prometheus, Grafana, Jaeger), NetworkPolicies.

## Инструкция по обновлению

### Breaking changes
- Добавлены новые required env vars для observability:
  - `OTEL_EXPORTER_OTLP_ENDPOINT` — endpoint для OpenTelemetry collector (по умолчанию `http://localhost:4317`)
  - `LOG_LEVEL` — уровень логирования (по умолчанию `info`)
  - `LOG_FORMAT` — формат логов: `json` или `console` (по умолчанию `json`)
- Inventory ledger и payment refunds требуют применения новых миграций базы данных.

### Новые env vars
| Переменная | Описание | Пример |
|------------|----------|--------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP endpoint для tracing | `http://jaeger:4317` |
| `LOG_LEVEL` | Уровень логирования | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | Формат вывода логов | `json`, `console` |
| `METRICS_PORT` | Порт для Prometheus metrics | `9090` |

### Миграции
- Примените новые миграции inventory-service (inventory ledger).
- Примените новые миграции payment-service (payment refunds).

## Ссылки

- [CHANGELOG](../CHANGELOG.md)
- [Архитектура](ARCHITECTURE.md)
- [Быстрый старт](QUICKSTART.md)
- [API](API.md)
- [Развёртывание](DEPLOYMENT.md)
