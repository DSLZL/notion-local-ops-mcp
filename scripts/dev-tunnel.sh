#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ACTION="${1:-start}"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/dev-tunnel.sh            # start Go MCP server and ngrok tunnel
  ./scripts/dev-tunnel.sh start      # same as above
  ./scripts/dev-tunnel.sh status     # check local MCP endpoint and ngrok public URL
  ./scripts/dev-tunnel.sh --help

Environment loading order:
1. .env in the repository root
2. Current shell environment fills keys not set in .env

Required for start:
- Go toolchain (`go`)
- ngrok CLI (`ngrok`)
- NOTION_LOCAL_OPS_AUTH_TOKEN

Optional:
- NOTION_LOCAL_OPS_WORKSPACE_ROOT (defaults to repo root)
- NOTION_LOCAL_OPS_HOST (defaults to 127.0.0.1)
- NOTION_LOCAL_OPS_PORT (defaults to 8766)
- NOTION_LOCAL_OPS_STATE_DIR (defaults to ~/.notion-local-ops-mcp)
- NOTION_LOCAL_OPS_AUTH_TOKEN
- NOTION_LOCAL_OPS_COMMAND_TIMEOUT
- NOTION_LOCAL_OPS_DEBUG_MCP_LOGGING
- NOTION_LOCAL_OPS_NGROK_COMMAND (defaults to ngrok)
- NOTION_LOCAL_OPS_NGROK_AUTHTOKEN (bridged to NGROK_AUTHTOKEN)
- NOTION_LOCAL_OPS_NGROK_DOMAIN
- NOTION_LOCAL_OPS_NGROK_REGION
- NOTION_LOCAL_OPS_NGROK_API_URL (defaults to http://127.0.0.1:4040/api/tunnels)
- NGROK_AUTHTOKEN
EOF
}

load_env_file() {
  if [[ -f "${ROOT_DIR}/.env" ]]; then
    set -a
    # shellcheck disable=SC1091
    source "${ROOT_DIR}/.env"
    set +a
  fi
}

require_command() {
  local name="$1"
  if ! command -v "${name}" >/dev/null 2>&1; then
    echo "Missing required command: ${name}" >&2
    exit 1
  fi
}

http_check() {
  local host="$1"
  local port="$2"
  python - "$host" "$port" <<'PY'
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])
with socket.socket() as sock:
    sock.settimeout(1.0)
    try:
        ok = sock.connect_ex((host, port)) == 0
    except OSError:
        ok = False
print("ok" if ok else "down")
PY
}

ngrok_public_url() {
  local api_url="$1"
  python - "$api_url" <<'PY'
import json
import sys
import urllib.request

api_url = sys.argv[1]
try:
    with urllib.request.urlopen(api_url, timeout=2) as response:
        payload = json.load(response)
except Exception:
    print("")
    raise SystemExit(0)

tunnels = payload.get("tunnels")
if not isinstance(tunnels, list):
    print("")
    raise SystemExit(0)

first_valid = ""
for entry in tunnels:
    if not isinstance(entry, dict):
        continue
    public_url = entry.get("public_url")
    if not isinstance(public_url, str):
        continue
    public_url = public_url.strip()
    if not public_url or "://" not in public_url:
        continue
    if not first_valid:
        first_valid = public_url
    if public_url.startswith("https://"):
        print(public_url)
        raise SystemExit(0)

print(first_valid)
PY
}

wait_for_local_server() {
  local host="$1"
  local port="$2"
  local deadline=$((SECONDS + 20))
  while (( SECONDS < deadline )); do
    if [[ "$(http_check "$host" "$port")" == "ok" ]]; then
      return 0
    fi
    sleep 1
  done
  return 1
}

wait_for_ngrok_url() {
  local api_url="$1"
  local deadline=$((SECONDS + 20))
  local url=""
  while (( SECONDS < deadline )); do
    url="$(ngrok_public_url "$api_url")"
    if [[ -n "$url" ]]; then
      printf '%s\n' "$url"
      return 0
    fi
    sleep 1
  done
  return 1
}

