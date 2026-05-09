# Ngrok Runtime Stability Design

Date: 2026-05-10
Repo: `notion-local-ops-mcp`

## 1. Scope And Goals

This design replaces all runtime paths related to `cloudflared`, `supervisor`, and `launchd` with a cross-platform ngrok-based runtime manager while keeping the user entrypoint unchanged.

Goals:

1. Keep `./scripts/dev-tunnel.sh` as the only public operator entrypoint.
2. Remove all code, scripts, tests, config keys, and docs related to `cloudflared`, `supervisor`, and `launchd`.
3. Embed ngrok process lifecycle into project runtime orchestration (not shell-only best effort).
4. Improve stability with dual watchdog behavior:
   1. Local MCP server health and recovery.
   2. Ngrok tunnel health and recovery.
5. Preserve cross-platform behavior for Windows/macOS/Linux.

Non-goals:

1. Reserved ngrok domain support.
2. Backward compatibility for old launchd workflows.
3. Partial migration mode.

## 2. High-Level Architecture

Runtime topology becomes:

1. `scripts/dev-tunnel.sh` (thin router)
2. `python -m notion_local_ops_mcp.runtime_manager` (single orchestrator)
3. Two managed subprocess workers:
   1. MCP server worker (`notion_local_ops_mcp.server`)
   2. Ngrok worker (`ngrok http ...`)

Design principle:

1. Process isolation: server and tunnel lifecycle are independent and recover independently.
2. State-driven orchestration: manager owns status transitions and writes persistent runtime state.
3. Observability-first: every recovery action and URL change must be inspectable via status command and state file.

## 3. Component Boundaries And File Plan

### 3.1 New Modules

1. `src/notion_local_ops_mcp/runtime_manager.py`
   1. Main event loop.
   2. Signal handling and graceful shutdown.
   3. Worker restart policy with exponential backoff.
   4. State aggregation and status output.

2. `src/notion_local_ops_mcp/runtime_state.py`
   1. State schema and serialization.
   2. Atomic write/read helpers under `STATE_DIR/runtime/`.
   3. URL-change event persistence.

3. `src/notion_local_ops_mcp/tunnel_ngrok.py`
   1. Ngrok subprocess command builder.
   2. Local API probing for tunnel URL discovery.
   3. Tunnel health evaluator and failure categorization.

### 3.2 Modified Existing Files

1. `scripts/dev-tunnel.sh`
   1. Keep actions `start|reload|status`.
   2. Route actions to runtime manager.
   3. Remove cloudflared/supervisor-specific logic.

2. `src/notion_local_ops_mcp/config.py`
   1. Remove cloudflared/supervisor/launchd runtime knobs.
   2. Add ngrok + stability knobs.

3. `README.md` and `README.zh-CN.md`
   1. Replace runtime workflow docs to ngrok manager model.
   2. Remove cloudflared/launchd/supervisor sections.

4. `.env.example`
   1. Remove cloudflared keys.
   2. Add ngrok keys and stability keys.

### 3.3 Removed Files

1. `src/notion_local_ops_mcp/supervisor.py`
2. `scripts/launchd-common.sh`
3. `scripts/install-launchd.sh`
4. `scripts/uninstall-launchd.sh`
5. `scripts/launchd-status.sh`
6. `scripts/launchd-reload.sh`
7. `scripts/launchd-restart.sh`
8. `scripts/launchd-doctor.sh`
9. `tests/test_launchd_support.py`
10. `tests/test_supervisor_reload.py`
11. `cloudflared-example.yml`

## 4. Runtime State Machine

Manager-level states:

1. `STARTING`: initializing workers.
2. `RUNNING`: server healthy and ngrok healthy.
3. `DEGRADED`: one worker healthy, the other unhealthy.
4. `RECOVERING`: restart/backoff cycle in progress.
5. `STOPPING`: graceful shutdown in progress.
6. `STOPPED`: terminal after controlled stop.

Worker-level state fields:

1. `pid`
2. `healthy` (boolean)
3. `last_ok_at`
4. `last_error`
5. `restart_count`
6. `next_retry_at`

Tunnel-specific fields:

1. `public_url`
2. `previous_public_url`
3. `url_changed_at`

State persistence path:

1. `STATE_DIR/runtime/state.json`
2. `STATE_DIR/runtime/events.log` (line-oriented)

## 5. Stability Strategy

### 5.1 Dual Health Checks

Server health:

1. Probe local MCP endpoint periodically.
2. Fail only after `healthcheck_failure_threshold` consecutive failures.
3. Restart only server worker on failure.

Ngrok health:

