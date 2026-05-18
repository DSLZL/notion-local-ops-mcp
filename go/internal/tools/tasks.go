package tools

import (
	"notion-local-ops-mcp-go/internal/taskrunner"
	"notion-local-ops-mcp-go/internal/taskstore"
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
	return latest
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
	recommendedPollStrategy, nextPollAfterSeconds := pollingAdvice(task.Status)
	return TaskPollStatusResult{
		TaskID:                  task.ID,
		Status:                  task.Status,
		Summary:                 store.ReadSummary(taskID),
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

func failedPollingResult(taskID, summary string) TaskPollStatusResult {
	return TaskPollStatusResult{
		TaskID:                  taskID,
		Status:                  "failed",
		Summary:                 summary,
		RecommendedPollStrategy: "stop",
		NextPollAfterSeconds:    0,
	}
}
