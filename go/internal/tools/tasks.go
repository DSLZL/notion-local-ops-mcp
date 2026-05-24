package tools

import (
	"fmt"
	"notion-local-ops-mcp-go/internal/taskrunner"
	"notion-local-ops-mcp-go/internal/taskstore"
	"strings"
	"time"
)

type TaskPollStatusResult struct {
	TaskID                  string `json:"task_id"`
	Status                  string `json:"status"`
	Summary                 string `json:"summary"`
	EventSeq                int64  `json:"event_seq"`
	ProgressPercent         *int   `json:"progress_percent,omitempty"`
	ProgressMessage         string `json:"progress_message,omitempty"`
	HeartbeatAt             string `json:"heartbeat_at,omitempty"`
	RecommendedPollStrategy string `json:"recommended_poll_strategy"`
	NextPollAfterSeconds    int    `json:"next_poll_after_seconds"`
}

const (
	defaultWaitTaskTimeoutSeconds = 1
	waitTaskPollInterval          = 25 * time.Millisecond
)

func RunCommandStreamForTest() TaskPollingResult {
	return RunCommandStream(taskPollingRoot(), ".", "echo stream-ok", "", 5)
}

func SubmitStreamCommandForTest() TaskPollingResult {
	return RunCommandStreamForTest()
}

func waitForTaskCompletionForTest(submitted TaskPollingResult) TaskPollStatusResult {
	if submitted.TaskID == "" {
		return TaskPollStatusResult{}
	}
	return WaitForTaskForTest(submitted.TaskID)
}

func WaitTaskForTest(taskID string) TaskPollStatusResult {
	return WaitTask(taskPollingRoot(), taskID, defaultWaitTaskTimeoutSeconds, 0)
}

func WaitForTaskForTest(taskID string) TaskPollStatusResult {
	deadline := time.Now().Add(3 * time.Second)
	lastEventSeq := int64(-1)
	latest := GetTask(taskPollingRoot(), taskID)
	for time.Now().Before(deadline) {
		current := WaitTask(taskPollingRoot(), taskID, defaultWaitTaskTimeoutSeconds, lastEventSeq)
		if current.TaskID != "" {
			latest = current
		}
		if isTerminalPollingStatus(current.Status) {
			return current
		}
		lastEventSeq = current.EventSeq
	}
	return latest
}

func GetTaskForTest(taskID string) TaskPollStatusResult {
	return GetTask(taskPollingRoot(), taskID)
}

func CancelTaskForTest(taskID string) TaskPollStatusResult {
	return CancelTask(taskPollingRoot(), taskID)
}

func WaitTask(stateDir, taskID string, timeoutSeconds int, lastEventSeq int64) TaskPollStatusResult {
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultWaitTaskTimeoutSeconds
	}

	current, err := readPollingResult(stateDir, taskID)
	if err != nil {
		return failedPollingResult(taskID, err.Error())
	}
	if shouldReturnWaitTaskResult(current, lastEventSeq) {
		return current
	}

	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	latest := current
	for time.Now().Before(deadline) {
		time.Sleep(waitTaskPollInterval)
		current, err = readPollingResult(stateDir, taskID)
		if err != nil {
			continue
		}
		latest = current
		if shouldReturnWaitTaskResult(current, lastEventSeq) {
			return current
		}
	}
	return withWaitTimeoutSummary(latest, timeoutSeconds)
}

func GetTask(stateDir, taskID string) TaskPollStatusResult {
	result, err := readPollingResult(stateDir, taskID)
	if err != nil {
		return failedPollingResult(taskID, err.Error())
	}
	return result
}

func CancelTask(stateDir, taskID string) TaskPollStatusResult {
	current, err := readPollingResult(stateDir, taskID)
	if err != nil {
		return failedPollingResult(taskID, err.Error())
	}
	if isTerminalPollingStatus(current.Status) {
		return current
	}

	store := taskstore.NewFSStore(stateDir)
	runner := taskrunner.NewRunner(store)
	if err := runner.Cancel(taskID); err != nil {
		return failedPollingResult(taskID, err.Error())
	}
	return GetTask(stateDir, taskID)
}

