# AGENTS.md — notion-local-ops-mcp

## What is this?

A local MCP (Model Context Protocol) server that gives Notion AI agents the ability to operate on your local filesystem and shell. The runtime in this worktree is the **Go** server started from repo-root `main.go`, normally exposed with **ngrok** at `http://127.0.0.1:8766/mcp`.

## Architecture

```
Notion Agent ──HTTP──▶ Go MCP Server (`main.go`)
                           │
           ┌───────────────┼───────────────┐
           ▼               ▼               ▼
     Files / Search    Shell / Git      Task polling
```

### Source layout

```
main.go                    # Repo-root Go entrypoint
go/bootstrap/run.go        # Shared Go startup path
go/internal/app/           # HTTP server wiring and MCP handlers
go/internal/mcp/           # MCP transport and tool registry
go/internal/tools/         # Default Go tool implementations
scripts/dev-tunnel.sh      # Start Go server + ngrok tunnel
```

## Tools exposed

| Tool | Purpose |
|---|---|
| `server_info` | Inspect runtime config and available MCP tools |
| `set_default_cwd` / `get_default_cwd` | Manage session default working directory |
| `list_files` | List directory contents (flat or recursive) |
| `search` | Canonical unified query tool (glob/regex/text); hides hidden and `.gitignore`d paths by default and supports regex/text against a single file |
| `read_text` | Canonical single/batch text reader with line pagination and optional line numbers |
| `write_file` | Create or overwrite a file (`dry_run` supported) |
| `apply_patch` | Default edit tool for existing files; rejects pure-context hunks, requires unique matches, and supports validation/dry-run |
| `git_status` / `git_diff` / `git_commit` / `git_log` / `git_show` / `git_blame` | Structured git workflows (when cwd is actually inside a git repo) |
| `run_command` | Execute a shell command (sync or background) |
| `run_command_stream` | Start long shell command and poll via task id |
| `get_task` / `wait_task` | Poll or block on delegated/background task completion |
| `cancel_task` | Cancel a running delegated task |
| `purge_tasks` | GC old task logs under `STATE_DIR/tasks` |

## Key concepts

- **WORKSPACE_ROOT** — Relative-path anchor and default cwd only (not a sandbox boundary). Set via `NOTION_LOCAL_OPS_WORKSPACE_ROOT`; defaults to `$HOME`.
- **Bearer auth** — Optional `NOTION_LOCAL_OPS_AUTH_TOKEN`; if set, every request must include a matching `Authorization: Bearer <token>` header.
- **Task state** — Background command results are persisted under `STATE_DIR/tasks/<id>/` so `wait_task` / `get_task` can poll them.
- **Safety** — `apply_patch`/`write_file` are the public write surface. `read_text` caps output at 200 lines / 32 KB, supports optional numbered lines for evidence output, and binary files are rejected.

## Configuration (env vars)

| Variable | Default | Description |
|---|---|---|
| `NOTION_LOCAL_OPS_HOST` | `127.0.0.1` | Bind address |
| `NOTION_LOCAL_OPS_PORT` | `8766` | Bind port |
| `NOTION_LOCAL_OPS_WORKSPACE_ROOT` | `$HOME` | Root for relative path resolution |
| `NOTION_LOCAL_OPS_STATE_DIR` | `~/.notion-local-ops-mcp` | Persistent task metadata |
| `NOTION_LOCAL_OPS_AUTH_TOKEN` | *(empty)* | Bearer token (auth disabled if empty) |
| `NOTION_LOCAL_OPS_COMMAND_TIMEOUT` | `120` | Default shell command timeout (seconds) |
| `NOTION_LOCAL_OPS_DEBUG_MCP_LOGGING` | `0` | Enable verbose MCP method/tool logging for handshake/debug sessions |
| `NOTION_LOCAL_OPS_NGROK_COMMAND` | `ngrok` | ngrok CLI binary used by `scripts/dev-tunnel.sh` |
| `NOTION_LOCAL_OPS_NGROK_API_URL` | `http://127.0.0.1:4040/api/tunnels` | ngrok local API used by `status` / public URL discovery |

## Quick start

```bash
cp .env.example .env   # edit values
./scripts/dev-tunnel.sh
```

Or start only the local service:

```bash
go run ./main.go
```

## Dev

```bash
$env:GOCACHE=(Join-Path (Get-Location) '.gocache'); go test ./go/internal/tools ./go/internal/config ./go/internal/taskstore ./go/internal/mcp ./go/internal/app -count=1
```
