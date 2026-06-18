#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VENV_DIR="${REPO_ROOT}/.graphify/venv"
GRAPH_JSON="${REPO_ROOT}/graphify-out/graph.json"

if [[ ! -x "${VENV_DIR}/bin/python" ]]; then
    echo "Graphify venv not found at ${VENV_DIR}." >&2
    echo "Run ./scripts/graphify.sh first to install Graphify." >&2
    exit 1
fi

if [[ ! -f "$GRAPH_JSON" ]]; then
    echo "Graph not found at ${GRAPH_JSON}." >&2
    echo "Run ./scripts/graphify.sh first to generate the graph." >&2
    exit 1
fi

cd "$REPO_ROOT"
exec "${VENV_DIR}/bin/python" -m graphify.serve "$GRAPH_JSON" --transport stdio
