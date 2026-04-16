from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from .pathing import resolve_path


class PatchError(RuntimeError):
    def __init__(self, code: str, message: str, **extra: object) -> None:
        super().__init__(message)
        self.code = code
        self.extra = extra


@dataclass(frozen=True)
class DiffLine:
    kind: str
    text: str


@dataclass(frozen=True)
class AddFilePatch:
    path: str
    lines: list[str]


@dataclass(frozen=True)
class DeleteFilePatch:
    path: str


@dataclass(frozen=True)
class UpdateHunk:
    lines: list[DiffLine]


@dataclass(frozen=True)
class UpdateFilePatch:
    path: str
    move_to: str | None
    hunks: list[UpdateHunk]


PatchOperation = AddFilePatch | DeleteFilePatch | UpdateFilePatch


def _error(code: str, message: str, **extra: object) -> dict[str, object]:
    payload: dict[str, object] = {
        "success": False,
        "error": {
            "code": code,
            "message": message,
        },
    }
    payload.update(extra)
    return payload


def _split_lines(text: str) -> list[str]:
    return text.splitlines()


def _join_lines(lines: list[str], *, trailing_newline: bool) -> str:
    if not lines:
        return ""
    suffix = "\n" if trailing_newline else ""
    return "\n".join(lines) + suffix


def _read_text(path: Path) -> str:
    raw = path.read_bytes()
    if b"\x00" in raw[:1024]:
        raise PatchError("not_text_file", f"Binary files are not supported: {path}", path=str(path))
    return raw.decode("utf-8", errors="replace")


def _next_is_operation_header(line: str) -> bool:
    return (
        line.startswith("*** Add File: ")
        or line.startswith("*** Delete File: ")
        or line.startswith("*** Update File: ")
        or line == "*** End Patch"
    )


def _parse_add_file(lines: list[str], start: int) -> tuple[AddFilePatch, int]:
    path = lines[start][len("*** Add File: ") :]
    index = start + 1
    content: list[str] = []
    while index < len(lines) and not _next_is_operation_header(lines[index]):
        line = lines[index]
        if not line.startswith("+"):
            raise PatchError("invalid_patch", f"Add file lines must start with '+': {line}")
        content.append(line[1:])
        index += 1
    return AddFilePatch(path=path, lines=content), index


def _parse_hunk(lines: list[str], start: int) -> tuple[UpdateHunk, int]:
    index = start
    if lines[index].startswith("@@"):
        index += 1

    diff_lines: list[DiffLine] = []
    while index < len(lines):
        line = lines[index]
        if _next_is_operation_header(line) or line.startswith("@@"):
            break
        if line == "*** End of File":
            index += 1
            continue
        if not line or line[0] not in {" ", "+", "-"}:
            raise PatchError("invalid_patch", f"Unexpected patch line: {line}")
        diff_lines.append(DiffLine(kind=line[0], text=line[1:]))
        index += 1

    if not diff_lines:
        raise PatchError("invalid_patch", "Update hunks must contain at least one diff line.")
    return UpdateHunk(lines=diff_lines), index


def _parse_update_file(lines: list[str], start: int) -> tuple[UpdateFilePatch, int]:
    path = lines[start][len("*** Update File: ") :]
    index = start + 1
    move_to: str | None = None
    if index < len(lines) and lines[index].startswith("*** Move to: "):
        move_to = lines[index][len("*** Move to: ") :]
        index += 1

    hunks: list[UpdateHunk] = []
    while index < len(lines) and not _next_is_operation_header(lines[index]):
        hunk, index = _parse_hunk(lines, index)
        hunks.append(hunk)

    if not hunks and move_to is None:
        raise PatchError("invalid_patch", f"Update file patch has no changes: {path}")
    return UpdateFilePatch(path=path, move_to=move_to, hunks=hunks), index


