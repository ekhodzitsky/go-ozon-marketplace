# Agent Instructions

## Назначение репозитория

Этот репозиторий — учебный проект для разбора system-design. Любой разработчик должен разобраться в коде быстро, без погружения в лишние детали.

### Принципы

- **Минимум кода.** Решай задачу самым простым способом. Без овер-инжиниринга.
- **Понятные комментарии на русском.** Пиши по-русски, разговорным языком, но точно. Без канцелярита и без перевода на английский.
- **Ясность важнее универсальности.** Лучше прямолинейный код, который сразу понятен, чем абстракция «на вырост».
- **Один сервис — одна ответственность.** Не смешивай бизнес-логику, инфраструктуру и транспортный слой.
- **Имена говорят сами за себя.** Переменные, функции и пакеты должны объяснять intent без комментариев.

### Когда можно отклониться

Если упрощение ломает безопасность, корректность или делает код хрупким — оставь минимально необходимую сложность и объясни в комментарии, почему так.

## Graphify

This project uses [Graphify](https://graphify.net/) to turn the codebase into a queryable knowledge graph. Use it to navigate the 8-service Go/Rust marketplace monorepo and to make better-informed changes.

### When to consult the graph

- Before changing shared packages in `pkg/`, proto contracts in `api/proto/`, or database migrations.
- When a task spans multiple services (e.g., `order-service` → `inventory-service` → `payment-service`).
- To understand the blast radius of a refactor.
- To find existing patterns similar to the code you are about to write.

### How to use Graphify

1. **Offline code-only graph (default, no API key):**
   ```bash
   ./scripts/graphify.sh
   ```
   Rebuilds `graphify-out/graph.json`, `graph.html`, and `GRAPH_REPORT.md` from code only.

2. **Semantic extraction (inside an AI assistant, requires API key):**
   ```bash
   ./scripts/graphify.sh --semantic
   ```

3. **Graphify slash commands (if available in your assistant):**
   - `/graphify query "which service calls PaymentClient?"`
   - `/graphify path "order-service" "payment-service"`
   - `/graphify explain "pkg/auth/jwt"`

4. **MCP tools (if the Graphify MCP server is connected):**
   - `query_graph`
   - `get_node`
   - `get_neighbors`
   - `shortest_path`
   - `graph_stats`

### Graph conventions

- Generated artifacts live in `graphify-out/` and are ignored by git.
- `.graphifyignore` controls what is excluded from the graph.
- CI regenerates the graph on every push to `master` and uploads it as a workflow artifact.