1. Check ngrok process alive.
2. Check ngrok local API (`/api/tunnels` or equivalent) returns usable `https` URL.
3. Restart only ngrok worker on sustained failure.

### 5.2 Backoff Policy

Per-worker independent backoff:

1. Delay formula: `min(max_backoff, base * 2^k + jitter)`.
2. `k` increases on consecutive restart failures for same worker.
3. `k` resets to 0 after stable healthy window.

### 5.3 Degraded-But-Alive Behavior

1. Server healthy + tunnel unhealthy:
   1. Local operations still available.
   2. Manager reports `DEGRADED` and keeps retrying ngrok.

2. Tunnel healthy + server unhealthy:
   1. Tunnel remains up.
   2. Manager recovers server only and reports `DEGRADED`.

### 5.4 URL Change Visibility

When ngrok reconnect gives a new URL:

1. Persist old/new/timestamp in state.
2. Append explicit event log record.
3. `dev-tunnel.sh status` prints warning banner with new URL.

## 6. Command UX Contract

Keep public command unchanged:

1. `./scripts/dev-tunnel.sh start`
2. `./scripts/dev-tunnel.sh reload`
3. `./scripts/dev-tunnel.sh status`

Behavior mapping:

1. `start`: launch manager if not running.
2. `reload`: request controlled server reload through manager (no full teardown).
3. `status`: print current state, worker health, tunnel URL, last errors, last transitions.

## 7. Configuration Contract

Required:

1. `NOTION_LOCAL_OPS_AUTH_TOKEN`
2. `NOTION_LOCAL_OPS_NGROK_AUTHTOKEN`

Optional ngrok/runtime knobs:

1. `NOTION_LOCAL_OPS_NGROK_BIN` (default `ngrok`)
2. `NOTION_LOCAL_OPS_NGROK_API_ADDR` (default `127.0.0.1:4040`)
3. `NOTION_LOCAL_OPS_HEALTHCHECK_INTERVAL_SECONDS` (default `5`)
4. `NOTION_LOCAL_OPS_HEALTHCHECK_FAILURE_THRESHOLD` (default `3`)
5. `NOTION_LOCAL_OPS_RESTART_BACKOFF_BASE_SECONDS` (default `1`)
6. `NOTION_LOCAL_OPS_RESTART_BACKOFF_MAX_SECONDS` (default `60`)

Deprecated and removed variables:

1. `NOTION_LOCAL_OPS_CLOUDFLARED_CONFIG`
2. `NOTION_LOCAL_OPS_TUNNEL_NAME`
3. launchd-related env vars
4. supervisor-only reload timeout knobs that no longer apply

## 8. Testing Plan

### 8.1 Unit Tests

1. `tests/test_runtime_manager.py`
   1. State transitions for normal startup and failure paths.
   2. Independent worker restart behavior.
   3. Backoff saturation and reset.
   4. Graceful stop semantics.

2. `tests/test_tunnel_ngrok.py`
   1. Parse public URL from local API responses.
   2. Handle local API unavailable and malformed payloads.
   3. URL change detection logic.

3. `tests/test_dev_tunnel_cli.py` (new or expanded)
   1. Ensure `start|reload|status` route to manager APIs.

### 8.2 Integration Verification

1. Start manager.
2. Verify local `/mcp` reachable.
3. Verify ngrok URL discovered and exposed in status.
4. Kill server process and verify auto-recovery.
5. Kill ngrok process and verify auto-recovery.
6. Confirm state transitions `DEGRADED -> RUNNING` are persisted.

### 8.3 Cleanup Regression

1. Repo scan for `cloudflared|supervisor|launchd` should be zero in runtime paths (allowed only in migration notes if intentionally retained).
2. Docs and `.env.example` must match implemented behavior.
3. Full test suite green.

## 9. Risks And Mitigations

1. Risk: ngrok local API schema drift.
   1. Mitigation: defensive parser with explicit errors and fallback checks.

2. Risk: aggressive restart loops during unstable network.
   1. Mitigation: capped exponential backoff + jitter + failure thresholds.

3. Risk: reload semantics differ from prior supervisor behavior.
   1. Mitigation: define explicit manager reload contract and test coverage.

4. Risk: stale docs and scripts after large deletion.
   1. Mitigation: enforced grep-based cleanup gate in verification.

## 10. Acceptance Criteria

A change set is complete only if all are true:

1. `./scripts/dev-tunnel.sh` remains the user entrypoint.
2. Runtime no longer depends on cloudflared/supervisor/launchd.
3. Dual watchdog recovery works for both server and tunnel failures.
4. Status command exposes health, URL, and recent errors/transitions.
5. Tests cover new behavior and pass.
6. Documentation and env example align with runtime reality.

