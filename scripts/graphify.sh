#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VENV_DIR="${REPO_ROOT}/.graphify/venv"
PYTHON_CMD=""

find_python() {
    # Prefer stable CPython versions that are known to work with graphifyy.
    for py in python3.12 python3.11 python3.10 python3.13; do
        if command -v "$py" >/dev/null 2>&1; then
            PYTHON_CMD="$py"
            return 0
        fi
    done
    # Fallback to generic python3 only if nothing better is available.
    if command -v python3 >/dev/null 2>&1; then
        PYTHON_CMD="python3"
        return 0
    fi
    return 1
}

require_python() {
    if ! find_python; then
        echo "Error: python3 is required but not installed." >&2
        exit 1
    fi
    local py_version
    py_version=$("$PYTHON_CMD" -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")')
    if [[ "$(printf '%s\n' "3.10" "$py_version" | sort -V | head -n1)" != "3.10" ]]; then
        echo "Error: python3 >= 3.10 is required (found $py_version via ${PYTHON_CMD})." >&2
        exit 1
    fi
    echo "Using Python ${py_version} (${PYTHON_CMD})" >&2
}

install_graphify() {
    echo "Graphify not found. Installing into ${VENV_DIR}..." >&2
    mkdir -p "$(dirname "$VENV_DIR")"

    # Prefer uv when available because system Pythons are often PEP 668 externally-managed
    # and cannot create a venv with ensurepip. uv handles this robustly.
    if command -v uv >/dev/null 2>&1; then
        uv venv --python "$PYTHON_CMD" "$VENV_DIR"
        uv pip install --python "$VENV_DIR/bin/python" --upgrade pip
        uv pip install --python "$VENV_DIR/bin/python" "graphifyy[mcp]>=0.8.0,<0.9.0"
    else
        "$PYTHON_CMD" -m venv "$VENV_DIR"
        "$VENV_DIR/bin/pip" install --upgrade pip
        "$VENV_DIR/bin/pip" install "graphifyy[mcp]>=0.8.0,<0.9.0"
    fi

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
        echo "Running Graphify offline code-only update (AST only, no API calls)..."
        "$graphify_bin" update . --force
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
