package tools

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"notion-local-ops-mcp-go/internal/taskstore"
)

type RecentTaskSummary struct {
	TaskID          string `json:"task_id"`
	Status          string `json:"status"`
	CreatedAt       string `json:"created_at,omitempty"`
	StartedAt       string `json:"started_at,omitempty"`
	FinishedAt      string `json:"finished_at,omitempty"`
	EventSeq        int64  `json:"event_seq"`
	ProgressMessage string `json:"progress_message,omitempty"`
	Summary         string `json:"summary"`
}

type ListRecentTasksResult struct {
	Success bool                `json:"success"`
	Tasks   []RecentTaskSummary `json:"tasks"`
}

func ListRecentTasks(stateDir, status string, limit int) ListRecentTasksResult {
	if limit <= 0 {
		limit = 20
	}

	tasksRoot := filepath.Join(stateDir, "tasks")
	if _, err := os.Stat(tasksRoot); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ListRecentTasksResult{Success: true, Tasks: emptyRecentTaskSummaries()}
		}
		return ListRecentTasksResult{Success: false, Tasks: emptyRecentTaskSummaries()}
	}

	entries, err := os.ReadDir(tasksRoot)
	if err != nil {
		return ListRecentTasksResult{Success: false, Tasks: emptyRecentTaskSummaries()}
	}

	filter := strings.TrimSpace(strings.ToLower(status))
	items := make([]recentTaskWithSortKey, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		task, ok := readRecentTask(tasksRoot, entry.Name())
		if !ok {
			continue
		}
		if filter != "" && strings.ToLower(task.Status) != filter {
			continue
		}

		items = append(items, task)
	}

	slices.SortFunc(items, func(a, b recentTaskWithSortKey) int {
		if cmp := b.sortKey.Compare(a.sortKey); cmp != 0 {
			return cmp
		}
		return strings.Compare(b.TaskID, a.TaskID)
	})

	if len(items) > limit {
		items = items[:limit]
	}

	result := make([]RecentTaskSummary, len(items))
	for i, item := range items {
		result[i] = item.RecentTaskSummary
	}

	return ListRecentTasksResult{Success: true, Tasks: result}
}

func emptyRecentTaskSummaries() []RecentTaskSummary {
	return make([]RecentTaskSummary, 0)
}

func readRecentTaskSummary(tasksRoot, taskID string) string {
	raw, err := os.ReadFile(filepath.Join(tasksRoot, taskID, "summary.txt"))
	if err != nil {
		return ""
	}
	return string(raw)
}

type recentTaskWithSortKey struct {
	RecentTaskSummary
	sortKey time.Time
}

func readRecentTask(tasksRoot, taskID string) (recentTaskWithSortKey, bool) {
	raw, err := os.ReadFile(filepath.Join(tasksRoot, taskID, "meta.json"))
	if err != nil {
		return recentTaskWithSortKey{}, false
	}

	var task taskstore.Task
	if err := json.Unmarshal(raw, &task); err != nil {
		return recentTaskWithSortKey{}, false
	}

	return recentTaskWithSortKey{
		RecentTaskSummary: RecentTaskSummary{
			TaskID:          task.ID,
			Status:          task.Status,
			CreatedAt:       task.CreatedAt,
			StartedAt:       task.StartedAt,
			FinishedAt:      task.FinishedAt,
			EventSeq:        task.EventSeq,
			ProgressMessage: task.ProgressMessage,
			Summary:         readRecentTaskSummary(tasksRoot, task.ID),
		},
		sortKey: recentTaskSortTime(task),
	}, true
}

func recentTaskSortTime(task taskstore.Task) time.Time {
	for _, value := range []string{task.UpdatedAt, task.CreatedAt, task.StartedAt, task.FinishedAt} {
		if ts, ok := parseRecentTaskTime(value); ok {
			return ts
		}
	}
	return time.Time{}
}

func parseRecentTaskTime(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}
