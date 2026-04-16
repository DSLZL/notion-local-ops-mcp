from pathlib import Path

from notion_local_ops_mcp.search import search_files


def test_search_files_finds_text_matches(tmp_path: Path) -> None:
    (tmp_path / "one.py").write_text("hello world\n", encoding="utf-8")
    (tmp_path / "two.txt").write_text("ignore me\n", encoding="utf-8")

    result = search_files(tmp_path, query="hello", glob_pattern="*.py", limit=20)

    assert result["success"] is True
    assert len(result["matches"]) == 1
    assert result["matches"][0]["path"].endswith("one.py")
