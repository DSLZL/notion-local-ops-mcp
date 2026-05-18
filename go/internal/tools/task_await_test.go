package tools

import (
	"testing"
	"time"
)

func TestAwaitTaskReturnsTerminalStateImmediately(t *testing.T) {
	stateDir := t.TempDir()
	store, task := createPollingTaskForTest(t, stateDir)
	task.Status = "succeeded"
	task.EventSeq = 9
	task = updatePollingTaskForTest(t, store, task)
	writePollingSummaryForTest(t, store, task.ID, "done")

	result := AwaitTask(stateDir, task.ID, 5, task.EventSeq, false, "stdout", 0)
	if !result.Success {
		t.Fatal("Success = false, want true")
	}
	if !result.Terminal {
		t.Fatal("Terminal = false, want true")
	}
	if result.Status != "succeeded" {
		t.Fatalf("Status = %q, want succeeded", result.Status)
	}
	if result.RecommendedNextAction != "stop" {
		t.Fatalf("RecommendedNextAction = %q, want stop", result.RecommendedNextAction)
	}
}

func TestAwaitTaskReturnsLatestRunningStateAfterTimeout(t *testing.T) {
	stateDir := t.TempDir()
	store, task := createPollingTaskForTest(t, stateDir)
	progress := 25
	task.Status = "running"
	task.EventSeq = 3
	task.ProgressPercent = &progress
	task.ProgressMessage = "working"
	task = updatePollingTaskForTest(t, store, task)

	start := time.Now()
	result := AwaitTask(stateDir, task.ID, 1, task.EventSeq, false, "stdout", 0)
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Fatalf("AwaitTask() took %v, want it to wait close to timeout", elapsed)
	}
	if !result.Success {
		t.Fatal("Success = false, want true")
	}
	if result.Terminal {
		t.Fatal("Terminal = true, want false")
	}
	if result.Status != "running" {
		t.Fatalf("Status = %q, want running", result.Status)
	}
	if result.RecommendedNextAction != "await_task" {
		t.Fatalf("RecommendedNextAction = %q, want await_task", result.RecommendedNextAction)
	}
}

func TestAwaitTaskIncludesOptionalLogs(t *testing.T) {
	stateDir := t.TempDir()
	store, task := createPollingTaskForTest(t, stateDir)
	task.Status = "succeeded"
	task.EventSeq = 7
	task = updatePollingTaskForTest(t, store, task)
	writePollingSummaryForTest(t, store, task.ID, "done")
	if err := store.WriteLogs(task.ID, "hello\nworld\n", ""); err != nil {
		t.Fatalf("WriteLogs() error = %v", err)
	}

	result := AwaitTask(stateDir, task.ID, 1, 0, true, "stdout", 5)
	if result.Logs == nil {
		t.Fatal("Logs = nil, want log payload")
	}
	if result.Logs.Content != "hello" {
		t.Fatalf("Logs.Content = %q, want hello", result.Logs.Content)
	}
	if !result.Logs.Truncated {
		t.Fatal("Logs.Truncated = false, want true")
	}
}

func TestAwaitTaskReturnsFailureWhenTaskMissing(t *testing.T) {
	stateDir := t.TempDir()

	result := AwaitTask(stateDir, "missing-task", 1, 0, false, "stdout", 0)
	if result.Success {
		t.Fatal("Success = true, want false")
	}
	if result.TaskID != "missing-task" {
		t.Fatalf("TaskID = %q, want %q", result.TaskID, "missing-task")
	}
	if result.Status != "failed" {
		t.Fatalf("Status = %q, want failed", result.Status)
	}
	if !result.Terminal {
		t.Fatal("Terminal = false, want true")
	}
	if result.RecommendedNextAction != "stop" {
		t.Fatalf("RecommendedNextAction = %q, want stop", result.RecommendedNextAction)
	}
}

