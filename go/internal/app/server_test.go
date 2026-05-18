package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"notion-local-ops-mcp-go/internal/config"
	"notion-local-ops-mcp-go/internal/tools"
)

func TestServerExposesBootstrapRuntimeFields(t *testing.T) {
	server := NewServer(config.Config{
		Host:          "127.0.0.1",
		Port:          8766,
		WorkspaceRoot: "/workspace",
		StateDir:      "/state",
		AuthToken:     "secret",
	})

	if server.Addr() != "127.0.0.1:8766" {
		t.Fatalf("Addr() = %q, want 127.0.0.1:8766", server.Addr())
	}

	info := server.ServerInfo()
	toolNames, ok := info["tools"].([]string)
	if !ok {
		t.Fatalf("tools = %T, want []string", info["tools"])
	}
	if info["app_name"] == "" {
		t.Fatal("app_name must be present")
	}
	if info["app_name"] != tools.AppName {
		t.Fatalf("app_name = %v, want %q", info["app_name"], tools.AppName)
	}
	if info["transport"] != tools.Transport {
		t.Fatalf("transport = %v, want %q", info["transport"], tools.Transport)
	}
	if info["mcp_path"] != tools.MCPPath {
		t.Fatalf("mcp_path = %v, want %q", info["mcp_path"], tools.MCPPath)
	}
	if info["host"] != "127.0.0.1" {
		t.Fatalf("host = %v, want 127.0.0.1", info["host"])
	}
	if info["port"] != 8766 {
		t.Fatalf("port = %v, want 8766", info["port"])
	}
	if info["workspace_root"] != "/workspace" {
		t.Fatalf("workspace_root = %v, want /workspace", info["workspace_root"])
	}
	if info["state_dir"] != "/state" {
		t.Fatalf("state_dir = %v, want /state", info["state_dir"])
	}
	if info["auth_enabled"] != true {
		t.Fatalf("auth_enabled = %v, want true", info["auth_enabled"])
	}
	if info["auth_mode"] != "shared_token" {
		t.Fatalf("auth_mode = %v, want shared_token", info["auth_mode"])
	}
	if info["command_timeout_seconds"] != 0 {
		t.Fatalf("command_timeout_seconds = %v, want 0 for explicit zero config", info["command_timeout_seconds"])
	}
	if len(toolNames) == 0 {
		t.Fatal("tools must not be empty")
	}
	if info["tool_count"] != len(toolNames) {
		t.Fatalf("tool_count = %v, want %d", info["tool_count"], len(toolNames))
	}
}

func TestServerRootHandlerReturnsBootstrapStatus(t *testing.T) {
	server := NewServer(config.Config{
		Host:          "127.0.0.1",
		Port:          8766,
		WorkspaceRoot: "/workspace",
		StateDir:      "/state",
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("root status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["status"] != "bootstrap-active" {
		t.Fatalf("status = %v, want bootstrap-active", payload["status"])
	}
	if payload["server_info"] == nil {
		t.Fatal("server_info must be present")
	}
}

func TestServerMCPHandlerReturnsDiscoveryDocument(t *testing.T) {
	server := NewServer(config.Config{
		Host:          "127.0.0.1",
		Port:          8766,
		WorkspaceRoot: "/workspace",
		StateDir:      "/state",
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.test"+tools.MCPPath, nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("/mcp status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["protocolVersion"] == "" {
		t.Fatal("protocolVersion must be present")
	}
	transport, ok := payload["transport"].(map[string]any)
	if !ok {
		t.Fatalf("transport = %T, want map", payload["transport"])
	}
	if transport["type"] != tools.Transport {
		t.Fatalf("transport.type = %v, want %q", transport["type"], tools.Transport)
	}
	if transport["endpoint"] != tools.MCPPath {
		t.Fatalf("transport.endpoint = %v, want %q", transport["endpoint"], tools.MCPPath)
	}
}

func TestServerListenBindsEphemeralPort(t *testing.T) {
	server := NewServer(config.Config{
		Host:          "127.0.0.1",
		Port:          0,
		WorkspaceRoot: "/workspace",
		StateDir:      "/state",
	})

	listener, err := server.Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	if listener.Addr() == nil {
		t.Fatal("listener.Addr() must not be nil")
	}
}

func TestBuildDefaultServerPrefersDotEnvOverShell(t *testing.T) {
	tempDir := t.TempDir()
	nested := filepath.Join(tempDir, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte("NOTION_LOCAL_OPS_WORKSPACE_ROOT=/from-dotenv\nNOTION_LOCAL_OPS_PORT=8766\n"), 0o600); err != nil {
		t.Fatalf("Write .env error = %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("Chdir error = %v", err)
	}

	t.Setenv("NOTION_LOCAL_OPS_WORKSPACE_ROOT", "/from-shell")
	server, envPath, err := BuildDefaultServer()
	if err != nil {
		t.Fatalf("BuildDefaultServer() error = %v", err)
	}
	if server.cfg.WorkspaceRoot != "/from-dotenv" {
		t.Fatalf("WorkspaceRoot = %q, want /from-dotenv", server.cfg.WorkspaceRoot)
	}
	if envPath != filepath.Join(tempDir, ".env") {
		t.Fatalf("envPath = %q, want %q", envPath, filepath.Join(tempDir, ".env"))
	}
}
