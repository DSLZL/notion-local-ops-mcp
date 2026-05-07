#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/launchd-common.sh"

FIX=0
QUIET=0
RETRY_DELAY_SECONDS="${NOTION_LOCAL_OPS_DOCTOR_RETRY_DELAY_SECONDS:-5}"
LOCAL_WAIT_SECONDS="${NOTION_LOCAL_OPS_DOCTOR_LOCAL_WAIT_SECONDS:-20}"
PUBLIC_WAIT_SECONDS="${NOTION_LOCAL_OPS_DOCTOR_PUBLIC_WAIT_SECONDS:-30}"

usage() {
  cat >&2 <<'USAGE'
Usage: ./scripts/launchd-doctor.sh [--fix] [--quiet]

Checks local /mcp and the public cloudflared hostname. With --fix, restarts
only the failed launchd service after one retry:
  - local /mcp down  -> restart mcp
  - public /mcp down -> restart cloudflared, if local /mcp is healthy
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --fix)
      FIX=1
      ;;
    --quiet)
      QUIET=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      exit 2
      ;;
  esac
  shift
done

log() {
  if [[ "${QUIET}" != "1" ]]; then
    printf '%s\n' "$*"
  fi
}

warn() {
  printf '%s\n' "$*" >&2
}

prepare_launchd_env
require_command curl
require_command launchctl

MCP_LABEL="$(mcp_label)"
CLOUDFLARED_LABEL="$(cloudflared_label)"
MCP_TARGET="$(launchctl_target "${MCP_LABEL}")"
CLOUDFLARED_TARGET="$(launchctl_target "${CLOUDFLARED_LABEL}")"
LOCAL_URL="http://${NOTION_LOCAL_OPS_HOST}:${NOTION_LOCAL_OPS_PORT}/mcp"
PUBLIC_URL=""

if CLOUDFLARED_CONFIG="$(pick_cloudflared_config 2>/dev/null || true)"; then
  hostname="$(awk '
    /hostname:/ {
      for (i = 1; i <= NF; i++) {
        if ($i == "hostname:") { print $(i + 1); exit }
        if ($i ~ /^hostname:/) { sub(/^hostname:/, "", $i); print $i; exit }
      }
    }
  ' "${CLOUDFLARED_CONFIG}" 2>/dev/null || true)"
  if [[ -n "${hostname}" ]]; then
    PUBLIC_URL="https://${hostname}/mcp"
  fi
fi

service_loaded() {
  local target="$1"
  launchctl print "${target}" >/dev/null 2>&1
}

head_ok() {
  local url="$1"
  local max_time="$2"
  curl -fsSI --max-time "${max_time}" "${url}" >/dev/null 2>&1
}

wait_head_ok() {
  local url="$1"
  local max_time="$2"
  local deadline_seconds="$3"
  local start now
  start="$(date +%s)"
  while true; do
    if head_ok "${url}" "${max_time}"; then
      return 0
    fi
    now="$(date +%s)"
    if (( now - start >= deadline_seconds )); then
      return 1
    fi
    sleep 2
  done
}

restart_service() {
  local target="$1"
  local name="$2"
  warn "Restarting ${name}: ${target}"
  launchctl kickstart -k "${target}"
}

if ! service_loaded "${MCP_TARGET}"; then
  warn "MCP launchd service is not loaded: ${MCP_TARGET}"
  exit 1
fi
if ! service_loaded "${CLOUDFLARED_TARGET}"; then
  warn "cloudflared launchd service is not loaded: ${CLOUDFLARED_TARGET}"
  exit 1
fi

log "=== local MCP ==="
if head_ok "${LOCAL_URL}" 5; then
  log "OK ${LOCAL_URL}"
  local_ok=1
else
  sleep "${RETRY_DELAY_SECONDS}"
  if head_ok "${LOCAL_URL}" 5; then
    log "OK after retry ${LOCAL_URL}"
    local_ok=1
  else
    local_ok=0
    warn "Local /mcp is not reachable: ${LOCAL_URL}"
  fi
fi

if [[ "${local_ok}" != "1" ]]; then
  if [[ "${FIX}" == "1" ]]; then
    restart_service "${MCP_TARGET}" "mcp"
    if wait_head_ok "${LOCAL_URL}" 5 "${LOCAL_WAIT_SECONDS}"; then
      warn "Recovered local /mcp: ${LOCAL_URL}"
      exit 0
    fi
    warn "MCP restart did not recover local /mcp within ${LOCAL_WAIT_SECONDS}s. Check ${NOTION_LOCAL_OPS_LAUNCHD_LOG_DIR}/mcp-server.log"
  fi
  exit 1
fi

if [[ -z "${PUBLIC_URL}" ]]; then
  log "No cloudflared hostname configured; skipped public /mcp check."
  exit 0
fi

log "=== public MCP ==="
if head_ok "${PUBLIC_URL}" 10; then
  log "OK ${PUBLIC_URL}"
  exit 0
fi

sleep "${RETRY_DELAY_SECONDS}"
if head_ok "${PUBLIC_URL}" 10; then
  log "OK after retry ${PUBLIC_URL}"
  exit 0
fi

warn "Public /mcp is not reachable while local /mcp is healthy: ${PUBLIC_URL}"
if [[ "${FIX}" == "1" ]]; then
  restart_service "${CLOUDFLARED_TARGET}" "cloudflared"
  if wait_head_ok "${PUBLIC_URL}" 10 "${PUBLIC_WAIT_SECONDS}"; then
    warn "Recovered public /mcp: ${PUBLIC_URL}"
    exit 0
  fi
  warn "cloudflared restart did not recover public /mcp within ${PUBLIC_WAIT_SECONDS}s. Check ${NOTION_LOCAL_OPS_LAUNCHD_LOG_DIR}/cloudflared.stderr.log"
fi
exit 1
