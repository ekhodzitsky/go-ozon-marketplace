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
        echo "Error: Graphify installation completed but binary is not executable: ${VENV_DIR}/bin/graphify" >&2
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
