//go:build !linux

package tools

import (
	"errors"
	"io"
)

var errShellSessionsUnsupported = errors.New("shell sessions are unsupported on this platform in phase 1")

type openedPlatformShellSession struct {
	runtime shellRuntime
	output  io.Reader
	wait    func() (int, error)
	pid     *int
}

func defaultShellProgram() string {
	return ""
}

func openPlatformShellSession(shell, cwd string) (openedPlatformShellSession, error) {
	return openedPlatformShellSession{}, errShellSessionsUnsupported
}

func copyShellOutput(stateDir, sessionID string, output io.Reader) {}

func waitForShellExit(stateDir, sessionID string, wait func() (int, error)) {}
