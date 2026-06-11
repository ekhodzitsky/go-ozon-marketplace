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

- Следуйте [Effective Go](https://go.dev/doc/effective_go) и [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- Используйте `gofmt` и `golangci-lint`
- Все публичные функции и типы — с godoc комментариями
- Ошибки — через `pkg/errors`, не `fmt.Errorf` напрямую

### Именование

- Файлы: `snake_case.go` для тестов, `camelCase.go` для обычных
- Интерфейсы: заканчиваются на `er` (`Reader`, `Writer`) или описательные (`OrderRepository`)
- Моки: `mock_*.go` через gomock

### Логирование

```go
logger.Info(ctx, "order created", zap.String("order_id", orderID))
```

- Всегда передавайте `context.Context`
- Используйте structured fields, не форматирование строк
- Уровни: `debug` для деталей, `info` для бизнес-событий, `warn` для проблем, `error` для ошибок

### Работа с БД

- Только `pgx`/`pgxpool`, не `database/sql` напрямую
- Параметризованные запросы — обязательно
- Миграции через `golang-migrate/migrate`
- Деньги — `int64` (копейки), не `float64`

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
- Каждый тест — изолированная БД

### E2E тесты

```bash
go test -tags=e2e ./tests/e2e/...
```

- Требуется запущенный Docker Compose стек
- Проверяют полный сценарий: регистрация → товар → заказ

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
cd api && buf generate

# Проверить на breaking changes
make proto
```

- Всегда запускайте `buf lint` и `buf breaking`
- Не ломайте обратную совместимость без согласования

## Миграции

```bash
# Новая миграция
migrate create -ext sql -dir services/<service>/migrations <name>

# Применить
make migrate-<service> DB_URL=postgres://...
```

- Имена: `000001_create_orders.up.sql`, `000001_create_orders.down.sql`
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