def parse_patch(patch: str) -> list[PatchOperation]:
    lines = patch.splitlines()
    if not lines or lines[0] != "*** Begin Patch":
        raise PatchError("invalid_patch", "Patch must start with '*** Begin Patch'.")

    operations: list[PatchOperation] = []
    index = 1
    while index < len(lines):
        line = lines[index]
        if line == "*** End Patch":
            return operations
        if line.startswith("*** Add File: "):
            operation, index = _parse_add_file(lines, index)
            operations.append(operation)
            continue
        if line.startswith("*** Delete File: "):
            operations.append(DeleteFilePatch(path=line[len("*** Delete File: ") :]))
            index += 1
            continue
        if line.startswith("*** Update File: "):
            operation, index = _parse_update_file(lines, index)
            operations.append(operation)
            continue
        raise PatchError("invalid_patch", f"Unexpected patch header: {line}")

    raise PatchError("invalid_patch", "Patch must end with '*** End Patch'.")


def _find_sequence(lines: list[str], needle: list[str], start: int) -> int:
    if not needle:
        return min(start, len(lines))
    max_start = len(lines) - len(needle) + 1
    for index in range(max(start, 0), max_start + 1):
        if lines[index : index + len(needle)] == needle:
            return index
    return -1


def _apply_hunk(lines: list[str], hunk: UpdateHunk, cursor: int) -> tuple[list[str], int]:
    old_lines = [line.text for line in hunk.lines if line.kind != "+"]
    new_lines = [line.text for line in hunk.lines if line.kind != "-"]
    search_start = max(cursor - len(old_lines), 0)
    match_index = _find_sequence(lines, old_lines, search_start)
    if match_index == -1:
        raise PatchError("patch_context_not_found", "Could not match update hunk in target file.")
    updated = lines[:match_index] + new_lines + lines[match_index + len(old_lines) :]
    return updated, match_index + len(new_lines)


def _apply_update(path: Path, move_to: Path | None, hunks: list[UpdateHunk]) -> dict[str, object]:
    if not path.exists():
        raise PatchError("file_not_found", f"File not found: {path}", path=str(path))
    if not path.is_file():
        raise PatchError("not_a_file", f"Path is not a file: {path}", path=str(path))

    original = _read_text(path)
    lines = _split_lines(original)
    cursor = 0
    for hunk in hunks:
        lines, cursor = _apply_hunk(lines, hunk, cursor)

    target = move_to or path
    if move_to and move_to.exists() and move_to != path:
        raise PatchError("target_exists", f"Move target already exists: {move_to}", path=str(move_to))

    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(_join_lines(lines, trailing_newline=original.endswith("\n")), encoding="utf-8")
    if move_to and move_to != path:
        path.unlink()
        return {"kind": "move", "path": str(target), "previous_path": str(path)}
    return {"kind": "update", "path": str(target)}


def _apply_add(path: Path, lines: list[str]) -> dict[str, object]:
    if path.exists():
        raise PatchError("path_exists", f"Path already exists: {path}", path=str(path))
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(_join_lines(lines, trailing_newline=bool(lines)), encoding="utf-8")
    return {"kind": "add", "path": str(path)}


def _apply_delete(path: Path) -> dict[str, object]:
    if not path.exists():
        raise PatchError("path_not_found", f"Path not found: {path}", path=str(path))
    if path.is_dir():
        raise PatchError("not_a_file", f"Delete file patch only supports files: {path}", path=str(path))
    path.unlink()
    return {"kind": "delete", "path": str(path)}


def apply_patch(*, patch: str, workspace_root: Path) -> dict[str, object]:
    try:
        operations = parse_patch(patch)
        changes: list[dict[str, object]] = []
        for operation in operations:
            if isinstance(operation, AddFilePatch):
                changes.append(_apply_add(resolve_path(operation.path, workspace_root), operation.lines))
                continue
            if isinstance(operation, DeleteFilePatch):
                changes.append(_apply_delete(resolve_path(operation.path, workspace_root)))
                continue
            target = resolve_path(operation.path, workspace_root)
            move_to = resolve_path(operation.move_to, workspace_root) if operation.move_to else None
            changes.append(_apply_update(target, move_to, operation.hunks))
        return {"success": True, "changes": changes}
    except PatchError as exc:
        return _error(exc.code, str(exc), **exc.extra)
