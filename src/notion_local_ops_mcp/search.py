from __future__ import annotations

import re
from pathlib import Path


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


def _validate_directory(path: Path) -> dict[str, object] | None:
    if not path.exists():
        return _error(
            "path_not_found",
            f"Path not found: {path}",
            resolved_path=str(path),
        )
    if not path.is_dir():
        return _error(
            "not_a_directory",
            f"Path is not a directory: {path}",
            resolved_path=str(path),
        )
    return None


def _paginate(items: list[object], *, offset: int, limit: int) -> tuple[list[object], bool, int | None]:
    start = max(offset, 0)
    if limit == 0:
        selected = items[start:]
    else:
        selected = items[start : start + max(limit, 0)]
    truncated = start + len(selected) < len(items)
    next_offset = start + len(selected) if truncated else None
    return selected, truncated, next_offset


def _iter_matching_files(base_path: Path, glob_pattern: str | None) -> list[Path]:
    matcher = glob_pattern or "*"
    return [path for path in sorted(base_path.rglob(matcher), key=lambda item: str(item)) if path.is_file()]


def _read_text(path: Path) -> str | None:
    try:
        raw = path.read_bytes()
    except OSError:
        return None
    if b"\x00" in raw[:1024]:
        return None
    return raw.decode("utf-8", errors="replace")


def glob_files(
    base_path: Path,
    *,
    pattern: str,
    limit: int,
    offset: int,
) -> dict[str, object]:
    validation_error = _validate_directory(base_path)
    if validation_error:
        return validation_error

    matches = [
        {
            "path": str(path),
            "is_dir": path.is_dir(),
        }
        for path in sorted(base_path.rglob(pattern), key=lambda item: str(item))
    ]
    selected, truncated, next_offset = _paginate(matches, offset=offset, limit=limit)
    return {
        "success": True,
        "base_path": str(base_path),
        "pattern": pattern,
        "matches": selected,
        "truncated": truncated,
        "next_offset": next_offset,
    }


def grep_files(
    base_path: Path,
    *,
    pattern: str,
    glob_pattern: str | None,
    output_mode: str,
    before: int = 0,
    after: int = 0,
    ignore_case: bool = False,
    head_limit: int,
    offset: int,
    multiline: bool = False,
) -> dict[str, object]:
    validation_error = _validate_directory(base_path)
    if validation_error:
        return validation_error

    if output_mode not in {"content", "files_with_matches", "count"}:
        return _error("invalid_output_mode", f"Unsupported output_mode: {output_mode}")

    flags = re.MULTILINE
    if ignore_case:
        flags |= re.IGNORECASE
    if multiline:
        flags |= re.DOTALL

    try:
        compiled = re.compile(pattern, flags)
    except re.error as exc:
        return _error("invalid_pattern", str(exc), pattern=pattern)

    if output_mode == "files_with_matches":
        files: list[str] = []
        for path in _iter_matching_files(base_path, glob_pattern):
            content = _read_text(path)
            if content is None:
                continue
            if compiled.search(content):
                files.append(str(path))
        selected, truncated, next_offset = _paginate(files, offset=offset, limit=head_limit)
        return {
            "success": True,
            "base_path": str(base_path),
            "pattern": pattern,
            "output_mode": output_mode,
            "files": selected,
            "truncated": truncated,
            "next_offset": next_offset,
        }

    if output_mode == "count":
        counts: list[dict[str, object]] = []
        for path in _iter_matching_files(base_path, glob_pattern):
            content = _read_text(path)
            if content is None:
                continue
            count = sum(1 for _ in compiled.finditer(content))
            if count:
                counts.append({"path": str(path), "count": count})
        selected, truncated, next_offset = _paginate(counts, offset=offset, limit=head_limit)
        return {
            "success": True,
            "base_path": str(base_path),
            "pattern": pattern,
            "output_mode": output_mode,
            "counts": selected,
            "truncated": truncated,
            "next_offset": next_offset,
        }

    matches: list[dict[str, object]] = []
    for path in _iter_matching_files(base_path, glob_pattern):
        content = _read_text(path)
        if content is None:
            continue
        lines = content.splitlines()
        if multiline:
            for match in compiled.finditer(content):
                start_line = content.count("\n", 0, match.start()) + 1
                end_line = content.count("\n", 0, match.end()) + 1
                matches.append(
                    {
                        "path": str(path),
                        "line_number": start_line,
                        "end_line_number": end_line,
                        "line": match.group(0),
                        "context_before": lines[max(start_line - 1 - before, 0) : start_line - 1],
                        "context_after": lines[end_line : end_line + after],
                    }
                )
        else:
            for line_number, line in enumerate(lines, start=1):
                if not compiled.search(line):
                    continue
                matches.append(
                    {
                        "path": str(path),
                        "line_number": line_number,
                        "line": line,
                        "context_before": lines[max(line_number - 1 - before, 0) : line_number - 1],
                        "context_after": lines[line_number : line_number + after],
                    }
                )

    selected, truncated, next_offset = _paginate(matches, offset=offset, limit=head_limit)
    return {
        "success": True,
        "base_path": str(base_path),
        "pattern": pattern,
        "output_mode": output_mode,
        "matches": selected,
        "truncated": truncated,
        "next_offset": next_offset,
    }


def search_files(
    base_path: Path,
    *,
    query: str,
    glob_pattern: str | None,
    limit: int,
) -> dict[str, object]:
    result = grep_files(
        base_path,
        pattern=re.escape(query),
        glob_pattern=glob_pattern,
        output_mode="content",
        before=0,
        after=0,
        ignore_case=False,
        head_limit=limit,
        offset=0,
        multiline=False,
    )
    if not result["success"]:
        return result
    return {
        "success": True,
        "matches": [
            {
                "path": match["path"],
                "line_number": match["line_number"],
                "line": match["line"],
            }
            for match in result["matches"]
        ],
        "truncated": result["truncated"],
    }
