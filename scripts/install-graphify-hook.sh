#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GRAPHIFY_BIN="${REPO_ROOT}/.graphify/venv/bin/graphify"

echo "Installing Graphify git hooks..."

if [[ ! -x "$GRAPHIFY_BIN" ]]; then
    echo "Graphify not found at ${GRAPHIFY_BIN}." >&2
    echo "Run ./scripts/graphify.sh first to install it." >&2
    exit 1
fi

cd "$REPO_ROOT"
"$GRAPHIFY_BIN" hook install

echo "Graphify hooks installed. They will rebuild graphify-out/ after relevant git operations."
echo "Note: git hooks are local to this clone and are not committed to the repository."
