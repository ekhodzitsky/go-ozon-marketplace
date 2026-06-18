# Graphify Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Graphify (code-intelligence knowledge-graph tool) configuration, helper script, CI validation and documentation to the marketplace repository.

**Architecture:** A project-local `.graphifyignore` file excludes noise directories; `scripts/graphify.sh` provides a reproducible install-and-run wrapper; a GitHub Actions workflow validates ignore rules on PRs and uploads generated graph artifacts on `main`; `docs/GRAPHIFY.md` documents usage for the team.

**Tech Stack:** Graphify (Python package `graphifyy`), Bash, GitHub Actions.

---

## File map

| File | Responsibility |
|------|----------------|
| `.graphifyignore` | Exclude generated code, dependencies, caches, VCS, IDE files and tests from graph extraction (gitignore syntax). |
| `.gitignore` | Ignore `graphify-out/` and Graphify cache/venv so generated artifacts never enter git. |
| `scripts/graphify.sh` | Convenience wrapper: check/install Graphify, create venv if needed, run offline `graphify extract .` or assistant-driven `graphify .`. |
| `.github/workflows/graphify.yml` | CI job: install Python/Graphify, validate `.graphifyignore`, generate graph artifact on `main`. |
| `docs/GRAPHIFY.md` | Team guide: what Graphify is, how to run locally, how CI works, how to read outputs. |
| `docs/INDEX.md` | Add Graphify guide to the documentation index. |
| `scripts/graphify_test.sh` | Minimal shell test that verifies the helper script is executable and handles missing CLI gracefully. |

---

### Task 1: Create Graphify ignore rules

**Files:**
- Create: `.graphifyignore`

**Context:** Graphify respects `.graphifyignore` using standard gitignore syntax. If it is missing, Graphify falls back to `.gitignore`, but we want explicit control over graph-specific exclusions.

- [ ] **Step 1: Write the ignore file**

Create `.graphifyignore`:

```gitignore
# Generated code / build output
api/gen/
bin/
dist/
*.exe
*.dll
*.so
*.dylib
scripts/scripts

# Dependencies
vendor/

# VCS / git metadata
.git/

# Project tooling caches
.omc/
.worktrees/
.graphify/venv/
.graphify/cache/

# IDE / OS
.idea/
*.iml
.vscode/
*.swp
*.swo
*~
.DS_Store
Thumbs.db

# Env / secrets
.env
.env.local

# Test artifacts
*.test
*.out
coverage.html
coverage.out
*_test.go

# Docker data
infra/docker/data/

# Graphify output (also covered by .gitignore)
graphify-out/
```

- [ ] **Step 2: Validate gitignore syntax**

Run:
```bash
git check-ignore -n -v graphify-out/graph.json api/gen/go/example.go vendor/foo.go .idea/misc.xml
```
Expected: git reports ignore rules for each path (non-zero exit is normal because `-n` lists non-ignored paths too; verify that ignored paths are marked as ignored).

- [ ] **Step 3: Commit**

```bash
git add .graphifyignore
git commit -m "feat(graphify): add project ignore rules"
```

---

### Task 2: Ignore Graphify output directory in git

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: Add ignore rules**

Append to `.gitignore`:

```gitignore
# Graphify
graphify-out/
.graphify/cache/
.graphify/venv/
```

- [ ] **Step 2: Verify the rules are active**

Run:
```bash
mkdir -p graphify-out .graphify/cache .graphify/venv
touch graphify-out/graph.json .graphify/cache/state.json .graphify/venv/bin/python
git check-ignore -v graphify-out/graph.json .graphify/cache/state.json .graphify/venv/bin/python
```
Expected: git reports the ignore rules from `.gitignore`.

- [ ] **Step 3: Clean up and commit**

```bash
rm -rf graphify-out .graphify/cache .graphify/venv
git add .gitignore
git commit -m "chore(graphify): ignore generated graph outputs, cache and venv"
```

---

### Task 3: Create helper script

**Files:**
- Create: `scripts/graphify.sh`

- [ ] **Step 1: Write the helper script**

