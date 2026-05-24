package tools

import (
	"os"
	"path/filepath"
	"strings"
	"runtime"
	"testing"
	"time"

	"notion-local-ops-mcp-go/internal/taskstore"
)

func TestRunCommandStreamReturnsTaskID(t *testing.T) {
	stateDir := t.TempDir()
	task := RunCommandStream(stateDir, t.TempDir(), "echo stream-ok", "", 9)
	if task.TaskID == "" {
		t.Fatal("task_id must not be empty")
	}
	if task.Status != "queued" {
		t.Fatalf("status = %q, want queued", task.Status)
	}
	if task.Summary != "task accepted" {
		t.Fatalf("summary = %q, want %q", task.Summary, "task accepted")
	}

	store := taskstore.NewFSStore(stateDir)
	meta, err := store.Get(task.TaskID)
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if meta.Timeout != 9 {
		t.Fatalf("Timeout = %d, want 9", meta.Timeout)
	}
}

func TestRunCommandStreamReturnsBeforeCommandCompletes(t *testing.T) {
	stateDir := t.TempDir()
	workspace := t.TempDir()

	begin := time.Now()
	task := RunCommandStream(stateDir, workspace, slowStreamCommand(), "", 5)
	elapsed := time.Since(begin)
	if elapsed > 400*time.Millisecond {
		t.Fatalf("RunCommandStream() took %v, want immediate return", elapsed)
	}
	if task.TaskID == "" {
		t.Fatal("task_id must not be empty")
	}
	if task.Status != "queued" {
		t.Fatalf("status = %q, want queued", task.Status)
	}
	if task.Summary != "task accepted" {
		t.Fatalf("summary = %q, want %q", task.Summary, "task accepted")
	}

	store := taskstore.NewFSStore(stateDir)
	waitForTaskInStore(t, store, task.TaskID, func(current taskstore.Task) bool {
		return current.Status == "running" && current.PID != nil && *current.PID > 0 && current.HeartbeatAt != ""
	}, "running task with pid and heartbeat")

	completed := waitForTaskInStore(t, store, task.TaskID, func(current taskstore.Task) bool {
		return current.Status == "succeeded"
	}, "succeeded task")
	if completed.ExitCode == nil || *completed.ExitCode != 0 {
		t.Fatalf("ExitCode = %v, want 0", completed.ExitCode)
	}
}

func TestRunCommandReturnsStdoutAndExitCode(t *testing.T) {
	result := RunCommand(".", "echo hello", "", "", 5)
	if !result.Success {
		t.Fatalf("success = %v, want true; stderr=%q", result.Success, result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit_code = %d, want 0", result.ExitCode)
	}
	if result.TimedOut {
		t.Fatal("timed_out = true, want false")
	}
	if got := result.Stdout; got == "" {
		t.Fatal("stdout must not be empty")
	}
}

func TestRunCommandMissingCWDReturnsUnifiedShape(t *testing.T) {
	result := RunCommand(".", "echo hello", "does-not-exist", "", 5)
	if result.Success {
		t.Fatal("success = true, want false")
	}
	if result.TimedOut {
		t.Fatal("timed_out = true, want false")
	}
	if result.ExitCode != TimeoutExitCode {
		t.Fatalf("exit_code = %d, want %d", result.ExitCode, TimeoutExitCode)
	}
	if result.Stdout != "" {
		t.Fatalf("stdout = %q, want empty", result.Stdout)
	}
	if result.Stderr != "" {
		t.Fatalf("stderr = %q, want empty", result.Stderr)
	}
	if result.Error["code"] != "cwd_not_found" {
		t.Fatalf("error.code = %v, want cwd_not_found", result.Error["code"])
	}
}

func TestRunCommandWithStdinContent(t *testing.T) {
	result := RunCommand(".", stdinEchoCommand(), "", "stdin-ok\n", 5)
	if !result.Success {
		t.Fatalf("success = %v, want true; stderr=%q", result.Success, result.Stderr)
	}
	if strings.TrimSpace(result.Stdout) != "stdin-ok" {
		t.Fatalf("stdout = %q, want %q", result.Stdout, "stdin-ok")
	}
}

func TestRunCommandTimeoutReturnsStableHint(t *testing.T) {
	result := RunCommand(".", slowTimeoutCommand(), "", "", 1)
	if result.Success {
		t.Fatal("success = true, want false")
	}
	if !result.TimedOut {
		t.Fatal("timed_out = false, want true")
	}
	if result.Error == nil || result.Error["code"] != "timed_out" {
		t.Fatalf("error.code = %v, want timed_out", result.Error["code"])
	}
	if result.Hint != runCommandTimeoutHint {
		t.Fatalf("hint = %q, want %q", result.Hint, runCommandTimeoutHint)
	}
	msg, _ := result.Error["message"].(string)
	if !strings.Contains(msg, "run_command_stream") {
		t.Fatalf("timeout message = %q, want mention run_command_stream", msg)
	}
	if !strings.Contains(msg, "stdout.log") || !strings.Contains(msg, "stderr.log") {
		t.Fatalf("timeout message = %q, want mention task logs", msg)
	}
}

func TestRunCommandCWDNotDirectoryReturnsUnifiedShape(t *testing.T) {
	workspace := t.TempDir()
	fileCWD := filepath.Join(workspace, "not-a-dir.txt")
	if err := os.WriteFile(fileCWD, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup write failed: %v", err)
	}
	result := RunCommand(workspace, "echo hello", fileCWD, "", 5)
	if result.Success {
		t.Fatal("success = true, want false")
	}
	if result.TimedOut {
		t.Fatal("timed_out = true, want false")
	}
	if result.Error["code"] != "cwd_not_directory" {
		t.Fatalf("error.code = %v, want cwd_not_directory", result.Error["code"])
	}
}

func TestRunCommandStartFailureReturnsUnifiedShape(t *testing.T) {
	result := RunCommand(".", "echo hello\x00world", "", "", 5)
	if result.Success {
		t.Fatal("success = true, want false")
	}
	if result.TimedOut {
		t.Fatal("timed_out = true, want false")
	}
	if result.Error["code"] != "command_start_failed" {
		t.Fatalf("error.code = %v, want command_start_failed", result.Error["code"])
	}
	if result.Stderr == "" {
		t.Fatal("stderr = empty, want non-empty")
	}
}

func waitForTaskInStore(t *testing.T, store taskstore.Store, taskID string, predicate func(taskstore.Task) bool, description string) taskstore.Task {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	var latest taskstore.Task
	for time.Now().Before(deadline) {
		task, err := store.Get(taskID)
		if err == nil {
			latest = task
			if predicate(task) {
				return task
			}
			if task.Status == "failed" || task.Status == "cancelled" {
				t.Fatalf("task %s reached terminal status %q while waiting for %s", taskID, task.Status, description)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("task %s last status = %q after timeout, waiting for %s", taskID, latest.Status, description)
	return taskstore.Task{}
}

func slowStreamCommand() string {
	if runtime.GOOS == "windows" {
		return `powershell -NoProfile -Command "Start-Sleep -Milliseconds 900; Write-Output stream-ok"`
	}
	return `sleep 0.9; printf 'stream-ok\n'`
}

func slowTimeoutCommand() string {
	if runtime.GOOS == "windows" {
		return `ping 127.0.0.1 -n 6 > NUL`
	}
	return `sleep 2; printf 'done\n'`
}

func stdinEchoCommand() string {
	if runtime.GOOS == "windows" {
		return `more`
	}
	return `cat`
}
