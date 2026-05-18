package taskrunner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"notion-local-ops-mcp-go/internal/process"
	"notion-local-ops-mcp-go/internal/taskstore"
)

const (
	heartbeatInterval = 50 * time.Millisecond
	progressPrefix    = "MCP_PROGRESS "
)

var (
	errMissingStore   = errors.New("task store is required")
	errMissingExecute = errors.New("task execute function is required")
)

type StartCallback func(pid int)
type ExecuteFunc func(ctx context.Context, onStart StartCallback) (process.Result, error)

type SubmitOptions struct {
	Task    taskstore.TaskInput
	Execute ExecuteFunc
}

type Runner struct {
	store taskstore.Store
}

type activeTaskControl struct {
	mu       sync.Mutex
	cancel   context.CancelFunc
	cancelCh chan struct{}
	finished bool
}

var (
	activeTasksMu sync.Mutex
	activeTasks   = map[string]*activeTaskControl{}
)

func NewRunner(store taskstore.Store) *Runner {
	return &Runner{store: store}
}

func (r *Runner) Submit(options SubmitOptions) (taskstore.Task, error) {
	if r == nil || r.store == nil {
		return taskstore.Task{}, errMissingStore
	}
	if options.Execute == nil {
		return taskstore.Task{}, errMissingExecute
	}

	task, err := r.store.Create(options.Task)
	if err != nil {
		return taskstore.Task{}, err
	}

	executeCtx, cancel := context.WithCancel(context.Background())
	control := &activeTaskControl{
		cancel:   cancel,
		cancelCh: make(chan struct{}, 1),
	}
	registerActiveTask(task.ID, control)

	go r.run(task, control, executeCtx, options.Execute)
	return task, nil
}

func (r *Runner) Cancel(taskID string) error {
	if r == nil || r.store == nil {
		return errMissingStore
	}

	control := getActiveTask(taskID)
	if control == nil {
		return nil
	}
	control.requestCancel()
	return nil
}

func (r *Runner) run(task taskstore.Task, control *activeTaskControl, executeCtx context.Context, execute ExecuteFunc) {
	defer unregisterActiveTask(task.ID)

	var mu sync.Mutex

	updateTask := func(bumpEvent bool, mutate func(*taskstore.Task)) error {
		mu.Lock()
		defer mu.Unlock()

		updated, err := r.store.UpdateStatus(task.ID, func(current *taskstore.Task) error {
			mutate(current)
			if bumpEvent {
				current.EventSeq++
			}
			return nil
		})
		if err == nil {
			task = updated
		}
		return err
	}

	startedAt := timestamp()
	runningUpdateErr := updateTask(true, func(current *taskstore.Task) {
		current.Status = "running"
		current.StartedAt = startedAt
		current.HeartbeatAt = startedAt
		current.UpdatedAt = startedAt
	})

	done := make(chan struct{})
	var heartbeatWG sync.WaitGroup
	heartbeatWG.Add(1)
	go func() {
		defer heartbeatWG.Done()

		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = updateTask(true, func(current *taskstore.Task) {
					current.HeartbeatAt = timestamp()
					current.UpdatedAt = current.HeartbeatAt
				})
			}
		}
	}()

	type executeOutcome struct {
		result process.Result
		err    error
	}
	outcomeCh := make(chan executeOutcome, 1)
	go func() {
		result, err := execute(executeCtx, func(pid int) {
			if pid <= 0 {
				return
			}
			_ = updateTask(true, func(current *taskstore.Task) {
				current.PID = intPtr(pid)
				current.HeartbeatAt = timestamp()
				current.UpdatedAt = current.HeartbeatAt
			})
		})
		outcomeCh <- executeOutcome{result: result, err: err}
	}()

	var (
		result           process.Result
		runErr           error
		cancelRequested  bool
		executionSettled bool
	)
	cancelSignals := (<-chan struct{})(nil)
	if control != nil {
		cancelSignals = control.cancelRequested()
	}
	for !executionSettled {
		select {
		case outcome := <-outcomeCh:
			result = outcome.result
			runErr = outcome.err
			executionSettled = true
		default:
		}
		if executionSettled {
			break
		}
		select {
		case outcome := <-outcomeCh:
			result = outcome.result
			runErr = outcome.err
			executionSettled = true
		case <-cancelSignals:
			select {
			case outcome := <-outcomeCh:
				result = outcome.result
				runErr = outcome.err
				executionSettled = true
			default:
			}
			if executionSettled || cancelRequested {
				continue
			}
			cancelRequested = true
			_ = updateTask(true, func(current *taskstore.Task) {
				if current.CancelRequested || isTerminalStatus(current.Status) {
					return
				}
				current.CancelRequested = true
			})
			control.cancelExecution()
		}
	}

	outputPersistErr := r.persistResultOutput(task.ID, result, func(mutate func(*taskstore.Task)) error {
		return updateTask(true, mutate)
	})
	close(done)
	heartbeatWG.Wait()

	if runErr != nil && result.ExitCode == 0 {
		result.ExitCode = -1
	}

	status, summary := taskOutcome(result, runErr, cancelRequested)
	if outputPersistErr != nil {
		status = "failed"
		summary = persistenceFailureSummary(summary, runningUpdateErr, outputPersistErr)
	}

	finishedAt := timestamp()
	terminalUpdateErr := updateTask(true, func(current *taskstore.Task) {
		current.Status = status
		current.FinishedAt = finishedAt
		current.HeartbeatAt = finishedAt
		current.UpdatedAt = finishedAt
		current.ResultPreview = summary
		current.ExitCode = intPtr(result.ExitCode)
	})
	if terminalUpdateErr != nil {
		failureSummary := persistenceFailureSummary(summary, runningUpdateErr, terminalUpdateErr)
		r.bestEffortPersistFailure(func(mutate func(*taskstore.Task)) error {
			return updateTask(true, mutate)
		}, task, result, failureSummary)
		return
	}

	if err := r.store.WriteSummary(task.ID, summary); err != nil {
		failureSummary := persistenceFailureSummary(summary, runningUpdateErr, err)
		r.bestEffortPersistFailure(func(mutate func(*taskstore.Task)) error {
			return updateTask(true, mutate)
		}, task, result, failureSummary)
	}
}

