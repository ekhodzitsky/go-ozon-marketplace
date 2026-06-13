# Детальный код-аудит go-ozon-marketplace

**Версия кода:** v0.3.0  
**Дата аудита:** 2026-06-12  
**Метод:** параллельный аудит 13 `explore`-агентами по доменам + статический анализ (`go vet`, `golangci-lint`, `go test`)  
**Фокус:** качество кода, безопасность, архитектура, инфраструктура, тесты.  
**Примечание:** известные расхождения между документацией и кодом уже зафиксированы в `docs/AUDIT_REPORT.md`. Этот отчёт дополняет их техническими дефектами и точками рефакторинга.

---

## 1. Резюме

Аудит выявил **множество критических проблем**, которые делают проект неготовым к production-эксплуатации:

- **Безопасность:** шлюз не отклоняет невалидные JWT, резолверы не проверяют владение ресурсами (IDOR), admin/WebSocket открыты без авторизации, mTLS по умолчанию отключён.
- **Целостность данных:** дублирующиеся номера миграций, двойное списание остатков при повторном резервировании, подмена цены в заказе воспринимается системой как валидная, `Refund` не создаёт записи в `refunds`.
- **Инфраструктура / CI:** CI настроен на ветку `main` вместо `master`, корневые команды `Makefile` падают в workspace без `go.mod`, NetworkPolicy блокирует egress, observability не работает из-за несогласованных портов.
- **Тесты:** e2e-тест `TestPriceTamper` легализует баг, chaos-тесты завязаны на `localhost`, покрытие usecase / repository близко к нулю.
- **Код:** создание логгера на каждый запрос, устаревший `grpc.DialContext`, игнорирование ошибок `Close`, отсутствие `protovalidate`.

**Статистика по важности (агрегированная):**

| Severity | Количество | Основные группы |
|----------|-----------|-----------------|
| CRITICAL | ~25 | миграции, auth/IDOR, целостность данных, health endpoints, CI |
| HIGH | ~45 | TLS/mTLS, Kafka offset handling, бизнес-логика, observability |
| MEDIUM / LOW / REFACTOR | ~80 | логирование, тесты, proto, нейминг, дублирование |

---

## 2. Методология

1. **Разбиение на домены:** 8 микросервисов + `pkg` + `api` + `tests` + `infra` + `scripts/CI`.
2. **Параллельный аудит:** каждый агент получил чёткий scope, провёл read-only обход кода, запустил `go vet`/`go test` в своём модуле.
3. **Статический анализ:** `go vet` — по модулям, `golangci-lint run ./...` — по модулям, `go test -count=1 ./...` — по модулям.
4. **Верификация ключевых находок:** вручную проверены дублирующиеся миграции, ветка CI, `AuthHTTP`, `CreateOrder` resolver.

---

## 3. Кросс-категориальные проблемы

### 3.1. JWT и авторизация
- `pkg/auth` генерирует токены с `aud="go-ozon-marketplace"`, а middleware ожидает `"api-gateway"` — токены от `pkg/auth` отклоняются.
- `AuthHTTP` (`pkg/middleware/http.go:38-69`) при невалидном/отсутствующем токене **молча** вызывает `next.ServeHTTP`.
- Резолверы GraphQL (`api-gateway/graph/schema.resolvers.go`) не сверяют `userID` из аргументов с токеном; любой клиент может создавать заказы/отменять/смотреть чужие профили.

### 3.2. Деньги и точность
- В proto финансовые поля (`price`, `amount`) объявлены как `double`. В коде используется `int64(req.Price * 100)`, что теряет копейки (например, `19.99*100 → 1998`).
- E2E-тест `TestPriceTamper` ожидает успешное создание заказа с ценой `0.01` вместо отклонения подмены.

### 3.3. Миграции БД
- Дублирующиеся номера версий:
  - `services/user-service/migrations/002_*` — два файла.
  - `services/order-service/migrations/007_*` и `008_*` — по два файла.
  - `services/payment-service/migrations/005_*` — два файла.
- Любой стандартный мигратор упадёт или применит только часть схемы.

