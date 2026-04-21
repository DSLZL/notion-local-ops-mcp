# notion-local-ops-mcp

[中文说明](./README.zh-CN.md)

Turn a Notion **MCP Agent** into a local coding agent for local files, shell, git, and delegated tasks.

![MCP Agent working in a local repo](./assets/notion/notion-handoff-chat.png)

## What This Project Does

- exposes local files, shell, git, and patch-style editing through MCP
- lets an MCP Agent work on a real local repo instead of only editing Notion pages
- supports delegated long-running tasks through local `codex` or `claude`

## Prerequisites

Install these once before you start. On Windows, run every command shown in this README inside **Git Bash** or **WSL** so the `.sh` scripts work.

| Tool | Why you need it | Check it is installed |
| --- | --- | --- |
| Python 3.11+ | runs the MCP server | `python --version` |
| Git | clones this repository | `git --version` |
| `cloudflared` | exposes your local server to Notion over HTTPS | `cloudflared --version` |

Optional (only if you want `delegate_task` to hand work off to another CLI agent):

- `codex` CLI — https://github.com/openai/codex
- `claude` CLI — https://docs.anthropic.com/claude/docs/claude-cli

You also need a Notion workspace where you can configure an **MCP Agent** with custom MCP support.

## Quick Start (5 steps)

### 1. Clone the repository

```bash
git clone https://github.com/<your-account>/notion-local-ops-mcp.git
cd notion-local-ops-mcp
```

### 2. Create your `.env` file

`.env` holds your local secrets. It is gitignored, so nothing you put there is committed.

```bash
cp .env.example .env
```

Open `.env` in your editor and fill in at least these two values:

```bash
# Absolute path to the folder the MCP agent is allowed to read and write.
#   macOS / Linux example:      /Users/you/Code/my-project
#   Windows Git Bash example:   /c/Users/you/Code/my-project
NOTION_LOCAL_OPS_WORKSPACE_ROOT="/absolute/path/to/workspace"

# A long random string. Treat it like a password — you will paste the same value into Notion.
# Tip: generate one with `openssl rand -hex 32`.
NOTION_LOCAL_OPS_AUTH_TOKEN="replace-me"
```

### 3. Start the local server + public tunnel (one command)

```bash
./scripts/dev-tunnel.sh
```

The first run will:

- create a virtualenv at `.venv/` and install Python dependencies automatically
- start the MCP server on `http://127.0.0.1:8766/mcp`
- open a `cloudflared` quick tunnel and print a public HTTPS URL similar to:

```text
https://random-words-1234.trycloudflare.com
```

**Keep this terminal open.** That printed URL is how Notion reaches your computer; if you close the terminal the connection drops.

### 4. Add the server to a Notion MCP Agent

In Notion, open (or create) an MCP Agent and add a custom MCP server with these exact values:

| Field | Value |
| --- | --- |
| URL | the tunnel URL from step 3 with `/mcp` appended, e.g. `https://random-words-1234.trycloudflare.com/mcp` |
| Auth type | `Bearer` |
| Token | the exact value of `NOTION_LOCAL_OPS_AUTH_TOKEN` from your `.env` |

Save. Notion should load the tool list within a few seconds.

### 5. Paste the MCP Agent prompt

Copy the prompt in the next section into the MCP Agent's **prompt** field (not the Notion AI instruction page). It teaches the agent how to use the local tools.