func (r *Runner) persistResultOutput(taskID string, result process.Result, updateTask func(func(*taskstore.Task)) error) error {
	if _, err := r.appendStream(taskID, "stdout", result.Stdout, func(chunk string) error {
		progress, ok := parseProgressUpdate(chunk)
		if !ok {
			return nil
		}
		return updateTask(func(current *taskstore.Task) {
			if progress.Percent != nil {
				current.ProgressPercent = intPtr(*progress.Percent)
			}
			if progress.HasMessage {
				current.ProgressMessage = progress.Message
			}
			current.HeartbeatAt = timestamp()
			current.UpdatedAt = current.HeartbeatAt
		})
	}); err != nil {
		return err
	}

	if _, err := r.appendStream(taskID, "stderr", result.Stderr, nil); err != nil {
		return err
	}

	return nil
}

func (r *Runner) appendStream(taskID, stream, content string, afterAppend func(string) error) (int64, error) {
	if content == "" {
		return 0, nil
	}

	var offset int64
	for _, chunk := range splitOutputChunks(content) {
		if chunk == "" {
			continue
		}

		nextOffset, err := r.store.AppendLog(taskID, stream, chunk)
		if err != nil {
			return offset, err
		}
		offset = nextOffset

		if afterAppend != nil {
			if err := afterAppend(chunk); err != nil {
				return offset, err
			}
		}
	}

	return offset, nil
}

func (r *Runner) bestEffortPersistFailure(updateTask func(func(*taskstore.Task)) error, task taskstore.Task, result process.Result, summary string) {
	_ = r.store.WriteSummary(task.ID, summary)

	failedAt := timestamp()
	_ = updateTask(func(current *taskstore.Task) {
		current.Status = "failed"
		current.FinishedAt = failedAt
		current.HeartbeatAt = failedAt
		current.UpdatedAt = failedAt
		current.ResultPreview = summary
		current.ExitCode = intPtr(result.ExitCode)
	})

	_ = r.store.WriteSummary(task.ID, summary)
}

