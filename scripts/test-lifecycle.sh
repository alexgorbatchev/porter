#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
just --justfile ./justfile setup
just --justfile ./justfile setup
if output=$(just --justfile ./justfile run </dev/null 2>&1); then
    echo "expected an interactive-terminal error" >&2
    exit 1
fi
[[ "$output" == *"requires an interactive terminal"* ]]
