from __future__ import annotations

import shlex
import shutil
import subprocess
import threading
from dataclasses import dataclass
from pathlib import Path

from .tasks import TaskStore


def _command_available(command: str | None) -> bool:
    if not command:
        return False
    parts = shlex.split(command)
    if not parts:
        return False
    binary = parts[0]
    if Path(binary).exists():
        return True
    return shutil.which(binary) is not None


def _summarize(stdout: str, stderr: str) -> str:
    for candidate in (stdout.strip(), stderr.strip()):
        if candidate:
            return candidate.splitlines()[-1]
    return ""


@dataclass(frozen=True)
class Invocation:
    args: list[str] | str
    use_shell: bool


class ExecutorRegistry:
    def __init__(self, *, store: TaskStore, codex_command: str | None, claude_command: str | None) -> None:
        self.store = store
        self.codex_command = codex_command
        self.claude_command = claude_command
        self._lock = threading.Lock()
        self._processes: dict[str, subprocess.Popen[str]] = {}
        self._cancel_events: dict[str, threading.Event] = {}

    def submit(
        self,
        *,
        task: str,
        executor: str,
        cwd: Path,
        timeout: int,
        context_files: list[str] | None = None,
    ) -> dict[str, object]:
        chosen_executor, command = self._resolve_executor(executor)
        created = self.store.create(
            task=task,
            executor=chosen_executor,
            cwd=str(cwd),
            timeout=timeout,
            context_files=context_files,
        )
        cancel_event = threading.Event()
        with self._lock:
            self._cancel_events[created["task_id"]] = cancel_event
        thread = threading.Thread(
            target=self._run_task,
            args=(created["task_id"], chosen_executor, command, task, cwd, timeout, cancel_event, context_files or []),
            daemon=True,
        )
        thread.start()
        return {
            "task_id": created["task_id"],
            "executor": chosen_executor,
            "status": created["status"],
        }

    def get(self, task_id: str) -> dict[str, object]:
        meta = self.store.get(task_id)
        meta["summary"] = self.store.read_summary(task_id)
        meta["stdout_tail"] = self.store.read_stdout(task_id)[-4000:]
        meta["stderr_tail"] = self.store.read_stderr(task_id)[-4000:]
        meta["artifacts"] = []
        return meta

    def cancel(self, task_id: str) -> dict[str, object]:
        with self._lock:
            cancel_event = self._cancel_events.get(task_id)
            process = self._processes.get(task_id)
        if cancel_event is not None:
            cancel_event.set()
        if process is not None and process.poll() is None:
            process.kill()
        updated = self.store.update(task_id, status="cancelled")
        return {
            "task_id": task_id,
            "status": updated["status"],
            "cancelled": True,
        }

    def _resolve_executor(self, executor: str) -> tuple[str, str]:
        if executor == "codex":
            if not _command_available(self.codex_command):
                raise RuntimeError("Codex command is not available.")
            return "codex", self.codex_command or ""
        if executor == "claude-code":
            if not _command_available(self.claude_command):
                raise RuntimeError("Claude Code command is not available.")
            return "claude-code", self.claude_command or ""
        if _command_available(self.codex_command):
            return "codex", self.codex_command or ""
        if _command_available(self.claude_command):
            return "claude-code", self.claude_command or ""
        raise RuntimeError("No delegate executor command is available.")

    def _run_task(
        self,
        task_id: str,
        executor_name: str,
        command: str,
        task: str,
        cwd: Path,
        timeout: int,
        cancel_event: threading.Event,
        context_files: list[str],
    ) -> None:
        if cancel_event.is_set():
            self.store.update(task_id, status="cancelled")
            return

        self.store.update(task_id, status="running")
        invocation = self._build_invocation(
            executor_name=executor_name,
            command=command,
            task=task,
            cwd=cwd,
            context_files=context_files,
        )
        process = subprocess.Popen(
            invocation.args,
            cwd=str(cwd),
            shell=invocation.use_shell,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        with self._lock:
            self._processes[task_id] = process

        if cancel_event.is_set() and process.poll() is None:
            process.kill()

        try:
            stdout, stderr = process.communicate(timeout=timeout)
        except subprocess.TimeoutExpired:
            process.kill()
            stdout, stderr = process.communicate()
            self.store.write_logs(task_id, stdout=stdout, stderr=stderr)
            self.store.write_summary(task_id, _summarize(stdout, stderr))
            self.store.update(task_id, status="failed", timed_out=True)
            return
        finally:
            with self._lock:
                self._processes.pop(task_id, None)

        self.store.write_logs(task_id, stdout=stdout, stderr=stderr)
        self.store.write_summary(task_id, _summarize(stdout, stderr))

        if cancel_event.is_set() or self.store.get(task_id)["status"] == "cancelled":
            self.store.update(task_id, status="cancelled")
            return

        status = "succeeded" if process.returncode == 0 else "failed"
        self.store.update(task_id, status=status, exit_code=process.returncode)

    def _build_invocation(
        self,
        *,
        executor_name: str,
        command: str,
        task: str,
        cwd: Path,
        context_files: list[str],
    ) -> Invocation:
        prompt = self._build_prompt(task=task, context_files=context_files)
        if executor_name == "codex":
            parts = shlex.split(command)
            if Path(parts[0]).name == "codex":
                args = [
                    *parts,
                    "exec",
                    "--dangerously-bypass-approvals-and-sandbox",
                    "-C",
                    str(cwd),
                ]
                if not (cwd / ".git").exists():
                    args.append("--skip-git-repo-check")
                args.append(prompt)
                return Invocation(args=args, use_shell=False)
        if executor_name == "claude-code":
            parts = shlex.split(command)
            if Path(parts[0]).name == "claude":
                return Invocation(
                    args=[
                        *parts,
                        "--print",
                        "--dangerously-skip-permissions",
                        "--permission-mode",
                        "bypassPermissions",
                        "--output-format",
                        "text",
                        prompt,
                    ],
                    use_shell=False,
                )
        return Invocation(args=command, use_shell=True)

    def _build_prompt(self, *, task: str, context_files: list[str]) -> str:
        if not context_files:
            return task
        lines = [task, "", "Context files:"]
        lines.extend(f"- {path}" for path in context_files)
        return "\n".join(lines)
