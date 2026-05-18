package tools

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"notion-local-ops-mcp-go/internal/taskstore"
)

func TestWaitTaskReturnsTerminalTaskImmediately(t *testing.T) {
	stateDir := t.TempDir()
	store, task := createPollingTaskForTest(t, stateDir)
	task.Status = "succeeded"
	task.EventSeq = 9
	task.HeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
	task = updatePollingTaskForTest(t, store, task)
	writePollingSummaryForTest(t, store, task.ID, "task finished")

	begin := time.Now()
	result := WaitTask(stateDir, task.ID, 1, task.EventSeq)
	if elapsed := time.Since(begin); elapsed > 150*time.Millisecond {
		t.Fatalf("WaitTask() took %v, want immediate return for terminal task", elapsed)
	}
	if result.Status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", result.Status)
	}
	if result.EventSeq != task.EventSeq {
		t.Fatalf("EventSeq = %d, want %d", result.EventSeq, task.EventSeq)
	}
	if result.RecommendedPollStrategy != "stop" {
		t.Fatalf("RecommendedPollStrategy = %q, want stop", result.RecommendedPollStrategy)
	}
	if result.NextPollAfterSeconds != 0 {
		t.Fatalf("NextPollAfterSeconds = %d, want 0", result.NextPollAfterSeconds)
	}
	if result.Summary != "task finished" {
		t.Fatalf("Summary = %q, want %q", result.Summary, "task finished")
	}
}

func TestWaitTaskBlocksUntilEventSeqChanges(t *testing.T) {
	stateDir := t.TempDir()
	store, task := createPollingTaskForTest(t, stateDir)
	initialPercent := 10
	task.Status = "running"
	task.EventSeq = 3
	task.ProgressPercent = &initialPercent
	task.ProgressMessage = "warming up"
	task.HeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
	task = updatePollingTaskForTest(t, store, task)
	writePollingSummaryForTest(t, store, task.ID, "warming up")

	baselineEventSeq := task.EventSeq
	go func() {
		time.Sleep(150 * time.Millisecond)
		current, err := store.Get(task.ID)
		if err != nil {
			return
		}
		nextPercent := 80
		current.EventSeq = baselineEventSeq + 1
		current.ProgressPercent = &nextPercent
		current.ProgressMessage = "almost there"
		current.HeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := store.Update(task.ID, current); err != nil {
			return
		}
		_ = store.WriteSummary(task.ID, "almost there")
	}()

	begin := time.Now()
	result := WaitTask(stateDir, task.ID, 1, baselineEventSeq)
	if elapsed := time.Since(begin); elapsed < 100*time.Millisecond {
		t.Fatalf("WaitTask() took %v, want it to block until event_seq changes", elapsed)
	}
	if result.EventSeq != baselineEventSeq+1 {
		t.Fatalf("EventSeq = %d, want %d", result.EventSeq, baselineEventSeq+1)
	}
	if result.ProgressPercent == nil || *result.ProgressPercent != 80 {
		t.Fatalf("ProgressPercent = %v, want 80", result.ProgressPercent)
	}
	if result.ProgressMessage != "almost there" {
		t.Fatalf("ProgressMessage = %q, want %q", result.ProgressMessage, "almost there")
	}
	if result.RecommendedPollStrategy != "wait_task" {
		t.Fatalf("RecommendedPollStrategy = %q, want wait_task", result.RecommendedPollStrategy)
	}
	if result.NextPollAfterSeconds != 1 {
		t.Fatalf("NextPollAfterSeconds = %d, want 1", result.NextPollAfterSeconds)
	}
}

