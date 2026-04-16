from __future__ import annotations

from typing import Any

from fastmcp import FastMCP
from fastmcp.exceptions import AuthorizationError
from fastmcp.server.dependencies import get_http_request
from fastmcp.server.middleware import Middleware
import uvicorn

from .config import (
    APP_NAME,
    AUTH_TOKEN,
    CLAUDE_COMMAND,
    CODEX_COMMAND,
    COMMAND_TIMEOUT,
    DELEGATE_TIMEOUT,
    HOST,
    PORT,
    STATE_DIR,
    WORKSPACE_ROOT,
    ensure_runtime_directories,
)
from .executors import ExecutorRegistry
from .files import list_files as list_files_impl
from .files import read_file as read_file_impl
from .files import replace_in_file as replace_in_file_impl
from .files import write_file as write_file_impl
from .pathing import resolve_cwd, resolve_path
from .search import search_files as search_files_impl
from .shell import run_command as run_command_impl
from .tasks import TaskStore


def _extract_bearer_token(headers: dict[str, str]) -> str:
    authorization = headers.get("authorization", "").strip()
    if authorization.lower().startswith("bearer "):
        return authorization[7:].strip()
    return ""


class BearerAuthMiddleware(Middleware):
    async def on_request(self, context: Any, call_next: Any) -> Any:
        if not AUTH_TOKEN:
            return await call_next(context)
        request = get_http_request()
        headers = {str(key).lower(): str(value) for key, value in request.headers.items()}
        token = _extract_bearer_token(headers)
        if token != AUTH_TOKEN:
            raise AuthorizationError("Unauthorized: invalid bearer token.")
        return await call_next(context)


store = TaskStore(STATE_DIR)
registry = ExecutorRegistry(
    store=store,
    codex_command=CODEX_COMMAND,
    claude_command=CLAUDE_COMMAND,
)

mcp = FastMCP(
    APP_NAME,
    instructions=(
        "Use direct tools first for normal tasks: list/search/read/replace/write/run. "
        "Use delegate_task only when direct tools are insufficient for a complex, long-running, or multi-file task."
    ),
    middleware=[BearerAuthMiddleware()],
)


@mcp.tool(
    name="list_files",
    description="List files and directories. Use this before reading or editing when you need folder context.",
)
def list_files(path: str | None = None, recursive: bool = False, limit: int = 200) -> dict[str, object]:
    target = resolve_path(path or ".", WORKSPACE_ROOT)
    return list_files_impl(target, recursive=recursive, limit=limit)


@mcp.tool(
    name="search_files",
    description="Search text in files. Use this to locate functions, config, or strings before editing.",
)
def search_files(
    query: str,
    path: str | None = None,
    glob: str | None = None,
    limit: int = 100,
) -> dict[str, object]:
    target = resolve_path(path or ".", WORKSPACE_ROOT)
    return search_files_impl(target, query=query, glob_pattern=glob, limit=limit)


@mcp.tool(
    name="read_file",
    description="Read a text file with optional offset and limit.",
)
def read_file(path: str, offset: int | None = None, limit: int | None = None) -> dict[str, object]:
    target = resolve_path(path, WORKSPACE_ROOT)
    return read_file_impl(target, offset=offset, limit=limit, max_lines=200, max_bytes=32768)


@mcp.tool(
    name="replace_in_file",
    description="Replace one exact text fragment in a file. Prefer this over full rewrites for small edits.",
)
def replace_in_file(path: str, old_text: str, new_text: str) -> dict[str, object]:
    target = resolve_path(path, WORKSPACE_ROOT)
    return replace_in_file_impl(target, old_text=old_text, new_text=new_text)


@mcp.tool(
    name="write_file",
    description="Write full content to a file, creating parent directories when needed.",
)
def write_file(path: str, content: str) -> dict[str, object]:
    target = resolve_path(path, WORKSPACE_ROOT)
    return write_file_impl(target, content=content)


@mcp.tool(
    name="run_command",
    description="Run a local shell command. Use for tests, builds, scripts, git, or other daily tasks.",
)
def run_command(command: str, cwd: str | None = None, timeout: int | None = None) -> dict[str, object]:
    resolved_cwd = resolve_cwd(cwd, WORKSPACE_ROOT)
    return run_command_impl(
        command=command,
        cwd=resolved_cwd,
        timeout=timeout if timeout is not None else COMMAND_TIMEOUT,
    )


@mcp.tool(
    name="delegate_task",
    description=(
        "Fallback only. Use this when direct tools are insufficient for a complex, long-running, or "
        "multi-file task. Supported executors: auto, codex, claude-code."
    ),
)
def delegate_task(
    task: str,
    executor: str = "auto",
    cwd: str | None = None,
    context_files: list[str] | None = None,
    timeout: int | None = None,
) -> dict[str, object]:
    resolved_cwd = resolve_cwd(cwd, WORKSPACE_ROOT)
    return registry.submit(
        task=task,
        executor=executor,
        cwd=resolved_cwd,
        timeout=timeout if timeout is not None else DELEGATE_TIMEOUT,
        context_files=context_files,
    )


@mcp.tool(
    name="get_task",
    description="Get the current status and output tail for a delegated task.",
)
def get_task(task_id: str) -> dict[str, object]:
    return registry.get(task_id)


@mcp.tool(
    name="cancel_task",
    description="Cancel a delegated task if it is still running.",
)
def cancel_task(task_id: str) -> dict[str, object]:
    return registry.cancel(task_id)


def build_http_app():
    return mcp.http_app(
        path="/mcp",
        transport="sse",
    )


def main() -> None:
    ensure_runtime_directories()
    print(f"Starting {APP_NAME} on {HOST}:{PORT}")
    print(f"workspace_root={WORKSPACE_ROOT}")
    print(f"state_dir={STATE_DIR}")
    print("sse_path=/mcp")
    print("message_path=/messages/")
    app = build_http_app()
    uvicorn.run(app, host=HOST, port=PORT)


if __name__ == "__main__":
    main()