func taskOutcome(result process.Result, runErr error, cancelRequested bool) (string, string) {
	if cancelRequested && errors.Is(runErr, context.Canceled) {
		return "cancelled", "cancelled"
	}
	if runErr != nil {
		summary := strings.TrimSpace(result.Stderr)
		if summary == "" {
			summary = runErr.Error()
		}
		if summary == "" {
			summary = "failed"
		}
		return "failed", summary
	}

	summary := strings.TrimSpace(stripProgressLines(result.Stdout))
	if summary == "" {
		summary = "completed"
	}
	return "succeeded", summary
}

func isTerminalStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func registerActiveTask(taskID string, control *activeTaskControl) {
	activeTasksMu.Lock()
	defer activeTasksMu.Unlock()

	activeTasks[taskID] = control
}

func getActiveTask(taskID string) *activeTaskControl {
	activeTasksMu.Lock()
	defer activeTasksMu.Unlock()

	return activeTasks[taskID]
}

func unregisterActiveTask(taskID string) {
	activeTasksMu.Lock()
	defer activeTasksMu.Unlock()

	if control := activeTasks[taskID]; control != nil {
		control.markFinished()
	}
	delete(activeTasks, taskID)
}

func (c *activeTaskControl) requestCancel() {
	if c == nil {
		return
	}
	c.mu.Lock()
	finished := c.finished
	cancelCh := c.cancelCh
	c.mu.Unlock()

	if finished {
		return
	}
	select {
	case cancelCh <- struct{}{}:
	default:
	}
}

func (c *activeTaskControl) cancelRequested() <-chan struct{} {
	if c == nil {
		return nil
	}
	return c.cancelCh
}

func (c *activeTaskControl) cancelExecution() {
	if c == nil {
		return
	}
	c.mu.Lock()
	cancel := c.cancel
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (c *activeTaskControl) markFinished() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.finished = true
}

type progressUpdate struct {
	Percent    *int
	Message    string
	HasMessage bool
}

func parseProgressUpdate(chunk string) (progressUpdate, bool) {
	line := strings.TrimSpace(chunk)
	if !strings.HasPrefix(line, progressPrefix) {
		return progressUpdate{}, false
	}

	payload := strings.TrimSpace(strings.TrimPrefix(line, progressPrefix))
	if payload == "" {
		return progressUpdate{}, false
	}

	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.UseNumber()

	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		return progressUpdate{}, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return progressUpdate{}, false
	}

	var update progressUpdate
	hasPercent := false
	if value, ok := raw["percent"]; ok {
		hasPercent = true
		number, ok := value.(json.Number)
		if ok {
			if parsed, err := number.Int64(); err == nil {
				if parsed >= 0 && parsed <= 100 {
					percent := int(parsed)
					update.Percent = &percent
				}
			}
		}
		if update.Percent == nil {
			return progressUpdate{}, false
		}
	}
	if value, ok := raw["message"]; ok {
		message, ok := value.(string)
		if ok {
			update.Message = message
			update.HasMessage = true
		}
	}

	if update.Percent == nil && !update.HasMessage {
		return progressUpdate{}, false
	}
	if hasPercent && update.Percent == nil {
		return progressUpdate{}, false
	}
	return update, true
}

func splitOutputChunks(content string) []string {
	if content == "" {
		return nil
	}
	return strings.SplitAfter(content, "\n")
}

func stripProgressLines(stdout string) string {
	if stdout == "" {
		return ""
	}

	var builder strings.Builder
	for _, chunk := range splitOutputChunks(stdout) {
		if chunk == "" {
			continue
		}
		if _, ok := parseProgressUpdate(chunk); ok {
			continue
		}
		builder.WriteString(chunk)
	}
	return builder.String()
}

func persistenceFailureSummary(summary string, errs ...error) string {
	parts := []string{"failed to persist task state"}
	for _, err := range errs {
		if err == nil {
			continue
		}
		parts = append(parts, err.Error())
	}

	message := strings.Join(parts, ": ")
	if strings.TrimSpace(summary) == "" {
		return message
	}
	return message + "; result: " + summary
}

func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func intPtr(value int) *int {
	return &value
}