func TestWaitTaskReturnsLatestStateAfterTimeout(t *testing.T) {
	stateDir := t.TempDir()
	store, task := createPollingTaskForTest(t, stateDir)
	progressPercent := 42
	task.Status = "running"
	task.EventSeq = 7
	task.ProgressPercent = &progressPercent
	task.ProgressMessage = "halfway"
	task.HeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
	task = updatePollingTaskForTest(t, store, task)
	writePollingSummaryForTest(t, store, task.ID, "halfway")

	begin := time.Now()
	result := WaitTask(stateDir, task.ID, 1, task.EventSeq)
	if elapsed := time.Since(begin); elapsed < 900*time.Millisecond {
		t.Fatalf("WaitTask() took %v, want it to wait close to timeout when event_seq is unchanged", elapsed)
	}
	if result.TaskID != task.ID {
		t.Fatalf("TaskID = %q, want %q", result.TaskID, task.ID)
	}
	if result.Status != "running" {
		t.Fatalf("Status = %q, want running", result.Status)
	}
	if result.EventSeq != task.EventSeq {
		t.Fatalf("EventSeq = %d, want %d", result.EventSeq, task.EventSeq)
	}
	if result.ProgressPercent == nil || *result.ProgressPercent != progressPercent {
		t.Fatalf("ProgressPercent = %v, want %d", result.ProgressPercent, progressPercent)
	}
	if result.ProgressMessage != "halfway" {
		t.Fatalf("ProgressMessage = %q, want %q", result.ProgressMessage, "halfway")
	}
	if result.HeartbeatAt == "" {
		t.Fatal("HeartbeatAt must not be empty")
	}
	if result.RecommendedPollStrategy != "wait_task" {
		t.Fatalf("RecommendedPollStrategy = %q, want wait_task", result.RecommendedPollStrategy)
	}
	if result.NextPollAfterSeconds != 1 {
		t.Fatalf("NextPollAfterSeconds = %d, want 1", result.NextPollAfterSeconds)
	}
	if result.Summary != "halfway" {
		t.Fatalf("Summary = %q, want %q", result.Summary, "halfway")
	}
}

func TestWaitTaskReturnsTerminalTaskWhenItCompletesDuringWait(t *testing.T) {
	stateDir := t.TempDir()
	store, task := createPollingTaskForTest(t, stateDir)
	initialPercent := 15
	task.Status = "running"
	task.EventSeq = 4
	task.ProgressPercent = &initialPercent
	task.ProgressMessage = "working"
	task.HeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
	task = updatePollingTaskForTest(t, store, task)
	writePollingSummaryForTest(t, store, task.ID, "working")

	baselineEventSeq := task.EventSeq
	go func() {
		time.Sleep(150 * time.Millisecond)
		current, err := store.Get(task.ID)
		if err != nil {
			return
		}
		finalPercent := 100
		current.Status = "succeeded"
		current.EventSeq = baselineEventSeq + 2
		current.ProgressPercent = &finalPercent
		current.ProgressMessage = "done"
		current.HeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := store.Update(task.ID, current); err != nil {
			return
		}
		_ = store.WriteSummary(task.ID, "done")
	}()

	begin := time.Now()
	result := WaitTask(stateDir, task.ID, 1, baselineEventSeq)
	if elapsed := time.Since(begin); elapsed < 100*time.Millisecond {
		t.Fatalf("WaitTask() took %v, want it to block until task completion is observed", elapsed)
	}
	if result.Status != "succeeded" {
		t.Fatalf("Status = %q, want succeeded", result.Status)
	}
	if result.EventSeq != baselineEventSeq+2 {
		t.Fatalf("EventSeq = %d, want %d", result.EventSeq, baselineEventSeq+2)
	}
	if result.ProgressPercent == nil || *result.ProgressPercent != 100 {
		t.Fatalf("ProgressPercent = %v, want 100", result.ProgressPercent)
	}
	if result.ProgressMessage != "done" {
		t.Fatalf("ProgressMessage = %q, want %q", result.ProgressMessage, "done")
	}
	if result.RecommendedPollStrategy != "stop" {
		t.Fatalf("RecommendedPollStrategy = %q, want stop", result.RecommendedPollStrategy)
	}
	if result.NextPollAfterSeconds != 0 {
		t.Fatalf("NextPollAfterSeconds = %d, want 0", result.NextPollAfterSeconds)
	}
	if result.Summary != "done" {
		t.Fatalf("Summary = %q, want %q", result.Summary, "done")
	}
}

