package tools

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"notion-local-ops-mcp-go/internal/fsx"
)

func TestSessionDefaultCWDLifecycle(t *testing.T) {
	ClearDefaultCWD()
	if got := GetDefaultCWD(); got != "" {
		t.Fatalf("GetDefaultCWD() = %q, want empty", got)
	}

	SetDefaultCWD("C:/repo")
	if got := GetDefaultCWD(); got != "C:/repo" {
		t.Fatalf("GetDefaultCWD() = %q, want C:/repo", got)
	}

	ClearDefaultCWD()
	if got := GetDefaultCWD(); got != "" {
		t.Fatalf("GetDefaultCWD() after clear = %q, want empty", got)
	}
}

func TestResolveSessionDirectoryRejectsWorkspaceEscape(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()

	_, _, err := ResolveSessionDirectory(workspace, outside)
	if !errors.Is(err, fsx.ErrPathEscapesWorkspace) {
		t.Fatalf("ResolveSessionDirectory() error = %v, want %v", err, fsx.ErrPathEscapesWorkspace)
	}
}

func TestResolveSessionDirectoryAcceptsWorkspaceSubdir(t *testing.T) {
	workspace := t.TempDir()
	subdir := filepath.Join(workspace, "CTF")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	got, fromSession, err := ResolveSessionDirectory(workspace, subdir)
	if err != nil {
		t.Fatalf("ResolveSessionDirectory() error = %v", err)
	}
	if fromSession {
		t.Fatal("fromSession = true, want false for explicit input")
	}
	if got != subdir {
		t.Fatalf("ResolveSessionDirectory() = %q, want %q", got, subdir)
	}
}
