from __future__ import annotations

import subprocess
from pathlib import Path


def run_command(*, command: str, cwd: Path, timeout: int) -> dict[str, object]:
    if not cwd.exists():
        return {
            "success": False,
            "error": {
                "code": "cwd_not_found",
                "message": f"Working directory not found: {cwd}",
            },
            "cwd": str(cwd),
            "command": command,
        }
    if not cwd.is_dir():
        return {
            "success": False,
            "error": {
                "code": "cwd_not_directory",
                "message": f"Working directory is not a directory: {cwd}",
            },
            "cwd": str(cwd),
            "command": command,
        }

    try:
        completed = subprocess.run(
            command,
            cwd=str(cwd),
            shell=True,
            text=True,
            capture_output=True,
            timeout=timeout,
        )
        return {
            "success": completed.returncode == 0,
            "command": command,
            "cwd": str(cwd),
            "exit_code": completed.returncode,
            "stdout": completed.stdout,
            "stderr": completed.stderr,
            "timed_out": False,
        }
    except subprocess.TimeoutExpired as exc:
        return {
            "success": False,
            "command": command,
            "cwd": str(cwd),
            "exit_code": None,
            "stdout": exc.stdout or "",
            "stderr": exc.stderr or "",
            "timed_out": True,
        }
