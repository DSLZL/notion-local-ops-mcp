#!/usr/bin/env python3
"""dev-tunnel launcher (Windows-friendly, no supervisor).

Usage:
  uv run .\\main-claude.py            # start MCP server
  uv run .\\main-claude.py start      # same as above
  uv run .\\main-claude.py status     # show endpoint status
  uv run .\\main-claude.py --help

Required tools on PATH:
  - uv

Environment loading order:
  1. Current shell environment is loaded first
  2. .env in the repository root overrides matching keys

Required for start:
  - NOTION_LOCAL_OPS_AUTH_TOKEN

Optional:
  - NOTION_LOCAL_OPS_WORKSPACE_ROOT (defaults to repo root)
  - NOTION_LOCAL_OPS_HOST (defaults to 127.0.0.1)
  - NOTION_LOCAL_OPS_PORT (defaults to 8766)
  - NOTION_LOCAL_OPS_STATE_DIR (defaults to ~/.notion-local-ops-mcp)
  - NOTION_LOCAL_OPS_DEBUG_MCP_LOGGING (set to 1/true/on to log MCP methods/tools)

Cloudflared (or any other tunneling solution) is managed manually outside of
this script. Start the MCP server here, then point your tunnel at
http://HOST:PORT yourself.
"""
from __future__ import annotations

import atexit
import os
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request
from pathlib import Path
from typing import Optional

# --------------------------------------------------------------------------- #
# Locate the repo root by walking up until we find pyproject.toml.
# --------------------------------------------------------------------------- #
def _find_repo_root() -> Path:
    override = os.environ.get("NOTION_LOCAL_OPS_REPO_ROOT")
    if override:
        p = Path(override).resolve()
        if (p / "pyproject.toml").is_file():
            return p
        print(
            f"NOTION_LOCAL_OPS_REPO_ROOT={p} does not contain pyproject.toml",
            file=sys.stderr,
        )
        sys.exit(1)

    here = Path(__file__).resolve().parent
    for candidate in (here, *here.parents):
        if (candidate / "pyproject.toml").is_file():
            return candidate

    print(
        "Could not locate pyproject.toml by walking up from "
        f"{here}. Set NOTION_LOCAL_OPS_REPO_ROOT to the project root.",
        file=sys.stderr,
    )
    sys.exit(1)

ROOT_DIR = _find_repo_root()
ENV_FILE = ROOT_DIR / ".env"
IS_WINDOWS = os.name == "nt"

# Tracked child processes so cleanup can tear them down.
_server_proc: Optional[subprocess.Popen] = None

# --------------------------------------------------------------------------- #
# Helpers
# --------------------------------------------------------------------------- #
def eprint(*args, **kwargs) -> None:
    print(*args, file=sys.stderr, **kwargs)

def usage() -> None:
    print((__doc__ or "").strip())

def require_command(name: str) -> None:
    if shutil.which(name) is None:
        eprint(f"Missing required command: {name}")
        sys.exit(1)

def load_env_file() -> None:
    if not ENV_FILE.is_file():
        return
    with ENV_FILE.open("r", encoding="utf-8") as fh:
        for raw in fh:
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("export "):
                line = line[len("export "):].lstrip()
            if "=" not in line:
                continue
            key, _, value = line.partition("=")
            key = key.strip()
            value = value.strip()
            if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
                value = value[1:-1]
            os.environ[key] = value

def get_tmp_dir() -> Path:
    return Path(os.environ.get("TMPDIR") or tempfile.gettempdir())

def wait_for_server(host: str, port: int, timeout: float = 15.0) -> bool:
    deadline = time.time() + timeout
    while time.time() < deadline:
        with socket.socket() as sock:
            sock.settimeout(0.5)
            try:
                if sock.connect_ex((host, port)) == 0:
                    return True
            except OSError:
                pass
        time.sleep(0.2)
    eprint(f"Timed out waiting for {host}:{port}")
    return False

