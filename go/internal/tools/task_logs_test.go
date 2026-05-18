package tools

import (
	"testing"

	"notion-local-ops-mcp-go/internal/taskstore"
)

func TestGetTaskLogsReadsStdoutFromOffset(t *testing.T) {
	root := t.TempDir()
	store := taskstore.NewFSStore(root)
	task, err := store.Create(taskstore.TaskInput{Task: "job", Executor: "run_command_stream"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := store.WriteLogs(task.ID, "hello\nworld\n", ""); err != nil {
		t.Fatalf("WriteLogs() error = %v", err)
	}

	result := GetTaskLogs(root, task.ID, "stdout", 6, 5)
	if !result.Success {
		t.Fatal("Success = false, want true")
	}
	if result.Content != "world" {
		t.Fatalf("Content = %q, want %q", result.Content, "world")
	}
	if result.NextOffset != 11 {
		t.Fatalf("NextOffset = %d, want 11", result.NextOffset)
	}
	if !result.Truncated {
		t.Fatal("Truncated = false, want true")
	}
}

func TestGetTaskLogsReturnsTruncatedWindow(t *testing.T) {
	root := t.TempDir()
	store := taskstore.NewFSStore(root)
	task, err := store.Create(taskstore.TaskInput{Task: "job", Executor: "run_command_stream"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := store.WriteLogs(task.ID, "hello\nworld\n", ""); err != nil {
		t.Fatalf("WriteLogs() error = %v", err)
	}

	result := GetTaskLogs(root, task.ID, "stdout", 0, 5)
	if !result.Success {
		t.Fatal("Success = false, want true")
	}
	if result.Content != "hello" {
		t.Fatalf("Content = %q, want %q", result.Content, "hello")
	}
	if result.NextOffset != 5 {
		t.Fatalf("NextOffset = %d, want 5", result.NextOffset)
	}
	if !result.Truncated {
		t.Fatal("Truncated = false, want true")
	}
}

func TestGetTaskLogsRejectsUnknownStream(t *testing.T) {
	root := t.TempDir()
	result := GetTaskLogs(root, "task-123", "combined", 0, 10)
	if result.Success {
		t.Fatal("Success = true, want false")
	}
	if result.Stream != "combined" {
		t.Fatalf("Stream = %q, want combined", result.Stream)
	}
}

func TestGetTaskLogsTreatsMissingLogFileAsEmptySuccess(t *testing.T) {
	root := t.TempDir()
	store := taskstore.NewFSStore(root)
	task, err := store.Create(taskstore.TaskInput{Task: "job", Executor: "run_command_stream"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	result := GetTaskLogs(root, task.ID, "stderr", 0, 10)
	if !result.Success {
		t.Fatal("Success = false, want true for empty missing log stream")
	}
	if result.Content != "" {
		t.Fatalf("Content = %q, want empty", result.Content)
	}
	if result.Offset != 0 || result.NextOffset != 0 {
		t.Fatalf("Offset/NextOffset = %d/%d, want 0/0", result.Offset, result.NextOffset)
	}
	if result.Truncated {
		t.Fatal("Truncated = true, want false")
	}
}
