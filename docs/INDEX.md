# Документация go-ozon-marketplace

Навигация по документации production-grade микросервисного маркетплейса.

## 🚀 С чего начать?

1. **[QUICKSTART.md](QUICKSTART.md)** — поднять проект за 10 минут
2. **[ARCHITECTURE.md](ARCHITECTURE.md)** — понять архитектуру (15 минут)
3. **[API.md](API.md)** — научиться вызывать API (20 минут)

## 📖 Разделы

### Архитектура и дизайн

| Документ | Описание | Время чтения |
|----------|----------|--------------|
| [ARCHITECTURE.md](ARCHITECTURE.md) | Сервисы, потоки данных, Saga, CQRS, WebSocket, GitOps | 15 мин |
| [DECISION_LOG.md](DECISION_LOG.md) | 7 ключевых архитектурных решений с обоснованием | 10 мин |
| [design.md](design.md) | Оригинальный design document и ADR | 20 мин |

### Разработка

| Документ | Описание | Время чтения |
|----------|----------|--------------|
| [CONTRIBUTING.md](CONTRIBUTING.md) | Как участвовать в разработке | 10 мин |
| [API.md](API.md) | GraphQL, gRPC, WebSocket, JWT | 20 мин |
| [CHANGELOG](../CHANGELOG.md) | История изменений по версиям | 5 мин |

### Безопасность и надёжность

| Документ | Описание | Время чтения |
|----------|----------|--------------|
| [SECURITY.md](SECURITY.md) | JWT, mTLS, rate limiting, circuit breaker, CORS, input validation | 15 мин |
| [OPERATIONS.md](OPERATIONS.md) | Runbook для on-call: падения сервисов, откат релизов, масштабирование | 15 мин |
| [SLO.md](SLO.md) | SLI/SLO определения и PromQL запросы | 5 мин |

### Производительность и развёртывание

| Документ | Описание | Время чтения |
|----------|----------|--------------|
| [PERFORMANCE.md](PERFORMANCE.md) | Бенчмарки, bottleneck'и, рекомендации | 10 мин |
| [DEPLOYMENT.md](DEPLOYMENT.md) | Docker Compose, Kubernetes, Helm, ArgoCD, Canary | 20 мин |

### История и аудит

| Документ | Описание | Время чтения |
|----------|----------|--------------|
| [AUDIT_REPORT.md](AUDIT_REPORT.md) | Аудит актуальности документации | 5 мин |
| [RELEASE_NOTES_v0.3.0.md](RELEASE_NOTES_v0.3.0.md) | Release notes для v0.3.0 | 5 мин |

## 🛠️ Makefile

Ключевые команды:

```bash
make dev-up          # Поднять всё в Docker Compose
make dev-seed        # Заполнить тестовыми данными
make dev-down        # Остановить всё
make test            # Unit + integration тесты
make test-e2e        # E2E тесты
make chaos-test      # Chaos engineering тесты
make lint            # Линтер
make proto           # Генерация proto
make bench-grpc      # gRPC бенчмарки
make bench-graphql   # GraphQL бенчмарки
make ws-test         # WebSocket пример
```

## 🏷️ Версии

- **[v1.0.0](../CHANGELOG.md)** — production-ready релиз
- [v0.3.0](RELEASE_NOTES_v0.3.0.md) — GraphQL expansion, observability, chaos engineering