# --------------------------------------------------------------------------- #
# uv-managed environment
# --------------------------------------------------------------------------- #
def ensure_env_with_uv() -> tuple[Path, Path]:
    """Run `uv sync`; return (venv_python_path, venv_bin_dir)."""
    venv_dir = ROOT_DIR / ".venv"
    bin_dir = venv_dir / ("Scripts" if IS_WINDOWS else "bin")
    venv_python = bin_dir / ("python.exe" if IS_WINDOWS else "python")

    subprocess.run(["uv", "sync"], cwd=str(ROOT_DIR), check=True)

    if not venv_python.exists():
        eprint(f"uv sync did not produce expected interpreter at {venv_python}")
        sys.exit(1)

    version_check = subprocess.run(
        [
            str(venv_python),
            "-c",
            "import sys; sys.exit(0 if sys.version_info >= (3, 11) else 1)",
        ]
    )
    if version_check.returncode != 0:
        eprint("Python 3.11+ is required.")
        sys.exit(1)

    os.environ["VIRTUAL_ENV"] = str(venv_dir)
    os.environ["PATH"] = f"{bin_dir}{os.pathsep}{os.environ.get('PATH', '')}"
    return venv_python, bin_dir

def resolve_server_command(bin_dir: Path, venv_python: Path) -> list[str]:
    """
    Prefer the installed CLI entry point `notion-local-ops-mcp`.
    Fall back to `python -m notion_local_ops_mcp` if the entry point isn't there.
    """
    exe_name = "notion-local-ops-mcp.exe" if IS_WINDOWS else "notion-local-ops-mcp"
    entry = bin_dir / exe_name
    if entry.exists():
        return [str(entry)]
    return [str(venv_python), "-m", "notion_local_ops_mcp"]

# --------------------------------------------------------------------------- #
# Status
# --------------------------------------------------------------------------- #
def latest_server_log() -> Optional[str]:
    try:
        logs = sorted(
            get_tmp_dir().glob("notion-local-ops-mcp-server.*.log"),
            key=lambda p: p.stat().st_mtime,
            reverse=True,
        )
    except OSError:
        return None
    return str(logs[0]) if logs else None

def print_status(host: str, port: str, server_log: Optional[str]) -> None:
    endpoint = f"http://{host}:{port}/mcp"
    print(f"Endpoint: {endpoint}")
    # Just try an HTTP HEAD — cross-platform, no curl required.
    try:
        req = urllib.request.Request(endpoint, method="HEAD")
        with urllib.request.urlopen(req, timeout=2):
            print("Local MCP endpoint is reachable")
    except Exception:
        # Fallback: raw TCP connect check
        with socket.socket() as sock:
            sock.settimeout(1.0)
            try:
                reachable = sock.connect_ex((host, int(port))) == 0
            except OSError:
                reachable = False
        print("Local MCP endpoint is reachable" if reachable
              else "Local MCP endpoint is not reachable")
    if server_log:
        print(f"Most recent server log: {server_log}")

# --------------------------------------------------------------------------- #
# Cleanup / signal handling
# --------------------------------------------------------------------------- #
def _terminate(proc: Optional[subprocess.Popen], name: str) -> None:
    if proc is None or proc.poll() is not None:
        return
    try:
        proc.terminate()
    except OSError:
        return
    try:
        proc.wait(timeout=10)
    except subprocess.TimeoutExpired:
        eprint(f"{name} did not exit in time; killing.")
        try:
            proc.kill()
        except OSError:
            pass

def cleanup() -> None:
    _terminate(_server_proc, "MCP server")

def _signal_handler(signum, _frame):
    cleanup()
    sys.exit(128 + signum)

