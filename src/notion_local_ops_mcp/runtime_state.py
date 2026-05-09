from __future__ import annotations

import json
from dataclasses import asdict, dataclass
from pathlib import Path


@dataclass
class RuntimeState:
    manager_state: str
    server_pid: int | None
    server_healthy: bool
    ngrok_pid: int | None
    ngrok_healthy: bool
    public_url: str | None
    last_error: str | None


def save_state(path: Path, state: RuntimeState) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp_path = path.with_name(f"{path.name}.tmp")
    payload = json.dumps(asdict(state), ensure_ascii=False, separators=(",", ":"))
    tmp_path.write_text(payload, encoding="utf-8")
    tmp_path.replace(path)


def load_state(path: Path) -> RuntimeState:
    payload = json.loads(path.read_text(encoding="utf-8"))
    return RuntimeState(**payload)
