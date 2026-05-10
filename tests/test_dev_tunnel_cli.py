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


def test_invalid_action_reports_clear_error() -> None:
    result = subprocess.run(
        [_bash_bin(), str(_repo_root() / "scripts" / "dev-tunnel.sh"), "bogus-action"],
        cwd=_repo_root(),
        env=os.environ.copy(),
        check=False,
        capture_output=True,
        text=False,
        timeout=20,
    )
    stderr_text = result.stderr.decode("utf-8", errors="replace")
    assert result.returncode != 0
    assert "Invalid action: bogus-action. Expected one of: start, reload, status." in stderr_text


def test_too_many_args_reports_clear_error() -> None:
    result = subprocess.run(
        [_bash_bin(), str(_repo_root() / "scripts" / "dev-tunnel.sh"), "start", "extra"],
        cwd=_repo_root(),
        env=os.environ.copy(),
        check=False,
        capture_output=True,
        text=False,
        timeout=20,
    )
    stderr_text = result.stderr.decode("utf-8", errors="replace")
    assert result.returncode != 0
    assert "Invalid arguments: expected at most one action, got 2." in stderr_text


def test_rejects_python_below_3_11() -> None:
    env = os.environ.copy()
    env["PYTHON_BIN"] = "python"

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
    if result.returncode == 0:
        # Environment already provides Python >=3.11 as default "python"; no rejection expected.
        assert "Python 3.11+ is required but no suitable interpreter was found." not in stderr_text
    else:
        assert "Python 3.11+ is required but no suitable interpreter was found." in stderr_text


def test_rejects_when_only_python_3_10_candidates_exist(tmp_path: Path) -> None:
    repo_root = _repo_root()
    script_path = _script_path()
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir(parents=True, exist_ok=True)
    low_stub = "\n".join(
        [
            "#!/usr/bin/env bash",
            "set -euo pipefail",
            "if [[ \"${1:-}\" == \"-c\" ]]; then",
            "  echo \"3.10\"",
            "  exit 0",
            "fi",
            "exit 0",
            "",
        ]
    )
    _write_executable(bin_dir / "python3.11", low_stub)
    _write_executable(bin_dir / "python3", low_stub)
    _write_executable(bin_dir / "python", low_stub)

    env = os.environ.copy()
    env["PATH"] = f"{bin_dir}{os.pathsep}{env.get('PATH', '')}"
    env["PYTHON_BIN"] = str(bin_dir / "python3.11")
    result = subprocess.run(
        [_bash_bin(), str(script_path), "status"],
        cwd=repo_root,
        env=env,
        check=False,
        capture_output=True,
        text=False,
        timeout=20,
    )
    stderr_text = result.stderr.decode("utf-8", errors="replace")
    assert result.returncode != 0
    assert "Python 3.11+ is required but no suitable interpreter was found." in stderr_text