func TestWaitTaskIgnoresTransientReadFailuresDuringWait(t *testing.T) {
	stateDir := t.TempDir()
	store, task := createPollingTaskForTest(t, stateDir)
	initialPercent := 10
	task.Status = "running"
	task.EventSeq = 3
	task.ProgressPercent = &initialPercent
	task.ProgressMessage = "warming up"
	task.HeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
	task = updatePollingTaskForTest(t, store, task)
	writePollingSummaryForTest(t, store, task.ID, "warming up")

	metaPath := filepath.Join(stateDir, "tasks", task.ID, "meta.json")
	tmpPath := metaPath + ".bak"
	baselineEventSeq := task.EventSeq
	go func() {
		time.Sleep(100 * time.Millisecond)
		if err := os.Rename(metaPath, tmpPath); err != nil {
			return
		}
		time.Sleep(75 * time.Millisecond)
		_ = os.Rename(tmpPath, metaPath)
		time.Sleep(75 * time.Millisecond)

		current, err := store.Get(task.ID)
		if err != nil {
			return
		}
		nextPercent := 80
		current.EventSeq = baselineEventSeq + 1
		current.ProgressPercent = &nextPercent
		current.ProgressMessage = "almost there"
		current.HeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := store.Update(task.ID, current); err != nil {
			return
		}
		_ = store.WriteSummary(task.ID, "almost there")
	}()

	result := WaitTask(stateDir, task.ID, 1, baselineEventSeq)
	if result.Status != "running" {
		t.Fatalf("Status = %q, want running after transient read failure recovery", result.Status)
	}
	if result.EventSeq != baselineEventSeq+1 {
		t.Fatalf("EventSeq = %d, want %d", result.EventSeq, baselineEventSeq+1)
	}
	if result.RecommendedPollStrategy != "wait_task" {
		t.Fatalf("RecommendedPollStrategy = %q, want wait_task", result.RecommendedPollStrategy)
	}
	if result.NextPollAfterSeconds != 1 {
		t.Fatalf("NextPollAfterSeconds = %d, want 1", result.NextPollAfterSeconds)
	}
	if result.Summary != "almost there" {
		t.Fatalf("Summary = %q, want %q", result.Summary, "almost there")
	}
}

func TestGetTaskReturnsRicherPollingFields(t *testing.T) {
	stateDir := t.TempDir()
	store, task := createPollingTaskForTest(t, stateDir)
	progressPercent := 42
	task.Status = "running"
	task.EventSeq = 7
	task.ProgressPercent = &progressPercent
	task.ProgressMessage = "halfway"
	task.HeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
	task = updatePollingTaskForTest(t, store, task)
	writePollingSummaryForTest(t, store, task.ID, "halfway")

	result := GetTask(stateDir, task.ID)
	if result.TaskID != task.ID {
		t.Fatalf("TaskID = %q, want %q", result.TaskID, task.ID)
	}
	if result.Status != "running" {
		t.Fatalf("Status = %q, want running", result.Status)
	}
	if result.EventSeq != 7 {
		t.Fatalf("EventSeq = %d, want 7", result.EventSeq)
	}
	if result.ProgressPercent == nil || *result.ProgressPercent != 42 {
		t.Fatalf("ProgressPercent = %v, want 42", result.ProgressPercent)
	}
	if result.ProgressMessage != "halfway" {
		t.Fatalf("ProgressMessage = %q, want %q", result.ProgressMessage, "halfway")
	}
	if result.HeartbeatAt == "" {
		t.Fatal("HeartbeatAt must not be empty")
	}
	if result.RecommendedPollStrategy != "wait_task" {
		t.Fatalf("RecommendedPollStrategy = %q, want wait_task", result.RecommendedPollStrategy)
	}
	if result.NextPollAfterSeconds != 1 {
		t.Fatalf("NextPollAfterSeconds = %d, want 1", result.NextPollAfterSeconds)
	}
	if result.Summary != "halfway" {
		t.Fatalf("Summary = %q, want %q", result.Summary, "halfway")
	}
}

