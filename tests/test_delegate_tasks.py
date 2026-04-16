import time
from pathlib import Path

from notion_local_ops_mcp.executors import ExecutorRegistry
from notion_local_ops_mcp.tasks import TaskStore


def test_executor_registry_prefers_codex_when_present(tmp_path: Path) -> None:
    store = TaskStore(tmp_path / "state")
    registry = ExecutorRegistry(
        store=store,
        codex_command="python3 -c \"print('codex')\"",
        claude_command="python3 -c \"print('claude')\"",
    )

    task = registry.submit(task="say hi", executor="auto", cwd=tmp_path, timeout=5)
    loaded = store.get(task["task_id"])

    assert loaded["executor"] == "codex"


def test_task_store_persists_status_updates(tmp_path: Path) -> None:
    store = TaskStore(tmp_path / "state")
    created = store.create(task="check", executor="codex", cwd=str(tmp_path))
    store.update(created["task_id"], status="running")
    loaded = store.get(created["task_id"])

    assert loaded["status"] == "running"


def test_submitted_task_eventually_succeeds(tmp_path: Path) -> None:
    store = TaskStore(tmp_path / "state")
    registry = ExecutorRegistry(
        store=store,
        codex_command="python3 -c \"print('done')\"",
        claude_command="python3 -c \"print('claude')\"",
    )

    task = registry.submit(task="finish", executor="codex", cwd=tmp_path, timeout=5)

    for _ in range(50):
        loaded = store.get(task["task_id"])
        if loaded["status"] == "succeeded":
            break
        time.sleep(0.05)

    loaded = store.get(task["task_id"])
    assert loaded["status"] == "succeeded"
    assert "done" in store.read_stdout(task["task_id"])


def test_cancel_marks_long_running_task_cancelled(tmp_path: Path) -> None:
    store = TaskStore(tmp_path / "state")
    registry = ExecutorRegistry(
        store=store,
        codex_command="python3 -c \"import time; time.sleep(2)\"",
        claude_command="python3 -c \"print('claude')\"",
    )

    task = registry.submit(task="cancel", executor="codex", cwd=tmp_path, timeout=5)
    cancelled = registry.cancel(task["task_id"])

    assert cancelled["cancelled"] is True
