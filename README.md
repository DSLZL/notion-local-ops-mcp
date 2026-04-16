# notion-local-ops-mcp

Use Notion AI with your local files, shell, and fallback local agents.

📖 **[Project Introduction (Notion Page)](https://www.notion.so/notion-local-ops-mcp-344b4da3979d80e8958ae3fdf1d5e4d9?source=copy_link)**


## What It Provides

- `list_files`
- `glob_files`
- `grep_files`
- `search_files`
- `read_file`
- `replace_in_file`
- `write_file`
- `apply_patch`
- `run_command`
- `delegate_task`
- `get_task`
- `wait_task`
- `cancel_task`

`delegate_task` supports local `codex` and `claude` CLIs.

## Requirements

- Python 3.11+
- `cloudflared`
- Notion Custom Agent with custom MCP support
- Optional: `codex` CLI
- Optional: `claude` CLI

## Quick Start

For a fresh clone, the shortest path is:

```bash
git clone https://github.com/<your-account>/notion-local-ops-mcp.git
cd notion-local-ops-mcp

cp .env.example .env
```

Edit `.env` and set at least:

```bash
NOTION_LOCAL_OPS_WORKSPACE_ROOT="/absolute/path/to/workspace"
NOTION_LOCAL_OPS_AUTH_TOKEN="replace-me"
```

Then run:

```bash
./scripts/dev-tunnel.sh
```

What you should expect:

- the script creates or reuses `.venv`
- the script installs missing Python dependencies automatically
- the script starts the local MCP server on `http://127.0.0.1:8766/mcp`
- the script prefers `cloudflared.local.yml` for a named tunnel
- otherwise it falls back to a `cloudflared` quick tunnel and prints a public HTTPS URL

Use the printed tunnel URL with `/mcp` appended in Notion, and use `NOTION_LOCAL_OPS_AUTH_TOKEN` as the Bearer token.

## Manual Install

```bash
git clone https://github.com/<your-account>/notion-local-ops-mcp.git
cd notion-local-ops-mcp

python3.11 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
pip install -e .
```

## Configure

If you are not using the one-command flow, copy `.env.example` to `.env` and set at least:

```bash
cp .env.example .env
NOTION_LOCAL_OPS_WORKSPACE_ROOT="/absolute/path/to/workspace"
NOTION_LOCAL_OPS_AUTH_TOKEN="replace-me"
```

Optional:

```bash
NOTION_LOCAL_OPS_CODEX_COMMAND="codex"
NOTION_LOCAL_OPS_CLAUDE_COMMAND="claude"
NOTION_LOCAL_OPS_COMMAND_TIMEOUT="30"
NOTION_LOCAL_OPS_DELEGATE_TIMEOUT="1800"
```

## Manual Start

```bash
source .venv/bin/activate
notion-local-ops-mcp
```

Local endpoint:

```text
http://127.0.0.1:8766/mcp
```

## One-Command Local Dev Tunnel

Recommended local workflow:

```bash
./scripts/dev-tunnel.sh
```

What it does:

- reuses or creates `.venv`
- installs missing runtime dependencies
- loads `.env` from the repo root if present
- starts `notion-local-ops-mcp`
- prefers `cloudflared.local.yml` or `cloudflared.local.yaml` if present
- otherwise opens a `cloudflared` quick tunnel to your local server

Notes:

- `.env` is gitignored, so your local token and workspace path stay out of git
- `cloudflared.local.yml` is gitignored, so your local named tunnel config stays out of git
- if `NOTION_LOCAL_OPS_WORKSPACE_ROOT` is unset, the script defaults it to the repo root
- if `NOTION_LOCAL_OPS_AUTH_TOKEN` is unset, the script exits with an error instead of guessing
- for a fresh clone, you do not need to run `pip install` manually before using this script

## Expose With cloudflared

### Quick tunnel

```bash
cloudflared tunnel --url http://127.0.0.1:8766
```

Use the generated HTTPS URL with `/mcp`.

### Named tunnel

Copy [`cloudflared-example.yml`](./cloudflared-example.yml) to `cloudflared.local.yml`, fill in your real values, then run:

```bash
cp cloudflared-example.yml cloudflared.local.yml
./scripts/dev-tunnel.sh
```

Or run cloudflared manually:

```bash
cloudflared tunnel --config ./cloudflared-example.yml run <your-tunnel-name>
```

## Add To Notion

Use:

- URL: `https://<your-domain-or-tunnel>/mcp`
- Auth type: `Bearer`
- Token: your `NOTION_LOCAL_OPS_AUTH_TOKEN`

Recommended agent instruction:

```text
Use direct tools first: list_files, glob_files, grep_files, read_file, replace_in_file, write_file, apply_patch, run_command.
Use search_files only for simple substring search.
Use delegate_task only for complex multi-file work, long-running tasks, or when direct tools are insufficient.
Use apply_patch for multi-change edits, file moves, file deletes, or file creation through one atomic patch.
Use run_command with run_in_background=true when a command may take longer and you want to poll with get_task or block with wait_task.
Use wait_task after delegate_task or background run_command when you want a blocking wait instead of manual polling.
```

Recommended full prompt for Notion Agent:

```text
You are an execution-oriented local operations agent connected to my computer through MCP.

Goals:
- Complete local file, code, shell, and task workflows with minimal interruption.
- Be proactive, concise, and outcome-focused.

Working style:
- First restate the goal in one sentence.
- Default to the current workspace root unless the target path is genuinely ambiguous.
- For non-trivial tasks, give a short plan and keep progress updated.
- Prefer direct tools first. Use delegate_task only for complex multi-file work, long-running tasks, or when direct tools are not enough.
- Keep moving forward instead of asking for information that can be discovered via tools.

Tool strategy:
- list_files: inspect directory structure; paginate with limit and offset when needed.
- glob_files: narrow candidate paths by pattern.
- grep_files: search code or text with regex, glob filtering, and output modes.
- search_files: use only for simple substring search.
- read_file: read relevant file sections before editing.
- replace_in_file: make small exact edits; use replace_all only when clearly intended.
- write_file: create new files or rewrite short files when needed.
- apply_patch: use for multi-hunk edits, moves, deletes, or adds in one patch.
- run_command: proactively use for non-destructive commands such as pwd, ls, rg, git status, tests, or builds; set run_in_background=true for longer jobs.
- delegate_task: use for long-running or difficult tasks that should be handed to local codex or claude.
- get_task / wait_task: check delegated task or background command status; prefer wait_task when blocking is useful.
- cancel_task: stop a delegated task if needed.

Execution rules:
- Do the minimum necessary read/explore work before editing.
- After each edit, re-read the changed section or run a minimal verification command when useful.
- For destructive actions such as deleting files, resetting changes, or dangerous shell commands, ask first.
- If a command or delegated task fails, summarize the root cause and adjust the approach instead of retrying blindly.

Output style:
- Before tool use, briefly say what you are about to do.
- During longer tasks, send short progress updates.
- At the end, summarize result, verification, and any remaining risk or next step.
```

## Environment Variables

| Variable | Required | Default |
| --- | --- | --- |
| `NOTION_LOCAL_OPS_HOST` | no | `127.0.0.1` |
| `NOTION_LOCAL_OPS_PORT` | no | `8766` |
| `NOTION_LOCAL_OPS_WORKSPACE_ROOT` | yes | home directory |
| `NOTION_LOCAL_OPS_STATE_DIR` | no | `~/.notion-local-ops-mcp` |
| `NOTION_LOCAL_OPS_AUTH_TOKEN` | no | empty |
| `NOTION_LOCAL_OPS_CLOUDFLARED_CONFIG` | no | empty |
| `NOTION_LOCAL_OPS_TUNNEL_NAME` | no | empty |
| `NOTION_LOCAL_OPS_CODEX_COMMAND` | no | `codex` |
| `NOTION_LOCAL_OPS_CLAUDE_COMMAND` | no | `claude` |
| `NOTION_LOCAL_OPS_COMMAND_TIMEOUT` | no | `30` |
| `NOTION_LOCAL_OPS_DELEGATE_TIMEOUT` | no | `1800` |

## Tool Notes

- `list_files`: list files and directories, with `limit` and `offset` pagination
- `glob_files`: find files or directories by glob pattern
- `grep_files`: advanced regex search with glob filtering and output modes
- `search_files`: simple substring search for backward compatibility
- `read_file`: read text files with offset and limit
- `replace_in_file`: replace one exact text fragment or all exact matches
- `write_file`: write full file content
- `apply_patch`: apply codex-style add/update/move/delete patches
- `run_command`: run local shell commands, optionally in background
- `delegate_task`: send a task to local `codex` or `claude`
- `get_task`: read task status and output tail
- `wait_task`: block until a delegated or background shell task completes or times out
- `cancel_task`: stop a delegated or background shell task

## Verify

```bash
source .venv/bin/activate
pytest -q
python -m compileall src tests
```

## Troubleshooting

### Notion says it cannot connect

- Check the URL ends with `/mcp`
- Check the auth type is `Bearer`
- Check the token matches `NOTION_LOCAL_OPS_AUTH_TOKEN`
- Check `cloudflared` is still running

### SSE path works locally but not over tunnel

- Retry with a named tunnel instead of a quick tunnel
- Confirm `GET /mcp` returns `text/event-stream`

### `delegate_task` fails

- Check `codex --help`
- Check `claude --help`
- Set `NOTION_LOCAL_OPS_CODEX_COMMAND` or `NOTION_LOCAL_OPS_CLAUDE_COMMAND` if needed
