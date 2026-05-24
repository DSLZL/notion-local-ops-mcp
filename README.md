# notion-local-ops-mcp

[中文说明](./README.zh-CN.md)

Turn a Notion **MCP Agent** into a local coding agent for local files, shell, git, and long-running task polling.

![MCP Agent working in a local repo](./assets/notion/notion-handoff-chat.png)

## What Changed

- the default runtime is now the Go server started from the repo root
- `./scripts/dev-tunnel.sh` now starts `go run ./main.go` and exposes it with `ngrok`
- the repository now uses a Go-only MCP runtime path with no Python server dependency
- supervisor / launchd / cloudflared runtime paths are no longer part of the supported setup

## Prerequisites

On Windows, run the shell snippets in **Git Bash** or **WSL**.

| Tool | Why you need it | Check |
| --- | --- | --- |
| Go | run the default MCP server | `go version` |
| ngrok | expose the local MCP endpoint over HTTPS | `ngrok version` |
| Git | clone and work in the repo | `git --version` |

## Quick Start

### 1. Clone and configure

```bash
git clone https://github.com/<your-account>/notion-local-ops-mcp.git
cd notion-local-ops-mcp
cp .env.example .env
```

Set at least:

```bash
NOTION_LOCAL_OPS_WORKSPACE_ROOT="/absolute/path/to/workspace"
NOTION_LOCAL_OPS_AUTH_TOKEN="replace-me"
```

### 2. Start the local server and tunnel

```bash
./scripts/dev-tunnel.sh
```

The script will:

- start the Go MCP server on `http://127.0.0.1:8766/mcp`
- start `ngrok http http://127.0.0.1:8766`
- print `Public MCP URL: https://.../mcp` when the tunnel is ready

Keep that terminal open while Notion is connected.

### 3. Configure the MCP Agent in Notion

- URL: `https://<your-ngrok-domain>/mcp`
- Auth type: `Bearer`
- Token: `NOTION_LOCAL_OPS_AUTH_TOKEN`

## Run Only `main.go`

If you only want the local service and will manage the tunnel yourself:

```bash
go run ./main.go
```

`go run ./main.go` now auto-loads the nearest ancestor `.env` from the current working directory upward. Values defined in that `.env` take precedence over same-named shell variables.

PowerShell example:

```powershell
$env:NOTION_LOCAL_OPS_HOST='127.0.0.1'
$env:NOTION_LOCAL_OPS_PORT='8766'
$env:NOTION_LOCAL_OPS_WORKSPACE_ROOT='C:\absolute\path\to\workspace'
$env:NOTION_LOCAL_OPS_STATE_DIR='C:\absolute\path\to\state-dir'
$env:NOTION_LOCAL_OPS_AUTH_TOKEN='replace-me'
go run .\main.go
```

## `dev-tunnel.sh` Contract

Supported actions:

- `./scripts/dev-tunnel.sh`
- `./scripts/dev-tunnel.sh start`
- `./scripts/dev-tunnel.sh status`

`status` reports:

- local MCP endpoint reachability
- the current public ngrok URL, if ngrok's local API is available

There is no `reload` action anymore.

## Environment Variables

Core runtime:

| Variable | Default | Purpose |
| --- | --- | --- |
| `NOTION_LOCAL_OPS_HOST` | `127.0.0.1` | bind address |
| `NOTION_LOCAL_OPS_PORT` | `8766` | bind port |
| `NOTION_LOCAL_OPS_WORKSPACE_ROOT` | `$HOME` | relative-path anchor |
| `NOTION_LOCAL_OPS_STATE_DIR` | `~/.notion-local-ops-mcp` | task metadata directory |
| `NOTION_LOCAL_OPS_AUTH_TOKEN` | empty | bearer token |

ngrok:

