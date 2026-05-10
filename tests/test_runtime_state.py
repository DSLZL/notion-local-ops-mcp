from __future__ import annotations

import os
from pathlib import Path

import pytest

from notion_local_ops_mcp.runtime_state import (
    RuntimeState,
    default_state_path,
    load_state,
    record_public_url,
    save_state,
)


def test_runtime_state_roundtrip(tmp_path) -> None:
    path = tmp_path / "runtime" / "state.json"
    state = RuntimeState(
        manager_state="running",
        server_pid=12345,
        server_healthy=True,
        ngrok_pid=23456,
        ngrok_healthy=False,
        public_url="https://demo.ngrok-free.app",
        last_error=None,
    )

    save_state(path, state)
    loaded = load_state(path)

    assert loaded == state


def test_runtime_state_overwrite_keeps_latest(tmp_path) -> None:
    path = tmp_path / "runtime" / "state.json"
    first = RuntimeState(
        manager_state="starting",
        server_pid=11111,
        server_healthy=False,
        ngrok_pid=None,
        ngrok_healthy=False,
        public_url=None,
        last_error="booting",
    )
    second = RuntimeState(
        manager_state="running",
        server_pid=22222,
        server_healthy=True,
        ngrok_pid=33333,
        ngrok_healthy=True,
        public_url="https://latest.ngrok-free.app",
        last_error=None,
    )

    save_state(path, first)
    save_state(path, second)

    assert load_state(path) == second


def test_runtime_state_failed_replace_preserves_existing(tmp_path, monkeypatch) -> None:
    path = tmp_path / "runtime" / "state.json"
    original = RuntimeState(
        manager_state="running",
        server_pid=100,
        server_healthy=True,
        ngrok_pid=200,
        ngrok_healthy=True,
        public_url="https://original.ngrok-free.app",
        last_error=None,
    )
    incoming = RuntimeState(
        manager_state="error",
        server_pid=None,
        server_healthy=False,
        ngrok_pid=None,
        ngrok_healthy=False,
        public_url=None,
        last_error="replace failed",
    )
    save_state(path, original)

    real_replace = os.replace

    def fail_replace(src, dst):
        if dst == path:
            raise OSError("simulated replace failure")
        return real_replace(src, dst)

    monkeypatch.setattr("notion_local_ops_mcp.runtime_state.os.replace", fail_replace)

    with pytest.raises(OSError, match="simulated replace failure"):
        save_state(path, incoming)

    assert load_state(path) == original


def test_runtime_state_record_public_url_tracks_previous_and_new() -> None:
    initial = RuntimeState(
        manager_state="running",
        server_pid=10,
        server_healthy=True,
        ngrok_pid=20,
        ngrok_healthy=True,
        public_url="https://old.ngrok-free.app",
        previous_public_url=None,
        last_error=None,
    )
    updated, change = record_public_url(initial, "https://new.ngrok-free.app")
    unchanged, unchanged_change = record_public_url(updated, "https://new.ngrok-free.app")

    assert change == "https://old.ngrok-free.app->https://new.ngrok-free.app"
    assert updated.previous_public_url == "https://old.ngrok-free.app"
    assert updated.public_url == "https://new.ngrok-free.app"
    assert unchanged_change is None
    assert unchanged.previous_public_url == "https://old.ngrok-free.app"


def test_runtime_state_default_path_uses_state_dir(tmp_path: Path) -> None:
    assert default_state_path(tmp_path) == tmp_path / "runtime" / "state.json"