If anything fails, jump to [Troubleshooting](#troubleshooting).

## MCP Agent Prompt

Paste the prompt below into your MCP Agent's prompt field.

<details>
<summary><strong>Recommended MCP Agent prompt</strong></summary>

```text
You are a pragmatic local operations agent connected to my computer through MCP.

Goals:
- Complete file, code, shell, and task workflows end-to-end with minimal interruption.
- Act more like a coding agent than a chat assistant.
- Stay concise, direct, and outcome-focused.

Disambiguation rules:
- If the context contains local repo paths, filenames, code extensions, README, AGENTS.md, CLAUDE.md, or .cursorrules, treat "document", "file", "notes", "instructions", and "docs" as local files unless the user explicitly says Notion page, wiki, or workspace page.
- If the user asks to edit AGENTS.md, CLAUDE.md, README, or project instructions inside the repo, edit the local file. Do not switch into self-configuration or setup behavior unless the user explicitly says to change the agent itself.
- For local file edits, do not use <edit_reference>. That is for Notion page editing, not MCP file changes.
- When answering code questions, prefer file paths, line references, function names, command output, or git diff over Notion-style citation footnotes.

Working style:
- First restate the goal in one sentence.
- Default to the current workspace root unless the target path is genuinely ambiguous.
- For non-trivial tasks, give a short plan and keep progress updated.
- Prefer direct tools first. Use delegate_task only when direct tools are not enough.
- Keep moving forward instead of asking for information that can be discovered via tools.
- If the user says fix, change, implement, deploy, update, or similar imperative requests, execute directly instead of stopping after analysis.
- If information is missing, probe with tools first. Use ask-survey only when tool probing still cannot resolve a decision and the next step is destructive or high-risk.

Tool strategy:
- list_skills: use when the user asks what skills are available in this repo or globally.
- server_info: call first when troubleshooting connection/runtime mismatches.
- set_default_cwd / get_default_cwd: set once for repeated repo operations instead of passing cwd every time.
- In coding tasks, search the local repo first. Do not default to searching the Notion workspace.
- apply_patch: use this as the default edit tool for existing files, including small edits, multi-hunk edits, moves, deletes, or adds in one patch. Use dry_run=true, validate_only=true, or return_diff=true when you want validation or a preview before writing.
- write_file: create new files or rewrite short files when that is simpler than patching; use dry_run=true for no-write preview.
- run_command_stream: start long-running shell jobs with immediate task_id return for polling progress. Prefer it for tests, installs, builds, compile steps, and other jobs that may take a while.
- get_task / wait_task: check delegated task or background command status; prefer wait_task when blocking is useful.
- run_command: proactively use for short non-destructive commands such as pwd, ls, rg, or small smoke checks.
- search: canonical query tool. mode='glob' for path discovery, mode='regex' for regex/code search, mode='text' for literal substring search. Hidden entries and .gitignore'd paths are excluded by default; regex/text search can target a single file path directly.
- list_files: inspect directory structure only when structure matters; paginate with limit and offset when needed.
- read_text: canonical single/batch file reader with line-based pagination; set include_line_numbers=true when the result will be cited or reviewed line-by-line.
- git_status / git_diff / git_commit / git_log / git_show / git_blame: use these as the default repository workflow and traceability tools only when the current cwd is actually inside a git repo.
- delegate_task: use only for complex multi-file reasoning, long-running fallback execution, or repeated failed attempts with direct tools by local codex or claude-code. For non-trivial work, pass goal, acceptance_criteria, verification_commands, and commit_mode.
- cancel_task: stop a delegated task if needed.
- purge_tasks: garbage-collect stale task artifacts under STATE_DIR/tasks (dry_run first).

Execution rules:
- When exploring a codebase, prefer search(mode='glob' or 'regex') over broad list_files calls.
- Follow the loop: probe, edit, verify, summarize.
- Do the minimum necessary read/explore work before editing.
- After each edit, re-read the changed section or run a minimal verification command when useful.
- Prefer apply_patch for edits to existing files; reserve write_file for new files or full rewrites.
- Do not issue parallel writes to the same file.
- After a logically meaningful change, inspect git_status and git_diff, then create a small focused commit instead of waiting until the end.
- Use focused commits. Do not mix unrelated changes in one commit.
- Use clear commit messages, preferably conventional commit style such as fix, feat, docs, test, refactor, or chore.
- For destructive actions such as deleting files, resetting changes, or dangerous shell commands, ask first.
- If a command or delegated task fails, summarize the root cause and adjust the approach instead of retrying blindly.

Verification rules:
- After code changes, prefer this minimum verification ladder when applicable:
- 1. Syntax or compile check such as cargo check, tsc --noEmit, python -m py_compile, or equivalent.
- 2. Focused tests for the changed area, or the nearest relevant test target.
- 3. Smoke test for the changed behavior, such as starting a service or running curl against the affected endpoint.
- Do not skip verification unless the user explicitly says not to run it.

Output style:
- Before tool use, briefly say what you are about to do.
- During longer tasks, send short progress updates.
- At the end, summarize result, verification, and any remaining risk or next step.
```

</details>

## Optional Use Case

If you also want the **Notion AI instruction page + project-management** workflow, see:

- [Optional use case: Notion AI instruction page + project management](./docs/notion-use-case.md)
- [可选应用场景：Notion AI 页面级指令 + 项目管理](./docs/notion-use-case.zh-CN.md)

## Manual Install (alternative to `dev-tunnel.sh`)

Use this path only if you want to run each piece yourself. If `./scripts/dev-tunnel.sh` from [Quick Start](#quick-start-5-steps) works for you, skip this section.

### 1. Create a virtualenv and install the package

```bash
python3.11 -m venv .venv
source .venv/bin/activate         # Windows Git Bash: source .venv/Scripts/activate
pip install -e ".[dev]"
```

### 2. Create and edit `.env`

Same as step 2 of [Quick Start](#quick-start-5-steps). The required keys are:

```bash
NOTION_LOCAL_OPS_WORKSPACE_ROOT="/absolute/path/to/workspace"
NOTION_LOCAL_OPS_AUTH_TOKEN="replace-me"
```

Optional keys (full list in [Environment Variables](#environment-variables)):

```bash
NOTION_LOCAL_OPS_CODEX_COMMAND="codex"
NOTION_LOCAL_OPS_CLAUDE_COMMAND="claude"
NOTION_LOCAL_OPS_COMMAND_TIMEOUT="120"
NOTION_LOCAL_OPS_DELEGATE_TIMEOUT="1800"
NOTION_LOCAL_OPS_GRACEFUL_SHUTDOWN_SECONDS="30"
```

### 3. Start the MCP server in the foreground

```bash
source .venv/bin/activate
notion-local-ops-mcp
```

Local endpoint:

```text
http://127.0.0.1:8766/mcp
```

## How `./scripts/dev-tunnel.sh` behaves

Good to know when you use the one-command script from [Quick Start](#quick-start-5-steps):

- it reuses or creates `.venv` and installs missing runtime dependencies
- it loads `.env` from the repo root if present
- it starts `notion-local-ops-mcp` behind a rolling-reload supervisor
- if `NOTION_LOCAL_OPS_WORKSPACE_ROOT` is unset, the script defaults it to the repo root
- if `NOTION_LOCAL_OPS_AUTH_TOKEN` is unset, the script exits with an error instead of guessing
- `.env` and `cloudflared.local.yml` are both gitignored, so your local secrets and named-tunnel config stay out of git
- it prefers `cloudflared.local.yml` or `cloudflared.local.yaml` if present; otherwise it opens a `cloudflared` quick tunnel to your local server
- you do **not** need to run `pip install` manually before using this script on a fresh clone

## Rolling Reload Without Dropping The Tunnel

Once `./scripts/dev-tunnel.sh` is already running in one terminal or tmux pane, use this from another shell:

```bash
./scripts/dev-tunnel.sh reload
```

This keeps `cloudflared` attached to the same local port while the supervisor starts a fresh MCP server process, waits for readiness, and then drains the old one. It is the recommended way to pick up code changes without causing transient 502 responses to Notion.

## Expose With cloudflared

#### Quick tunnel

```bash
cloudflared tunnel --url http://127.0.0.1:8766
```

Use the generated HTTPS URL with `/mcp`.

#### Named tunnel

Copy [`cloudflared-example.yml`](./cloudflared-example.yml) to `cloudflared.local.yml`, fill in your real values, then run:

```bash
cp cloudflared-example.yml cloudflared.local.yml
./scripts/dev-tunnel.sh
```

Or run cloudflared manually:

```bash
cloudflared tunnel --config ./cloudflared-example.yml run <your-tunnel-name>
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
| `NOTION_LOCAL_OPS_COMMAND_TIMEOUT` | no | `120` |
| `NOTION_LOCAL_OPS_DELEGATE_TIMEOUT` | no | `1800` |
| `NOTION_LOCAL_OPS_DEBUG_MCP_LOGGING` | no | `0` |
| `NOTION_LOCAL_OPS_GRACEFUL_SHUTDOWN_SECONDS` | no | `30` |

## MCP Tools

- `list_files`: list files and directories with pagination; excludes hidden/junk dirs and respects `.gitignore` by default
- `list_skills`: discover project and global skills with name and description summaries
- `search`: canonical query tool that unifies glob path search, regex grep, and literal substring search; excludes hidden and `.gitignore`d paths by default and supports regex/text search against a single file path
- `read_text`: canonical single/batch reader with line-based pagination (`start_line`/`line_limit`), optional `include_line_numbers`, and `language` hint
- `write_file`: write full file content, supports `dry_run`
- `apply_patch`: default edit tool for existing files; supports add/update/move/delete patches plus `dry_run`, `validate_only`, and optional diff output
- `server_info`: inspect runtime config and the registered MCP tool list
- `set_default_cwd`: set session default working directory for subsequent calls
- `get_default_cwd`: inspect current session/effective working directory
- `git_status`: structured repository status (use when cwd is inside a git repo)
- `git_diff`: structured diff output grouped by file with per-file truncation
- `git_commit`: stage selected paths or all changes and create a commit (`amend` / `allow_empty` / `author` / `sign_off` / `dry_run`)
- `git_log`: recent commit history
- `git_show`: inspect metadata and per-file diff for a commit/ref
- `git_blame`: line-level blame metadata for a file/range
- `run_command`: run local shell commands, optionally in background
- `run_command_stream`: start a background shell job and poll output by task id; this is the preferred route for long tests/builds/installs
- `delegate_task`: send a task to local `codex` or `claude-code`, with optional `goal`, `acceptance_criteria`, `verification_commands`, and `commit_mode`
- `get_task`: read task status and output tail
- `wait_task`: block until a delegated or background shell task completes or times out
- `cancel_task`: stop a delegated or background shell task
- `purge_tasks`: clean old task artifacts from `STATE_DIR/tasks` with dry-run support

## Debugging Notion / MCP handshake issues

If a client appears connected but hangs during initialize, tools/list, or tool calls, enable verbose MCP request logging:

```bash
NOTION_LOCAL_OPS_DEBUG_MCP_LOGGING=1 ./scripts/dev-tunnel.sh
```

When enabled, the server log includes `MCP_DEBUG` lines with:

- HTTP method and path
- session id hint
- JSON-RPC method
- tool name for `tools/call`
- truncated `arguments` summary for `tools/call`
- response status and duration

## Verify

```bash
source .venv/bin/activate
pytest -q
python -m compileall src tests
```

### Local MCP call simulation tests

Use these to simulate real MCP client/server flows locally (initialize + call_tool + wait_task):

```bash
source .venv/bin/activate
pytest -q tests/test_server_transport.py tests/test_concurrent_clients.py tests/test_mcp_local_simulation.py
```

## Troubleshooting

### Notion says it cannot connect

- Check the URL ends with `/mcp`
- Check the auth type is `Bearer`
- Check the token matches `NOTION_LOCAL_OPS_AUTH_TOKEN`
- Check `cloudflared` is still running
- If you are updating the server while users are connected, prefer `./scripts/dev-tunnel.sh reload` over killing and restarting the whole tunnel session

### MCP endpoint works locally but not over tunnel

- Retry with a named tunnel instead of a quick tunnel
- Confirm a real MCP client can list tools from `/mcp`, for example:

```bash
source .venv/bin/activate
fastmcp list http://127.0.0.1:8766/mcp
```

### Notion saw a temporary 502 while you were restarting

- A Cloudflare 502 during restart usually means the origin was briefly unavailable, not that Cloudflare blocked the request
- If this happened while you manually killed the tmux pane, switch to `./scripts/dev-tunnel.sh reload` so the supervisor overlaps the new server with the old one
- Check the newest `notion-local-ops-mcp-server.*.log` file to confirm the replacement process reached readiness before the old one drained

### Logs show repeated 404s

- If the 404 is for `GET /`, the configured URL likely missed the `/mcp` suffix
- If the 404/405 happens while using `/mcp`, upgrade to a build that serves streamable HTTP on `/mcp`

### `delegate_task` fails

- Check `codex --help`
- Check `claude --help`
- Set `NOTION_LOCAL_OPS_CODEX_COMMAND` or `NOTION_LOCAL_OPS_CLAUDE_COMMAND` if needed