### 3.4. gRPC health checks
- `notification-service`, `analytics-service`, `inventory-service`, `catalog-service`, `payment-service`, `user-service`, `api-gateway` не регистрируют `grpc.health.v1.HealthServer`, хотя Kubernetes probes и e2e-хелперы ждут именно его.

### 3.5. Kafka consumers
- `notification-service` и `analytics-service` вызывают `session.MarkMessage` даже при ошибке обработки — события теряются.
- Дедупликация в analytics использует `userID` как ключ, отбрасывая все события одного пользователя, кроме первого.

### 3.6. Инфраструктура и CI
- `.github/workflows/ci.yml:5-7` триггерится на `main`, default-ветка — `master`.
- `Makefile` запускает `go test ./...` и `golangci-lint run ./...` из корня без `go.mod` — падают.
- `Makefile` заворачивает интеграционные тесты в `|| true`, маскируя падения.
- `.env.example:6` задаёт `JWT_SECRET` длиной 31 символ при требовании ≥32.

---

## 4. Критические находки (CRITICAL)

| # | Зона | Файл / место | Проблема | Рекомендация |
|---|------|--------------|----------|--------------|
| 1 | api-gateway | `graph/schema.resolvers.go:90` | `CreateOrder` принимает `userID` и цену от клиента | `userID` из JWT; цену брать из catalog-service |
| 2 | api-gateway | `internal/app/app.go:310-313`, `internal/admin/admin.go` | admin API фича-флагов без авторизации | `AuthHTTP` + `RequireRole(admin)` |
| 3 | api-gateway | `internal/ws/server.go:16-18,147-158` | WebSocket без проверки origin, auth и userID | Проверять origin, аутентифицировать upgrade, `userID` из токена |
| 4 | api-gateway | `internal/app/app.go:101-111` | mTLS: пустой `ServerName`, insecure fallback | Требовать hostname и TLS по умолчанию |
| 5 | api-gateway | `pkg/middleware/http.go:38-69` | `AuthHTTP` не возвращает 401 | fail-closed для защищённых endpoint'ов |
| 6 | user-service | `migrations/002_*.sql` | Два файла с номером `002` | Перенумеровать в `002` и `003` |
| 7 | user-service | `internal/delivery/grpc/handler.go:57-75` | `GetUser` без проверки владельца | Проверять `req.UserId == ctx.userID` или `admin` |
| 8 | catalog-service | `internal/unitofwork/postgres/uow.go:39` | `Rollback` паникует при `nil`-транзакции | Проверять `u.tx != nil` |
| 9 | catalog-service | `internal/delivery/grpc/handler.go:36`, `toProtoProduct` | `double` цена → `int64(price*100)` и обратно `float64(cents)` | Хранить и передавать цену в копейках как `int64` |
| 10 | catalog-service | `internal/usecase/usecase.go:104-152` | `UpdateProduct`/`DeleteProduct` не создают outbox-событий | Публиковать `ProductUpdated`/`ProductDeleted` и обновлять ES |
| 11 | order-service | `migrations/007_*.sql`, `008_*.sql` | Дублирующиеся номера миграций | Перенумеровать последовательно |
| 12 | order-service | `internal/usecase/usecase.go:172-200` | `CancelOrder` отменяет любой статус без refund | Разрешить только `pending`/`awaiting_payment`; для оплаченных — saga-компенсация |
| 13 | order-service | `internal/outbox/relay.go:82-86` | `FOR UPDATE` вне транзакции — дублирование событий | Оборачивать `poll` в транзакцию |
| 14 | inventory-service | `internal/repository/postgres/inventory_postgres.go:70-108` | `Reserve` неидемпотентен — двойное списание | `ON CONFLICT DO NOTHING RETURNING`, не обновлять inventory при конфликте |
| 15 | inventory-service | `internal/app/app.go:55-103` | Нет gRPC health-сервера | Зарегистрировать `grpc.health.v1.Health` |
| 16 | payment-service | `migrations/005_*.sql` | Два файла с номером `005` | Перенумеровать `005_create_refunds` → `006` |
| 17 | payment-service | `internal/usecase/usecase.go:101-118` | `Refund` не создаёт запись в `refunds` | Обернуть в tx, вставлять в `refunds`, условный `UPDATE` |
| 18 | notification-service | `internal/usecase/usecase.go:25-34` | `SendEmail` — noop-заглушка | Реализовать `EmailProvider` (SMTP/SES) |
| 19 | notification-service | `internal/consumer/consumer.go:112` | Offset коммитится при любой ошибке | Коммитить только при `err == nil` |
| 20 | analytics-service | `internal/consumer/consumer.go:108-112` | Offset коммитится при ошибке `TrackEvent` | Коммитить только при успехе, retry/DLQ |
| 21 | analytics-service | `internal/repository/clickhouse/event_ch.go:40-48` | Таблица `events` создаётся без `amount`, но `GetDailyRevenue` суммирует `amount` | Единый источник схемы через миграции, добавить `amount` в событие |
| 22 | analytics-service | `internal/consumer/consumer.go:99-103` | Дедупликация по `userID` теряет события | Использовать уникальный `event_id` |
| 23 | pkg | `pkg/featureflags/featureflags.go:145-183` | Data race при обновлении флага | Передавать копию флага в `saveFlag` |
| 24 | infra | `.github/workflows/ci.yml:5-7` | CI на `main`, ветка — `master` | Исправить триггеры |
| 25 | infra | `.env.example:6` | `JWT_SECRET` 31 символ | Сделать ≥32 символов и валидировать |

