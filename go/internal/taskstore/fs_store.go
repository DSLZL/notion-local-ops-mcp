package taskstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var ErrInvalidTaskID = errors.New("invalid task id")
var taskIDPattern = regexp.MustCompile(`^task-[0-9]+$`)

type FSStore struct {
	root string
	mu   sync.RWMutex
}

func NewFSStore(root string) *FSStore {
	return &FSStore{root: root}
}

func (s *FSStore) Create(input TaskInput) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	now := time.Now().UTC().Format(time.RFC3339Nano)
	task := Task{
		ID:           taskID,
		Task:         input.Task,
		Executor:     input.Executor,
		CWD:          input.CWD,
		Status:       "queued",
		Timeout:      input.Timeout,
		ContextFiles: input.ContextFiles,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	applyTaskMetadata(&task, input.Metadata)

	taskDir := filepath.Join(s.root, "tasks", taskID)
	if err := os.MkdirAll(taskDir, 0o700); err != nil {
		return Task{}, err
	}

	if err := s.writeTask(taskID, task); err != nil {
		return Task{}, err
	}

	return task, nil
}

func (s *FSStore) Get(taskID string) (Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getUnlocked(taskID)
}

func (s *FSStore) getUnlocked(taskID string) (Task, error) {
	metaPath, err := s.metaPath(taskID)
	if err != nil {
		return Task{}, err
	}

	raw, err := os.ReadFile(metaPath)
	if err != nil {
		return Task{}, err
	}

	var task Task
	if err := json.Unmarshal(raw, &task); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *FSStore) Update(taskID string, task Task) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.updateUnlocked(taskID, task)
}

func (s *FSStore) UpdateStatus(taskID string, mutate func(*Task) error) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, err := s.getUnlocked(taskID)
	if err != nil {
		return Task{}, err
	}
	if err := mutate(&task); err != nil {
		return Task{}, err
	}
	task.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return s.updateUnlocked(taskID, task)
}

func (s *FSStore) AppendLog(taskID, stream, chunk string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	logPath, err := s.logPath(taskID, stream)
	if err != nil {
		return 0, err
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	if _, err := file.WriteString(chunk); err != nil {
		return 0, err
	}
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}

	offset := info.Size()
	task, err := s.getUnlocked(taskID)
	if err != nil {
		return offset, err
	}
	switch stream {
	case "stdout":
		task.StdoutSize = offset
	case "stderr":
		task.StderrSize = offset
	}
	task.LastLogOffset = offset
	task.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.writeTask(taskID, task); err != nil {
		return offset, err
	}

	return offset, nil
}

func (s *FSStore) writeTask(taskID string, task Task) error {
	metaPath, err := s.metaPath(taskID)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := metaPath + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, metaPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func (s *FSStore) updateUnlocked(taskID string, task Task) (Task, error) {
	task.ID = taskID
	if task.CreatedAt == "" {
		if existing, getErr := s.getUnlocked(taskID); getErr == nil {
			task.CreatedAt = existing.CreatedAt
		}
	}
	if task.UpdatedAt == "" {
		task.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if err := s.writeTask(taskID, task); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *FSStore) metaPath(taskID string) (string, error) {
	if !taskIDPattern.MatchString(taskID) {
		return "", ErrInvalidTaskID
	}
	return filepath.Join(s.root, "tasks", taskID, "meta.json"), nil
}

func applyTaskMetadata(task *Task, metadata map[string]any) {
	if task == nil || metadata == nil {
		return
	}
	if value, ok := metadata["goal"].(string); ok {
		task.Goal = value
	}
	if values, ok := metadata["acceptance_criteria"].([]string); ok {
		task.AcceptanceCriteria = values
	}
	if values, ok := metadata["verification_commands"].([]string); ok {
		task.VerificationCommands = values
	}
	if value, ok := metadata["commit_mode"].(string); ok {
		task.CommitMode = value
	}
	if value, ok := metadata["output_schema"].(map[string]any); ok {
		task.OutputSchema = value
	}
	if value, ok := metadata["parse_structured_output"].(bool); ok {
		task.ParseStructuredOutput = value
	}
}

func (s *FSStore) WriteLogs(taskID string, stdout, stderr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stdoutPath, err := s.logPath(taskID, "stdout")
	if err != nil {
		return err
	}
	if err := os.WriteFile(stdoutPath, []byte(stdout), 0o600); err != nil {
		return err
	}
	stderrPath, err := s.logPath(taskID, "stderr")
	if err != nil {
		return err
	}
	if err := os.WriteFile(stderrPath, []byte(stderr), 0o600); err != nil {
		return err
	}

	task, err := s.getUnlocked(taskID)
	if err != nil {
		return err
	}
	task.StdoutSize = int64(len(stdout))
	task.StderrSize = int64(len(stderr))
	if task.StderrSize > 0 {
		task.LastLogOffset = task.StderrSize
	} else {
		task.LastLogOffset = task.StdoutSize
	}
	task.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return s.writeTask(taskID, task)
}

func (s *FSStore) WriteSummary(taskID string, summary string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	summaryPath, err := s.summaryPath(taskID)
	if err != nil {
		return err
	}
	return os.WriteFile(summaryPath, []byte(summary), 0o600)
}

func (s *FSStore) ReadSummary(taskID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summaryPath, err := s.summaryPath(taskID)
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(summaryPath)
	if err != nil {
		return ""
	}
	return string(raw)
}

func (s *FSStore) ReadStdout(taskID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stdoutPath, err := s.logPath(taskID, "stdout")
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(stdoutPath)
	if err != nil {
		return ""
	}
	return string(raw)
}

func (s *FSStore) ReadStderr(taskID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stderrPath, err := s.logPath(taskID, "stderr")
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(stderrPath)
	if err != nil {
		return ""
	}
	return string(raw)
}

func (s *FSStore) summaryPath(taskID string) (string, error) {
	if !taskIDPattern.MatchString(taskID) {
		return "", ErrInvalidTaskID
	}
	return filepath.Join(s.root, "tasks", taskID, "summary.txt"), nil
}

func (s *FSStore) PurgeOlderThan(olderThan time.Duration, dryRun bool) (int, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasksRoot := filepath.Join(s.root, "tasks")
	entries, err := os.ReadDir(tasksRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil, nil
		}
		return 0, nil, err
	}

	cutoff := time.Now().UTC().Add(-olderThan)
	purged := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		taskID := entry.Name()
		task, err := s.getUnlocked(taskID)
		if err != nil {
			continue
		}
		updatedAt, err := time.Parse(time.RFC3339Nano, task.UpdatedAt)
		if err != nil || updatedAt.Before(cutoff) {
			purged = append(purged, taskID)
			if !dryRun {
				_ = os.RemoveAll(filepath.Join(tasksRoot, taskID))
			}
		}
	}
	return len(entries), purged, nil
}

func (s *FSStore) logPath(taskID, stream string) (string, error) {
	if !taskIDPattern.MatchString(taskID) {
		return "", ErrInvalidTaskID
	}
	switch stream {
	case "stdout", "stderr":
		return filepath.Join(s.root, "tasks", taskID, stream+".log"), nil
	default:
		return "", fmt.Errorf("invalid stream %q", stream)
	}
}
