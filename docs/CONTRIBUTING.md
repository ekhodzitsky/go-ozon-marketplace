# Руководство для контрибьюторов

Как участвовать в разработке маркетплейса.

## Начало работы

```bash
# Клонировать репозиторий
git clone https://github.com/ekhodzitsky/go-ozon-marketplace.git
cd go-ozon-marketplace

# Установить зависимости
go version  # требуется Go 1.26+
docker --version  # требуется Docker
make --version  # требуется make

# Установить инструменты
make proto  # buf generate
```

## Структура проекта

```
├── api/           # Proto контракты и сгенерированный код
├── pkg/           # Общие библиотеки (middleware, logger, errors)
├── services/      # Микросервисы (каждый — отдельный go.mod)
├── infra/         # Docker Compose, Helm, monitoring
├── tests/         # E2E и integration тесты
├── docs/          # Документация
└── scripts/       # Вспомогательные скрипты
```

## Запуск локально

```bash
# Поднять инфраструктуру
make up

# Запустить сервис (в отдельном терминале)
export POSTGRES_DSN="postgres://ozon:ozonpass@localhost:5432/marketplace?sslmode=disable"
export JWT_SECRET="min-32-chars-secret-key-here!!!"
cd services/<service> && go run ./cmd/...

# Запустить тесты
make test

# Запустить линтер
make lint

# Остановить всё
make down
```

## Как писать код

### Стиль

- Следуйте [Effective Go](https://go.dev/doc/effective_go)
- Используйте `gofmt` и `golangci-lint`
- Все публичные функции и типы — с godoc комментариями (желательно)
- Ошибки — через `pkg/errors` для sentinel-ошибок, `fmt.Errorf` допустим для обёртывания

### Именование

- Файлы: `snake_case.go` для тестов, `camelCase.go` для обычных
- Интерфейсы: описательные (`OrderRepository`, `PaymentUsecase`)
- Моки: `mock_*.go` через gomock

### Логирование

```go
logger.Info(ctx, "order created", zap.String("order_id", orderID))
```

- Всегда передавайте `context.Context`
- Используйте structured fields
- Уровни: `debug`, `info`, `warn`, `error`

### Работа с БД

- Только `pgx`/`pgxpool`
- Параметризованные запросы — обязательно
- Миграции через `golang-migrate/migrate`
- Деньги — `BIGINT` в БД (копейки), но в proto используется `double`

## Тесты

### Unit тесты

```go
func TestCreateOrder(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    
    repo := mock_repository.NewMockOrderRepository(ctrl)
    svc := usecase.NewOrderService(repo)
    
    // table-driven tests
    tests := []struct{...}
}
```

- Table-driven — стандарт
- Моки через gomock
- Целевое покрытие: > 60%

### Integration тесты

```bash
go test -tags=integration ./...
```

- testcontainers-go для PostgreSQL, Redis, Kafka, ClickHouse, ES
- **Примечание:** В CI integration тесты не запускаются (нет `-tags=integration`)

### E2E тесты

```bash
cd tests && go test -tags=e2e ./e2e/...
```

- Требуется запущенный Docker Compose стек

## Коммиты

Используйте [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add inventory reservation endpoint
fix: handle nil pointer in order service
docs: update API description
test: add saga compensation test
refactor: extract payment processor interface
chore: update dependencies
```

## Proto контракты

```bash
# После изменения .proto файлов
make proto

# Проверить lint и breaking changes
make proto-lint  # если добавлен в Makefile
```

- Не ломайте обратную совместимость без согласования

## Миграции

```bash
# Новая миграция
migrate create -ext sql -dir services/<service>/migrations <name>

# Применить
make migrate-<service> DB_URL=postgres://...

# Для user-service (особая переменная)
make migrate-user USER_DB_URL=postgres://...
```

- Имена: `001_create_orders.up.sql`, `001_create_orders.down.sql`
- Всегда пишите `down` миграции
- Не меняйте уже применённые миграции

## Pull Request

1. Создайте ветку: `git checkout -b feature/my-feature`
2. Сделайте изменения с тестами
3. Убедитесь, что `make ci` проходит локально
4. Запушьте и создайте PR
5. В описании PR укажите:
   - Что изменено
   - Зачем (мотивация)
   - Как тестировали

## Code Review

- Минимум 1 approve
- CI должен быть зелёным
- Разрешаем squash merge

## Вопросы

- Открывайте Issue для багов и фич
- Обсуждайте архитектурные изменения в Issue до реализации
