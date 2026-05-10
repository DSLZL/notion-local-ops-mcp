from __future__ import annotations

import os
import stat
import subprocess
import shutil
from pathlib import Path
from typing import Callable


def _repo_root() -> Path:
    return Path(__file__).resolve().parents[1]


def _script_path() -> Path:
    return _repo_root() / "scripts" / "dev-tunnel.sh"


def _write_executable(path: Path, content: str) -> None:
    path.write_text(content, encoding="utf-8")
    mode = path.stat().st_mode
    path.chmod(mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)


def _bash_bin() -> str:
    preferred = [
        Path(r"C:\Program Files\Git\bin\bash.exe"),
        Path(r"C:\Program Files\Git\usr\bin\bash.exe"),
        Path(r"C:\msys64\usr\bin\bash.exe"),
    ]
    for candidate in preferred:
        if candidate.is_file():
            return str(candidate)
    bash_bin = shutil.which("bash")
    if not bash_bin:
        raise AssertionError("No bash executable found in PATH.")
    return bash_bin


def _run_routing_probe(tmp_path: Path, action: str) -> str:
    repo_root = _repo_root()
    script_path = _script_path()

    # Replace python with a lightweight stub that logs argv and exits 0.
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir(parents=True, exist_ok=True)
    stub_content = "\n".join(
        [
            "#!/usr/bin/env bash",
            "set -euo pipefail",
            "if [[ \"${1:-}\" == \"-c\" ]]; then",
            "  echo \"3.11\"",
            "  exit 0",
            "fi",
            "echo \"$@\" > \"${NOTION_TEST_CAPTURE}\"",
            "exit 0",
            "",
        ]
    )
    # Provide multiple candidate names used by pick_python().
    _write_executable(bin_dir / "python3.11", stub_content)
    _write_executable(bin_dir / "python3", stub_content)
    _write_executable(bin_dir / "python", stub_content)

    capture_file = tmp_path / f"{action}.txt"
    env = os.environ.copy()
    env["PATH"] = f"{bin_dir}{os.pathsep}{env.get('PATH', '')}"
    env["NOTION_TEST_CAPTURE"] = str(capture_file)

    env["PYTHON_BIN"] = str(bin_dir / "python3.11")

    result = subprocess.run(
        [_bash_bin(), str(script_path), action],
        cwd=repo_root,
        env=env,
        check=False,
        capture_output=True,
        text=False,
        timeout=20,
    )
    stderr_text = result.stderr.decode("utf-8", errors="replace")
    assert result.returncode == 0, stderr_text
    assert capture_file.exists()
    return capture_file.read_text(encoding="utf-8").strip()


def test_start_routes_to_runtime_manager_command(tmp_path: Path) -> None:
    args = _run_routing_probe(tmp_path, "start")
    assert args == "-m notion_local_ops_mcp.runtime_manager start"


def test_reload_routes_to_runtime_manager_command(tmp_path: Path) -> None:
    args = _run_routing_probe(tmp_path, "reload")
    assert args == "-m notion_local_ops_mcp.runtime_manager reload"


def test_status_routes_to_runtime_manager_command(tmp_path: Path) -> None:
    args = _run_routing_probe(tmp_path, "status")
    assert args == "-m notion_local_ops_mcp.runtime_manager status"


def test_status_routes_to_runtime_manager_real_module(tmp_path: Path) -> None:
    state_dir = tmp_path / "state"
    runtime_dir = state_dir / "runtime"
    runtime_dir.mkdir(parents=True, exist_ok=True)
    (runtime_dir / "state.json").write_text(
        '{"manager_state":"running","server_pid":1,"server_healthy":true,"ngrok_pid":2,"ngrok_healthy":true,"public_url":"https://demo.ngrok.app","last_error":null,"previous_public_url":null}',
        encoding="utf-8",
    )
    env = os.environ.copy()
    env["NOTION_LOCAL_OPS_STATE_DIR"] = str(state_dir)
    env["PYTHONPATH"] = str(_repo_root() / "src")
    result = subprocess.run(
        [_bash_bin(), str(_repo_root() / "scripts" / "dev-tunnel.sh"), "status"],
        cwd=_repo_root(),
        env=env,
        check=False,
        capture_output=True,
        text=False,
        timeout=20,
    )
    stderr_text = result.stderr.decode("utf-8", errors="replace")
    stdout_text = result.stdout.decode("utf-8", errors="replace")
    assert result.returncode == 0, stderr_text
    assert "runtime-manager status manager_state=running" in stdout_text
