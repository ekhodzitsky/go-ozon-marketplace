# Graphify integration design

## Goal

Add [Graphify](https://graphify.net/) — an open-source code-intelligence / knowledge-graph tool — to the project so AI coding assistants and human developers can navigate the multi-service Go marketplace codebase more effectively.

Graphify turns the repository (source code, protobuf files, docs, SQL migrations) into an interactive, queryable knowledge graph (`graph.html`, `graph.json`, `GRAPH_REPORT.md`, `wiki/`) and exposes commands such as `/graphify`, `/graphify query`, `/graphify path`, `/graphify explain` for supported assistants.

## Context

`go-ozon-marketplace` is a production-grade pet marketplace built as 8 Go microservices:

- `api-gateway`, `user-service`, `catalog-service`, `order-service`, `inventory-service`, `payment-service`, `notification-service`, `analytics-service`
- Shared packages under `pkg/`
- Protobuf contracts under `api/proto/`
- Docs, migrations, Docker/Kubernetes manifests, tests

The codebase is large enough that cross-service dependency tracing (e.g., "which gRPC methods does `order-service` call?", "which migrations affect the `users` table?") is non-trivial. Graphify provides a structured graph view with explicit edges between files, symbols, configs and docs.

## Non-goals

- Do **not** add Graphify as a runtime dependency of any microservice.
- Do **not** change business logic, gRPC contracts, database schemas or API behavior.
- Do **not** host a persistent Graphify server inside the cluster.
- Do **not** require every developer to install Graphify; it remains an optional dev-tooling enhancement.

## Approaches considered

### 1. Minimal: ignore rules + documentation only

Add a `.graphifyignore` tuned for this Go monorepo and a `docs/GRAPHIFY.md` usage guide.

**Pros:**
- Fastest to implement.
- No CI changes, no generated artifacts in git.
- Developers opt-in by running `pip install graphifyy && graphify .` locally.

**Cons:**
- Graph can become stale.
- No enforcement that the ignore rules or helper script stay valid.

### 2. Recommended: ignore rules + helper script + CI validation

Add `.graphifyignore`, a `scripts/graphify.sh` wrapper, update `.gitignore` for `graphify-out/`, and a lightweight CI job that validates the ignore rules and optionally generates the graph on `main` (artifacts only, no commits).

**Pros:**
- Reproducible invocation across machines.
- CI catches broken config early.
- Generated artifacts are available as workflow artifacts without polluting the repo.
- Keeps the door open for future automation (e.g., pre-commit hook).

**Cons:**
- Adds a CI job that needs Python/Graphify setup (~30–60 s).
- Artifact storage is required if graph outputs are retained.

### 3. Advanced: Graphify MCP server integration

Run Graphify as an MCP server or REST service and expose it to the local development environment / cluster.

**Pros:**
- Deep assistant integration with queryable graph via tools.
- Team-shared graph instance possible.

**Cons:**
- Requires Python runtime in the development loop or cluster.
- Adds operational complexity (health checks, versioning, networking).
- Far beyond the current scope of a dev-tooling addition.

**Decision:** Proceed with **Approach 2** (config + helper script + CI validation). It balances immediate value, low maintenance and future extensibility.

## Target architecture

```text
project-root/
├── .graphifyignore          # exclude generated/noise files from graph extraction
├── .gitignore               # ignore graphify-out/
├── scripts/
│   ├── graphify.sh          # convenience wrapper: install + run
│   └── graphify_test.sh     # smoke tests for the wrapper
├── .github/
│   └── workflows/
│       └── graphify.yml     # validate ignore rules / generate graph artifact
└── docs/
    └── GRAPHIFY.md          # team usage guide
```

## Design decisions

1. **Exclusion rules.** `.graphifyignore` uses standard gitignore syntax to exclude generated code (`api/gen/`, `bin/`), dependencies (`vendor/`, `node_modules/`), VCS/git data, IDE files, caches, build output and test files. This keeps the graph focused on source code, proto contracts, migrations, docs and infrastructure definitions.

2. **No generated artifacts in git.** `graphify-out/` is added to `.gitignore`. The CI job uploads the outputs as workflow artifacts instead of committing them, avoiding repo bloat and merge conflicts.

3. **AST-first / offline by default.** The helper script runs `graphify extract .` for fully offline code-only extraction when no assistant/LLM is available. Inside an AI assistant session developers can still run `/graphify .` for full semantic extraction.

4. **Helper script.** `scripts/graphify.sh` checks for the `graphify` CLI, offers to install it via `pip install graphifyy`, then runs Graphify with project-local ignore rules. This ensures every developer uses the same settings.

5. **CI validation.** The GitHub Actions workflow installs Graphify, validates `.graphifyignore` syntax on PRs and generates the graph artifact on pushes to `main` using the offline `graphify extract .` command. The workflow is pinned by SHA and uses Python 3.12.

5. **Documentation.** `docs/GRAPHIFY.md` explains: what Graphify is, how to install it, how to run the helper script, how to read the outputs, and how the CI job works. It is linked from the main README under the documentation index.

## Success criteria

- `.graphifyignore` exists and excludes generated/noise directories.
- `scripts/graphify.sh` runs successfully on a clean machine with Python 3.10+ and produces `graphify-out/graph.json`.
- The generated graph contains nodes for all 8 microservices and key shared packages.
- CI workflow passes on PRs and uploads artifacts on `main`.
- `graphify-out/` is ignored by git.
- `docs/GRAPHIFY.md` is added to the docs index.

## Risks

- Graphify is a young open-source project (single maintainer, frequent releases). Pinning the install command to a known-good version range in the script reduces breakage.
- LLM-based semantic extraction can be slow/costly on first run. The helper script defaults to offline `graphify extract .`; full assistant-driven extraction is opt-in.
- Python dependency conflicts. The helper script uses a virtual environment (`venv`) when available.
- Graphify's ignore syntax is gitignore-compatible, but edge cases (e.g., nested `.gitignore` interaction) should be validated in CI.

## Dependencies to add

None inside the Go modules. External tools used by the new files:

- `python3` >= 3.10
- `pip`
- `graphifyy` Python package (installed on demand by `scripts/graphify.sh`)
- `git` (for ignore validation and hook integration)