Create `scripts/graphify.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VENV_DIR="${REPO_ROOT}/.graphify/venv"

require_python() {
    if ! command -v python3 >/dev/null 2>&1; then
        echo "Error: python3 is required but not installed." >&2
        exit 1
    fi
    local py_version
    py_version=$(python3 -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")')
    if [[ "$(printf '%s\n' "3.10" "$py_version" | sort -V | head -n1)" != "3.10" ]]; then
        echo "Error: python3 >= 3.10 is required (found $py_version)." >&2
        exit 1
    fi
}

install_graphify() {
    echo "Graphify not found. Installing into ${VENV_DIR}..." >&2
    mkdir -p "$(dirname "$VENV_DIR")"
    python3 -m venv "$VENV_DIR"
    "$VENV_DIR/bin/pip" install --upgrade pip
    # Pin a known-good version to reduce breakage from rapid upstream releases.
    "$VENV_DIR/bin/pip" install "graphifyy>=0.8.0,<0.9.0"
    if [[ ! -x "${VENV_DIR}/bin/graphify" ]]; then
        echo "Error: graphify binary not found after install." >&2
        exit 1
    fi
}

ensure_graphify() {
    if [[ -x "${VENV_DIR}/bin/graphify" ]]; then
        echo "${VENV_DIR}/bin/graphify"
        return
    fi
    if command -v graphify >/dev/null 2>&1; then
        command -v graphify
        return
    fi
    install_graphify
    echo "${VENV_DIR}/bin/graphify"
}

run_graphify() {
    local graphify_bin
    graphify_bin=$(ensure_graphify)
    cd "$REPO_ROOT"

    # Default to offline AST-only extraction for safety/cost.
    # Pass --semantic to use assistant-driven semantic extraction.
    local mode_flag="extract"
    if [[ "${1:-}" == "--semantic" ]]; then
        mode_flag=""
    fi

    if [[ -n "$mode_flag" ]]; then
        echo "Running Graphify offline extraction (AST only, no API calls)..."
        "$graphify_bin" "$mode_flag" .
    else
        echo "Running Graphify with semantic extraction..."
        "$graphify_bin" .
    fi
}

main() {
    require_python
    run_graphify "$@"
}

main "$@"
```

- [ ] **Step 2: Make executable**

Run: `chmod +x scripts/graphify.sh`

- [ ] **Step 3: Verify script parses**

Run: `bash -n scripts/graphify.sh`
Expected: exits 0.

- [ ] **Step 4: Commit**

```bash
git add scripts/graphify.sh
git commit -m "feat(graphify): add helper script for reproducible graph generation"
```

---

### Task 4: Add minimal test for helper script

**Files:**
- Create: `scripts/graphify_test.sh`

- [ ] **Step 1: Write the test**

Create `scripts/graphify_test.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GRAPHIFY_SCRIPT="${SCRIPT_DIR}/graphify.sh"

echo "Test 1: script exists and is executable"
[[ -x "$GRAPHIFY_SCRIPT" ]]

echo "Test 2: script parses without errors"
bash -n "$GRAPHIFY_SCRIPT"

echo "Test 3: script fails gracefully when python3 is missing"
if ! err=$(env PATH="" /bin/bash "$GRAPHIFY_SCRIPT" 2>&1); then
    if [[ "$err" == *"python3"* ]]; then
        echo "  -> correctly errors when python3 is unavailable"
    else
        echo "  -> ERROR: unexpected failure: $err" >&2
        exit 1
    fi
else
    echo "  -> ERROR: script should have failed without python3" >&2
    exit 1
fi

echo "All tests passed."
```

- [ ] **Step 2: Make executable and run**

Run:
```bash
chmod +x scripts/graphify_test.sh
./scripts/graphify_test.sh
```
Expected output:
```text
Test 1: script exists and is executable
Test 2: script parses without errors
Test 3: script fails gracefully when python3 is missing
  -> correctly errors when python3 is unavailable
All tests passed.
```

- [ ] **Step 3: Commit**

```bash
git add scripts/graphify_test.sh
git commit -m "test(graphify): add minimal smoke test for helper script"
```

---

### Task 5: Create GitHub Actions workflow

**Files:**
- Create: `.github/workflows/graphify.yml`

- [ ] **Step 1: Write the workflow**

Create `.github/workflows/graphify.yml`:

```yaml
name: graphify

permissions:
  contents: read

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

on:
  pull_request:
    paths:
      - '.graphifyignore'
      - 'scripts/graphify.sh'
      - 'scripts/graphify_test.sh'
      - '.github/workflows/graphify.yml'
  push:
    branches: [master]

jobs:
  validate:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      contents: read
    steps:
      - name: Checkout
        uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2

      - name: Set up Python
        uses: actions/setup-python@a26af69be951a213d495a4c3e4e4022e16d87065 # v5.6.0
        with:
          python-version: '3.12'

      - name: Check helper script syntax
        run: bash -n scripts/graphify.sh

      - name: Run helper script smoke test
        run: ./scripts/graphify_test.sh

      - name: Validate .graphifyignore syntax
        run: |
          git check-ignore -v graphify-out/graph.json .graphify/cache/state.json .idea/misc.xml vendor/foo.go

  generate:
    if: github.event_name == 'push' && github.ref == 'refs/heads/master'
    needs: validate
    runs-on: ubuntu-latest
    timeout-minutes: 15
    permissions:
      contents: read
      actions: write
    steps:
      - name: Checkout
        uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2

      - name: Set up Python
        uses: actions/setup-python@a26af69be951a213d495a4c3e4e4022e16d87065 # v5.6.0
        with:
          python-version: '3.12'

      - name: Install Graphify
        run: |
          python3 -m pip install --upgrade pip
          # PyPI package name is graphifyy (two y's) — see https://pypi.org/project/graphifyy/
          python3 -m pip install "graphifyy>=0.8.0,<0.9.0"

      - name: Generate knowledge graph (offline)
        run: graphify extract .

      - name: Upload graph artifact
        uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02 # v4.6.2
        with:
          name: graphify-out
          path: graphify-out/
          retention-days: 7
```

