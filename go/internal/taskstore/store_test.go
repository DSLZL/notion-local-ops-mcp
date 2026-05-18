package taskstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateTaskPersistsQueuedMetadata(t *testing.T) {
	root := t.TempDir()
	store := NewFSStore(root)

	task, err := store.Create(TaskInput{
		Task:     "echo hi",
		Executor: "codex",
		CWD:      "C:/repo",
		Timeout:  60,
		Metadata: map[string]any{
			"goal":                  "ship parity",
			"acceptance_criteria":   []string{"works"},
			"verification_commands": []string{"go test ./..."},
			"commit_mode":           "allowed",
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if task.Status != "queued" {
		t.Fatalf("Status = %q, want queued", task.Status)
	}

	saved, err := store.Get(task.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if saved.Task != "echo hi" {
		t.Fatalf("Task = %q, want echo hi", saved.Task)
	}
	if saved.Goal != "ship parity" {
		t.Fatalf("Goal = %q, want ship parity", saved.Goal)
	}
	if len(saved.AcceptanceCriteria) != 1 || saved.AcceptanceCriteria[0] != "works" {
		t.Fatalf("AcceptanceCriteria = %#v, want [works]", saved.AcceptanceCriteria)
	}
	if saved.Timeout != 60 {
		t.Fatalf("Timeout = %d, want 60", saved.Timeout)
	}

	metaPath := filepath.Join(root, "tasks", task.ID, "meta.json")
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("meta.json stat error = %v", err)
	}
}

func TestCreateTaskInitializesLongTaskDefaults(t *testing.T) {
	root := t.TempDir()
	store := NewFSStore(root)

	task, err := store.Create(TaskInput{
		Task:     "sleep 1",
		Executor: "shell",
		CWD:      root,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	saved, err := store.Get(task.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	assertDefaults := func(label string, task Task) {
		t.Helper()
		if task.StartedAt != "" {
			t.Fatalf("%s StartedAt = %q, want empty", label, task.StartedAt)
		}
		if task.HeartbeatAt != "" {
			t.Fatalf("%s HeartbeatAt = %q, want empty", label, task.HeartbeatAt)
		}
		if task.FinishedAt != "" {
			t.Fatalf("%s FinishedAt = %q, want empty", label, task.FinishedAt)
		}
		if task.ProgressPercent != nil {
			t.Fatalf("%s ProgressPercent = %v, want nil", label, task.ProgressPercent)
		}
		if task.ProgressMessage != "" {
			t.Fatalf("%s ProgressMessage = %q, want empty", label, task.ProgressMessage)
		}
		if task.PID != nil {
			t.Fatalf("%s PID = %v, want nil", label, task.PID)
		}
		if task.ResultPreview != "" {
			t.Fatalf("%s ResultPreview = %q, want empty", label, task.ResultPreview)
		}
		if task.StdoutSize != 0 {
			t.Fatalf("%s StdoutSize = %d, want 0", label, task.StdoutSize)
		}
		if task.StderrSize != 0 {
			t.Fatalf("%s StderrSize = %d, want 0", label, task.StderrSize)
		}
		if task.LastLogOffset != 0 {
			t.Fatalf("%s LastLogOffset = %d, want 0", label, task.LastLogOffset)
		}
		if task.EventSeq != 0 {
			t.Fatalf("%s EventSeq = %d, want 0", label, task.EventSeq)
		}
		if task.CancelRequested {
			t.Fatalf("%s CancelRequested = true, want false", label)
		}
	}

	assertDefaults("returned", task)
	assertDefaults("saved", saved)
}

func TestGetRejectsInvalidTaskID(t *testing.T) {
	store := NewFSStore(t.TempDir())

	for _, taskID := range []string{"../escape", ".", "", "task/123", `task\123`} {
		_, err := store.Get(taskID)
		if !errors.Is(err, ErrInvalidTaskID) {
			t.Fatalf("Get(%q) error = %v, want %v", taskID, err, ErrInvalidTaskID)
		}
	}
}

func TestTaskReadWriteHelpersRejectInvalidTaskID(t *testing.T) {
	store := NewFSStore(t.TempDir())

	for _, taskID := range []string{"../escape", ".", "", "task/123", `task\123`} {
		if err := store.WriteSummary(taskID, "summary"); !errors.Is(err, ErrInvalidTaskID) {
			t.Fatalf("WriteSummary(%q) error = %v, want %v", taskID, err, ErrInvalidTaskID)
		}
		if got := store.ReadSummary(taskID); got != "" {
			t.Fatalf("ReadSummary(%q) = %q, want empty", taskID, got)
		}
		if got := store.ReadStdout(taskID); got != "" {
			t.Fatalf("ReadStdout(%q) = %q, want empty", taskID, got)
		}
		if got := store.ReadStderr(taskID); got != "" {
			t.Fatalf("ReadStderr(%q) = %q, want empty", taskID, got)
		}
	}
}

func TestUpdateAndPurgeOlderThan(t *testing.T) {
	root := t.TempDir()
	store := NewFSStore(root)

	task, err := store.Create(TaskInput{
		Task:     "echo hi",
		Executor: "shell",
		CWD:      root,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	task.Status = "running"
	task.UpdatedAt = "2000-01-01T00:00:00Z"
	if _, err := store.Update(task.ID, task); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	scanned, purged, err := store.PurgeOlderThan(time.Hour, true)
	if err != nil {
		t.Fatalf("PurgeOlderThan() error = %v", err)
	}
	if scanned != 1 {
		t.Fatalf("scanned = %d, want 1", scanned)
	}
	if len(purged) != 1 || purged[0] != task.ID {
		t.Fatalf("purged = %#v, want [%s]", purged, task.ID)
	}
}

func TestPurgeOlderThanSkipsUnreadableTask(t *testing.T) {
	root := t.TempDir()
	store := NewFSStore(root)

	taskDir := filepath.Join(root, "tasks", "task-123")
	if err := os.MkdirAll(taskDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "meta.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	scanned, purged, err := store.PurgeOlderThan(time.Hour, false)
	if err != nil {
		t.Fatalf("PurgeOlderThan() error = %v", err)
	}
	if scanned != 1 {
		t.Fatalf("scanned = %d, want 1", scanned)
	}
	if len(purged) != 0 {
		t.Fatalf("purged = %#v, want empty", purged)
	}
	if _, err := os.Stat(taskDir); err != nil {
		t.Fatalf("task dir stat error = %v", err)
	}
}

func TestUpdatePersistsLongTaskFields(t *testing.T) {
	root := t.TempDir()
	store := NewFSStore(root)

	task, err := store.Create(TaskInput{
		Task:     "sleep 5",
		Executor: "shell",
		CWD:      root,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	pid := 4242
	progress := 0
	task.Status = "running"
	task.StartedAt = "2026-05-17T08:00:00Z"
	task.HeartbeatAt = "2026-05-17T08:00:05Z"
	task.ProgressPercent = &progress
	task.ProgressMessage = "streaming output"
	task.PID = &pid
	task.ResultPreview = "first line"
	task.StdoutSize = 128
	task.StderrSize = 16
	task.LastLogOffset = 144
	task.EventSeq = 7
	task.CancelRequested = true

	if _, err := store.Update(task.ID, task); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	saved, err := store.Get(task.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if saved.StartedAt != task.StartedAt {
		t.Fatalf("StartedAt = %q, want %q", saved.StartedAt, task.StartedAt)
	}
	if saved.HeartbeatAt != task.HeartbeatAt {
		t.Fatalf("HeartbeatAt = %q, want %q", saved.HeartbeatAt, task.HeartbeatAt)
	}
	if saved.ProgressPercent == nil || *saved.ProgressPercent != progress {
		t.Fatalf("ProgressPercent = %v, want pointer to %d", saved.ProgressPercent, progress)
	}
	if saved.ProgressMessage != task.ProgressMessage {
		t.Fatalf("ProgressMessage = %q, want %q", saved.ProgressMessage, task.ProgressMessage)
	}
	if saved.PID == nil || *saved.PID != pid {
		t.Fatalf("PID = %v, want %d", saved.PID, pid)
	}
	if saved.ResultPreview != task.ResultPreview {
		t.Fatalf("ResultPreview = %q, want %q", saved.ResultPreview, task.ResultPreview)
	}
	if saved.StdoutSize != task.StdoutSize {
		t.Fatalf("StdoutSize = %d, want %d", saved.StdoutSize, task.StdoutSize)
	}
	if saved.StderrSize != task.StderrSize {
		t.Fatalf("StderrSize = %d, want %d", saved.StderrSize, task.StderrSize)
	}
	if saved.LastLogOffset != task.LastLogOffset {
		t.Fatalf("LastLogOffset = %d, want %d", saved.LastLogOffset, task.LastLogOffset)
	}
	if saved.EventSeq != task.EventSeq {
		t.Fatalf("EventSeq = %d, want %d", saved.EventSeq, task.EventSeq)
	}
	if !saved.CancelRequested {
		t.Fatalf("CancelRequested = false, want true")
	}
}

func TestUpdateOnlyBackfillsCreatedAt(t *testing.T) {
	root := t.TempDir()
	store := NewFSStore(root)

	created, err := store.Create(TaskInput{
		Task:     "sleep 5",
		Executor: "shell",
		CWD:      root,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updatedAt := "2026-05-17T08:00:15Z"
	replacement := Task{
		UpdatedAt: updatedAt,
	}

	updated, err := store.Update(created.ID, replacement)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.CreatedAt != created.CreatedAt {
		t.Fatalf("CreatedAt = %q, want %q", updated.CreatedAt, created.CreatedAt)
	}
	if updated.Task != "" {
		t.Fatalf("Task = %q, want empty", updated.Task)
	}
	if updated.Executor != "" {
		t.Fatalf("Executor = %q, want empty", updated.Executor)
	}
	if updated.CWD != "" {
		t.Fatalf("CWD = %q, want empty", updated.CWD)
	}
	if updated.Status != "" {
		t.Fatalf("Status = %q, want empty", updated.Status)
	}

	saved, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if saved.CreatedAt != created.CreatedAt {
		t.Fatalf("saved CreatedAt = %q, want %q", saved.CreatedAt, created.CreatedAt)
	}
	if saved.UpdatedAt != updatedAt {
		t.Fatalf("saved UpdatedAt = %q, want %q", saved.UpdatedAt, updatedAt)
	}
	if saved.Task != "" {
		t.Fatalf("saved Task = %q, want empty", saved.Task)
	}
	if saved.Executor != "" {
		t.Fatalf("saved Executor = %q, want empty", saved.Executor)
	}
	if saved.CWD != "" {
		t.Fatalf("saved CWD = %q, want empty", saved.CWD)
	}
	if saved.Status != "" {
		t.Fatalf("saved Status = %q, want empty", saved.Status)
	}
}