func TestCancelTaskReturnsTerminalTaskState(t *testing.T) {
	submitted := RunCommandStreamForTest()
	if submitted.TaskID == "" {
		t.Fatal("task_id must not be empty")
	}
	waited := waitForTaskCompletionForTest(submitted)
	if waited.Status != "succeeded" {
		t.Fatalf("wait status = %q, want succeeded", waited.Status)
	}

	result := CancelTaskForTest(submitted.TaskID)
	if result.TaskID == "" {
		t.Fatal("task_id must not be empty")
	}
	if result.Status != "succeeded" {
		t.Fatalf("status = %q, want succeeded for already completed task", result.Status)
	}
}

func TestCancelTaskCancelsRunningTask(t *testing.T) {
	stateDir := t.TempDir()
	workspace := t.TempDir()

	submitted := RunCommandStream(stateDir, workspace, slowStreamCommand(), "", 5)
	if submitted.TaskID == "" {
		t.Fatal("task_id must not be empty")
	}

	store := taskstore.NewFSStore(stateDir)
	waitForTaskState(t, store, submitted.TaskID, func(task taskstore.Task) bool {
		return task.Status == "running" && task.PID != nil && *task.PID > 0
	}, "running task before cancellation")

	result := CancelTask(stateDir, submitted.TaskID)
	if result.TaskID != submitted.TaskID {
		t.Fatalf("TaskID = %q, want %q", result.TaskID, submitted.TaskID)
	}
	if result.Status != "running" && result.Status != "cancelled" {
		t.Fatalf("Status = %q, want running or cancelled immediately after cancel request", result.Status)
	}

	cancelled := waitForTaskState(t, store, submitted.TaskID, func(task taskstore.Task) bool {
		return task.Status == "cancelled"
	}, "cancelled task")
	if !cancelled.CancelRequested {
		t.Fatal("CancelRequested = false, want true")
	}
	if cancelled.FinishedAt == "" {
		t.Fatal("FinishedAt must not be empty after cancellation")
	}

	final := GetTask(stateDir, submitted.TaskID)
	if final.Status != "cancelled" {
		t.Fatalf("final status = %q, want cancelled", final.Status)
	}
	if final.RecommendedPollStrategy != "stop" {
		t.Fatalf("RecommendedPollStrategy = %q, want stop", final.RecommendedPollStrategy)
	}
	if final.NextPollAfterSeconds != 0 {
		t.Fatalf("NextPollAfterSeconds = %d, want 0", final.NextPollAfterSeconds)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		final = GetTask(stateDir, submitted.TaskID)
		if final.Summary == "cancelled" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Summary = %q, want cancelled", final.Summary)
}

func createPollingTaskForTest(t *testing.T, stateDir string) (*taskstore.FSStore, taskstore.Task) {
	t.Helper()

	store := taskstore.NewFSStore(stateDir)
	task, err := store.Create(taskstore.TaskInput{
		Task:     "echo stream-ok",
		Executor: "run_command_stream",
		CWD:      ".",
		Timeout:  5,
	})
	if err != nil {
		t.Fatalf("store.Create() error = %v", err)
	}
	return store, task
}

func updatePollingTaskForTest(t *testing.T, store *taskstore.FSStore, task taskstore.Task) taskstore.Task {
	t.Helper()

	updated, err := store.Update(task.ID, task)
	if err != nil {
		t.Fatalf("store.Update() error = %v", err)
	}
	return updated
}

func writePollingSummaryForTest(t *testing.T, store *taskstore.FSStore, taskID, summary string) {
	t.Helper()

	if err := store.WriteSummary(taskID, summary); err != nil {
		t.Fatalf("store.WriteSummary() error = %v", err)
	}
}

func waitForTaskState(t *testing.T, store taskstore.Store, taskID string, predicate func(taskstore.Task) bool, description string) taskstore.Task {
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
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("task %s last status = %q after timeout, waiting for %s", taskID, latest.Status, description)
	return taskstore.Task{}
}
