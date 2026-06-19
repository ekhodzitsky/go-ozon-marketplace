# План исправлений по результатам аудита

**Цель:** устранить критические баги, упростить код, привести комментарии и инфраструктуру в соответствие с `AGENTS.md`.

**Ветка:** `fix/audit-followup`

## Этап 1. Критические баги (безопасность и корректность)

1. **Починить проброс authorization в downstream gRPC.**
   - `pkg/auth/service_token.go` — `UserAuthInterceptor` должен брать заголовок из `auth.IdentityFromContext`, а не из `ContextKeyAuthorizationHeader`.
   - `pkg/middleware/http_auth.go` — убедиться, что `Identity` заполняется с заголовком.
   - Удалить мёртвый `auth_forwarding.go` и `ContextKeyAuthorizationHeader`, если он больше не нужен.
   - Верификация: тесты `pkg/auth`, `pkg/middleware`, `services/api-gateway`.

2. **Исправить `GetUser` в api-gateway.**
   - `services/api-gateway/internal/usecase/user.go:46-57` — передать `id` в `GetUserRequest{UserId: id}`.
   - Верификация: unit-тесты api-gateway.

3. **Исправить race condition в inventory-service.**
   - `services/inventory-service/internal/usecase/usecase.go:123-159` — при `rowsAffected == 0` не списывать stock повторно.
   - `services/inventory-service/internal/repository/postgres/inventory_postgres.go:63-75` — добавить `SelectReservationForUpdate` или проверять существование резерва.
   - Верификация: интеграционные тесты inventory.

4. **Сделать refund в payment-service идемпотентным.**
   - `services/payment-service/internal/usecase/usecase.go:114-125` — сначала искать refund по `idempotency_key`, затем создавать.
   - `services/payment-service/internal/repository/postgres/payment_postgres.go:101-108` — обрабатывать unique violation.
   - Верификация: unit-тесты payment.

5. **Исправить обработку `UNIQUE VIOLATION` в catalog-service.**
   - `services/catalog-service/internal/repository/postgres/product_postgres.go:36-54` — не читать из aborted-транзакции.
   - Верификация: unit-тесты catalog.

6. **Починить потерю сообщений в analytics-service.**
   - `services/analytics-service/internal/consumer/processor.go:38-57` — возвращать ошибки вместо `nil` при bad unmarshal/unknown type.
   - Верификация: unit-тесты analytics.

7. **Устранить утечку памяти в analytics batcher.**
   - `services/analytics-service/internal/usecase/batcher.go` — убрать вечный `seen` или очищать после flush.
   - Верификация: unit-тесты analytics.

## Этап 2. Упрощение и соответствие AGENTS.md

1. Перевести комментарии на русский в `pkg/` и критичных сервисах.
2. Упростить `pkg/fxmodules` — убрать лишние дженерики.
3. Свернуть saga в `order-service` до 2–3 файлов.
4. Убрать `unitofwork` из catalog-service.
5. Вынести rate limiter/JWT из `user-service/usecase.go`.
6. Удалить мёртвый код (`api-gateway/internal/clients/adapter.go`, `delivery/grpc/context.go`).

## Этап 3. Proto и API

1. Добавить русскоязычные комментарии в proto.
2. Исправить `UpdateProductRequest` — `optional` или `FieldMask`.
3. Заменить строковые даты на `google.protobuf.Timestamp`.
4. Синхронизировать версии генератора и runtime.
5. Добавить сумму в `RefundRequest`.

## Этап 4. Тесты и скрипты

1. Вынести shared setup в `tests/e2e/helper_test.go`.
2. Исправить сортировку миграций в `tests/helper.go`.
3. Удалить бинарные артефакты и добавить в `.gitignore`.
4. Поправить benchmark payload и токены.
5. Упростить `tests/builder.go`, убрать `panic`.

## Этап 5. Инфраструктура k8s

1. Заполнить env во всех Helm-чартах.
2. Поправить namespace discovery в Prometheus.
3. Добавить provisioning dashboards в Grafana.
4. Включить OTLP в Jaeger.
5. Починить label-селекторы в chaos-манифестах.
6. Либо доделать canary, либо удалить.

## Этап 6. Финализация

1. Полный прогон тестов.
2. Final code review.
3. Merge в master.
