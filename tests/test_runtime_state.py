from __future__ import annotations

from notion_local_ops_mcp.runtime_state import RuntimeState, load_state, save_state


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
