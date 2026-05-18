package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyPatchAddsFile(t *testing.T) {
	root := t.TempDir()
	result := ApplyPatch(root, strings.Join([]string{
		"*** Begin Patch",
		"*** Add File: notes.txt",
		"+hello",
		"+world",
		"*** End Patch",
	}, "\n"), false, false, false)

	if !result.Success {
		t.Fatalf("success = false, want true; err=%+v", result.Error)
	}
	got, err := os.ReadFile(filepath.Join(root, "notes.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "hello\nworld\n" {
		t.Fatalf("content = %q, want %q", string(got), "hello\nworld\n")
	}
}

func TestApplyPatchUpdatesFileWithMultipleHunks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "app.py")
	if err := os.WriteFile(target, []byte("one\ntwo\nthree\nfour\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result := ApplyPatch(root, strings.Join([]string{
		"*** Begin Patch",
		"*** Update File: app.py",
		"@@",
		" one",
		"-two",
		"+TWO",
		" three",
		"@@",
		" three",
		"-four",
		"+FOUR",
		"*** End Patch",
	}, "\n"), false, false, false)

	if !result.Success {
		t.Fatalf("success = false, want true; err=%+v", result.Error)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "one\nTWO\nthree\nFOUR\n" {
		t.Fatalf("content = %q", string(got))
	}
}

func TestApplyPatchMovesAndUpdatesFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "src.txt")
	if err := os.WriteFile(source, []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result := ApplyPatch(root, strings.Join([]string{
		"*** Begin Patch",
		"*** Update File: src.txt",
		"*** Move to: moved.txt",
		"@@",
		" alpha",
		"-beta",
		"+gamma",
		"*** End Patch",
	}, "\n"), false, false, false)

	if !result.Success {
		t.Fatalf("success = false, want true; err=%+v", result.Error)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source file should be removed after move, err=%v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "moved.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "alpha\ngamma\n" {
		t.Fatalf("content = %q", string(got))
	}
	if len(result.Changes) == 0 || result.Changes[0].Kind != "move" {
		t.Fatalf("change kind = %+v, want move", result.Changes)
	}
}

func TestApplyPatchDeletesFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "trash.txt")
	if err := os.WriteFile(target, []byte("bye\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result := ApplyPatch(root, strings.Join([]string{
		"*** Begin Patch",
		"*** Delete File: trash.txt",
		"*** End Patch",
	}, "\n"), false, false, false)

	if !result.Success {
		t.Fatalf("success = false, want true; err=%+v", result.Error)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target should be deleted, err=%v", err)
	}
}

func TestApplyPatchDryRunReturnsDiffWithoutWriting(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "app.py")
	if err := os.WriteFile(target, []byte("before\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result := ApplyPatch(root, strings.Join([]string{
		"*** Begin Patch",
		"*** Update File: app.py",
		"@@",
		"-before",
		"+after",
		"*** End Patch",
	}, "\n"), true, false, true)

	if !result.Success {
		t.Fatalf("success = false, want true; err=%+v", result.Error)
	}
	if result.Applied {
		t.Fatal("applied = true, want false")
	}
	if !strings.HasPrefix(result.Diff, "--- ") {
		t.Fatalf("diff = %q, want unified diff", result.Diff)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "before\n" {
		t.Fatalf("content = %q, want unchanged", string(got))
	}
}

func TestApplyPatchValidateOnlyChecksPatchWithoutWriting(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "note.txt")
	if err := os.WriteFile(target, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result := ApplyPatch(root, strings.Join([]string{
		"*** Begin Patch",
		"*** Update File: note.txt",
		"@@",
		"-hello",
		"+world",
		"*** End Patch",
	}, "\n"), false, true, false)

	if !result.Success {
		t.Fatalf("success = false, want true; err=%+v", result.Error)
	}
	if !result.Validated {
		t.Fatal("validated = false, want true")
	}
	if result.Applied {
		t.Fatal("applied = true, want false")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "hello\n" {
		t.Fatalf("content = %q, want unchanged", string(got))
	}
}

func TestApplyPatchRejectsContextOnlyHunk(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "note.txt")
	if err := os.WriteFile(target, []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result := ApplyPatch(root, strings.Join([]string{
		"*** Begin Patch",
		"*** Update File: note.txt",
		"@@",
		" alpha",
		" beta",
		"*** End Patch",
	}, "\n"), false, false, false)

	if result.Success {
		t.Fatal("success = true, want false")
	}
	if result.Error == nil || result.Error.Code != "empty_hunk" {
		t.Fatalf("error = %+v, want empty_hunk", result.Error)
	}
}

func TestApplyPatchRequiresUniqueContextMatch(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "note.txt")
	if err := os.WriteFile(target, []byte("alpha\nbeta\nalpha\nbeta\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result := ApplyPatch(root, strings.Join([]string{
		"*** Begin Patch",
		"*** Update File: note.txt",
		"@@",
		" alpha",
		"-beta",
		"+BETA",
		"*** End Patch",
	}, "\n"), false, false, false)

	if result.Success {
		t.Fatal("success = true, want false")
	}
	if result.Error == nil || result.Error.Code != "ambiguous_context_match" {
		t.Fatalf("error = %+v, want ambiguous_context_match", result.Error)
	}
	if result.MatchCount != 2 {
		t.Fatalf("match_count = %d, want 2", result.MatchCount)
	}
}