| Variable | Default | Purpose |
| --- | --- | --- |
| `NOTION_LOCAL_OPS_NGROK_COMMAND` | `ngrok` | ngrok CLI path |
| `NOTION_LOCAL_OPS_NGROK_AUTHTOKEN` | empty | project-local ngrok token; bridged to `NGROK_AUTHTOKEN` |
| `NOTION_LOCAL_OPS_NGROK_DOMAIN` | empty | optional reserved ngrok domain |
| `NOTION_LOCAL_OPS_NGROK_REGION` | empty | optional ngrok region |
| `NOTION_LOCAL_OPS_NGROK_API_URL` | `http://127.0.0.1:4040/api/tunnels` | local ngrok API used by `status` |

If your machine already has `NGROK_AUTHTOKEN` in the environment, `./scripts/dev-tunnel.sh` will use it only when `.env` does not define `NOTION_LOCAL_OPS_NGROK_AUTHTOKEN`. If you want the token to live in this repo's `.env`, set `NOTION_LOCAL_OPS_NGROK_AUTHTOKEN`; the script exports it to `NGROK_AUTHTOKEN` before starting ngrok.

Task / shell behavior:

| Variable | Default | Purpose |
| --- | --- | --- |
| `NOTION_LOCAL_OPS_COMMAND_TIMEOUT` | `120` | foreground shell timeout in seconds |
| `NOTION_LOCAL_OPS_DEBUG_MCP_LOGGING` | `0` | verbose MCP request logging |
| `NOTION_LOCAL_OPS_EXTRA_WRITE_DIRS` | empty | extra writable directories, useful for container `/tmp` workflows |

OAuth-related variables in `.env.example` still apply if you need ChatGPT web OAuth mode. Use an HTTPS public base URL that matches your ngrok endpoint.

## Default Go Tool Surface

The default Go runtime now covers the everyday MCP surface that the old Python runtime was used for:

- `server_info`
- `list_skills`
- `set_default_cwd`
- `get_default_cwd`
- `list_files`
- `read_text`
- `write_file`
- `search`
- `apply_patch`
- `run_command`
- `run_command_stream`
- `open_shell_session`
- `get_shell_session`
- `send_shell_input`
- `read_shell_output`
- `close_shell_session`
- `wait_task`
- `get_task`
- `get_task_logs`
- `cancel_task`
- `purge_tasks`
- `git_status`
- `git_diff`
- `git_commit`
- `git_log`
- `git_show`
- `git_blame`

The repository no longer depends on a Python MCP implementation.

## Persistent Shell Sessions

Phase 1 adds a Linux-first PTY-backed shell session flow for stateful interactive work:

1. `open_shell_session` opens a persistent shell under the resolved workspace cwd.
2. `send_shell_input` writes commands into that PTY.
3. `read_shell_output` reads persisted output incrementally with `offset` / `next_offset`.
4. `get_shell_session` returns the stored session metadata and whether the session is still active.
5. `close_shell_session` terminates the PTY and marks the session closed.

This is intended to cover workflows that previously needed a long-lived terminal state, such as repeated `cd`, exported environment variables, REPLs, `nc`, `socat`, Python, or exploit tooling.

Current Phase 1 limits:

- fully supported only on Linux/container runtime
- non-Linux platforms return an explicit unsupported error
- sessions are separate from background tasks and persist under `STATE_DIR/sessions`

## Long-running Commands

Use `run_command` for short foreground commands that should finish within one MCP request.

Use `run_command_stream` for commands that may take longer or need progress/log polling. It returns a `task_id` immediately, then clients should prefer `wait_task` over busy-looping `get_task`.

### Command input guidance

If a command needs large inline input or quote-sensitive payloads, prefer request-driven stdin input over shell heredocs or complex escaping.

- `run_command` accepts `stdin`
- `run_command` also accepts `stdin_content` as an alias for clients that prefer a more explicit field name

Both fields feed the same standard-input path; use one or the other, not both.

Recommended polling flow:

1. Call `run_command_stream` and save the returned `task_id`.
2. Call `wait_task` with `last_event_seq` from the previous response.
3. If the task is still running, follow `recommended_poll_strategy` and `next_poll_after_seconds`.
4. Call `get_task` when you need the latest full task snapshot.
5. Call `get_task_logs` only when you need stdout/stderr details.

### Web chat task recovery

