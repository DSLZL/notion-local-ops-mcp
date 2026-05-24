package tools

import (
	"errors"
	"path/filepath"
	"sync"
	"time"

	"notion-local-ops-mcp-go/internal/sessionstore"
)

var errShellSessionNotActive = errors.New("shell session is not active")

type OpenShellSessionOptions struct {
	CWD   string
	Shell string
}

type ShellSessionResult struct {
	Success bool                 `json:"success"`
	Session sessionstore.Session `json:"session"`
	Active  bool                 `json:"active"`
}

type ShellOutputResult struct {
	Success    bool   `json:"success"`
	SessionID  string `json:"session_id"`
	Content    string `json:"content"`
	Offset     int64  `json:"offset"`
	NextOffset int64  `json:"next_offset"`
	Truncated  bool   `json:"truncated"`
}

type ShellInputResult struct {
	Success   bool   `json:"success"`
	SessionID string `json:"session_id"`
	Summary   string `json:"summary"`
}

type shellRuntime interface {
	send(input string) error
	close() error
}

type activeShellSession struct {
	runtime shellRuntime
}

var (
	activeShellSessionsMu sync.RWMutex
	activeShellSessions   = map[string]*activeShellSession{}
)

func OpenShellSession(stateDir, workspaceRoot string, options OpenShellSessionOptions) (ShellSessionResult, error) {
	store := sessionstore.NewFSStore(stateDir)
	cwd, _, err := ResolveSessionDirectory(workspaceRoot, options.CWD)
	if err != nil {
		return ShellSessionResult{}, err
	}

	shell := options.Shell
	if shell == "" {
		shell = defaultShellProgram()
	}

	opened, err := openPlatformShellSession(shell, cwd)
	if err != nil {
		return ShellSessionResult{}, err
	}

	session, err := store.Create(sessionstore.SessionInput{
		Shell: shell,
		CWD:   cwd,
		PID:   opened.pid,
	})
	if err != nil {
		_ = opened.runtime.close()
		return ShellSessionResult{}, err
	}

	registerActiveShellSession(session.ID, &activeShellSession{runtime: opened.runtime})
	go copyShellOutput(stateDir, session.ID, opened.output)
	go waitForShellExit(stateDir, session.ID, opened.wait)

	return ShellSessionResult{
		Success: true,
		Session: session,
		Active:  true,
	}, nil
}

func GetShellSession(stateDir, sessionID string) (ShellSessionResult, error) {
	store := sessionstore.NewFSStore(stateDir)
	session, err := store.Get(sessionID)
	if err != nil {
		return ShellSessionResult{}, err
	}
	return ShellSessionResult{
		Success: true,
		Session: session,
		Active:  isShellSessionActive(sessionID),
	}, nil
}

func SendShellInput(stateDir, sessionID, input string) (ShellInputResult, error) {
	active := getActiveShellSession(sessionID)
	if active == nil {
		return ShellInputResult{}, errShellSessionNotActive
	}
	if err := active.runtime.send(input); err != nil {
		return ShellInputResult{}, err
	}
	return ShellInputResult{
		Success:   true,
		SessionID: sessionID,
		Summary:   "input delivered",
	}, nil
}

func ReadShellOutput(stateDir, sessionID string, offset, limit int64) (ShellOutputResult, error) {
	store := sessionstore.NewFSStore(stateDir)
	read, err := store.ReadOutput(sessionID, offset, limit)
	if err != nil {
		return ShellOutputResult{}, err
	}
	return ShellOutputResult{
		Success:    true,
		SessionID:  sessionID,
		Content:    read.Content,
		Offset:     read.Offset,
		NextOffset: read.NextOffset,
		Truncated:  read.Truncated,
	}, nil
}

func CloseShellSession(stateDir, sessionID string) (ShellSessionResult, error) {
	store := sessionstore.NewFSStore(stateDir)
	session, err := store.Get(sessionID)
	if err != nil {
		return ShellSessionResult{}, err
	}

	if active := unregisterActiveShellSession(sessionID); active != nil {
		_ = active.runtime.close()
	}

	session.Status = "closed"
	session.FinishedAt = shellSessionTimestamp()
	session.UpdatedAt = session.FinishedAt
	updated, err := store.Update(sessionID, session)
	if err != nil {
		return ShellSessionResult{}, err
	}

	return ShellSessionResult{
		Success: true,
		Session: updated,
		Active:  false,
	}, nil
}

func shellSessionsRoot(stateDir string) string {
	return filepath.Join(stateDir, "sessions")
}

func registerActiveShellSession(sessionID string, active *activeShellSession) {
	activeShellSessionsMu.Lock()
	defer activeShellSessionsMu.Unlock()
	activeShellSessions[sessionID] = active
}

func getActiveShellSession(sessionID string) *activeShellSession {
	activeShellSessionsMu.RLock()
	defer activeShellSessionsMu.RUnlock()
	return activeShellSessions[sessionID]
}

func unregisterActiveShellSession(sessionID string) *activeShellSession {
	activeShellSessionsMu.Lock()
	defer activeShellSessionsMu.Unlock()
	active := activeShellSessions[sessionID]
	delete(activeShellSessions, sessionID)
	return active
}

func isShellSessionActive(sessionID string) bool {
	activeShellSessionsMu.RLock()
	defer activeShellSessionsMu.RUnlock()
	_, ok := activeShellSessions[sessionID]
	return ok
}

func shellSessionTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func shellSessionIntPtr(value int) *int {
	return &value
}
