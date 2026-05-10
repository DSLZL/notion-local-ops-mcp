from __future__ import annotations

from dataclasses import dataclass
import os
from pathlib import Path
import sys

from .runtime_state import RuntimeState, default_state_path, load_state, record_public_url, save_state

MANAGER_STARTING = "starting"
MANAGER_RUNNING = "running"
MANAGER_DEGRADED = "degraded"
MANAGER_RECOVERING = "recovering"
MANAGER_STOPPING = "stopping"


@dataclass(frozen=True, slots=True)
class ServerSnapshot:
    pid: int | None
    healthy: bool


@dataclass(frozen=True, slots=True)
class RuntimeManagerSnapshot:
    server: ServerSnapshot
    ngrok_pid: int | None
    ngrok_healthy: bool
    public_url: str | None
    ngrok_error: str | None = None
    stopping: bool = False


@dataclass(frozen=True, slots=True)
class OrchestrationResult:
    state: RuntimeState
    restart_server: bool
    restart_ngrok: bool
    url_change_event: str | None


class RuntimeManager:
    def __init__(self, *, state_dir: Path) -> None:
        self._state_path = default_state_path(state_dir)

    def orchestrate(
        self,
        *,
        previous_state: RuntimeState | None,
        snapshot: RuntimeManagerSnapshot,
    ) -> OrchestrationResult:
        manager_state, restart_server, restart_ngrok = _compute_manager_state(
            previous_state=previous_state,
            snapshot=snapshot,
        )

        next_state = RuntimeState(
            manager_state=manager_state,
            server_pid=snapshot.server.pid,
            server_healthy=snapshot.server.healthy,
            ngrok_pid=snapshot.ngrok_pid,
            ngrok_healthy=snapshot.ngrok_healthy,
            public_url=previous_state.public_url if previous_state is not None else None,
            previous_public_url=(
                previous_state.previous_public_url if previous_state is not None else None
            ),
            last_error=snapshot.ngrok_error,
        )
        next_state, url_change_event = record_public_url(next_state, snapshot.public_url)

        save_state(self._state_path, next_state)
        return OrchestrationResult(
            state=next_state,
            restart_server=restart_server,
            restart_ngrok=restart_ngrok,
            url_change_event=url_change_event,
        )


def _compute_manager_state(
    *,
    previous_state: RuntimeState | None,
    snapshot: RuntimeManagerSnapshot,
) -> tuple[str, bool, bool]:
    if snapshot.stopping:
        return MANAGER_STOPPING, False, False

    if not snapshot.server.healthy:
        return MANAGER_STARTING, True, False

    if not snapshot.ngrok_healthy:
        return MANAGER_DEGRADED, False, True

    if previous_state is not None and previous_state.manager_state == MANAGER_DEGRADED:
        return MANAGER_RECOVERING, False, False

    return MANAGER_RUNNING, False, False


def next_backoff_seconds(
    *,
    base: float,
    max_seconds: float,
    failures: int,
    jitter: float,
) -> float:
    safe_base = max(0.0, base)
    safe_max = max(0.0, max_seconds)
    raw = safe_base * (2 ** max(0, failures)) + max(0.0, jitter)
    return max(0.0, min(safe_max, raw))


def _state_dir_from_env() -> Path:
    return Path(
        os.environ.get("NOTION_LOCAL_OPS_STATE_DIR", str(Path.home() / ".notion-local-ops-mcp"))
    ).expanduser()


def main(action: str | None = None) -> int:
    requested = action or (sys.argv[1] if len(sys.argv) > 1 else "status")
    if requested not in {"start", "reload", "status"}:
        print(f"Invalid action: {requested}", file=sys.stderr)
        return 1

    if requested == "status":
        state_path = default_state_path(_state_dir_from_env())
        if not state_path.exists():
            print("runtime-manager status manager_state=unknown")
            return 0
        state = load_state(state_path)
        print(f"runtime-manager status manager_state={state.manager_state}")
        return 0

    print(f"runtime-manager {requested} action accepted")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