In a web chat session, a long-running task may outlive the current dialogue turn or even the whole conversation. Prefer `await_task` for user-facing follow-up because it keeps the server polling on behalf of the client instead of forcing short `wait_task` loops in the web chat.

If the conversation already ended, recover the task in this order: `list_recent_tasks` -> `get_task` -> `await_task` -> `get_task_logs`. User-facing follow-up messages should include the `resume_token` or the `task_id` so the next turn can reconnect to the same work without guessing.

Example `get_task_logs` request shape:

```json
{
  "name": "get_task_logs",
  "task_id": "task-123",
  "stream": "stdout",
  "offset": 0,
  "limit": 4096
}
```

`get_task_logs` returns incremental slices using `offset`, `next_offset`, and `truncated`. Missing log files for an empty stream are treated as an empty successful read.

To cancel a running task, call `cancel_task` with the same `task_id`. The task will move to `cancelled` only if execution was actually interrupted before completion.

### `write_file` content modes

`write_file` requires `path` and accepts exactly one of:

- `content` for normal text writes
- `content_base64` for byte-for-byte decoded writes sent safely through the request body

Both modes support `dry_run`. Invalid base64 and conflicting content fields return validation errors.

Python scripts can emit structured progress lines on stdout:

```text
MCP_PROGRESS {"percent": 25, "message": "Scanning workspace"}
MCP_PROGRESS {"percent": 80, "message": "Aggregating results"}
```

The Go runner parses these lines and exposes them via `progress_percent`, `progress_message`, and `event_seq`.

## Verify

Go-side checks:

```bash
$env:GOCACHE=(Join-Path (Get-Location) '.gocache'); go test ./go/internal/tools ./go/internal/taskstore ./go/internal/mcp ./go/internal/app -count=1
```

Manual smoke:

```bash
go run ./main.go
```

Then open `http://127.0.0.1:8766/mcp` or run `./scripts/dev-tunnel.sh status`.

## Container Deployment

This repo can run as a self-contained Docker container without ngrok. The container image copies only the MCP server binary and uses `/tmp` as the default in-container workspace, then starts the server with:

- `NOTION_LOCAL_OPS_WORKSPACE_ROOT=/tmp`
- `NOTION_LOCAL_OPS_STATE_DIR=/tmp/notion-local-ops-mcp`
- `HOME=/tmp`

The included [`docker-compose.yml`](./docker-compose.yml) also sets:

- `read_only: true`
- `tmpfs: /tmp`
- `NOTION_LOCAL_OPS_EXTRA_WRITE_DIRS=/tmp`
- `cap_drop: [ALL]`
- `no-new-privileges:true`

That means:

- commands execute only inside the container
- no host files are mounted
- the default workspace is the container-local `/tmp` directory
- `/tmp` stays executable for PTY shells and scratch tooling
- task state is ephemeral
- restarting or recreating the container resets all runtime state

Phase 1 runtime image also includes a minimal practical CTF baseline:

- `file`
- `strings` / `objdump` / `readelf` via `binutils`
- `gdb`
- Python packaging support
- `pwntools`
- `ROPGadget`

Build and run:

```bash
docker compose up --build -d
```

Before production use, set a real bearer token:

```yaml
environment:
  NOTION_LOCAL_OPS_AUTH_TOKEN: your-strong-token
```

The server now enforces workspace confinement for resolved paths and session cwd updates, so absolute paths outside the configured workspace root are rejected instead of being followed.

## Troubleshooting

### Notion cannot connect

- confirm `./scripts/dev-tunnel.sh` is still running
- confirm the printed public URL still ends with `/mcp`
- confirm the Notion token exactly matches `NOTION_LOCAL_OPS_AUTH_TOKEN`

### Local MCP works but public URL does not

- run `./scripts/dev-tunnel.sh status`
- confirm ngrok is still running
- if `ngrok public URL: unavailable` appears, restart the tunnel process

### Port 8766 is already in use

Stop the old process bound to `NOTION_LOCAL_OPS_HOST:NOTION_LOCAL_OPS_PORT`, then start again.
