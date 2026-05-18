package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultHost = "127.0.0.1"
	defaultPort = 8766
)

type Config struct {
	Host               string
	Port               int
	WorkspaceRoot      string
	StateDir           string
	AuthToken          string
	AuthMode           string
	CommandTimeout     int
	DebugMCPLogging    bool
	GracefulShutdown   int
}

func LoadFromEnv(env map[string]string) (Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		homeDir = "."
	}

	return loadFromEnv(env, homeDir)
}

func LoadFromWorkspaceEnv(env map[string]string, startDir string) (Config, error) {
	cfg, _, err := LoadFromWorkspaceEnvWithSource(env, startDir)
	return cfg, err
}

func LoadFromWorkspaceEnvWithSource(env map[string]string, startDir string) (Config, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		homeDir = "."
	}

	dotEnvValues, dotEnvPath := loadDotEnvFromTree(startDir)
	merged := mergeEnv(env, dotEnvValues)
	cfg, err := loadFromEnv(merged, homeDir)
	return cfg, dotEnvPath, err
}

func loadFromEnv(env map[string]string, homeDir string) (Config, error) {
	port, err := lookupInt(env, "NOTION_LOCAL_OPS_PORT", defaultPort)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Host:             lookupString(env, "NOTION_LOCAL_OPS_HOST", defaultHost),
		Port:             port,
		WorkspaceRoot:    lookupString(env, "NOTION_LOCAL_OPS_WORKSPACE_ROOT", homeDir),
		StateDir:         lookupString(env, "NOTION_LOCAL_OPS_STATE_DIR", filepath.Join(homeDir, ".notion-local-ops-mcp")),
		AuthToken:        lookupString(env, "NOTION_LOCAL_OPS_AUTH_TOKEN", ""),
		AuthMode:         normalizedAuthMode(lookupString(env, "NOTION_LOCAL_OPS_AUTH_MODE", "")),
		CommandTimeout:   lookupPositiveInt(env, "NOTION_LOCAL_OPS_COMMAND_TIMEOUT", 120),
		DebugMCPLogging:  lookupBool(env, "NOTION_LOCAL_OPS_DEBUG_MCP_LOGGING", false),
		GracefulShutdown: lookupPositiveInt(env, "NOTION_LOCAL_OPS_GRACEFUL_SHUTDOWN_SECONDS", 30),
	}, nil
}

func normalizedAuthMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "oauth":
		return "oauth"
	case "", "shared_token":
		return "shared_token"
	default:
		return "shared_token"
	}
}

func lookupString(env map[string]string, key, fallback string) string {
	if env != nil {
		if value, ok := env[key]; ok && value != "" {
			return value
		}
		return fallback
	}

	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func lookupInt(env map[string]string, key string, fallback int) (int, error) {
	value, ok := lookupOptionalString(env, key)
	if !ok {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than 0", key)
	}
	if parsed > 65535 {
		return 0, fmt.Errorf("%s must be less than or equal to 65535", key)
	}
	return parsed, nil
}

func lookupPositiveInt(env map[string]string, key string, fallback int) int {
	value, err := lookupInt(env, key, fallback)
	if err != nil {
		return fallback
	}
	return value
}

func lookupBool(env map[string]string, key string, fallback bool) bool {
	value, ok := lookupOptionalString(env, key)
	if !ok {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func lookupOptionalString(env map[string]string, key string) (string, bool) {
	if env != nil {
		value, ok := env[key]
		if !ok || value == "" {
			return "", false
		}
		return value, true
	}

	value := os.Getenv(key)
	if value == "" {
		return "", false
	}
	return value, true
}

func mergeEnv(base map[string]string, overlay map[string]string) map[string]string {
	merged := make(map[string]string)
	if base == nil {
		for _, entry := range os.Environ() {
			key, value, ok := strings.Cut(entry, "=")
			if ok {
				merged[key] = value
			}
		}
	} else {
		for key, value := range base {
			merged[key] = value
		}
	}

	for key, value := range overlay {
		merged[key] = value
	}
	return merged
}

func loadDotEnvFromTree(startDir string) (map[string]string, string) {
	envPath, err := findDotEnvPath(startDir)
	if err != nil || envPath == "" {
		return map[string]string{}, ""
	}

	loaded, err := parseDotEnvFile(envPath)
	if err != nil {
		return map[string]string{}, ""
	}
	return loaded, envPath
}

func findDotEnvPath(startDir string) (string, error) {
	dir := startDir
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = cwd
	}

	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(dir, ".env")
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

func parseDotEnvFile(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	entries := make(map[string]string)
	for _, rawLine := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = strings.TrimSuffix(strings.TrimPrefix(value, "\""), "\"")
			} else if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
				value = strings.TrimSuffix(strings.TrimPrefix(value, "'"), "'")
			}
		}
		entries[key] = value
	}
	return entries, nil
}
