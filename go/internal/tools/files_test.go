package tools

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"notion-local-ops-mcp-go/internal/fsx"
)

func TestListFilesReturnsVisibleEntries(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "docs")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "zeta.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hidden.txt"), []byte("hidden"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("alpha"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	rawEntries, err := ListFiles(workspace, "docs")
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}
	entries, ok := rawEntries.([]FileEntry)
	if !ok {
		t.Fatalf("entries type = %T, want []FileEntry", rawEntries)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}
	got := []string{entries[0].Path, entries[1].Path, entries[2].Path}
	want := []string{"alpha.txt", "nested", "zeta.txt"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entries[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestListFilesSupportsRecursiveAndPagination(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "docs")
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	for _, rel := range []string{"alpha.txt", "nested/beta.txt", "nested/gamma.txt"} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.WriteFile(full, []byte(rel), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", rel, err)
		}
	}

	rawResult, err := ListFiles(workspace, ListFilesOptions{
		Path:      "docs",
		Recursive: true,
		Limit:     2,
	})
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}
	result, ok := rawResult.(ListFilesResult)
	if !ok {
		t.Fatalf("result type = %T, want ListFilesResult", rawResult)
	}
	if !result.Success {
		t.Fatalf("success = false, result = %+v", result)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(result.Entries))
	}
	if !result.Truncated {
		t.Fatal("truncated = false, want true")
	}
	if result.NextOffset == nil || *result.NextOffset != 2 {
		t.Fatalf("next_offset = %#v, want 2", result.NextOffset)
	}
}

func TestListFilesSupportsExcludePatterns(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "docs")
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	for _, rel := range []string{"alpha.txt", "nested/beta.txt", "nested/ignore.log"} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.WriteFile(full, []byte(rel), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", rel, err)
		}
	}

	rawResult, err := ListFiles(workspace, ListFilesOptions{
		Path:            "docs",
		Recursive:       true,
		ExcludePatterns: []string{"*.log"},
	})
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}
	result, ok := rawResult.(ListFilesResult)
	if !ok {
		t.Fatalf("result type = %T, want ListFilesResult", rawResult)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(result.Entries))
	}
	for _, entry := range result.Entries {
		if entry.Path == "nested/ignore.log" {
			t.Fatal("exclude pattern should filter nested/ignore.log")
		}
	}
}

func TestReadTextReturnsUTF8Content(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "demo.txt")
	if err := os.WriteFile(filePath, []byte("hello world"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	rawContent, err := ReadText(workspace, "demo.txt")
	if err != nil {
		t.Fatalf("ReadText() error = %v", err)
	}
	content, ok := rawContent.(string)
	if !ok {
		t.Fatalf("content type = %T, want string", rawContent)
	}
	if content != "hello world" {
		t.Fatalf("ReadText() = %q, want hello world", content)
	}
}

func TestReadTextSupportsPaginationAndLineNumbers(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "demo.txt")
	if err := os.WriteFile(filePath, []byte("alpha\nbeta\ngamma\ndelta\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	rawResult, err := ReadText(workspace, ReadTextOptions{
		Path:               "demo.txt",
		StartLine:          2,
		LineLimit:          2,
		IncludeLineNumbers: true,
	})
	if err != nil {
		t.Fatalf("ReadText() error = %v", err)
	}
	result, ok := rawResult.(ReadTextResult)
	if !ok {
		t.Fatalf("result type = %T, want ReadTextResult", rawResult)
	}
	if !result.Success {
		t.Fatalf("success = false, result = %+v", result)
	}
	if result.StartLine != 2 || result.EndLine != 3 {
		t.Fatalf("line range = %d-%d, want 2-3", result.StartLine, result.EndLine)
	}
	if result.Content != "2: beta\n3: gamma" {
		t.Fatalf("content = %q, want numbered slice", result.Content)
	}
	if !result.Truncated {
		t.Fatal("truncated = false, want true")
	}
	if result.NextOffset == nil || *result.NextOffset != 4 {
		t.Fatalf("next_offset = %#v, want 4", result.NextOffset)
	}
}

func TestReadTextSupportsBatchMode(t *testing.T) {
	workspace := t.TempDir()
	for _, item := range []struct {
		path    string
		content string
	}{
		{path: "a.txt", content: "alpha\nbeta\n"},
		{path: "b.txt", content: "one\ntwo\n"},
	} {
		if err := os.WriteFile(filepath.Join(workspace, item.path), []byte(item.content), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", item.path, err)
		}
	}

	rawResult, err := ReadText(workspace, ReadTextOptions{
		Paths:              []string{"a.txt", "b.txt"},
		StartLine:          1,
		LineLimit:          1,
		IncludeLineNumbers: false,
	})
	if err != nil {
		t.Fatalf("ReadText() error = %v", err)
	}
	result, ok := rawResult.(BatchReadTextResult)
	if !ok {
		t.Fatalf("result type = %T, want BatchReadTextResult", rawResult)
	}
	if !result.Success {
		t.Fatalf("success = false, result = %+v", result)
	}
	if len(result.Results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(result.Results))
	}
	if result.Results[0].Content != "alpha" {
		t.Fatalf("first content = %q, want alpha", result.Results[0].Content)
	}
	if result.Results[1].Content != "one" {
		t.Fatalf("second content = %q, want one", result.Results[1].Content)
	}
}

func TestReadTextRejectsWorkspaceEscape(t *testing.T) {
	workspace := t.TempDir()
	_, err := ReadText(workspace, filepath.Join("..", "secret.txt"))
	if !errors.Is(err, fsx.ErrPathEscapesWorkspace) {
		t.Fatalf("ReadText() error = %v, want %v", err, fsx.ErrPathEscapesWorkspace)
	}
}

func TestWriteFileSupportsDryRunAndRealWrite(t *testing.T) {
	workspace := t.TempDir()

	dryRun, err := WriteFile(workspace, "notes/demo.txt", "hello", true)
	if err != nil {
		t.Fatalf("WriteFile() dry run error = %v", err)
	}
	if !dryRun.Success || !dryRun.DryRun {
		t.Fatalf("dryRun = %+v, want successful dry run", dryRun)
	}
	if _, err := os.Stat(filepath.Join(workspace, "notes", "demo.txt")); !os.IsNotExist(err) {
		t.Fatalf("dry run should not create file, stat error = %v", err)
	}

	result, err := WriteFile(workspace, "notes/demo.txt", "hello", false)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if !result.Success || result.DryRun {
		t.Fatalf("result = %+v, want successful real write", result)
	}
	content, err := os.ReadFile(filepath.Join(workspace, "notes", "demo.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("content = %q, want hello", string(content))
	}
}
