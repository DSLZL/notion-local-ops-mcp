package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"notion-local-ops-mcp-go/internal/taskstore"
)

func TestListRecentTasksFiltersByStatus(t *testing.T) {
	stateDir := t.TempDir()

	writeTaskFixture(t, stateDir, taskstore.Task{
		ID:        "task-100",
		Task:      "queued task",
		Status:    "queued",
		CreatedAt: "2026-05-17T10:00:00Z",
		UpdatedAt: "2026-05-17T10:00:00Z",
	}, "queued summary")
	writeTaskFixture(t, stateDir, taskstore.Task{
		ID:        "task-200",
		Task:      "running task",
		Status:    "Running",
		CreatedAt: "2026-05-17T11:00:00Z",
		UpdatedAt: "2026-05-17T11:30:00Z",
	}, "running summary")
	writeTaskFixture(t, stateDir, taskstore.Task{
		ID:        "task-300",
		Task:      "done task",
		Status:    "DONE",
		CreatedAt: "2026-05-17T12:00:00Z",
		UpdatedAt: "2026-05-17T12:30:00Z",
	}, "done summary")

	result := ListRecentTasks(stateDir, "running", 10)

	if !result.Success {
		t.Fatal("Success = false, want true")
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(result.Tasks))
	}
	if result.Tasks[0].TaskID != "task-200" {
		t.Fatalf("TaskID = %q, want task-200", result.Tasks[0].TaskID)
	}
	if result.Tasks[0].Status != "Running" {
		t.Fatalf("Status = %q, want Running", result.Tasks[0].Status)
	}
	if result.Tasks[0].Summary != "running summary" {
		t.Fatalf("Summary = %q, want running summary", result.Tasks[0].Summary)
	}
}

func TestListRecentTasksAppliesLimitNewestFirst(t *testing.T) {
	stateDir := t.TempDir()

	writeTaskFixture(t, stateDir, taskstore.Task{
		ID:        "task-100",
		Task:      "oldest task",
		Status:    "queued",
		CreatedAt: "2026-05-17T09:00:00Z",
		UpdatedAt: "2026-05-17T09:30:00Z",
	}, "oldest summary")
	writeTaskFixture(t, stateDir, taskstore.Task{
		ID:        "task-200",
		Task:      "middle task",
		Status:    "running",
		CreatedAt: "2026-05-17T13:00:00Z",
		UpdatedAt: "2026-05-17T13:30:00Z",
	}, "middle summary")
	writeTaskFixture(t, stateDir, taskstore.Task{
		ID:        "task-300",
		Task:      "newest by update",
		Status:    "done",
		CreatedAt: "2026-05-17T08:00:00Z",
		UpdatedAt: "2026-05-17T14:30:00Z",
	}, "newest summary")

	result := ListRecentTasks(stateDir, "", 2)

	if !result.Success {
		t.Fatal("Success = false, want true")
	}
	if len(result.Tasks) != 2 {
		t.Fatalf("len(Tasks) = %d, want 2", len(result.Tasks))
	}
	if result.Tasks[0].TaskID != "task-300" || result.Tasks[1].TaskID != "task-200" {
		t.Fatalf("TaskIDs = [%q %q], want [task-300 task-200]", result.Tasks[0].TaskID, result.Tasks[1].TaskID)
	}
	if result.Tasks[0].Summary != "newest summary" {
		t.Fatalf("first Summary = %q, want newest summary", result.Tasks[0].Summary)
	}
	if result.Tasks[1].Summary != "middle summary" {
		t.Fatalf("second Summary = %q, want middle summary", result.Tasks[1].Summary)
	}
}

func TestListRecentTasksReturnsEmptyArrayWhenTasksDirMissing(t *testing.T) {
	stateDir := t.TempDir()

	result := ListRecentTasks(stateDir, "", 10)

	if !result.Success {
		t.Fatal("Success = false, want true")
	}
	if result.Tasks == nil {
		t.Fatal("Tasks = nil, want non-nil empty slice")
	}
	if len(result.Tasks) != 0 {
		t.Fatalf("len(Tasks) = %d, want 0", len(result.Tasks))
	}

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(raw), `"tasks":null`) {
		t.Fatalf("JSON = %s, want tasks to encode as []", string(raw))
	}
}

func TestListRecentTasksFailsWhenTasksPathIsNotDirectory(t *testing.T) {
	stateDir := t.TempDir()
	tasksPath := filepath.Join(stateDir, "tasks")
	if err := os.WriteFile(tasksPath, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile(tasks) error = %v", err)
	}

	result := ListRecentTasks(stateDir, "", 10)

	if result.Success {
		t.Fatal("Success = true, want false")
	}
	if result.Tasks == nil {
		t.Fatal("Tasks = nil, want non-nil empty slice")
	}
	if len(result.Tasks) != 0 {
		t.Fatalf("len(Tasks) = %d, want 0", len(result.Tasks))
	}
}

func writeTaskFixture(t *testing.T, stateDir string, task taskstore.Task, summary string) {
	t.Helper()

	taskDir := filepath.Join(stateDir, "tasks", task.ID)
	if err := os.MkdirAll(taskDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	raw, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "meta.json"), raw, 0o600); err != nil {
		t.Fatalf("WriteFile(meta.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "summary.txt"), []byte(summary), 0o600); err != nil {
		t.Fatalf("WriteFile(summary.txt) error = %v", err)
	}
}