func TestAwaitTaskDoesNotMaskLogReadFailure(t *testing.T) {
	stateDir := t.TempDir()
	store, task := createPollingTaskForTest(t, stateDir)
	task.Status = "succeeded"
	task.EventSeq = 4
	task = updatePollingTaskForTest(t, store, task)
	writePollingSummaryForTest(t, store, task.ID, "done")

	result := AwaitTask(stateDir, task.ID, 1, 0, true, "invalid-stream", 10)
	if result.Success {
		t.Fatal("Success = true, want false")
	}
	if result.Logs != nil {
		t.Fatal("Logs != nil, want nil on log read failure")
	}
	if result.Summary == "done" {
		t.Fatal("Summary did not include log read failure context")
	}
}

func TestAwaitTaskWaitsForTerminalUpdateWithinMaxWait(t *testing.T) {
	stateDir := t.TempDir()
	store, task := createPollingTaskForTest(t, stateDir)
	progress := 20
	task.Status = "running"
	task.EventSeq = 5
	task.ProgressPercent = &progress
	task.ProgressMessage = "warming"
	task = updatePollingTaskForTest(t, store, task)
	writePollingSummaryForTest(t, store, task.ID, "warming")

	baseline := task.EventSeq
	go func() {
		time.Sleep(150 * time.Millisecond)
		current, err := store.Get(task.ID)
		if err != nil {
			return
		}
		finalPercent := 100
		current.Status = "succeeded"
		current.EventSeq = baseline + 2
		current.ProgressPercent = &finalPercent
		current.ProgressMessage = "done"
		current.HeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := store.Update(task.ID, current); err != nil {
			return
		}
		_ = store.WriteSummary(task.ID, "done")
	}()

	start := time.Now()
	result := AwaitTask(stateDir, task.ID, 2, baseline, false, "stdout", 0)
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("AwaitTask() took %v, want it to wait for completion update", elapsed)
	}
	if !result.Success {
		t.Fatal("Success = false, want true")
	}
	if !result.Terminal {
		t.Fatal("Terminal = false, want true")
	}
	if result.Status != "succeeded" {
		t.Fatalf("Status = %q, want succeeded", result.Status)
	}
	if result.EventSeq != baseline+2 {
		t.Fatalf("EventSeq = %d, want %d", result.EventSeq, baseline+2)
	}
	if result.RecommendedNextAction != "stop" {
		t.Fatalf("RecommendedNextAction = %q, want stop", result.RecommendedNextAction)
	}
}

func TestAwaitTaskReturnsEarlyOnRunningEventSeqAdvance(t *testing.T) {
	stateDir := t.TempDir()
	store, task := createPollingTaskForTest(t, stateDir)
	initialProgress := 15
	task.Status = "running"
	task.EventSeq = 10
	task.ProgressPercent = &initialProgress
	task.ProgressMessage = "phase-1"
	task = updatePollingTaskForTest(t, store, task)
	writePollingSummaryForTest(t, store, task.ID, "phase-1")

	baseline := task.EventSeq
	go func() {
		time.Sleep(150 * time.Millisecond)
		current, err := store.Get(task.ID)
		if err != nil {
			return
		}
		nextProgress := 45
		current.Status = "running"
		current.EventSeq = baseline + 1
		current.ProgressPercent = &nextProgress
		current.ProgressMessage = "phase-2"
		current.HeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := store.Update(task.ID, current); err != nil {
			return
		}
		_ = store.WriteSummary(task.ID, "phase-2")
	}()

	start := time.Now()
	result := AwaitTask(stateDir, task.ID, 2, baseline, false, "stdout", 0)
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("AwaitTask() took %v, want it to block until progress update", elapsed)
	}
	if result.EventSeq != baseline+1 {
		t.Fatalf("EventSeq = %d, want %d", result.EventSeq, baseline+1)
	}
	if result.Status != "running" {
		t.Fatalf("Status = %q, want running", result.Status)
	}
	if result.Terminal {
		t.Fatal("Terminal = true, want false")
	}
	if result.RecommendedNextAction != "await_task" {
		t.Fatalf("RecommendedNextAction = %q, want await_task", result.RecommendedNextAction)
	}
	if !result.Success {
		t.Fatal("Success = false, want true")
	}
}
