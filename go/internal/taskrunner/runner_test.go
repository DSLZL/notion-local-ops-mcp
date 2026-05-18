package taskrunner

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"notion-local-ops-mcp-go/internal/process"
	"notion-local-ops-mcp-go/internal/taskstore"
)

func TestSubmitDoesNotBlockUntilExecutionCompletes(t *testing.T) {
	store := taskstore.NewFSStore(t.TempDir())
	runner := NewRunner(store)

	release := make(chan struct{})
	started := make(chan struct{})

	begin := time.Now()
	submitted, err := runner.Submit(SubmitOptions{
		Task: taskstore.TaskInput{
			Task:     "echo async-ok",
			Executor: "runner_test",
			CWD:      ".",
			Timeout:  7,
		},
		Execute: func(ctx context.Context, onStart StartCallback) (process.Result, error) {
			onStart(4242)
			close(started)
			<-release
			return process.Result{Stdout: "async-ok\n", ExitCode: 0}, nil
		},
	})
	elapsed := time.Since(begin)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if elapsed > 250*time.Millisecond {
		t.Fatalf("Submit() took %v, want immediate return", elapsed)
	}
	if submitted.ID == "" {
		t.Fatal("Submit() must return task id")
	}
	if submitted.Status != "queued" {
		t.Fatalf("Submit() status = %q, want queued", submitted.Status)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background execution did not start")
	}

	running := waitForTaskMatch(t, store, submitted.ID, func(task taskstore.Task) bool {
		return task.Status == "running" && task.PID != nil && *task.PID == 4242 && task.HeartbeatAt != ""
	}, "running task with pid and heartbeat")
	if running.Timeout != 7 {
		t.Fatalf("Timeout = %d, want 7", running.Timeout)
	}
	if running.EventSeq == 0 {
		t.Fatal("EventSeq must advance when task starts running")
	}

	firstHeartbeat := running.HeartbeatAt
	firstEventSeq := running.EventSeq
	time.Sleep(150 * time.Millisecond)
	running = waitForTaskMatch(t, store, submitted.ID, func(task taskstore.Task) bool {
		return task.Status == "running" && task.HeartbeatAt != "" && task.HeartbeatAt != firstHeartbeat && task.EventSeq > firstEventSeq
	}, "running task with refreshed heartbeat")
	if running.EventSeq <= firstEventSeq {
		t.Fatalf("EventSeq = %d, want > %d after heartbeat", running.EventSeq, firstEventSeq)
	}

	close(release)
	waitForTaskMatch(t, store, submitted.ID, func(task taskstore.Task) bool {
		return task.Status == "succeeded"
	}, "succeeded task")
}

