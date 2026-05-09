#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR"

if ! command -v bun >/dev/null 2>&1; then
  echo "error: bun is required but was not found in PATH" >&2
  exit 1
fi

bun install --frozen-lockfile
