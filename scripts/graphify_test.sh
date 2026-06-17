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
