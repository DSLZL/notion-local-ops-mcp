#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ACTION="${1:-start}"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/dev-tunnel.sh            # start runtime manager
  ./scripts/dev-tunnel.sh start      # same as above
  ./scripts/dev-tunnel.sh reload     # request runtime manager reload
  ./scripts/dev-tunnel.sh status     # show runtime manager status
  ./scripts/dev-tunnel.sh --help

Environment loading order:
1. .env in the repository root
2. Current shell environment overrides matching keys

Optional:
- NOTION_LOCAL_OPS_WORKSPACE_ROOT (defaults to repo root)
- NOTION_LOCAL_OPS_HOST (defaults to 127.0.0.1)
- NOTION_LOCAL_OPS_PORT (defaults to 8766)
- NOTION_LOCAL_OPS_STATE_DIR (defaults to ~/.notion-local-ops-mcp)
- NOTION_LOCAL_OPS_AUTH_TOKEN
- NOTION_LOCAL_OPS_CODEX_COMMAND
- NOTION_LOCAL_OPS_CLAUDE_COMMAND
- NOTION_LOCAL_OPS_COMMAND_TIMEOUT
- NOTION_LOCAL_OPS_DELEGATE_TIMEOUT
- NOTION_LOCAL_OPS_DEBUG_MCP_LOGGING
- NOTION_LOCAL_OPS_GRACEFUL_SHUTDOWN_SECONDS
EOF
}

pick_python() {
  local candidate version
  for candidate in "${PYTHON_BIN:-}" python3.11 python3 python; do
    if [[ -z "${candidate}" ]] || ! command -v "${candidate}" >/dev/null 2>&1; then
      continue
    fi
    version="$("${candidate}" -c 'import sys; print(f"{sys.version_info[0]}.{sys.version_info[1]}")' 2>/dev/null || true)"
    if [[ "${version}" =~ ^3\.([0-9]+)$ ]] && [[ "${BASH_REMATCH[1]}" -ge 11 ]]; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done
  echo "Python 3.11+ is required but no suitable interpreter was found." >&2
  exit 1
}

load_env_file() {
  if [[ -f "${ROOT_DIR}/.env" ]]; then
    set -a
    # shellcheck disable=SC1091
    source "${ROOT_DIR}/.env"
    set +a
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

if [[ "${ACTION}" != "start" && "${ACTION}" != "reload" && "${ACTION}" != "status" ]]; then
  echo "Invalid action: ${ACTION}. Expected one of: start, reload, status." >&2
  usage >&2
  exit 1
fi

PYTHON_BIN="$(pick_python)"

OVERRIDE_HOST="${NOTION_LOCAL_OPS_HOST:-}"
OVERRIDE_PORT="${NOTION_LOCAL_OPS_PORT:-}"
OVERRIDE_WORKSPACE_ROOT="${NOTION_LOCAL_OPS_WORKSPACE_ROOT:-}"
OVERRIDE_STATE_DIR="${NOTION_LOCAL_OPS_STATE_DIR:-}"
OVERRIDE_AUTH_TOKEN="${NOTION_LOCAL_OPS_AUTH_TOKEN:-}"
OVERRIDE_CODEX_COMMAND="${NOTION_LOCAL_OPS_CODEX_COMMAND:-}"
OVERRIDE_CLAUDE_COMMAND="${NOTION_LOCAL_OPS_CLAUDE_COMMAND:-}"
OVERRIDE_COMMAND_TIMEOUT="${NOTION_LOCAL_OPS_COMMAND_TIMEOUT:-}"
OVERRIDE_DELEGATE_TIMEOUT="${NOTION_LOCAL_OPS_DELEGATE_TIMEOUT:-}"
OVERRIDE_DEBUG_MCP_LOGGING="${NOTION_LOCAL_OPS_DEBUG_MCP_LOGGING:-}"
OVERRIDE_GRACEFUL_SHUTDOWN_SECONDS="${NOTION_LOCAL_OPS_GRACEFUL_SHUTDOWN_SECONDS:-}"

load_env_file

export NOTION_LOCAL_OPS_HOST="${OVERRIDE_HOST:-${NOTION_LOCAL_OPS_HOST:-127.0.0.1}}"
export NOTION_LOCAL_OPS_PORT="${OVERRIDE_PORT:-${NOTION_LOCAL_OPS_PORT:-8766}}"
export NOTION_LOCAL_OPS_WORKSPACE_ROOT="${OVERRIDE_WORKSPACE_ROOT:-${NOTION_LOCAL_OPS_WORKSPACE_ROOT:-${ROOT_DIR}}}"
export NOTION_LOCAL_OPS_STATE_DIR="${OVERRIDE_STATE_DIR:-${NOTION_LOCAL_OPS_STATE_DIR:-${HOME}/.notion-local-ops-mcp}}"

if [[ -n "${OVERRIDE_AUTH_TOKEN}" ]]; then
  export NOTION_LOCAL_OPS_AUTH_TOKEN="${OVERRIDE_AUTH_TOKEN}"
fi

if [[ -n "${OVERRIDE_CODEX_COMMAND}" ]]; then
  export NOTION_LOCAL_OPS_CODEX_COMMAND="${OVERRIDE_CODEX_COMMAND}"
fi

if [[ -n "${OVERRIDE_CLAUDE_COMMAND}" ]]; then
  export NOTION_LOCAL_OPS_CLAUDE_COMMAND="${OVERRIDE_CLAUDE_COMMAND}"
fi

if [[ -n "${OVERRIDE_COMMAND_TIMEOUT}" ]]; then
  export NOTION_LOCAL_OPS_COMMAND_TIMEOUT="${OVERRIDE_COMMAND_TIMEOUT}"
fi

if [[ -n "${OVERRIDE_DELEGATE_TIMEOUT}" ]]; then
  export NOTION_LOCAL_OPS_DELEGATE_TIMEOUT="${OVERRIDE_DELEGATE_TIMEOUT}"
fi

if [[ -n "${OVERRIDE_DEBUG_MCP_LOGGING}" ]]; then
  export NOTION_LOCAL_OPS_DEBUG_MCP_LOGGING="${OVERRIDE_DEBUG_MCP_LOGGING}"
fi

if [[ -n "${OVERRIDE_GRACEFUL_SHUTDOWN_SECONDS}" ]]; then
  export NOTION_LOCAL_OPS_GRACEFUL_SHUTDOWN_SECONDS="${OVERRIDE_GRACEFUL_SHUTDOWN_SECONDS}"
fi

case "${ACTION}" in
  start)
    exec "${PYTHON_BIN}" -m notion_local_ops_mcp.runtime_manager start
    ;;
  reload)
    exec "${PYTHON_BIN}" -m notion_local_ops_mcp.runtime_manager reload
    ;;
  status)
    exec "${PYTHON_BIN}" -m notion_local_ops_mcp.runtime_manager status
    ;;
esac