# --------------------------------------------------------------------------- #
# Actions
# --------------------------------------------------------------------------- #
def do_start() -> int:
    global _server_proc

    atexit.register(cleanup)
    signal.signal(signal.SIGINT, _signal_handler)
    if hasattr(signal, "SIGTERM"):
        signal.signal(signal.SIGTERM, _signal_handler)

    require_command("uv")

    venv_python, bin_dir = ensure_env_with_uv()

    if not os.environ.get("NOTION_LOCAL_OPS_AUTH_TOKEN"):
        eprint(
            "Missing NOTION_LOCAL_OPS_AUTH_TOKEN. "
            "Set it in .env or export it before running."
        )
        return 1

    host = os.environ["NOTION_LOCAL_OPS_HOST"]
    port = os.environ["NOTION_LOCAL_OPS_PORT"]
    server_url = f"http://{host}:{port}"

    # Refuse to start if something is already on the port.
    with socket.socket() as sock:
        sock.settimeout(0.5)
        if sock.connect_ex((host, int(port))) == 0:
            eprint(
                f"Something is already listening on {host}:{port}. "
                "Stop it or change NOTION_LOCAL_OPS_PORT."
            )
            return 1

    tmp = get_tmp_dir()
    server_log = tmp / f"notion-local-ops-mcp-server.{os.getpid()}.log"
    server_cmd = resolve_server_command(bin_dir, venv_python)

    print("Starting notion-local-ops-mcp server...")
    print(f"Command: {' '.join(server_cmd)}")
    print(f"Server log: {server_log}")

    log_handle = server_log.open("wb")
    try:
        _server_proc = subprocess.Popen(
            server_cmd,
            cwd=str(ROOT_DIR),
            stdout=log_handle,
            stderr=log_handle,
        )
    finally:
        # The child keeps its own handle; we can close ours.
        log_handle.close()

    if not wait_for_server(host, int(port)):
        eprint("MCP server did not become ready. Recent log output:")
        try:
            with server_log.open("r", encoding="utf-8", errors="replace") as fh:
                tail = fh.readlines()[-40:]
                eprint("".join(tail))
        except OSError:
            pass
        cleanup()
        return 1

    print(f"MCP endpoint: {server_url}/mcp")
    print(f"Workspace root: {os.environ['NOTION_LOCAL_OPS_WORKSPACE_ROOT']}")
    print(f"State dir: {os.environ['NOTION_LOCAL_OPS_STATE_DIR']}")
    print(f"Server pid: {_server_proc.pid}")
    print("Press Ctrl+C to stop the server.")

    try:
        while True:
            if _server_proc.poll() is not None:
                eprint(f"MCP server exited with code {_server_proc.returncode}.")
                return _server_proc.returncode or 1
            time.sleep(0.5)
    finally:
        cleanup()

# --------------------------------------------------------------------------- #
# Main
# --------------------------------------------------------------------------- #
def main(argv: list[str]) -> int:
    os.chdir(ROOT_DIR)

    if len(argv) > 2:
        usage()
        return 1

    action = argv[1] if len(argv) == 2 else "start"

    if action in ("--help", "-h"):
        usage()
        return 0

    if action not in ("start", "status"):
        usage()
        return 1

    load_env_file()

    os.environ.setdefault("NOTION_LOCAL_OPS_HOST", "127.0.0.1")
    os.environ.setdefault("NOTION_LOCAL_OPS_PORT", "8766")
    os.environ.setdefault("NOTION_LOCAL_OPS_WORKSPACE_ROOT", str(ROOT_DIR))
    os.environ.setdefault(
        "NOTION_LOCAL_OPS_STATE_DIR",
        str(Path.home() / ".notion-local-ops-mcp"),
    )

    if action == "status":
        print_status(
            os.environ["NOTION_LOCAL_OPS_HOST"],
            os.environ["NOTION_LOCAL_OPS_PORT"],
            latest_server_log(),
        )
        return 0

    return do_start()

if __name__ == "__main__":
    try:
        sys.exit(main(sys.argv))
    except KeyboardInterrupt:
        cleanup()
        sys.exit(130)