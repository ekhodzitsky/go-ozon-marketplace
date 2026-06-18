# Graphify

[Graphify](https://graphify.net/) — open-source инструмент для построения knowledge graph из codebase. Он помогает быстрее ориентироваться в микросервисном Go-проекте и даёт AI-ассистентам структурированный контекст.

## Что даёт интеграция

- Интерактивная визуализация зависимостей между сервисами, пакетами и proto-контрактами.
- `graph.json` для программного доступа к графу.
- `GRAPH_REPORT.md` с кратким обзором ключевых узлов и неожиданных связей.

## Быстрый старт

```bash
# Сгенерировать граф локально (offline, AST-only, без API-ключей)
./scripts/graphify.sh

# Или с семантической (LLM) экстракцией внутри AI-ассистента
./scripts/graphify.sh --semantic

# Результаты появятся в graphify-out/
ls graphify-out/
```

Скрипт сам проверит наличие Python 3.10+, создаст виртуальное окружение в `.graphify/venv` и установит Graphify.

## Исключения из графа

Файл `.graphifyignore` определяет, что не попадает в граф:

- сгенерированный код (`api/gen/`, `bin/`);
- зависимости (`vendor/`);
- тест-файлы (`*_test.go`);
- кэши, VCS, IDE-файлы.

Синтаксис совпадает с `.gitignore`.

## Git hooks

Чтобы граф пересобирался автоматически после коммитов и переключений веток:

```bash
./scripts/install-graphify-hook.sh
```

Хуки устанавливаются локально (в `.git/hooks`) и не коммитятся.

## MCP server

Если ваш ассистент поддерживает MCP, подключите Graphify как stdio-сервер:

- Конфигурация для Claude Code уже в `.mcp.json`.
- Скрипт запуска: `./scripts/graphify-mcp.sh`.
- Требуется предварительно сгенерированный `graphify-out/graph.json`.

Доступные инструменты: `query_graph`, `get_node`, `get_neighbors`, `shortest_path`, `graph_stats`.

Для Cursor добавьте аналогичную конфигурацию в `.cursor/mcp.json` или настройки IDE.

## Правила для AI-ассистентов

- `.cursorrules` — правила для Cursor.
- `AGENTS.md` — инструкции для Claude Code / Codex / Gemini CLI и других ассистентов, читающих `AGENTS.md`.

## CI

- На PR: smoke-test helper-скрипта и проверка правил `.graphifyignore` через `git check-ignore`.
- На push в `master`: offline-генерация графа (`graphify update . --force`) и загрузка артефактов в workflow `graphify`.

## Ограничения

- Graphify — опциональный dev-tool, не добавляется в runtime сервисов.
- Семантическая (LLM) экстракция требует API-ключ и запуска через AI-ассистента или явного backend.
