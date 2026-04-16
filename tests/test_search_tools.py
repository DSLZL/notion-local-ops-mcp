from pathlib import Path

from notion_local_ops_mcp.search import glob_files, grep_files, search_files


def test_search_files_finds_text_matches(tmp_path: Path) -> None:
    (tmp_path / "one.py").write_text("hello world\n", encoding="utf-8")
    (tmp_path / "two.txt").write_text("ignore me\n", encoding="utf-8")

    result = search_files(tmp_path, query="hello", glob_pattern="*.py", limit=20)

    assert result["success"] is True
    assert len(result["matches"]) == 1
    assert result["matches"][0]["path"].endswith("one.py")


def test_glob_files_matches_nested_paths(tmp_path: Path) -> None:
    (tmp_path / "a.py").write_text("print('a')\n", encoding="utf-8")
    (tmp_path / "nested").mkdir()
    (tmp_path / "nested" / "b.py").write_text("print('b')\n", encoding="utf-8")
    (tmp_path / "nested" / "c.txt").write_text("nope\n", encoding="utf-8")

    result = glob_files(tmp_path, pattern="*.py", limit=20, offset=0)

    assert result["success"] is True
    assert [Path(match["path"]).name for match in result["matches"]] == ["a.py", "b.py"]
    assert result["truncated"] is False


def test_grep_files_content_mode_supports_context_and_ignore_case(tmp_path: Path) -> None:
    target = tmp_path / "app.py"
    target.write_text("alpha\nBeta HELLO\ngamma\n", encoding="utf-8")

    result = grep_files(
        tmp_path,
        pattern="hello",
        glob_pattern="*.py",
        output_mode="content",
        before=1,
        after=1,
        ignore_case=True,
        head_limit=20,
        offset=0,
    )

    assert result["success"] is True
    assert len(result["matches"]) == 1
    assert result["matches"][0]["line_number"] == 2
    assert result["matches"][0]["context_before"] == ["alpha"]
    assert result["matches"][0]["context_after"] == ["gamma"]


def test_grep_files_files_with_matches_mode_returns_unique_paths(tmp_path: Path) -> None:
    (tmp_path / "one.py").write_text("hello\nhello again\n", encoding="utf-8")
    (tmp_path / "two.py").write_text("hello once\n", encoding="utf-8")

    result = grep_files(
        tmp_path,
        pattern="hello",
        glob_pattern="*.py",
        output_mode="files_with_matches",
        head_limit=20,
        offset=0,
    )

    assert result["success"] is True
    assert [Path(path).name for path in result["files"]] == ["one.py", "two.py"]


def test_grep_files_count_mode_returns_per_file_counts(tmp_path: Path) -> None:
    (tmp_path / "one.py").write_text("hello\nhello\n", encoding="utf-8")
    (tmp_path / "two.py").write_text("hello\n", encoding="utf-8")

    result = grep_files(
        tmp_path,
        pattern="hello",
        glob_pattern="*.py",
        output_mode="count",
        head_limit=20,
        offset=0,
    )

    assert result["success"] is True
    assert result["counts"] == [
        {"path": str(tmp_path / "one.py"), "count": 2},
        {"path": str(tmp_path / "two.py"), "count": 1},
    ]