---

## 5. Высокий приоритет (HIGH)

| # | Зона | Проблема | Рекомендация |
|---|------|----------|--------------|
| 1 | api-gateway | CORS `*` + credentials | echo-возвращать конкретный origin |
| 2 | api-gateway | Rate-limiter fail-open при ошибке Redis | fail-closed или локальный fallback |
| 3 | api-gateway | Блокирующий dial всех downstream-сервисов при старте | `grpc.NewClient` без `WithBlock` |
| 4 | api-gateway | WebSocket/Redis горутины не останавливаются | Cancellable context + `Hub.Stop()` |
| 5 | api-gateway | Сырые gRPC-ошибки пробрасываются в GraphQL | Маппинг статусов на публичные ошибки |
| 6 | user-service | `Login` различает «нет пользователя» и «неверный пароль` | Единый `ErrInvalidCredentials` |
| 7 | user-service | Регистрация не обрабатывает `unique_violation` | Ловить `23505` → `ErrAlreadyExists` |
| 8 | catalog-service | `UpdateProduct` перезаписывает все поля | PATCH-семантика или `FieldMask` |
| 9 | catalog-service | Read-методы каталога не в публичном whitelist | Добавить `GetProduct`/`ListProducts`/`SearchProducts` |
| 10 | order-service | `UpdateOrderStatus` принимает любую строку | Enum `OrderStatus` + finite state machine |
| 11 | order-service | Saga Recovery Worker без распределённой блокировки | `FOR UPDATE SKIP LOCKED` или advisory lock |
| 12 | order-service | `UpdateStatus` не проверяет `RowsAffected` | Возвращать `ErrNotFound` |
| 13 | order-service | `serviceAuthInterceptor` игнорирует ошибку `SignedString` | Обработать ошибку |
| 14 | inventory-service | `GetStock` требует JWT, но README говорит обратное | Синхронизировать whitelist и README |
| 15 | payment-service | IDOR в `GetRefund`/`ListRefunds` | Проверять владельца |
| 16 | payment-service | DLQ-отправка синхронная, ошибка игнорируется | Асинхронная отправка + логирование |
| 17 | payment-service | Сервис не стартует без Kafka/DLQ | Ленивая инициализация DLQ |
| 18 | notification-service | `SendEmail` логирует PII (`body`, `to`) | Маскировать/не логировать чувствительные поля |
| 19 | analytics-service | Single-row inserts в ClickHouse | Batching / buffered insert |
| 20 | analytics-service | Нет gRPC health-сервера | Зарегистрировать `HealthServer` |
| 21 | pkg | `pkg/middleware/ratelimit.go` — `now` в мс, `window` в с | Привести к одной шкале |
| 22 | pkg | `pkg/middleware/ratelimit.go` — `ClientIP` берёт левый XFF | Брать крайний правый доверенный адрес |
| 23 | pkg | `pkg/server/server.go` — TLS без `MinVersion` | `tls.VersionTLS12` минимум |
| 24 | api | `double` для денег во всех proto | `int64` minor units |
| 25 | api | Отсутствует `protovalidate` | Аннотации полей + единый валидирующий interceptor |
| 26 | tests | `TestPriceTamper` легализует подмену цены | Инвертировать ожидание |
| 27 | tests | JWT в `saga_compensation_test.go` несовместим с middleware | Использовать `CustomClaims` |
| 28 | infra | NetworkPolicy default-deny без egress-правил | Разрешить egress к БД/брокерам/monitoring |
| 29 | infra | Metrics-порты в Helm не совпадают с кодом | Передать `METRICS_PORT` и согласовать |
| 30 | infra | ArgoCD `Application` указывает на директорию с несколькими чартами | Umbrella-chart или ApplicationSet |

---

## 6. Средний приоритет и рефакторинг (MEDIUM / LOW / REFACTOR)

- **Логирование:** `AccessLog`, `LoggingUnaryInterceptor`, `NewGRPC`, `NewHTTP` создают `*zap.Logger` на каждый запрос. Внедрить logger через DI.
- **gRPC:** заменить `grpc.DialContext` + `grpc.WithBlock` на `grpc.NewClient`.
- **Ошибки:** много мест, где `defer x.Close()` игнорирует ошибку; использовать именованные возвраты или явную обработку.
- **Конфигурация:** `DefaultCallTimeout`/`HTTPPort` в сервисах объявлены, но не используются; удалить мёртвые поля.
- **Proto:** `created_at`/`updated_at` как `string` вместо `google.protobuf.Timestamp`; статусы как `string` вместо enum.
- **Тесты:** покрытие usecase/repository/app/config близко к нулю; добавить unit + integration-тесты с testcontainers.
- **Кодстайл:** `gofmt` для `pkg/errors`; godoc для экспортируемых типов.
- **Бинарные артефакты:** `services/*/main`, `*.test`, `coverage.out` закоммичены — удалить и добавить в `.gitignore`.
- **GraphQL:** нет complexity/depth limits; добавить `extension.FixedComplexityLimit`.
- **HTTP серверы:** отсутствуют `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes`.

---

## 7. Аудит по доменам — кратко

### 7.1. api-gateway
- **CRITICAL:** auth bypass, IDOR в резолверах, admin/WebSocket без защиты, mTLS insecure.
- **HIGH:** CORS `*` + credentials, rate-limiter fail-open, блокирующий startup dial, сырые gRPC-ошибки.
- **Топ-5 действий:** закрыть auth/IDOR, защитить admin/WebSocket, исправить TLS/mTLS, graceful shutdown WebSocket, расширить тесты и лимиты GraphQL.

### 7.2. user-service
- **CRITICAL:** дублирующийся номер миграции `002`, IDOR в `GetUser`.
- **HIGH:** `Login` раскрывает email, нет health-сервера, race при регистрации.
- **Топ-5 действий:** исправить миграции, закрыть IDOR, унифицировать ошибки `Login`, зарегистрировать health, добавить rate limiting на auth.

### 7.3. catalog-service
- **CRITICAL:** panic в `UoW.Rollback`, финансовые ошибки `price*100`, outbox не публикует update/delete.
- **HIGH:** `UpdateProduct` — full overwrite, read-методы не публичные, ES-индекс не создаётся.
- **Топ-5 действий:** исправить цену, добавить outbox-события, защитить `Rollback`, проверять `RowsAffected`, health + ES index creation.

### 7.4. order-service
- **CRITICAL:** дублирующиеся миграции `007`/`008`, `CancelOrder` без refund, `UpdateOrderStatus` произвольная строка, outbox `FOR UPDATE` вне транзакции.
- **HIGH:** saga recovery без блокировки, `RowsAffected` не проверяется, `float64*100` для цены.
- **Топ-5 действий:** перенумеровать миграции, state machine статусов, транзакционный outbox, distributed lock recovery, тесты usecase/repository.

### 7.5. inventory-service
- **CRITICAL:** неидемпотентный `Reserve` → двойное списание.
- **HIGH:** `inventory_ledger` никто не пишет, нет health-сервера.
- **Топ-5 действий:** исправить идемпотентность, писать в ledger, health server, интеграционные тесты конкурентности, улучшить кэш-инвалидацию.

### 7.6. payment-service
- **CRITICAL:** дублирующийся номер миграции `005`, `Refund` не создаёт запись в `refunds`.
- **HIGH:** IDOR в `ProcessPayment`/`GetRefund`, `double` для денег, DLQ синхронный, старт падает без Kafka.
- **Топ-5 действий:** исправить миграции, починить refund, устранить IDOR, точное представление денег, health + repository tests.

### 7.7. notification-service
- **CRITICAL:** `SendEmail` — noop, offset коммитится при ошибке, нет health-сервера.
- **HIGH:** отсутствует валидация, логирование PII, mTLS отключён по умолчанию.
- **Топ-5 действий:** реальная отправка email, корректный commit offset, health server, валидация, маскирование PII.

### 7.8. analytics-service
- **CRITICAL:** потеря событий при ошибках, схема ClickHouse не содержит `amount`, дедупликация по `userID`.
- **HIGH:** нет health-сервера, отсутствует валидация, DDL в репозитории вместо миграций.
- **Топ-5 действий:** commit offset только при успехе, единая схема через миграции, уникальный idempotency key, health server, batching вставок.

### 7.9. pkg (общие пакеты)
- **CRITICAL:** data race в `featureflags`.
- **HIGH:** logger на каждый запрос, rate-limiter с разными единицами времени, `ClientIP` берёт левый XFF, TLS без `MinVersion`.
- **Топ-5 действий:** унифицировать JWT, сделать `AuthHTTP` fail-closed, исправить race в featureflags, починить rate-limiter, DI для logger'а.

### 7.10. api (proto / gen)
- **CRITICAL:** IDOR через `user_id` в запросах, `double` для денег.
- **HIGH:** отсутствует `protovalidate`, мутации без идемпотентности.
- **Топ-5 действий:** убрать `user_id` из запросов (metadata), `int64` minor units, `protovalidate`, `idempotency_key`, enum для статусов.

### 7.11. tests
- **CRITICAL:** `TestPriceTamper` легализует баг, JWT в e2e несовместим с middleware.
- **HIGH:** chaos-тесты хардкод `localhost`, `StartService` убивает процесс, общая БД для сервисов.
- **Топ-5 действий:** инвертировать `TestPriceTamper`, единый JWT-хелпер, параметризовать chaos, graceful shutdown + сбор логов, версионирование миграций.

### 7.12. infra
- **CRITICAL:** CI на `main`, `JWT_SECRET` 31 символ, ArgoCD сломан, NetworkPolicy блокирует egress.
- **HIGH:** metrics-порты не согласованы, Jaeger без OTLP, namespace mismatch.
- **Топ-5 действий:** исправить CI-триггеры, секреты, umbrella-chart / ApplicationSet, network policies, observability порты.

### 7.13. scripts / CI / Makefile
- **CRITICAL:** CI не запускается на `master`, интеграционные тесты с `|| true`, мерж coverage неверной утилитой.
- **HIGH:** `seed.go` не проверяет HTTP-статусы, хардкод DSN/паролей, `http.DefaultClient` без таймаута.
- **Топ-5 действий:** исправить триггеры CI, убрать `|| true`, починить merge coverage, переписать seed с env/tls/validation, синхронизировать env-переменные.

---

## 8. Статический анализ

### 8.1. `go vet ./...` (по модулям)
- Прошёл без замечаний во всех модулях.

### 8.2. `golangci-lint run ./...` (по модулям)

| Модуль | Issues | Типы |
|--------|--------|------|
| `pkg` | 0 | — |
| `api` | 0 | — |
| `scripts` | 3 | errcheck (`Close`) |
| `tests` | 2 | errcheck |
| `services/user-service` | 0 | — |
| `services/api-gateway` | 24 | 18 errcheck, 6 staticcheck (`grpc.DialContext` deprecated) |
| `services/catalog-service` | 3 | errcheck (`Rollback`) |
| `services/inventory-service` | 2 | errcheck (`Rollback`) |
| `services/notification-service` | 2 | 1 ineffassign, 1 staticcheck (sarama) |
| `services/order-service` | 6 | 2 errcheck, 4 staticcheck (`grpc.DialContext`/`WithBlock`) |
| `services/payment-service` | 1 | errcheck |
| `services/analytics-service` | 1 | staticcheck (sarama) |
| **Итого** | **44** | |

### 8.3. `go test -count=1 ./...` (по модулям)

- Юнит-тесты прошли в пакетах `delivery/grpc`, `usecase`, `saga`, `consumer`, `dlq`, `middleware`, `server`.
- Большинство пакетов `internal/app`, `internal/config`, `internal/repository`, `internal/domain` не имеют тестовых файлов.
- Корневые команды `go test ./...` и `golangci-lint run ./...` из workspace-root **падают** с ошибкой `directory prefix . does not contain modules listed in go.work...`.

---

## 9. Дорожная карта (приоритезированная)

### Этап 1 — Stop-the-bleed (CRITICAL)
1. Исправить дублирующиеся номера миграций в user-service, order-service, payment-service.
2. Закрыть auth bypass / IDOR в api-gateway (strict `AuthHTTP`, проверка владения в резолверах).
3. Защитить admin API и WebSocket аутентификацией/авторизацией.
4. Исправить идемпотентность `Reserve` в inventory-service.
5. Починить `Refund` в payment-service (транзакция + запись в `refunds`).
6. Исправить обработку ошибок в Kafka consumers (notification, analytics).
7. Исправить триггеры CI на `master` и убрать `|| true` у интеграционных тестов.
8. Увеличить `JWT_SECRET` до ≥32 символов и валидировать длину.

### Этап 2 — Безопасность и целостность (HIGH)
1. mTLS: hostname verification, `MinVersion: TLS 1.2`, insecure только по явному флагу.
2. Цены: перевести proto на `int64` minor units, убрать `price*100`.
3. State machine для статусов заказа; ограничить `CancelOrder`/`UpdateOrderStatus`.
4. Транзакционный outbox relay в order-service.
5. Distributed lock для saga recovery worker.
6. IDOR в `GetRefund`/`ListRefunds`, `GetUser`, `ProcessPayment`.

### Этап 3 — Надёжность и observability (MEDIUM)
1. Зарегистрировать `grpc.health.v1.Health` во всех сервисах.
2. Настроить metrics-порты, Prometheus SD, Jaeger OTLP.
3. Исправить NetworkPolicy и namespace consistency в k8s.
4. Добавить `ReadTimeout`/`WriteTimeout` к HTTP-серверам.
5. Graceful shutdown для WebSocket hub, Redis pubsub, background workers.

### Этап 4 — Тесты и качество кода (REFACTOR)
1. Инвертировать `TestPriceTamper`; унифицировать JWT-хелперы в тестах.
2. Параметризовать chaos-тесты, перейти на health-based ожидания.
3. Добавить unit + integration тесты для usecase/repository/app/config.
4. Внедрить logger через DI; устранить дублирование JWT-логики.
5. Внедрить `protovalidate`, заменить string-статусы на enum.
6. Удалить закоммиченные бинарники из репозитория.

---

## 10. Заключение

Проект демонстрирует хорошую структуру (чистая архитектура, DI через `fx`, Transactional Outbox, Saga, gRPC + GraphQL), но на текущий момент содержит критические дефекты безопасности, целостности данных и инфраструктуры. Рекомендуется сначала закрыть этап 1 (stop-the-bleed), затем последовательно пройти остальные этапы. Без исправления миграций, авторизации и обработки ошибок в Kafka/ClickHouse деплой в production будет нестабильным и небезопасным.
