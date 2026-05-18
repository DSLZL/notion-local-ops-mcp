package tools

import (
	"fmt"
	"strconv"
	"time"
)

type AwaitTaskLogs struct {
	Stream     string `json:"stream"`
	Offset     int64  `json:"offset"`
	NextOffset int64  `json:"next_offset"`
	Truncated  bool   `json:"truncated"`
	Content    string `json:"content"`
}

type AwaitTaskResult struct {
	Success               bool           `json:"success"`
	TaskID                string         `json:"task_id"`
	Status                string         `json:"status"`
	Terminal              bool           `json:"terminal"`
	EventSeq              int64          `json:"event_seq"`
	ProgressPercent       *int           `json:"progress_percent,omitempty"`
	ProgressMessage       string         `json:"progress_message,omitempty"`
	Summary               string         `json:"summary"`
	RecommendedNextAction string         `json:"recommended_next_action"`
	NextPollAfterSeconds  int            `json:"next_poll_after_seconds"`
	ResumeToken           string         `json:"resume_token"`
	Logs                  *AwaitTaskLogs `json:"logs,omitempty"`
}

func AwaitTask(stateDir, taskID string, maxWaitSeconds int, lastEventSeq int64, includeLogs bool, logStream string, logLimit int64) AwaitTaskResult {
	if maxWaitSeconds <= 0 {
		maxWaitSeconds = 30
	}

	deadline := time.Now().Add(time.Duration(maxWaitSeconds) * time.Second)
	latest, err := readPollingResult(stateDir, taskID)
	if err != nil {
		return failedAwaitTaskResult(taskID, err.Error())
	}

	for {
		if shouldReturnWaitTaskResult(latest, lastEventSeq) {
			return buildAwaitTaskResult(latest, includeLogs, stateDir, taskID, logStream, logLimit)
		}
		if time.Now().After(deadline) {
			return buildAwaitTaskResult(latest, includeLogs, stateDir, taskID, logStream, logLimit)
		}

		remaining := int(time.Until(deadline).Seconds())
		if remaining <= 0 {
			remaining = 1
		}
		_ = WaitTask(stateDir, taskID, remaining, lastEventSeq)
		current, err := readPollingResult(stateDir, taskID)
		if err != nil {
			return failedAwaitTaskResult(taskID, err.Error())
		}
		latest = current
	}
}

func buildAwaitTaskResult(task TaskPollStatusResult, includeLogs bool, stateDir, taskID, logStream string, logLimit int64) AwaitTaskResult {
	result := AwaitTaskResult{
		Success:              true,
		TaskID:               task.TaskID,
		Status:               task.Status,
		Terminal:             isTerminalPollingStatus(task.Status),
		EventSeq:             task.EventSeq,
		ProgressPercent:      task.ProgressPercent,
		ProgressMessage:      task.ProgressMessage,
		Summary:              task.Summary,
		NextPollAfterSeconds: task.NextPollAfterSeconds,
		ResumeToken:          buildResumeToken(task.TaskID, task.EventSeq),
	}
	if result.Terminal {
		result.RecommendedNextAction = "stop"
	} else {
		result.RecommendedNextAction = "await_task"
	}
	if includeLogs {
		logs := GetTaskLogs(stateDir, taskID, logStream, 0, logLimit)
		if !logs.Success {
			result.Success = false
			result.Logs = nil
			result.Summary = appendAwaitSummary(result.Summary, fmt.Sprintf("failed to read logs for stream %q", logStream))
			return result
		}
		result.Logs = &AwaitTaskLogs{
			Stream:     logs.Stream,
			Offset:     logs.Offset,
			NextOffset: logs.NextOffset,
			Truncated:  logs.Truncated,
			Content:    logs.Content,
		}
	}
	return result
}

func buildResumeToken(taskID string, eventSeq int64) string {
	return taskID + ":" + strconv.FormatInt(eventSeq, 10)
}

func failedAwaitTaskResult(taskID, summary string) AwaitTaskResult {
	return AwaitTaskResult{
		Success:               false,
		TaskID:                taskID,
		Status:                "failed",
		Terminal:              true,
		Summary:               summary,
		RecommendedNextAction: "stop",
		NextPollAfterSeconds:  0,
		ResumeToken:           buildResumeToken(taskID, 0),
	}
}

func appendAwaitSummary(base, suffix string) string {
	if base == "" {
		return suffix
	}
	return base + "; " + suffix
}
