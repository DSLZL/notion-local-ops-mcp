package process

import (
	"runtime"
	"strings"
	"testing"
)

func TestRunnerCapturesStdout(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific shell command")
	}

	result, err := Run(Command{
		Name: "cmd",
		Args: []string{"/c", "echo stream-ok"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "stream-ok\r\n" && result.Stdout != "stream-ok\n" {
		t.Fatalf("Stdout = %q, want stream-ok newline", result.Stdout)
	}
}

func TestRunnerCapturesStderrAndExitCode(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific shell command")
	}

	result, err := Run(Command{
		Name: "cmd",
		Args: []string{"/c", "echo boom 1>&2 & exit /b 7"},
	})
	if err == nil {
		t.Fatal("expected non-zero exit to return an error")
	}
	if result.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", result.ExitCode)
	}
	if strings.TrimSpace(result.Stderr) != "boom" {
		t.Fatalf("Stderr = %q, want text containing boom only", result.Stderr)
	}
}

func TestRunnerCapturesStderrOnSuccessfulExit(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific shell command")
	}

	result, err := Run(Command{
		Name: "cmd",
		Args: []string{"/c", "echo warn 1>&2"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
	if strings.TrimSpace(result.Stderr) != "warn" {
		t.Fatalf("Stderr = %q, want text containing warn only", result.Stderr)
	}
}
