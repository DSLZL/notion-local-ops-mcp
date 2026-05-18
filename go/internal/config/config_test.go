package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("NOTION_LOCAL_OPS_HOST", "ambient-host")
	t.Setenv("NOTION_LOCAL_OPS_PORT", "9999")

	cfg, err := loadFromEnv(map[string]string{}, "/tmp/notion-local-ops")
	if err != nil {
		t.Fatalf("loadFromEnv() error = %v", err)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("Host = %q, want 127.0.0.1", cfg.Host)
	}
	if cfg.Port != 8766 {
		t.Fatalf("Port = %d, want 8766", cfg.Port)
	}
	if cfg.WorkspaceRoot != "/tmp/notion-local-ops" {
		t.Fatalf("WorkspaceRoot = %q, want /tmp/notion-local-ops", cfg.WorkspaceRoot)
	}
	wantStateDir := filepath.Join("/tmp/notion-local-ops", ".notion-local-ops-mcp")
	if cfg.StateDir != wantStateDir {
		t.Fatalf("StateDir = %q, want %q", cfg.StateDir, wantStateDir)
	}
	if cfg.AuthToken != "" {
		t.Fatalf("AuthToken = %q, want empty", cfg.AuthToken)
	}
	if cfg.AuthMode != "shared_token" {
		t.Fatalf("AuthMode = %q, want shared_token", cfg.AuthMode)
	}
	if cfg.CommandTimeout != 120 {
		t.Fatalf("CommandTimeout = %d, want 120", cfg.CommandTimeout)
	}
	if cfg.DebugMCPLogging {
		t.Fatal("DebugMCPLogging = true, want false")
	}
}

func TestLoadConfigRejectsInvalidPort(t *testing.T) {
	t.Run("non-integer", func(t *testing.T) {
		_, err := loadFromEnv(map[string]string{
			"NOTION_LOCAL_OPS_PORT": "nope",
		}, "/tmp/notion-local-ops")
		if err == nil || !strings.Contains(err.Error(), "NOTION_LOCAL_OPS_PORT") {
			t.Fatalf("loadFromEnv() error = %v, want port validation error", err)
		}
	})

	t.Run("non-positive", func(t *testing.T) {
		_, err := loadFromEnv(map[string]string{
			"NOTION_LOCAL_OPS_PORT": "0",
		}, "/tmp/notion-local-ops")
		if err == nil || !strings.Contains(err.Error(), "greater than 0") {
			t.Fatalf("loadFromEnv() error = %v, want non-positive port validation error", err)
		}
	})

	t.Run("too-large", func(t *testing.T) {
		_, err := loadFromEnv(map[string]string{
			"NOTION_LOCAL_OPS_PORT": "65536",
		}, "/tmp/notion-local-ops")
		if err == nil || !strings.Contains(err.Error(), "65535") {
			t.Fatalf("loadFromEnv() error = %v, want upper-bound port validation error", err)
		}
	})
}

func TestLoadConfigAcceptsExplicitOverrides(t *testing.T) {
	cfg, err := loadFromEnv(map[string]string{
		"NOTION_LOCAL_OPS_HOST":           "0.0.0.0",
		"NOTION_LOCAL_OPS_PORT":           "9000",
		"NOTION_LOCAL_OPS_WORKSPACE_ROOT": "/workspace",
		"NOTION_LOCAL_OPS_STATE_DIR":      "/state",
		"NOTION_LOCAL_OPS_AUTH_TOKEN":     "secret",
		"NOTION_LOCAL_OPS_AUTH_MODE":      "oauth",
		"NOTION_LOCAL_OPS_COMMAND_TIMEOUT": "45",
		"NOTION_LOCAL_OPS_DEBUG_MCP_LOGGING": "1",
	}, "/tmp/notion-local-ops")
	if err != nil {
		t.Fatalf("loadFromEnv() error = %v", err)
	}

	if cfg.Host != "0.0.0.0" {
		t.Fatalf("Host = %q, want 0.0.0.0", cfg.Host)
	}
	if cfg.Port != 9000 {
		t.Fatalf("Port = %d, want 9000", cfg.Port)
	}
	if cfg.WorkspaceRoot != "/workspace" {
		t.Fatalf("WorkspaceRoot = %q, want /workspace", cfg.WorkspaceRoot)
	}
	if cfg.StateDir != "/state" {
		t.Fatalf("StateDir = %q, want /state", cfg.StateDir)
	}
	if cfg.AuthToken != "secret" {
		t.Fatalf("AuthToken = %q, want secret", cfg.AuthToken)
	}
	if cfg.AuthMode != "oauth" {
		t.Fatalf("AuthMode = %q, want oauth", cfg.AuthMode)
	}
	if cfg.CommandTimeout != 45 {
		t.Fatalf("CommandTimeout = %d, want 45", cfg.CommandTimeout)
	}
	if !cfg.DebugMCPLogging {
		t.Fatal("DebugMCPLogging = false, want true")
	}
}

