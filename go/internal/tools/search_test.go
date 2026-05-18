package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearchFindsLiteralMatch(t *testing.T) {
	workspace := t.TempDir()
	source := filepath.Join(workspace, "docs", "demo.txt")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(source, []byte("alpha\nTODO item\nomega\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	rawResult, err := Search(workspace, "TODO", "docs")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	result, ok := rawResult.([]string)
	if !ok {
		t.Fatalf("result type = %T, want []string", rawResult)
	}
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if got := filepath.ToSlash(result[0]); got != "docs/demo.txt:2: TODO item" {
		t.Fatalf("result[0] = %q, want docs/demo.txt:2: TODO item", got)
	}
}

func TestSearchSupportsTextModeWithContext(t *testing.T) {
	workspace := t.TempDir()
	source := filepath.Join(workspace, "docs", "demo.txt")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(source, []byte("alpha\nTODO item\nomega\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	rawResult, err := Search(workspace, SearchOptions{
		Mode:   "text",
		Path:   "docs",
		Query:  "TODO",
		Before: 1,
		After:  1,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	result, ok := rawResult.(SearchResult)
	if !ok {
		t.Fatalf("result type = %T, want SearchResult", rawResult)
	}
	if !result.Success {
		t.Fatalf("success = false, result = %+v", result)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1", len(result.Matches))
	}
	if result.Matches[0].LineNumber != 2 {
		t.Fatalf("line_number = %d, want 2", result.Matches[0].LineNumber)
	}
	if len(result.Matches[0].ContextBefore) != 1 || result.Matches[0].ContextBefore[0] != "alpha" {
		t.Fatalf("context_before = %#v, want [alpha]", result.Matches[0].ContextBefore)
	}
	if len(result.Matches[0].ContextAfter) != 1 || result.Matches[0].ContextAfter[0] != "omega" {
		t.Fatalf("context_after = %#v, want [omega]", result.Matches[0].ContextAfter)
	}
}

func TestSearchSupportsGlobMode(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "docs", "nested"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	for _, rel := range []string{"docs/demo.txt", "docs/nested/demo.md", "docs/nested/ignore.log"} {
		if err := os.WriteFile(filepath.Join(workspace, filepath.FromSlash(rel)), []byte("x"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", rel, err)
		}
	}

	rawResult, err := Search(workspace, SearchOptions{
		Mode:    "glob",
		Path:    "docs",
		Pattern: "*.txt",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	result, ok := rawResult.(SearchResult)
	if !ok {
		t.Fatalf("result type = %T, want SearchResult", rawResult)
	}
	if !result.Success {
		t.Fatalf("success = false, result = %+v", result)
	}
	if len(result.GlobMatches) != 1 {
		t.Fatalf("len(glob matches) = %d, want 1", len(result.GlobMatches))
	}
}

func TestSearchSupportsFilesWithMatchesOutputMode(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "docs"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	for _, item := range []struct {
		path    string
		content string
	}{
		{path: "docs/a.txt", content: "TODO alpha\n"},
		{path: "docs/b.txt", content: "beta\n"},
		{path: "docs/c.txt", content: "TODO gamma\n"},
	} {
		if err := os.WriteFile(filepath.Join(workspace, filepath.FromSlash(item.path)), []byte(item.content), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", item.path, err)
		}
	}

	rawResult, err := Search(workspace, SearchOptions{
		Mode:       "regex",
		Path:       "docs",
		Pattern:    "TODO",
		OutputMode: "files_with_matches",
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	result, ok := rawResult.(SearchResult)
	if !ok {
		t.Fatalf("result type = %T, want SearchResult", rawResult)
	}
	if len(result.Files) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(result.Files))
	}
}

func TestSearchSupportsCountModeAndIgnoreCase(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "demo.txt"), []byte("Todo\nTODO\ntodo\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	rawResult, err := Search(workspace, SearchOptions{
		Mode:       "regex",
		Path:       "demo.txt",
		Pattern:    "todo",
		OutputMode: "count",
		IgnoreCase: true,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	result, ok := rawResult.(SearchResult)
	if !ok {
		t.Fatalf("result type = %T, want SearchResult", rawResult)
	}
	if len(result.Counts) != 1 || result.Counts[0].Count != 3 {
		t.Fatalf("counts = %#v, want one file with count 3", result.Counts)
	}
}

func TestSearchReturnsEmptyForUnknownLiteral(t *testing.T) {
	workspace := t.TempDir()
	source := filepath.Join(workspace, "demo.txt")
	if err := os.WriteFile(source, []byte("alpha\nbeta\ngamma\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	rawResult, err := Search(workspace, "NO_MATCH", ".")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	result, ok := rawResult.([]string)
	if !ok {
		t.Fatalf("result type = %T, want []string", rawResult)
	}
	if len(result) != 0 {
		t.Fatalf("len(result) = %d, want 0", len(result))
	}
}

func TestSearchLiteralMatchesSubstringAcrossCorpus(t *testing.T) {
	result := SearchLiteral("demo", []string{
		"alpha.txt:1: nothing here",
		"demo.py:2: TODO item",
		"demo.py:3: done",
	})
	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}
}
