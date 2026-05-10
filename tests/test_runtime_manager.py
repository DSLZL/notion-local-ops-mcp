from pathlib import Path

from notion_local_ops_mcp.runtime_manager import (
    MANAGER_DEGRADED,
    MANAGER_RECOVERING,
    MANAGER_STOPPING,
    RuntimeManager,
    RuntimeManagerSnapshot,
    ServerSnapshot,
    next_backoff_seconds,
)
from notion_local_ops_mcp.runtime_state import RuntimeState, load_state


def test_next_backoff_seconds_caps_at_max() -> None:
    assert (
        next_backoff_seconds(
            base=1.0,
            max_seconds=10.0,
            failures=0,
            jitter=0.0,
        )
        == 1.0
    )


def test_manager_marks_degraded_when_ngrok_unhealthy_but_server_healthy(tmp_path: Path) -> None:
    manager = RuntimeManager(state_dir=tmp_path)
    previous = RuntimeState(
        manager_state="running",
        server_pid=3001,
        server_healthy=True,
        ngrok_pid=4001,
        ngrok_healthy=True,
        public_url="https://ok.ngrok-free.app",
        last_error=None,
    )

    snapshot = RuntimeManagerSnapshot(
        server=ServerSnapshot(pid=3001, healthy=True),
        ngrok_pid=4001,
        ngrok_healthy=False,
        public_url=None,
        ngrok_error="api timeout",
    )
    result = manager.orchestrate(previous_state=previous, snapshot=snapshot)

    assert result.state.manager_state == MANAGER_DEGRADED
    assert result.state.server_pid == 3001
    assert result.restart_server is False
    assert result.restart_ngrok is True


def test_manager_recovers_ngrok_without_restarting_server(tmp_path: Path) -> None:
    manager = RuntimeManager(state_dir=tmp_path)
    previous = RuntimeState(
        manager_state="degraded",
        server_pid=8888,
        server_healthy=True,
        ngrok_pid=9999,
        ngrok_healthy=False,
        public_url=None,
        last_error="ngrok down",
    )

    recovering_snapshot = RuntimeManagerSnapshot(
        server=ServerSnapshot(pid=8888, healthy=True),
        ngrok_pid=10000,
        ngrok_healthy=True,
        public_url="https://recovered.ngrok-free.app",
        ngrok_error=None,
    )
    result = manager.orchestrate(previous_state=previous, snapshot=recovering_snapshot)

    assert result.state.manager_state in {MANAGER_RECOVERING, "running"}
    assert result.state.server_pid == 8888
    assert result.restart_server is False
    assert result.restart_ngrok is False

    persisted = load_state(tmp_path / "runtime" / "state.json")
    assert persisted.server_pid == 8888
    assert persisted.ngrok_pid == 10000
    assert persisted.public_url == "https://recovered.ngrok-free.app"


def test_manager_records_public_url_change_event(tmp_path: Path) -> None:
    manager = RuntimeManager(state_dir=tmp_path)
    previous = RuntimeState(
        manager_state="running",
        server_pid=5010,
        server_healthy=True,
        ngrok_pid=6010,
        ngrok_healthy=True,
        public_url="https://old.ngrok-free.app",
        last_error=None,
    )

    changed = RuntimeManagerSnapshot(
        server=ServerSnapshot(pid=5010, healthy=True),
        ngrok_pid=6011,
        ngrok_healthy=True,
        public_url="https://new.ngrok-free.app",
        ngrok_error=None,
    )
    result = manager.orchestrate(previous_state=previous, snapshot=changed)

    assert result.url_change_event == "https://old.ngrok-free.app->https://new.ngrok-free.app"
    assert result.state.previous_public_url == "https://old.ngrok-free.app"
    assert result.state.public_url == "https://new.ngrok-free.app"
    assert (
        next_backoff_seconds(
            base=1.0,
            max_seconds=10.0,
            failures=1,
            jitter=0.0,
        )
        == 2.0
    )
    assert (
        next_backoff_seconds(
            base=1.0,
            max_seconds=10.0,
            failures=4,
            jitter=0.0,
        )
        == 10.0
    )


def test_next_backoff_seconds_clamps_negative_inputs() -> None:
    assert (
        next_backoff_seconds(
            base=-1.0,
            max_seconds=10.0,
            failures=2,
            jitter=0.0,
        )
        == 0.0
    )
    assert (
        next_backoff_seconds(
            base=1.0,
            max_seconds=-5.0,
            failures=2,
            jitter=0.0,
        )
        == 0.0
    )
    assert (
        next_backoff_seconds(
            base=1.0,
            max_seconds=10.0,
            failures=-2,
            jitter=-3.0,
        )
        == 1.0
    )


def test_manager_reports_stopping_state_without_restarts(tmp_path: Path) -> None:
    manager = RuntimeManager(state_dir=tmp_path)
    previous = RuntimeState(
        manager_state="running",
        server_pid=7001,
        server_healthy=True,
        ngrok_pid=8001,
        ngrok_healthy=True,
        public_url="https://steady.ngrok-free.app",
        last_error=None,
    )
    stopping_snapshot = RuntimeManagerSnapshot(
        server=ServerSnapshot(pid=7001, healthy=True),
        ngrok_pid=8001,
        ngrok_healthy=True,
        public_url="https://steady.ngrok-free.app",
        ngrok_error=None,
        stopping=True,
    )
    result = manager.orchestrate(previous_state=previous, snapshot=stopping_snapshot)

    assert result.state.manager_state == MANAGER_STOPPING
    assert result.restart_server is False
    assert result.restart_ngrok is False
