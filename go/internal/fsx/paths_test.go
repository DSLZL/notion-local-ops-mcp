package fsx

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestResolveRelativePathAgainstWorkspace(t *testing.T) {
	workspace := filepath.Join("workspace", "repo")
	got, err := ResolvePath(workspace, filepath.Join("docs", "readme.md"))
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}
	want := filepath.Join(workspace, "docs", "readme.md")
	if got != want {
		t.Fatalf("ResolvePath() = %q, want %q", got, want)
	}
}

func TestResolveAbsolutePathWithinWorkspacePreserved(t *testing.T) {
	workspace := t.TempDir()
	absolute := filepath.Join(workspace, "notes.txt")
	got, err := ResolvePath(workspace, absolute)
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}
	if got != absolute {
		t.Fatalf("ResolvePath() = %q, want %q", got, absolute)
	}
}

func TestResolveAbsolutePathRejectsWorkspaceEscape(t *testing.T) {
	workspace := t.TempDir()
	absolute := filepath.Join(t.TempDir(), "notes.txt")
	_, err := ResolvePath(workspace, absolute)
	if !errors.Is(err, ErrPathEscapesWorkspace) {
		t.Fatalf("ResolvePath() error = %v, want %v", err, ErrPathEscapesWorkspace)
	}
}

func TestResolveRelativePathRejectsWorkspaceEscape(t *testing.T) {
	workspace := t.TempDir()
	_, err := ResolvePath(workspace, filepath.Join("..", "secret.txt"))
	if !errors.Is(err, ErrPathEscapesWorkspace) {
		t.Fatalf("ResolvePath() error = %v, want %v", err, ErrPathEscapesWorkspace)
	}
}
