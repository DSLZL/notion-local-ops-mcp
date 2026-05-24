package sessionstore

type Session struct {
	ID         string `json:"session_id"`
	Shell      string `json:"shell"`
	CWD        string `json:"cwd"`
	Status     string `json:"status"`
	PID        *int   `json:"pid,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	ExitCode   *int   `json:"exit_code,omitempty"`
}

type SessionInput struct {
	Shell string
	CWD   string
	PID   *int
}

type OutputRead struct {
	Content    string `json:"content"`
	Offset     int64  `json:"offset"`
	NextOffset int64  `json:"next_offset"`
	Truncated  bool   `json:"truncated"`
}

type Store interface {
	Create(input SessionInput) (Session, error)
	Get(sessionID string) (Session, error)
	Update(sessionID string, session Session) (Session, error)
	AppendOutput(sessionID, chunk string) (int64, error)
	ReadOutput(sessionID string, offset, limit int64) (OutputRead, error)
}
