package tools

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type TaskLogsResult struct {
	Success    bool   `json:"success"`
	TaskID     string `json:"task_id"`
	Stream     string `json:"stream"`
	Offset     int64  `json:"offset"`
	NextOffset int64  `json:"next_offset"`
	Truncated  bool   `json:"truncated"`
	Content    string `json:"content"`
}

func GetTaskLogs(stateDir, taskID, stream string, offset, limit int64) TaskLogsResult {
	if stream != "stdout" && stream != "stderr" {
		return TaskLogsResult{
			Success: false,
			TaskID:  taskID,
			Stream:  stream,
		}
	}

	logPath := filepath.Join(stateDir, "tasks", taskID, stream+".log")
	raw, err := os.ReadFile(logPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return TaskLogsResult{
				Success:    true,
				TaskID:     taskID,
				Stream:     stream,
				Offset:     0,
				NextOffset: 0,
				Truncated:  false,
				Content:    "",
			}
		}
		return TaskLogsResult{
			Success: false,
			TaskID:  taskID,
			Stream:  stream,
		}
	}

	size := int64(len(raw))
	if offset < 0 {
		offset = 0
	}
	if offset > size {
		offset = size
	}

	end := size
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	return TaskLogsResult{
		Success:    true,
		TaskID:     taskID,
		Stream:     stream,
		Offset:     offset,
		NextOffset: end,
		Truncated:  end < size,
		Content:    string(raw[offset:end]),
	}
}
