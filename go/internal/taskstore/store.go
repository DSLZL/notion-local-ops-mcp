package taskstore

import "time"

type Task struct {
	ID                    string         `json:"task_id"`
	Task                  string         `json:"task"`
	Executor              string         `json:"executor"`
	CWD                   string         `json:"cwd"`
	Status                string         `json:"status"`
	Timeout               int            `json:"timeout,omitempty"`
	ContextFiles          []string       `json:"context_files,omitempty"`
	Goal                  string         `json:"goal,omitempty"`
	AcceptanceCriteria    []string       `json:"acceptance_criteria,omitempty"`
	VerificationCommands  []string       `json:"verification_commands,omitempty"`
	CommitMode            string         `json:"commit_mode,omitempty"`
	CreatedAt             string         `json:"created_at,omitempty"`
	StartedAt             string         `json:"started_at,omitempty"`
	UpdatedAt             string         `json:"updated_at,omitempty"`
	FinishedAt            string         `json:"finished_at,omitempty"`
	HeartbeatAt           string         `json:"heartbeat_at,omitempty"`
	ExitCode              *int           `json:"exit_code,omitempty"`
	TimedOut              bool           `json:"timed_out,omitempty"`
	PID                   *int           `json:"pid,omitempty"`
	ProgressPercent       *int           `json:"progress_percent,omitempty"`
	ProgressMessage       string         `json:"progress_message,omitempty"`
	ResultPreview         string         `json:"result_preview,omitempty"`
	StdoutSize            int64          `json:"stdout_size,omitempty"`
	StderrSize            int64          `json:"stderr_size,omitempty"`
	LastLogOffset         int64          `json:"last_log_offset,omitempty"`
	EventSeq              int64          `json:"event_seq,omitempty"`
	CancelRequested       bool           `json:"cancel_requested,omitempty"`
	StructuredOutput      any            `json:"structured_output,omitempty"`
	OutputSchema          map[string]any `json:"output_schema,omitempty"`
	ParseStructuredOutput bool           `json:"parse_structured_output,omitempty"`
}

type TaskInput struct {
	Task         string
	Executor     string
	CWD          string
	Timeout      int
	ContextFiles []string
	Metadata     map[string]any
}

type Store interface {
	Create(input TaskInput) (Task, error)
	Get(taskID string) (Task, error)
	Update(taskID string, task Task) (Task, error)
	UpdateStatus(taskID string, mutate func(*Task) error) (Task, error)
	AppendLog(taskID, stream, chunk string) (int64, error)
	WriteLogs(taskID string, stdout, stderr string) error
	WriteSummary(taskID string, summary string) error
	ReadSummary(taskID string) string
	ReadStdout(taskID string) string
	ReadStderr(taskID string) string
	PurgeOlderThan(olderThan time.Duration, dryRun bool) (int, []string, error)
}
