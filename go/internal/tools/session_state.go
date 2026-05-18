package tools

import (
	"os"
	"sync"

	"notion-local-ops-mcp-go/internal/fsx"
)

var (
	sessionCWDMu sync.RWMutex
	sessionCWD   string
)

func GetDefaultCWD() string {
	sessionCWDMu.RLock()
	defer sessionCWDMu.RUnlock()
	return sessionCWD
}

func SetDefaultCWD(path string) string {
	sessionCWDMu.Lock()
	defer sessionCWDMu.Unlock()
	sessionCWD = path
	return sessionCWD
}

func ClearDefaultCWD() {
	SetDefaultCWD("")
}

func EffectiveCWD(workspaceRoot string) string {
	if session := GetDefaultCWD(); session != "" {
		return session
	}
	return workspaceRoot
}

func ResolveSessionDirectory(workspaceRoot, input string) (string, bool, error) {
	if input != "" {
		resolved, err := fsx.ResolvePath(workspaceRoot, input)
		if err != nil {
			return "", false, err
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return resolved, false, err
		}
		if !info.IsDir() {
			return resolved, false, os.ErrInvalid
		}
		return resolved, false, nil
	}

	session := GetDefaultCWD()
	if session != "" {
		return session, true, nil
	}
	return workspaceRoot, true, nil
}
