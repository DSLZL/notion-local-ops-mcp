from __future__ import annotations

from pathlib import Path


def search_files(
    base_path: Path,
    *,
    query: str,
    glob_pattern: str | None,
    limit: int,
) -> dict[str, object]:
    if not base_path.exists():
        return {
            "success": False,
            "error": {
                "code": "path_not_found",
                "message": f"Path not found: {base_path}",
            },
        }
    if not base_path.is_dir():
        return {
            "success": False,
            "error": {
                "code": "not_a_directory",
                "message": f"Path is not a directory: {base_path}",
            },
        }

    matcher = glob_pattern or "*"
    matches: list[dict[str, object]] = []
    truncated = False
    for path in sorted(base_path.rglob(matcher), key=lambda item: str(item)):
        if not path.is_file():
            continue
        try:
            content = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        for line_number, line in enumerate(content.splitlines(), start=1):
            if query not in line:
                continue
            if len(matches) >= limit:
                truncated = True
                break
            matches.append(
                {
                    "path": str(path),
                    "line_number": line_number,
                    "line": line,
                }
            )
        if truncated:
            break
    return {
        "success": True,
        "matches": matches,
        "truncated": truncated,
    }