func TestLoadFromWorkspaceEnvPrefersDotEnvOverShell(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte(strings.Join([]string{
		"NOTION_LOCAL_OPS_HOST=from-dotenv",
		"NOTION_LOCAL_OPS_PORT=9100",
		"NOTION_LOCAL_OPS_WORKSPACE_ROOT=/workspace-dotenv",
		"NOTION_LOCAL_OPS_AUTH_TOKEN=dotenv-secret",
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("Write .env error = %v", err)
	}

	t.Setenv("NOTION_LOCAL_OPS_HOST", "from-shell")
	t.Setenv("NOTION_LOCAL_OPS_PORT", "9200")
	t.Setenv("NOTION_LOCAL_OPS_WORKSPACE_ROOT", "/workspace-shell")
	t.Setenv("NOTION_LOCAL_OPS_AUTH_TOKEN", "shell-secret")

	cfg, err := LoadFromWorkspaceEnv(nil, tempDir)
	if err != nil {
		t.Fatalf("LoadFromWorkspaceEnv() error = %v", err)
	}

	if cfg.Host != "from-dotenv" {
		t.Fatalf("Host = %q, want from-dotenv", cfg.Host)
	}
	if cfg.Port != 9100 {
		t.Fatalf("Port = %d, want 9100", cfg.Port)
	}
	if cfg.WorkspaceRoot != "/workspace-dotenv" {
		t.Fatalf("WorkspaceRoot = %q, want /workspace-dotenv", cfg.WorkspaceRoot)
	}
	if cfg.AuthToken != "dotenv-secret" {
		t.Fatalf("AuthToken = %q, want dotenv-secret", cfg.AuthToken)
	}
}

func TestLoadFromWorkspaceEnvFindsNearestAncestorDotEnv(t *testing.T) {
	tempDir := t.TempDir()
	nested := filepath.Join(tempDir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte("NOTION_LOCAL_OPS_WORKSPACE_ROOT=/nearest-root\n"), 0o600); err != nil {
		t.Fatalf("Write .env error = %v", err)
	}

	cfg, err := LoadFromWorkspaceEnv(map[string]string{}, nested)
	if err != nil {
		t.Fatalf("LoadFromWorkspaceEnv() error = %v", err)
	}
	if cfg.WorkspaceRoot != "/nearest-root" {
		t.Fatalf("WorkspaceRoot = %q, want /nearest-root", cfg.WorkspaceRoot)
	}
}

func TestLoadFromWorkspaceEnvParsesQuotedValues(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte(strings.Join([]string{
		"NOTION_LOCAL_OPS_HOST='quoted-host'",
		"NOTION_LOCAL_OPS_WORKSPACE_ROOT=\"/quoted/workspace\"",
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("Write .env error = %v", err)
	}

	cfg, err := LoadFromWorkspaceEnv(map[string]string{}, tempDir)
	if err != nil {
		t.Fatalf("LoadFromWorkspaceEnv() error = %v", err)
	}
	if cfg.Host != "quoted-host" {
		t.Fatalf("Host = %q, want quoted-host", cfg.Host)
	}
	if cfg.WorkspaceRoot != "/quoted/workspace" {
		t.Fatalf("WorkspaceRoot = %q, want /quoted/workspace", cfg.WorkspaceRoot)
	}
}

func TestLoadFromWorkspaceEnvWithSourceReturnsDotEnvPath(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(envPath, []byte("NOTION_LOCAL_OPS_WORKSPACE_ROOT=/with-source\n"), 0o600); err != nil {
		t.Fatalf("Write .env error = %v", err)
	}

	cfg, sourcePath, err := LoadFromWorkspaceEnvWithSource(map[string]string{}, tempDir)
	if err != nil {
		t.Fatalf("LoadFromWorkspaceEnvWithSource() error = %v", err)
	}
	if cfg.WorkspaceRoot != "/with-source" {
		t.Fatalf("WorkspaceRoot = %q, want /with-source", cfg.WorkspaceRoot)
	}
	if sourcePath != envPath {
		t.Fatalf("sourcePath = %q, want %q", sourcePath, envPath)
	}
}
