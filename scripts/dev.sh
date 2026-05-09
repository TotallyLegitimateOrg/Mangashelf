#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HTTP_PORT="${MANGASHELF_DEV_HTTP_PORT:-3001}"
WEB_PORT="${MANGASHELF_DEV_WEB_PORT:-5173}"
EXTENSION_PORT="${MANGASHELF_DEV_EXTENSION_PORT:-38181}"

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: $1 is required but was not found in PATH" >&2
    exit 1
  fi
}

require_port_free() {
  local port="$1"
  local label="$2"

  if lsof -tiTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "error: $label port $port is already in use" >&2
    exit 1
  fi
}

cleanup() {
  trap - EXIT INT TERM

  for pid in "${PIDS[@]:-}"; do
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
    fi
  done

  for pid in "${PIDS[@]:-}"; do
    wait "$pid" 2>/dev/null || true
  done
}

wait_for_exit() {
  while true; do
    for pid in "${PIDS[@]}"; do
      if ! kill -0 "$pid" >/dev/null 2>&1; then
        wait "$pid" || return $?
      fi
    done
    sleep 1
  done
}

cd "$ROOT_DIR"

require_tool go
require_tool bun
require_tool curl
require_tool lsof

require_port_free "$HTTP_PORT" "API"
require_port_free "$WEB_PORT" "web"
require_port_free "$EXTENSION_PORT" "extension"

"$ROOT_DIR/scripts/bootstrap.sh"

trap cleanup EXIT INT TERM

echo "Starting Mangashelf dev environment"
echo "  API:       http://127.0.0.1:$HTTP_PORT"
echo "  Web:       http://127.0.0.1:$WEB_PORT"
echo "  Extension: http://127.0.0.1:$EXTENSION_PORT"

(
  cd "$ROOT_DIR/extension"
  bun run dev -- --port "$EXTENSION_PORT"
) &
extension_pid=$!

(
  cd "$ROOT_DIR/web"
  MANGASHELF_DEV_HTTP_PORT="$HTTP_PORT" \
  MANGASHELF_DEV_WEB_PORT="$WEB_PORT" \
  bun run dev
) &
web_pid=$!

(
  cd "$ROOT_DIR"
  MANGASHELF_HTTP_PORT="$HTTP_PORT" \
  MANGASHELF_DEV_WEB_URL="http://127.0.0.1:$WEB_PORT" \
  MANGASHELF_DEV_EXTENSION_URL="http://127.0.0.1:$EXTENSION_PORT" \
  go run github.com/air-verse/air@v1.65.1
) &
api_pid=$!

PIDS=("$extension_pid" "$web_pid" "$api_pid")

wait_for_exit
