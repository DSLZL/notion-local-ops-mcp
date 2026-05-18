package app

import "notion-local-ops-mcp-go/internal/config"

func BuildDefaultServer() (*Server, string, error) {
	cfg, envPath, err := config.LoadFromWorkspaceEnvWithSource(nil, "")
	if err != nil {
		return nil, "", err
	}
	return NewServer(cfg), envPath, nil
}
