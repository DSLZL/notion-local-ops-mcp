from __future__ import annotations

import json
import os
import stat
import subprocess
import shutil
from pathlib import Path


def _repo_root() -> Path:
    return Path(__file__).resolve().parents[1]


def _script_path() -> Path:
    return _repo_root() / "scripts" / "dev-tunnel.sh"


def _script_text() -> str:
    return _script_path().read_text(encoding="utf-8")


def _write_executable(path: Path, content: str) -> None:
    path.write_text(content, encoding="utf-8")
    mode = path.stat().st_mode
    path.chmod(mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)


def _shell_executable() -> str:
    bash_bin = shutil.which("bash")
    if bash_bin:
        return bash_bin
    sh_bin = shutil.which("sh")
    if sh_bin:
        return sh_bin
    raise AssertionError("No shell executable found in PATH (expected bash or sh).")


def test_start_routes_to_runtime_manager() -> None:
    script = _script_text()
    assert "start)" in script
    assert 'notion_local_ops_mcp.runtime_manager start' in script


def test_reload_routes_to_runtime_manager() -> None:
    script = _script_text()
    assert "reload)" in script
    assert 'notion_local_ops_mcp.runtime_manager reload' in script


def test_status_reads_runtime_state(tmp_path: Path) -> None:
    _run_status_state_probe(tmp_path)


def _run_status_state_probe(tmp_path: Path) -> None:
    repo_root = _repo_root()
    script_path = _script_path()
    state_dir = tmp_path / "state"
    runtime_dir = state_dir / "runtime"
    runtime_dir.mkdir(parents=True, exist_ok=True)
    (runtime_dir / "state.json").write_text(
        json.dumps({"manager_state": "running"}),
        encoding="utf-8",
    )

    stub_root = tmp_path / "stub"
    package_dir = stub_root / "notion_local_ops_mcp"
    package_dir.mkdir(parents=True, exist_ok=True)
    (package_dir / "__init__.py").write_text("", encoding="utf-8")
    (package_dir / "runtime_manager.py").write_text(
        "\n".join(
            [
                "from __future__ import annotations",
                "",
                "import json",
                "import os",
                "from pathlib import Path",
                "import sys",
                "",
                "def main() -> int:",
                "    action = sys.argv[1] if len(sys.argv) > 1 else \"\"",
                "    if action != \"status\":",
                "        print(f\"runtime-manager-stub action={action}\")",
                "        return 0",
                "    state_dir = Path(os.environ[\"NOTION_LOCAL_OPS_STATE_DIR\"])",
                "    payload = json.loads((state_dir / \"runtime\" / \"state.json\").read_text(encoding=\"utf-8\"))",
                "    manager_state = payload.get(\"manager_state\")",
                "    print(f\"runtime-manager-stub status manager_state={manager_state}\")",
                "    return 0",
                "",
                "if __name__ == \"__main__\":",
                "    raise SystemExit(main())",
                "",
            ]
        ),
        encoding="utf-8",
    )

    bin_dir = tmp_path / "bin"
    bin_dir.mkdir(parents=True, exist_ok=True)
    _write_executable(
        bin_dir / "python3.11",
        "#!/usr/bin/env bash\nexec python \"$@\"\n",
    )

    env = os.environ.copy()
    existing_pythonpath = env.get("PYTHONPATH", "")
    env["PYTHONPATH"] = (
        str(stub_root)
        if not existing_pythonpath
        else f"{stub_root}{os.pathsep}{existing_pythonpath}"
    )
    env["PATH"] = f"{bin_dir}{os.pathsep}{env.get('PATH', '')}"
    env["NOTION_LOCAL_OPS_STATE_DIR"] = str(state_dir)

    result = subprocess.run(
        [_shell_executable(), str(script_path), "status"],
        cwd=repo_root,
        env=env,
        capture_output=True,
        text=False,
        check=False,
        timeout=15,
    )
    stdout_text = result.stdout.decode("utf-8", errors="replace")
    stderr_text = result.stderr.decode("utf-8", errors="replace")
    assert result.returncode == 0, stderr_text
    assert "runtime-manager-stub status manager_state=running" in stdout_text