func TestSubmitEventuallyMarksTaskSucceeded(t *testing.T) {
	store := taskstore.NewFSStore(t.TempDir())
	runner := NewRunner(store)

	submitted, err := runner.Submit(SubmitOptions{
		Task: taskstore.TaskInput{
			Task:     "echo stream-ok",
			Executor: "runner_test",
			CWD:      ".",
			Timeout:  5,
		},
		Execute: func(ctx context.Context, onStart StartCallback) (process.Result, error) {
			onStart(31337)
			return process.Result{Stdout: "stream-ok\n", ExitCode: 0}, nil
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	task := waitForTaskMatch(t, store, submitted.ID, func(task taskstore.Task) bool {
		return task.Status == "succeeded"
	}, "succeeded task")
	if task.StartedAt == "" {
		t.Fatal("StartedAt must not be empty")
	}
	if task.FinishedAt == "" {
		t.Fatal("FinishedAt must not be empty")
	}
	if task.HeartbeatAt == "" {
		t.Fatal("HeartbeatAt must not be empty")
	}
	if task.PID == nil {
		t.Fatal("PID must not be nil")
	}
	if *task.PID != 31337 {
		t.Fatalf("PID = %d, want 31337", *task.PID)
	}
	if task.ExitCode == nil {
		t.Fatal("ExitCode must not be nil")
	}
	if *task.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", *task.ExitCode)
	}
	if got := waitForSummary(t, store, submitted.ID, "stream-ok"); got != "stream-ok" {
		t.Fatalf("summary = %q, want %q", got, "stream-ok")
	}
	if got := store.ReadStdout(submitted.ID); got != "stream-ok\n" && got != "stream-ok\r\n" {
		t.Fatalf("stdout = %q, want stream-ok newline", got)
	}
}

func TestSubmitCapturesStructuredProgressFromStdout(t *testing.T) {
	store := newProgressObservingStore(taskstore.NewFSStore(t.TempDir()))
	runner := NewRunner(store)

	submitted, err := runner.Submit(SubmitOptions{
		Task: taskstore.TaskInput{
			Task:     "echo progress",
			Executor: "runner_test",
			CWD:      ".",
			Timeout:  5,
		},
		Execute: func(ctx context.Context, onStart StartCallback) (process.Result, error) {
			onStart(777)
			return process.Result{
				Stdout:   "booting\nMCP_PROGRESS {\"percent\":25,\"message\":\"starting\"}\nMCP_PROGRESS {\"percent\":80,\"message\":\"almost there\"}\ndone\n",
				ExitCode: 0,
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	progressEvent := store.waitForProgress(t)
	if progressEvent.Before.EventSeq == 0 {
		t.Fatal("progress event baseline EventSeq must not be zero")
	}
	if progressEvent.After.EventSeq <= progressEvent.Before.EventSeq {
		t.Fatalf("progress EventSeq = %d, want > %d", progressEvent.After.EventSeq, progressEvent.Before.EventSeq)
	}

	task := waitForTaskMatch(t, store, submitted.ID, func(task taskstore.Task) bool {
		return task.Status == "succeeded"
	}, "succeeded task with progress")

	if task.ProgressPercent == nil {
		t.Fatal("ProgressPercent must not be nil")
	}
	if *task.ProgressPercent != 80 {
		t.Fatalf("ProgressPercent = %d, want 80", *task.ProgressPercent)
	}
	if task.ProgressMessage != "almost there" {
		t.Fatalf("ProgressMessage = %q, want %q", task.ProgressMessage, "almost there")
	}
	if task.EventSeq <= progressEvent.After.EventSeq {
		t.Fatalf("terminal EventSeq = %d, want > %d", task.EventSeq, progressEvent.After.EventSeq)
	}
	if got := waitForSummary(t, store, submitted.ID, "booting\ndone"); got != "booting\ndone" {
		t.Fatalf("summary = %q, want %q", got, "booting\ndone")
	}
	if got := store.ReadStdout(submitted.ID); got != "booting\nMCP_PROGRESS {\"percent\":25,\"message\":\"starting\"}\nMCP_PROGRESS {\"percent\":80,\"message\":\"almost there\"}\ndone\n" {
		t.Fatalf("stdout = %q, want original stdout with progress markers", got)
	}
	if task.StdoutSize != int64(len("booting\nMCP_PROGRESS {\"percent\":25,\"message\":\"starting\"}\nMCP_PROGRESS {\"percent\":80,\"message\":\"almost there\"}\ndone\n")) {
		t.Fatalf("StdoutSize = %d, want %d", task.StdoutSize, len("booting\nMCP_PROGRESS {\"percent\":25,\"message\":\"starting\"}\nMCP_PROGRESS {\"percent\":80,\"message\":\"almost there\"}\ndone\n"))
	}
	if task.LastLogOffset != task.StdoutSize {
		t.Fatalf("LastLogOffset = %d, want %d", task.LastLogOffset, task.StdoutSize)
	}
}

func TestSubmitKeepsInvalidProgressLinesInSummary(t *testing.T) {
	store := taskstore.NewFSStore(t.TempDir())
	runner := NewRunner(store)

	const stdout = "MCP_PROGRESS {\"percent\":80} trailing text\nMCP_PROGRESS {\"percent\":-1,\"message\":\"bad\"}\nMCP_PROGRESS {\"percent\":101,\"message\":\"too much\"}\n"

	submitted, err := runner.Submit(SubmitOptions{
		Task: taskstore.TaskInput{
			Task:     "echo invalid-progress",
			Executor: "runner_test",
			CWD:      ".",
			Timeout:  5,
		},
		Execute: func(ctx context.Context, onStart StartCallback) (process.Result, error) {
			onStart(9090)
			return process.Result{
				Stdout:   stdout,
				ExitCode: 0,
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	task := waitForTaskMatch(t, store, submitted.ID, func(task taskstore.Task) bool {
		return task.Status == "succeeded"
	}, "succeeded task with invalid progress markers")

	if task.ProgressPercent != nil {
		t.Fatalf("ProgressPercent = %v, want nil", *task.ProgressPercent)
	}
	if task.ProgressMessage != "" {
		t.Fatalf("ProgressMessage = %q, want empty", task.ProgressMessage)
	}
	if got := waitForSummary(t, store, submitted.ID, strings.TrimSpace(stdout)); got != strings.TrimSpace(stdout) {
		t.Fatalf("summary = %q, want %q", got, strings.TrimSpace(stdout))
	}
	if got := store.ReadStdout(submitted.ID); got != stdout {
		t.Fatalf("stdout = %q, want %q", got, stdout)
	}
	if task.StdoutSize != int64(len(stdout)) {
		t.Fatalf("StdoutSize = %d, want %d", task.StdoutSize, len(stdout))
	}
	if task.LastLogOffset != task.StdoutSize {
		t.Fatalf("LastLogOffset = %d, want %d", task.LastLogOffset, task.StdoutSize)
	}
}

func TestCancelStopsRunningTask(t *testing.T) {
	store := taskstore.NewFSStore(t.TempDir())
	runner := NewRunner(store)

	started := make(chan struct{})
	submitted, err := runner.Submit(SubmitOptions{
		Task: taskstore.TaskInput{
			Task:     "cancel-me",
			Executor: "runner_test",
			CWD:      ".",
			Timeout:  5,
		},
		Execute: func(ctx context.Context, onStart StartCallback) (process.Result, error) {
			onStart(5150)
			close(started)
			<-ctx.Done()
			return process.Result{ExitCode: -1, Stderr: ctx.Err().Error()}, ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background execution did not start")
	}

	running := waitForTaskMatch(t, store, submitted.ID, func(task taskstore.Task) bool {
		return task.Status == "running" && task.PID != nil && *task.PID == 5150
	}, "running task before cancellation")
	if running.CancelRequested {
		t.Fatal("CancelRequested = true, want false before cancellation")
	}

	if err := runner.Cancel(submitted.ID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	cancelled := waitForTaskTerminalState(t, store, submitted.ID, "cancelled")
	if !cancelled.CancelRequested {
		t.Fatal("CancelRequested = false, want true after cancellation")
	}
	if cancelled.ExitCode == nil || *cancelled.ExitCode != -1 {
		t.Fatalf("ExitCode = %v, want -1 after cancellation", cancelled.ExitCode)
	}
	if got := waitForSummary(t, store, submitted.ID, "cancelled"); got != "cancelled" {
		t.Fatalf("summary = %q, want cancelled", got)
	}
}

func TestLateCancelDoesNotOverrideCompletedTask(t *testing.T) {
	store := taskstore.NewFSStore(t.TempDir())
	runner := NewRunner(store)

	executeDone := make(chan struct{})
	submitted, err := runner.Submit(SubmitOptions{
		Task: taskstore.TaskInput{
			Task:     "finish-before-cancel",
			Executor: "runner_test",
			CWD:      ".",
			Timeout:  5,
		},
		Execute: func(ctx context.Context, onStart StartCallback) (process.Result, error) {
			onStart(6262)
			close(executeDone)
			return process.Result{Stdout: "done\n", ExitCode: 0}, nil
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	select {
	case <-executeDone:
	case <-time.After(time.Second):
		t.Fatal("execution did not complete")
	}

	if err := runner.Cancel(submitted.ID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	completed := waitForTaskTerminalState(t, store, submitted.ID, "succeeded")
	if completed.CancelRequested {
		t.Fatal("CancelRequested = true, want false for late cancel after completion")
	}
	if got := waitForSummary(t, store, submitted.ID, "done"); got != "done" {
		t.Fatalf("summary = %q, want done", got)
	}
}

type progressObservingStore struct {
	taskstore.Store
	progressSeen chan progressEvent
	once         sync.Once
}

type progressEvent struct {
	Before taskstore.Task
	After  taskstore.Task
}

func newProgressObservingStore(inner taskstore.Store) *progressObservingStore {
	return &progressObservingStore{
		Store:        inner,
		progressSeen: make(chan progressEvent, 1),
	}
}

func (s *progressObservingStore) UpdateStatus(taskID string, mutate func(*taskstore.Task) error) (taskstore.Task, error) {
	before, err := s.Store.Get(taskID)
	if err != nil {
		return taskstore.Task{}, err
	}

	updated, err := s.Store.UpdateStatus(taskID, mutate)
	if err != nil {
		return updated, err
	}
	progressChanged := !intPtrEqual(before.ProgressPercent, updated.ProgressPercent) || before.ProgressMessage != updated.ProgressMessage
	if updated.Status == "running" && progressChanged {
		s.once.Do(func() {
			s.progressSeen <- progressEvent{Before: before, After: updated}
		})
	}
	return updated, nil
}

func (s *progressObservingStore) waitForProgress(t *testing.T) progressEvent {
	t.Helper()

	select {
	case event := <-s.progressSeen:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for progress update")
		return progressEvent{}
	}
}

func intPtrEqual(left, right *int) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func waitForTaskMatch(t *testing.T, store taskstore.Store, taskID string, predicate func(taskstore.Task) bool, description string) taskstore.Task {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
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

func waitForTaskTerminalState(t *testing.T, store taskstore.Store, taskID, wantStatus string) taskstore.Task {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	var latest taskstore.Task
	for time.Now().Before(deadline) {
		task, err := store.Get(taskID)
		if err == nil {
			latest = task
			if task.Status == wantStatus {
				return task
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("task %s last status = %q after timeout, want terminal status %q", taskID, latest.Status, wantStatus)
	return taskstore.Task{}
}

func waitForSummary(t *testing.T, store taskstore.Store, taskID string, want string) string {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got := store.ReadSummary(taskID)
		if got == want {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}

	return store.ReadSummary(taskID)
}
