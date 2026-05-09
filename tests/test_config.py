from __future__ import annotations

import importlib.util
import os
from pathlib import Path

import pytest

from notion_local_ops_mcp import config


def _load_main_claude_module():
    script_path = Path(__file__).resolve().parents[1] / "scripts" / "main-claude.py"
    spec = importlib.util.spec_from_file_location("main_claude_for_tests", script_path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_main_claude_load_env_file_prefers_env_file_over_existing_env(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    module = _load_main_claude_module()
    env_file = tmp_path / ".env"
    env_file.write_text(
        'NOTION_LOCAL_OPS_WORKSPACE_ROOT="C:/Users/Long/Desktop/folder/Code/notion"\n',
        encoding="utf-8",
    )

    monkeypatch.setattr(module, "ENV_FILE", env_file)
    monkeypatch.setenv(
        "NOTION_LOCAL_OPS_WORKSPACE_ROOT",
        "C:/Users/Long/Desktop/folder/Code/z.ai",
    )

    module.load_env_file()

    assert os.environ["NOTION_LOCAL_OPS_WORKSPACE_ROOT"] == (
        "C:/Users/Long/Desktop/folder/Code/notion"
    )


def test_ensure_runtime_directories_requires_existing_workspace_root(
    tmp_path, monkeypatch: pytest.MonkeyPatch
) -> None:
    workspace_root = tmp_path / "missing-workspace"
    state_dir = tmp_path / "state"

    monkeypatch.setattr(config, "WORKSPACE_ROOT", workspace_root)
    monkeypatch.setattr(config, "STATE_DIR", state_dir)

    with pytest.raises(FileNotFoundError):
        config.ensure_runtime_directories()

    assert workspace_root.exists() is False
    assert state_dir.exists() is False


def test_ensure_runtime_directories_creates_state_dir_for_valid_workspace(
    tmp_path, monkeypatch: pytest.MonkeyPatch
) -> None:
    workspace_root = tmp_path / "workspace"
    state_dir = tmp_path / "state"
    workspace_root.mkdir()

    monkeypatch.setattr(config, "WORKSPACE_ROOT", workspace_root)
    monkeypatch.setattr(config, "STATE_DIR", state_dir)

    config.ensure_runtime_directories()

    assert workspace_root.is_dir() is True
    assert state_dir.is_dir() is True