- [ ] **Step 2: Validate workflow syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/graphify.yml'))"` (or fallback with `uv run --with pyyaml python -c "..."` if PyYAML is unavailable).
Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/graphify.yml
git commit -m "ci(graphify): add validation and artifact generation workflow"
```

---

### Task 6: Write team documentation

**Files:**
- Create: `docs/GRAPHIFY.md`

- [ ] **Step 1: Write the guide**

Create `docs/GRAPHIFY.md`:

```markdown
# Graphify

[Graphify](https://graphify.net/) — open-source инструмент для построения knowledge graph из codebase. Он помогает быстрее ориентироваться в микросервисном Go-проекте и даёт AI-ассистентам структурированный контекст.

## Что даёт интеграция

- Интерактивная визуализация зависимостей между сервисами, пакетами и proto-контрактами.
- `graph.json` для программного доступа к графу.
- `GRAPH_REPORT.md` с кратким обзором ключевых узлов и неожиданных связей.
- `wiki/` — markdown-вики по сообществам в графе (при генерации с `--wiki`).

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

## CI

- На PR: smoke-test helper-скрипта и проверка синтаксиса `.graphifyignore`.
- На push в `main`: offline-генерация графа (`graphify extract .`) и загрузка артефактов в workflow `graphify`.

## Ограничения

- Graphify — опциональный dev-tool, не добавляется в runtime сервисов.
- Семантическая (LLM) экстракция требует API-ключ и запуска через AI-ассистента или явного backend.
```

- [ ] **Step 2: Commit**

```bash
git add docs/GRAPHIFY.md
git commit -m "docs(graphify): add team usage guide"
```

---

### Task 7: Update documentation index

**Files:**
- Modify: `docs/INDEX.md`

- [ ] **Step 1: Add Graphify to the development section**

In `docs/INDEX.md`, add a row to the "Разработка" table:

```markdown
| [GRAPHIFY.md](GRAPHIFY.md) | Graphify — knowledge graph для навигации по codebase | 10 мин |
```

The table should look like:

```markdown
| Документ | Описание | Время чтения |
|----------|----------|--------------|
| [CONTRIBUTING.md](CONTRIBUTING.md) | Как участвовать в разработке | 10 мин |
| [API.md](API.md) | GraphQL, gRPC, WebSocket, JWT | 20 мин |
| [GRAPHIFY.md](GRAPHIFY.md) | Graphify — knowledge graph для навигации по codebase | 10 мин |
| [CHANGELOG](../CHANGELOG.md) | История изменений по версиям | 5 мин |
```

- [ ] **Step 2: Commit**

```bash
git add docs/INDEX.md
git commit -m "docs(graphify): add Graphify guide to docs index"
```

---

### Task 8: Final verification

**Files:**
- (no new files)

- [ ] **Step 1: Run all graphify-related checks locally**

Run:
```bash
bash -n scripts/graphify.sh
bash -n scripts/graphify_test.sh
./scripts/graphify_test.sh
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/graphify.yml'))" 2>/dev/null || uv run --with pyyaml python -c "import yaml; yaml.safe_load(open('.github/workflows/graphify.yml'))"
mkdir -p graphify-out .graphify/cache .graphify/venv
printf 'x' > scripts/scripts
git check-ignore -v graphify-out/graph.json .graphify/cache/state.json .graphify/venv/bin/python scripts/scripts
rm -rf graphify-out .graphify/cache .graphify/venv scripts/scripts
```

Expected: all commands succeed; git ignore check reports ignore rules.

- [ ] **Step 2: Commit any fixes**

If any check failed, fix the issue and commit with message like `fix(graphify): <description>`.

---

## Self-review checklist

| Spec requirement | Implementing task |
|------------------|-------------------|
| `.graphifyignore` tuned for Go monorepo | Task 1 |
| Ignore `graphify-out/`, `.graphify/cache/`, `.graphify/venv/` | Task 2 |
| Helper script for reproducible runs | Task 3 |
| Smoke test for helper script | Task 4 |
| CI validation + artifacts | Task 5 |
| Team documentation | Task 6 |
| Docs index link | Task 7 |
| Offline / AST-first default | Task 3 script, Task 5 CI |
