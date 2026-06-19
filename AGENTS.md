# Agent Instructions

## Graphify

This project uses [Graphify](https://graphify.net/) to turn the codebase into a queryable knowledge graph. Use it to navigate the 8-service Go marketplace monorepo and to make better-informed changes.

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