func readPollingResult(stateDir, taskID string) (TaskPollStatusResult, error) {
	store := taskstore.NewFSStore(stateDir)
	task, err := store.Get(taskID)
	if err != nil {
		return TaskPollStatusResult{}, err
	}
	summary := summarizeTaskStatus(task.Status, task.ProgressMessage, store.ReadSummary(taskID))
	recommendedPollStrategy, nextPollAfterSeconds := pollingAdvice(task.Status)
	return TaskPollStatusResult{
		TaskID:                  task.ID,
		Status:                  task.Status,
		Summary:                 summary,
		EventSeq:                task.EventSeq,
		ProgressPercent:         task.ProgressPercent,
		ProgressMessage:         task.ProgressMessage,
		HeartbeatAt:             task.HeartbeatAt,
		RecommendedPollStrategy: recommendedPollStrategy,
		NextPollAfterSeconds:    nextPollAfterSeconds,
	}, nil
}

func shouldReturnWaitTaskResult(result TaskPollStatusResult, lastEventSeq int64) bool {
	return isTerminalPollingStatus(result.Status) || result.EventSeq > lastEventSeq
}

func isTerminalPollingStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func pollingAdvice(status string) (string, int) {
	if isTerminalPollingStatus(status) {
		return "stop", 0
	}
	return "wait_task", defaultWaitTaskTimeoutSeconds
}

func withWaitTimeoutSummary(result TaskPollStatusResult, timeoutSeconds int) TaskPollStatusResult {
	if isTerminalPollingStatus(result.Status) {
		return result
	}
	progress := compactSummaryText(result.ProgressMessage, 96)
	if progress != "" {
		result.Summary = fmt.Sprintf("wait_task timed out after %ds with no new events; call wait_task again to continue polling, or use get_task_logs for stdout/stderr details (%s)", timeoutSeconds, progress)
		return result
	}
	result.Summary = fmt.Sprintf("wait_task timed out after %ds with no new events; call wait_task again to continue polling, or use get_task_logs for stdout/stderr details", timeoutSeconds)
	return result
}

func summarizeTaskStatus(status, progressMessage, storedSummary string) string {
	progress := compactSummaryText(firstNonEmpty(progressMessage, storedSummary), 96)
	switch status {
	case "running":
		if progress != "" {
			return fmt.Sprintf("task running; use wait_task to continue polling, use get_task_logs for stdout/stderr details (%s)", progress)
		}
		return "task running; use wait_task to continue polling, use get_task_logs for stdout/stderr details"
	case "failed":
		if progress != "" {
			return fmt.Sprintf("task failed; use get_task_logs for stdout/stderr details before retrying (%s)", progress)
		}
		return "task failed; use get_task_logs for stdout/stderr details before retrying"
	case "succeeded":
		return "task succeeded; polling can stop, use get_task_logs only when full stdout/stderr is needed"
	case "cancelled":
		return "task cancelled; polling can stop, use get_task_logs only when final output details are needed"
	default:
		return compactSummaryText(firstNonEmpty(storedSummary, progressMessage), 120)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func compactSummaryText(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return ""
	}
	joined := strings.Join(parts, " ")
	if maxLen <= 0 || len(joined) <= maxLen {
		return joined
	}
	if maxLen <= 3 {
		return joined[:maxLen]
	}
	return joined[:maxLen-3] + "..."
}

func failedPollingResult(taskID, summary string) TaskPollStatusResult {
	return TaskPollStatusResult{
		TaskID:                  taskID,
		Status:                  "failed",
		Summary:                 summary,
		RecommendedPollStrategy: "stop",
		NextPollAfterSeconds:    0,
	}
}
