package tools

import "notion-local-ops-mcp-go/internal/config"

const (
	AppName   = "notion-local-ops-mcp"
	Transport = "streamable-http"
	MCPPath   = "/mcp"
)

func BuildServerInfo(cfg config.Config, toolNames []string) map[string]any {
	authMode := cfg.AuthMode
	if authMode == "" {
		authMode = "shared_token"
	}

	extraWriteDirs := cfg.ExtraWriteDirs
	if extraWriteDirs == nil {
		extraWriteDirs = []string{}
	}

	return map[string]any{
		"success":                      true,
		"app_name":                     AppName,
		"transport":                    Transport,
		"mcp_path":                     MCPPath,
		"host":                         cfg.Host,
		"port":                         cfg.Port,
		"workspace_root":               cfg.WorkspaceRoot,
		"state_dir":                    cfg.StateDir,
		"auth_enabled":                 cfg.AuthToken != "",
		"auth_mode":                    authMode,
		"command_timeout_seconds":      cfg.CommandTimeout,
		"graceful_shutdown_seconds":    cfg.GracefulShutdown,
		"extra_write_dirs":             extraWriteDirs,
		"tools":                        append([]string(nil), toolNames...),
		"tool_count":                   len(toolNames),
	}
}
