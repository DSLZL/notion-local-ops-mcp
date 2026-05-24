package tools

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestOpenShellSessionPersistsStateAcrossInputs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("PTY lifecycle test only runs on linux")
	}

	stateDir := t.TempDir()
	workspace := t.TempDir()

	opened, err := OpenShellSession(stateDir, workspace, OpenShellSessionOptions{})
	if err != nil {
		t.Fatalf("OpenShellSession() error = %v", err)
	}
	t.Cleanup(func() {
		_, _ = CloseShellSession(stateDir, opened.Session.ID)
	})

	if opened.Session.ID == "" {
		t.Fatal("session id must not be empty")
	}
	if !opened.Active {
		t.Fatal("Active = false, want true")
	}

	if _, err := SendShellInput(stateDir, opened.Session.ID, "printf 'alpha\\n'\n"); err != nil {
		t.Fatalf("SendShellInput(alpha) error = %v", err)
	}

	first := waitForShellOutput(t, stateDir, opened.Session.ID, 0, "alpha")

	if _, err := SendShellInput(stateDir, opened.Session.ID, "printf 'beta\\n'\n"); err != nil {
		t.Fatalf("SendShellInput(beta) error = %v", err)
	}

	second := waitForShellOutput(t, stateDir, opened.Session.ID, first.NextOffset, "beta")
	if second.NextOffset <= first.NextOffset {
		t.Fatalf("NextOffset = %d, want greater than %d", second.NextOffset, first.NextOffset)
	}

	status, err := GetShellSession(stateDir, opened.Session.ID)
	if err != nil {
		t.Fatalf("GetShellSession() error = %v", err)
	}
	if status.Session.ID != opened.Session.ID {
		t.Fatalf("Session.ID = %q, want %q", status.Session.ID, opened.Session.ID)
	}
	if !status.Active {
		t.Fatal("Active = false, want true")
	}
}

func TestCloseShellSessionMarksSessionTerminated(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("PTY lifecycle test only runs on linux")
	}

	stateDir := t.TempDir()
	workspace := t.TempDir()

	opened, err := OpenShellSession(stateDir, workspace, OpenShellSessionOptions{})
	if err != nil {
		t.Fatalf("OpenShellSession() error = %v", err)
	}

	closed, err := CloseShellSession(stateDir, opened.Session.ID)
	if err != nil {
		t.Fatalf("CloseShellSession() error = %v", err)
	}
	if closed.Active {
		t.Fatal("Active = true, want false")
	}
	if closed.Session.FinishedAt == "" {
		t.Fatal("FinishedAt = empty, want timestamp")
	}
	if closed.Session.Status != "closed" && closed.Session.Status != "exited" {
		t.Fatalf("Status = %q, want closed or exited", closed.Session.Status)
	}

	if _, err := SendShellInput(stateDir, opened.Session.ID, "printf 'nope\\n'\n"); err == nil {
		t.Fatal("SendShellInput() after close error = nil, want non-nil")
	}
}

func TestOpenShellSessionReturnsUnsupportedErrorOffLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("unsupported-platform assertion only runs off linux")
	}

	_, err := OpenShellSession(t.TempDir(), t.TempDir(), OpenShellSessionOptions{})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unsupported") {
		t.Fatalf("err = %v, want explicit unsupported platform error", err)
	}
}

func waitForShellOutput(t *testing.T, stateDir, sessionID string, offset int64, needle string) ShellOutputResult {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		result, err := ReadShellOutput(stateDir, sessionID, offset, 4096)
		if err == nil && strings.Contains(result.Content, needle) {
			return result
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for shell output containing %q", needle)
	return ShellOutputResult{}
}
