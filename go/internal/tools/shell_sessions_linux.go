//go:build linux

package tools

import (
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"notion-local-ops-mcp-go/internal/sessionstore"
)

type openedPlatformShellSession struct {
	runtime shellRuntime
	output  io.Reader
	wait    func() (int, error)
	pid     *int
}

type ptyShellRuntime struct {
	cmd  *exec.Cmd
	pty  *os.File
	mu   sync.Mutex
	once sync.Once
}

func defaultShellProgram() string {
	return "bash"
}

func openPlatformShellSession(shell, cwd string) (openedPlatformShellSession, error) {
	cmd := exec.Command(shell)
	cmd.Dir = cwd

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return openedPlatformShellSession{}, err
	}

	runtime := &ptyShellRuntime{
		cmd: cmd,
		pty: ptmx,
	}
	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	return openedPlatformShellSession{
		runtime: runtime,
		output:  ptmx,
		wait: func() (int, error) {
			err := cmd.Wait()
			if err == nil {
				return 0, nil
			}
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode(), err
			}
			return -1, err
		},
		pid: shellSessionIntPtr(pid),
	}, nil
}

func (r *ptyShellRuntime) send(input string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, err := io.WriteString(r.pty, input)
	return err
}

func (r *ptyShellRuntime) close() error {
	var err error
	r.once.Do(func() {
		if r.cmd != nil && r.cmd.Process != nil {
			_ = r.cmd.Process.Kill()
		}
		if r.pty != nil {
			err = r.pty.Close()
		}
	})
	return err
}

func copyShellOutput(stateDir, sessionID string, output io.Reader) {
	store := sessionstore.NewFSStore(stateDir)
	buf := make([]byte, 1024)
	for {
		n, err := output.Read(buf)
		if n > 0 {
			_, _ = store.AppendOutput(sessionID, string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

func waitForShellExit(stateDir, sessionID string, wait func() (int, error)) {
	exitCode, _ := wait()
	store := sessionstore.NewFSStore(stateDir)
	session, err := store.Get(sessionID)
	if err != nil {
		unregisterActiveShellSession(sessionID)
		return
	}
	session.ExitCode = shellSessionIntPtr(exitCode)
	session.FinishedAt = shellSessionTimestamp()
	session.UpdatedAt = session.FinishedAt
	if session.Status == "running" {
		session.Status = "exited"
	}
	_, _ = store.Update(sessionID, session)
	unregisterActiveShellSession(sessionID)
}
