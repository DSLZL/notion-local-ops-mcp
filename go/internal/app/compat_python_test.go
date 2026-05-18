package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"notion-local-ops-mcp-go/internal/config"
	"notion-local-ops-mcp-go/internal/tools"
)

func PythonServerInfoFixtureForTest() map[string]any {
	return map[string]any{
		"transport": "streamable-http",
		"mcp_path":  "/mcp",
	}
}

func GoServerInfoFixtureForTest() map[string]any {
	cfg := config.Config{
		Host:          "127.0.0.1",
		Port:          8767,
		WorkspaceRoot: "/workspace",
		StateDir:      "/state",
		AuthToken:     "token",
		CommandTimeout: 120,
	}
	return tools.BuildServerInfo(cfg, []string{"server_info", "list_files"})
}

func TestCompatibilityServerInfoShape(t *testing.T) {
	py := PythonServerInfoFixtureForTest()
	goInfo := GoServerInfoFixtureForTest()
	if py["transport"] != goInfo["transport"] {
		t.Fatalf("transport mismatch: py=%v go=%v", py["transport"], goInfo["transport"])
	}
	if py["mcp_path"] != goInfo["mcp_path"] {
		t.Fatalf("mcp_path mismatch: py=%v go=%v", py["mcp_path"], goInfo["mcp_path"])
	}
	if goInfo["tool_count"] != 2 {
		t.Fatalf("tool_count = %v, want 2", goInfo["tool_count"])
	}
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve compat test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func readRepoFile(t *testing.T, relativePath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRootForTest(t), relativePath))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	return string(data)
}

func requireSnippets(t *testing.T, label, content string, snippets []string) {
	t.Helper()
	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("%s missing snippet %q", label, snippet)
		}
	}
}

func TestCurrentDefaultEntryDocs(t *testing.T) {
	readme := readRepoFile(t, "README.md")
	envExample := readRepoFile(t, ".env.example")

	requireSnippets(t, "README.md", readme, []string{
		"./scripts/dev-tunnel.sh",
		"go run ./main.go",
		"ngrok",
		"Public MCP URL",
	})
	requireSnippets(t, ".env.example", envExample, []string{
		"NOTION_LOCAL_OPS_NGROK_COMMAND",
		"NOTION_LOCAL_OPS_NGROK_API_URL",
		"NOTION_LOCAL_OPS_AUTH_TOKEN",
	})
}

func TestReadmeMentionsAwaitTaskAndTaskRecovery(t *testing.T) {
	readme := readRepoFile(t, "README.md")
	readmeZhCN := readRepoFile(t, "README.zh-CN.md")

	requireSnippets(t, "README.md", readme, []string{
		"await_task",
		"list_recent_tasks",
		"web chat",
		"resume_token",
	})

	requireSnippets(t, "README.zh-CN.md", readmeZhCN, []string{
		"await_task",
		"list_recent_tasks",
		"网页端",
		"resume_token",
	})
}
