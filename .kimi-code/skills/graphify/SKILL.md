# Graphify

This project uses [Graphify](https://graphify.net/) to turn the `go-ozon-marketplace` codebase into a queryable knowledge graph. Use it to navigate the 8-service Go monorepo and make better-informed changes.

## When to use Graphify

- Before changing shared packages in `pkg/`, proto contracts in `api/proto/`, or database migrations.
- When a task spans multiple services (e.g., `order-service` → `inventory-service` → `payment-service`).
- To understand the blast radius of a refactor.
- To find existing patterns similar to the code you are about to write.
- To answer questions like "which service calls PaymentClient?" or "what depends on pkg/auth?".

## How to use Graphify in this project

### 1. Offline code-only graph (default, no API key)

Run from the project root:

```bash
./scripts/graphify.sh
```

This rebuilds:

- `graphify-out/graph.json`
- `graphify-out/graph.html`
- `graphify-out/GRAPH_REPORT.md`

### 2. Semantic extraction (requires API key)

```bash
./scripts/graphify.sh --semantic
```

Use this only when you need LLM-based semantic relationships and are running inside an assistant with an API key configured.

### 3. Query the graph with Graphify CLI

After generating the graph, you can run:

```bash
.graphify/venv/bin/graphify query "which service calls PaymentClient?"
.graphify/venv/bin/graphify path "order-service" "payment-service"
.graphify/venv/bin/graphify explain "pkg/auth/jwt"
```

### 4. MCP server (Claude Code / future Kimi Code CLI versions)

- For Claude Code: `.mcp.json` is already configured.
- For Kimi Code CLI: once `kimi mcp add` is available, run:
  ```bash
  kimi mcp add --transport stdio graphify -- ./scripts/graphify-mcp.sh
  ```

### 5. Git hooks

To keep the graph fresh automatically:

```bash
./scripts/install-graphify-hook.sh
```

## Graph conventions

- Generated artifacts live in `graphify-out/` and are ignored by git.
- `.graphifyignore` controls what is excluded from the graph.
- CI regenerates the graph on every push to `master` and uploads it as a workflow artifact.

## Expected graph contents

The graph should contain nodes for all 8 microservices:

- `api-gateway`
- `user-service`
- `catalog-service`
- `order-service`
- `inventory-service`
- `payment-service`
- `notification-service`
- `analytics-service`

If any service is missing after generation, check `.graphifyignore` and the service's file layout.
