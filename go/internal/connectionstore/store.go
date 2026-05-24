package connectionstore

type Connection struct {
	ID         string `json:"connection_id"`
	Network    string `json:"network"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	RemoteAddr string `json:"remote_addr,omitempty"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	LastError  string `json:"last_error,omitempty"`
}

type ConnectionInput struct {
	Network    string
	Host       string
	Port       int
	RemoteAddr string
}

type LogRead struct {
	Content    string `json:"content"`
	Offset     int64  `json:"offset"`
	NextOffset int64  `json:"next_offset"`
	Truncated  bool   `json:"truncated"`
}

type Store interface {
	Create(input ConnectionInput) (Connection, error)
	Get(connectionID string) (Connection, error)
	Update(connectionID string, connection Connection) (Connection, error)
	AppendInput(connectionID, chunk string) (int64, error)
	ReadInput(connectionID string, offset, limit int64) (LogRead, error)
	AppendOutput(connectionID, chunk string) (int64, error)
	ReadOutput(connectionID string, offset, limit int64) (LogRead, error)
}
