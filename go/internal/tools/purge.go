package tools

import (
	"time"

	"notion-local-ops-mcp-go/internal/taskstore"
)

type PurgeTasksResult struct {
	Success bool     `json:"success"`
	Scanned int      `json:"scanned"`
	Purged  int      `json:"purged"`
	TaskIDs []string `json:"task_ids"`
	DryRun  bool     `json:"dry_run"`
	Cutoff  string   `json:"cutoff"`
}

func PurgeTasks(stateDir string, olderThanHours int, dryRun bool) (PurgeTasksResult, error) {
	if olderThanHours <= 0 {
		olderThanHours = 24 * 7
	}
	duration := time.Duration(olderThanHours) * time.Hour
	cutoff := time.Now().UTC().Add(-duration).Format(time.RFC3339Nano)

	store := taskstore.NewFSStore(stateDir)
	scanned, taskIDs, err := store.PurgeOlderThan(duration, dryRun)
	if err != nil {
		return PurgeTasksResult{}, err
	}

	return PurgeTasksResult{
		Success: true,
		Scanned: scanned,
		Purged:  len(taskIDs),
		TaskIDs: taskIDs,
		DryRun:  dryRun,
		Cutoff:  cutoff,
	}, nil
}
