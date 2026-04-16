from __future__ import annotations

import json
import uuid
from datetime import UTC, datetime
from pathlib import Path


def _now() -> str:
    return datetime.now(UTC).isoformat()


class TaskStore:
    def __init__(self, root: Path) -> None:
        self.root = root
        self.root.mkdir(parents=True, exist_ok=True)

    def _task_dir(self, task_id: str) -> Path:
        return self.root / "tasks" / task_id

    def _meta_path(self, task_id: str) -> Path:
        return self._task_dir(task_id) / "meta.json"

    def _stdout_path(self, task_id: str) -> Path:
        return self._task_dir(task_id) / "stdout.log"

    def _stderr_path(self, task_id: str) -> Path:
        return self._task_dir(task_id) / "stderr.log"

    def _summary_path(self, task_id: str) -> Path:
        return self._task_dir(task_id) / "summary.txt"

    def create(
        self,
        *,
        task: str,
        executor: str,
        cwd: str,
        timeout: int | None = None,
        context_files: list[str] | None = None,
    ) -> dict[str, object]:
        task_id = uuid.uuid4().hex[:12]
        task_dir = self._task_dir(task_id)
        task_dir.mkdir(parents=True, exist_ok=True)
        payload = {
            "task_id": task_id,
            "task": task,
            "executor": executor,
            "cwd": cwd,
            "timeout": timeout,
            "context_files": context_files or [],
            "status": "queued",
            "created_at": _now(),
            "updated_at": _now(),
        }
        self._meta_path(task_id).write_text(json.dumps(payload, indent=2), encoding="utf-8")
        self._stdout_path(task_id).write_text("", encoding="utf-8")
        self._stderr_path(task_id).write_text("", encoding="utf-8")
        self._summary_path(task_id).write_text("", encoding="utf-8")
        return payload

    def get(self, task_id: str) -> dict[str, object]:
        return json.loads(self._meta_path(task_id).read_text(encoding="utf-8"))

    def update(self, task_id: str, **fields: object) -> dict[str, object]:
        payload = self.get(task_id)
        payload.update(fields)
        payload["updated_at"] = _now()
        self._meta_path(task_id).write_text(json.dumps(payload, indent=2), encoding="utf-8")
        return payload

    def write_logs(self, task_id: str, *, stdout: str, stderr: str) -> None:
        self._stdout_path(task_id).write_text(stdout, encoding="utf-8")
        self._stderr_path(task_id).write_text(stderr, encoding="utf-8")

    def write_summary(self, task_id: str, summary: str) -> None:
        self._summary_path(task_id).write_text(summary, encoding="utf-8")

    def read_stdout(self, task_id: str) -> str:
        return self._stdout_path(task_id).read_text(encoding="utf-8")

    def read_stderr(self, task_id: str) -> str:
        return self._stderr_path(task_id).read_text(encoding="utf-8")

    def read_summary(self, task_id: str) -> str:
        return self._summary_path(task_id).read_text(encoding="utf-8").strip()
