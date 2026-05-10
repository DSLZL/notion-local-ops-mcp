from __future__ import annotations

import json
import os
import tempfile
from dataclasses import asdict, dataclass, replace
from pathlib import Path


@dataclass(frozen=True)
class RuntimeState:
    manager_state: str
    server_pid: int | None
    server_healthy: bool
    ngrok_pid: int | None
    ngrok_healthy: bool
    public_url: str | None
    last_error: str | None
    previous_public_url: str | None = None


def _fsync_directory(path: Path) -> None:
    try:
        dir_fd = os.open(path, os.O_RDONLY)
    except OSError:
        return
    try:
        os.fsync(dir_fd)
    except OSError:
        # Directory fsync is platform/filesystem-dependent.
        pass
    finally:
        os.close(dir_fd)


def default_state_path(state_dir: Path) -> Path:
    return state_dir / "runtime" / "state.json"


def record_public_url(state: RuntimeState, public_url: str | None) -> tuple[RuntimeState, str | None]:
    next_url = public_url.strip() if isinstance(public_url, str) else None
    if not next_url:
        if state.public_url is None:
            return state, None
        return replace(state, public_url=None), None

    if state.public_url == next_url:
        return state, None

    previous = state.public_url
    updated = replace(state, public_url=next_url, previous_public_url=previous)
    if previous is None:
        return updated, None
    return updated, f"{previous}->{next_url}"


def save_state(path: Path, state: RuntimeState) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = json.dumps(asdict(state), ensure_ascii=False, separators=(",", ":"))
    fd, tmp_name = tempfile.mkstemp(prefix=f"{path.name}.", suffix=".tmp", dir=path.parent)
    tmp_path = Path(tmp_name)

    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())

        os.replace(tmp_path, path)
        _fsync_directory(path.parent)
    except Exception:
        try:
            tmp_path.unlink(missing_ok=True)
        except OSError:
            pass
        raise


def load_state(path: Path) -> RuntimeState:
    payload = json.loads(path.read_text(encoding="utf-8"))
    return RuntimeState(**payload)