start() {
  require_command go
  require_command "${NOTION_LOCAL_OPS_NGROK_COMMAND}"

  if [[ -z "${NOTION_LOCAL_OPS_AUTH_TOKEN:-}" ]]; then
    echo "Missing NOTION_LOCAL_OPS_AUTH_TOKEN. Set it in .env or export it before running." >&2
    exit 1
  fi

  if [[ "$(http_check "${NOTION_LOCAL_OPS_HOST}" "${NOTION_LOCAL_OPS_PORT}")" == "ok" ]]; then
    echo "Port ${NOTION_LOCAL_OPS_PORT} is already serving on ${NOTION_LOCAL_OPS_HOST}. Stop the existing process before starting dev-tunnel." >&2
    exit 1
  fi

  local target="http://${NOTION_LOCAL_OPS_HOST}:${NOTION_LOCAL_OPS_PORT}"
  local -a ngrok_args=(http "$target")
  if [[ -n "${NOTION_LOCAL_OPS_NGROK_DOMAIN:-}" ]]; then
    ngrok_args+=(--domain "${NOTION_LOCAL_OPS_NGROK_DOMAIN}")
  fi
  if [[ -n "${NOTION_LOCAL_OPS_NGROK_REGION:-}" ]]; then
    ngrok_args+=(--region "${NOTION_LOCAL_OPS_NGROK_REGION}")
  fi

  echo "Starting Go MCP server on ${target}/mcp ..."
  go run ./main.go &
  local server_pid=$!

  cleanup() {
    local code=$?
    trap - EXIT INT TERM
    if kill -0 "${server_pid}" >/dev/null 2>&1; then
      kill "${server_pid}" >/dev/null 2>&1 || true
      wait "${server_pid}" 2>/dev/null || true
    fi
    exit "${code}"
  }
  trap cleanup EXIT INT TERM

  if ! wait_for_local_server "${NOTION_LOCAL_OPS_HOST}" "${NOTION_LOCAL_OPS_PORT}"; then
    echo "Timed out waiting for local MCP server on ${target}/mcp" >&2
    exit 1
  fi

  echo "Local MCP server is reachable on ${target}/mcp"
  echo "Starting ngrok tunnel ..."
  "${NOTION_LOCAL_OPS_NGROK_COMMAND}" "${ngrok_args[@]}" &
  local ngrok_pid=$!

  local public_url=""
  if public_url="$(wait_for_ngrok_url "${NOTION_LOCAL_OPS_NGROK_API_URL}")"; then
    echo "Public MCP URL: ${public_url}/mcp"
  else
    echo "ngrok started, but no public URL was discovered from ${NOTION_LOCAL_OPS_NGROK_API_URL} yet." >&2
    echo "Check the ngrok console output above." >&2
  fi

  wait "${ngrok_pid}"
}

status() {
  local endpoint="http://${NOTION_LOCAL_OPS_HOST}:${NOTION_LOCAL_OPS_PORT}/mcp"
  local local_status="not reachable"
  if [[ "$(http_check "${NOTION_LOCAL_OPS_HOST}" "${NOTION_LOCAL_OPS_PORT}")" == "ok" ]]; then
    local_status="reachable"
  fi

  local public_url=""
  public_url="$(ngrok_public_url "${NOTION_LOCAL_OPS_NGROK_API_URL}")"

  echo "Local MCP endpoint: ${endpoint}"
  echo "Local MCP status: ${local_status}"
  if [[ -n "${public_url}" ]]; then
    echo "ngrok public URL: ${public_url}/mcp"
  else
    echo "ngrok public URL: unavailable"
  fi
}

if [[ "${ACTION}" == "--help" || "${ACTION}" == "-h" ]]; then
  usage
  exit 0
fi

if [[ $# -gt 1 ]]; then
  echo "Invalid arguments: expected at most one action, got $#." >&2
  usage >&2
  exit 1
fi

if [[ "${ACTION}" != "start" && "${ACTION}" != "status" ]]; then
  echo "Invalid action: ${ACTION}. Expected one of: start, status." >&2
  usage >&2
  exit 1
fi

load_env_file

export NOTION_LOCAL_OPS_HOST="${NOTION_LOCAL_OPS_HOST:-127.0.0.1}"
export NOTION_LOCAL_OPS_PORT="${NOTION_LOCAL_OPS_PORT:-8766}"
export NOTION_LOCAL_OPS_WORKSPACE_ROOT="${NOTION_LOCAL_OPS_WORKSPACE_ROOT:-${ROOT_DIR}}"
export NOTION_LOCAL_OPS_STATE_DIR="${NOTION_LOCAL_OPS_STATE_DIR:-${HOME}/.notion-local-ops-mcp}"
export NOTION_LOCAL_OPS_NGROK_COMMAND="${NOTION_LOCAL_OPS_NGROK_COMMAND:-ngrok}"
export NOTION_LOCAL_OPS_NGROK_API_URL="${NOTION_LOCAL_OPS_NGROK_API_URL:-http://127.0.0.1:4040/api/tunnels}"

if [[ -n "${NOTION_LOCAL_OPS_NGROK_AUTHTOKEN:-}" ]]; then
  export NGROK_AUTHTOKEN="${NOTION_LOCAL_OPS_NGROK_AUTHTOKEN}"
fi

case "${ACTION}" in
  start)
    start
    ;;
  status)
    status
    ;;
esac
