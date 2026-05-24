package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"notion-local-ops-mcp-go/internal/config"
	"notion-local-ops-mcp-go/internal/taskstore"
	"notion-local-ops-mcp-go/internal/tools"
)

func newTask3TestServer() *Server {
	return NewServer(config.Config{
		Host:          "127.0.0.1",
		Port:          8766,
		WorkspaceRoot: filepath.Clean(filepath.Join("..", "..")),
		StateDir:      filepath.Clean(filepath.Join("..", "..", ".tmp", "mcp-handshake-state")),
		AuthToken:     "secret-token",
	})
}

func newAuthorizedMCPRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "http://example.test"+tools.MCPPath, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestToolsListIncludesServerInfo(t *testing.T) {
	server := newTask3TestServer()
	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/list status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			Tools []struct {
				Name        string         `json:"name"`
				InputSchema map[string]any `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	required := map[string]bool{
		"server_info":     false,
		"list_skills":     false,
		"set_default_cwd": false,
		"get_default_cwd": false,
		"write_file":      false,
		"purge_tasks":     false,
	}
	for _, tool := range payload.Result.Tools {
		if _, ok := required[tool.Name]; ok {
			required[tool.Name] = true
		}
	}
	for name, present := range required {
		if !present {
			t.Fatalf("%s must be listed", name)
		}
	}

	var waitSchema map[string]any
	var getSchema map[string]any
	var getLogsSchema map[string]any
	for _, tool := range payload.Result.Tools {
		switch tool.Name {
		case "wait_task":
			waitSchema = tool.InputSchema
		case "get_task":
			getSchema = tool.InputSchema
		case "get_task_logs":
			getLogsSchema = tool.InputSchema
		}
	}
	if waitSchema == nil {
		t.Fatal("wait_task schema must be listed")
	}
	waitProps, _ := waitSchema["properties"].(map[string]any)
	if _, ok := waitProps["timeout_seconds"]; !ok {
		t.Fatal("wait_task schema must expose timeout_seconds")
	}
	if _, ok := waitProps["last_event_seq"]; !ok {
		t.Fatal("wait_task schema must expose last_event_seq")
	}
	if getSchema == nil {
		t.Fatal("get_task schema must be listed")
	}
	getProps, _ := getSchema["properties"].(map[string]any)
	if _, ok := getProps["task_id"]; !ok {
		t.Fatal("get_task schema must expose task_id")
	}
	if getLogsSchema == nil {
		t.Fatal("get_task_logs schema must be listed")
	}
	getLogsProps, _ := getLogsSchema["properties"].(map[string]any)
	if _, ok := getLogsProps["task_id"]; !ok {
		t.Fatal("get_task_logs schema must expose task_id")
	}
	if _, ok := getLogsProps["stream"]; !ok {
		t.Fatal("get_task_logs schema must expose stream")
	}
	if _, ok := getLogsProps["offset"]; !ok {
		t.Fatal("get_task_logs schema must expose offset")
	}
	if _, ok := getLogsProps["limit"]; !ok {
		t.Fatal("get_task_logs schema must expose limit")
	}
}

func TestToolsListIncludesAwaitTaskAndListRecentTasksSchemas(t *testing.T) {
	server := newTask3TestServer()
	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/list status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			Tools []struct {
				Name        string         `json:"name"`
				InputSchema map[string]any `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	var awaitSchema map[string]any
	var recentSchema map[string]any
	for _, tool := range payload.Result.Tools {
		switch tool.Name {
		case "await_task":
			awaitSchema = tool.InputSchema
		case "list_recent_tasks":
			recentSchema = tool.InputSchema
		}
	}

	if awaitSchema == nil {
		t.Fatal("await_task schema must be listed")
	}
	awaitProps, _ := awaitSchema["properties"].(map[string]any)
	for _, field := range []string{"task_id", "max_wait_seconds", "last_event_seq", "include_logs", "log_stream", "log_limit"} {
		if _, ok := awaitProps[field]; !ok {
			t.Fatalf("await_task schema must expose %s", field)
		}
	}
	awaitRequired, _ := awaitSchema["required"].([]any)
	if len(awaitRequired) != 1 || awaitRequired[0] != "task_id" {
		t.Fatalf("await_task required = %#v, want [task_id]", awaitRequired)
	}

	if recentSchema == nil {
		t.Fatal("list_recent_tasks schema must be listed")
	}
	recentProps, _ := recentSchema["properties"].(map[string]any)
	for _, field := range []string{"status", "limit"} {
		if _, ok := recentProps[field]; !ok {
			t.Fatalf("list_recent_tasks schema must expose %s", field)
		}
	}
}

func TestInitializeReturnsProtocolVersionOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp initialize status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result["protocolVersion"] == "" {
		t.Fatal("protocolVersion must be present")
	}
}

func TestToolsCallReturnsSkeletonForKnownToolOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"server_info"}}`)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError           bool           `json:"isError"`
			StructuredContent map[string]any `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("tools/call server_info must succeed")
	}
	if payload.Result.StructuredContent["transport"] != "streamable-http" {
		t.Fatalf("structuredContent.transport = %v, want streamable-http", payload.Result.StructuredContent["transport"])
	}
	if len(payload.Result.Content) == 0 || !strings.Contains(payload.Result.Content[0].Text, "\"mcp_path\":\"/mcp\"") {
		t.Fatal("expected server_info tools/call response to include mcp_path")
	}
}

func TestToolsCallListFilesReturnsWorkspaceEntriesOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_files","path":"internal","recursive":false,"limit":20}}`)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call list_files status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success bool `json:"success"`
				Entries []struct {
					Path string `json:"path"`
				} `json:"entries"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("list_files must succeed")
	}
	if !payload.Result.StructuredContent.Success {
		t.Fatal("list_files success must be true")
	}
	if len(payload.Result.StructuredContent.Entries) == 0 {
		t.Fatal("list_files must return at least one entry")
	}
}

func TestToolsCallReadTextSupportsPaginationOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	workspace := t.TempDir()
	server.cfg.WorkspaceRoot = workspace
	if err := os.WriteFile(filepath.Join(workspace, "demo.txt"), []byte("alpha\nbeta\ngamma\ndelta\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_text","path":"demo.txt","start_line":2,"line_limit":2,"include_line_numbers":true}}`)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call read_text status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success   bool   `json:"success"`
				Content   string `json:"content"`
				StartLine int    `json:"start_line"`
				EndLine   int    `json:"end_line"`
				Truncated bool   `json:"truncated"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("read_text must succeed")
	}
	if payload.Result.StructuredContent.Content != "2: beta\n3: gamma" {
		t.Fatalf("content = %q, want numbered paged content", payload.Result.StructuredContent.Content)
	}
	if payload.Result.StructuredContent.StartLine != 2 || payload.Result.StructuredContent.EndLine != 3 {
		t.Fatalf("line range = %d-%d, want 2-3", payload.Result.StructuredContent.StartLine, payload.Result.StructuredContent.EndLine)
	}
	if !payload.Result.StructuredContent.Truncated {
		t.Fatal("truncated must be true")
	}
}

func TestToolsCallSearchSupportsTextModeOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	workspace := t.TempDir()
	server.cfg.WorkspaceRoot = workspace
	if err := os.MkdirAll(filepath.Join(workspace, "docs"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "docs", "demo.txt"), []byte("alpha\nTODO item\nomega\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search","mode":"text","path":"docs","query":"TODO","before":1,"after":1,"limit":10}}`)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call search status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success bool `json:"success"`
				Matches []struct {
					Path          string   `json:"path"`
					LineNumber    int      `json:"line_number"`
					ContextBefore []string `json:"context_before"`
					ContextAfter  []string `json:"context_after"`
				} `json:"matches"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("search must succeed")
	}
	if !payload.Result.StructuredContent.Success {
		t.Fatal("search success must be true")
	}
	if len(payload.Result.StructuredContent.Matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1", len(payload.Result.StructuredContent.Matches))
	}
	if payload.Result.StructuredContent.Matches[0].LineNumber != 2 {
		t.Fatalf("line_number = %d, want 2", payload.Result.StructuredContent.Matches[0].LineNumber)
	}
	if len(payload.Result.StructuredContent.Matches[0].ContextBefore) != 1 || payload.Result.StructuredContent.Matches[0].ContextBefore[0] != "alpha" {
		t.Fatalf("context_before = %#v, want [alpha]", payload.Result.StructuredContent.Matches[0].ContextBefore)
	}
	if len(payload.Result.StructuredContent.Matches[0].ContextAfter) != 1 || payload.Result.StructuredContent.Matches[0].ContextAfter[0] != "omega" {
		t.Fatalf("context_after = %#v, want [omega]", payload.Result.StructuredContent.Matches[0].ContextAfter)
	}
}

func TestToolsCallRunCommandReturnsForegroundResultOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"run_command","command":"echo hello"}}`)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call run_command status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success  bool   `json:"success"`
				ExitCode int    `json:"exit_code"`
				Stdout   string `json:"stdout"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("run_command must succeed")
	}
	if !payload.Result.StructuredContent.Success {
		t.Fatal("structuredContent.success must be true")
	}
	if payload.Result.StructuredContent.ExitCode != 0 {
		t.Fatalf("structuredContent.exit_code = %d, want 0", payload.Result.StructuredContent.ExitCode)
	}
	if !strings.Contains(payload.Result.StructuredContent.Stdout, "hello") {
		t.Fatalf("structuredContent.stdout = %q, want to contain hello", payload.Result.StructuredContent.Stdout)
	}
}

func TestToolsCallRunCommandAcceptsWrappedToolArgumentsOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"run_command","toolArguments":{"command":"echo hello wrapped"}}}`)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call wrapped run_command status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success  bool   `json:"success"`
				ExitCode int    `json:"exit_code"`
				Stdout   string `json:"stdout"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("wrapped run_command must succeed")
	}
	if !payload.Result.StructuredContent.Success {
		t.Fatal("wrapped structuredContent.success must be true")
	}
	if payload.Result.StructuredContent.ExitCode != 0 {
		t.Fatalf("wrapped structuredContent.exit_code = %d, want 0", payload.Result.StructuredContent.ExitCode)
	}
	if !strings.Contains(payload.Result.StructuredContent.Stdout, "hello wrapped") {
		t.Fatalf("wrapped structuredContent.stdout = %q, want to contain hello wrapped", payload.Result.StructuredContent.Stdout)
	}
}

func TestToolsCallGitStatusReturnsStructuredResultOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"git_status"}}`)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call git_status status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success  bool   `json:"success"`
				RepoRoot string `json:"repo_root"`
				Branch   string `json:"branch"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("git_status must succeed")
	}
	if !payload.Result.StructuredContent.Success {
		t.Fatal("structuredContent.success must be true")
	}
	if payload.Result.StructuredContent.RepoRoot == "" {
		t.Fatal("repo_root must not be empty")
	}
	if payload.Result.StructuredContent.Branch == "" {
		t.Fatal("branch must not be empty")
	}
}

func TestToolsCallGitCommitDryRunReturnsPlanOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"git_commit","message":"feat: dry run","paths":["README.md"],"dry_run":true}}`)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call git_commit status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success bool     `json:"success"`
				DryRun  bool     `json:"dry_run"`
				Summary string   `json:"summary"`
				Files   []string `json:"files"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("git_commit dry run must succeed")
	}
	if !payload.Result.StructuredContent.Success {
		t.Fatal("structuredContent.success must be true")
	}
	if !payload.Result.StructuredContent.DryRun {
		t.Fatal("structuredContent.dry_run must be true")
	}
	if payload.Result.StructuredContent.Summary != "feat: dry run" {
		t.Fatalf("summary = %q, want feat: dry run", payload.Result.StructuredContent.Summary)
	}
	if len(payload.Result.StructuredContent.Files) == 0 {
		t.Fatal("files must not be empty in dry run response")
	}
}

func TestToolsCallGitBlameUsesRepoRelativePathWhenSessionCWDIsSubdirOverMCPRoute(t *testing.T) {
	tools.ClearDefaultCWD()
	t.Cleanup(tools.ClearDefaultCWD)

	server := newTask3TestServer()
	repoRoot, err := filepath.Abs(server.cfg.WorkspaceRoot)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	sampleDir := filepath.Join(repoRoot, "docs")

	setReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"set_default_cwd","path":"` + strings.ReplaceAll(sampleDir, `\`, `\\`) + `"}}`)
	setRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(setRec, setReq)
	if setRec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call set_default_cwd status = %d, want %d", setRec.Code, http.StatusOK)
	}

	blameReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"git_blame","path":"README.md","start_line":1,"end_line":3}}`)
	blameRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(blameRec, blameReq)
	if blameRec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call git_blame status = %d, want %d", blameRec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success bool `json:"success"`
				Path    string `json:"path"`
				Entries []struct {
					Line        int    `json:"line"`
					Commit      string `json:"commit"`
					ShortCommit string `json:"short_commit"`
					Content     string `json:"content"`
				} `json:"entries"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(blameRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("git_blame must succeed")
	}
	if !payload.Result.StructuredContent.Success {
		t.Fatal("structuredContent.success must be true")
	}
	if payload.Result.StructuredContent.Path != "README.md" {
		t.Fatalf("path = %q, want README.md", payload.Result.StructuredContent.Path)
	}
	if len(payload.Result.StructuredContent.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(payload.Result.StructuredContent.Entries))
	}
	if payload.Result.StructuredContent.Entries[0].Line != 1 {
		t.Fatalf("first blamed line = %d, want 1", payload.Result.StructuredContent.Entries[0].Line)
	}
	if payload.Result.StructuredContent.Entries[0].Commit == "" {
		t.Fatal("git_blame must return commit metadata")
	}
}

func TestToolsCallTaskPollingChainWorksOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()

	startReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"run_command_stream"}}`)
	startRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call run_command_stream status = %d, want %d", startRec.Code, http.StatusOK)
	}

	var startPayload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				TaskID string `json:"task_id"`
				Status string `json:"status"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &startPayload); err != nil {
		t.Fatalf("json.Unmarshal() start error = %v", err)
	}
	if startPayload.Result.IsError {
		t.Fatal("run_command_stream must succeed")
	}
	if startPayload.Result.StructuredContent.TaskID == "" {
		t.Fatal("run_command_stream must return task_id")
	}

	taskID := startPayload.Result.StructuredContent.TaskID
	waitReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"wait_task","task_id":"` + taskID + `","timeout_seconds":1,"last_event_seq":0}}`)
	waitRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(waitRec, waitReq)
	if waitRec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call wait_task status = %d, want %d", waitRec.Code, http.StatusOK)
	}

	getReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_task","task_id":"` + taskID + `"}}`)
	getRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call get_task status = %d, want %d", getRec.Code, http.StatusOK)
	}

	logsReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_task_logs","task_id":"` + taskID + `","stream":"stdout","offset":0,"limit":64}}`)
	logsRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(logsRec, logsReq)
	if logsRec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call get_task_logs status = %d, want %d", logsRec.Code, http.StatusOK)
	}

	var waitPayload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				TaskID                  string `json:"task_id"`
				Status                  string `json:"status"`
				EventSeq                int64  `json:"event_seq"`
				ProgressPercent         *int   `json:"progress_percent"`
				ProgressMessage         string `json:"progress_message"`
				HeartbeatAt             string `json:"heartbeat_at"`
				RecommendedPollStrategy string `json:"recommended_poll_strategy"`
				NextPollAfterSeconds    int    `json:"next_poll_after_seconds"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(waitRec.Body.Bytes(), &waitPayload); err != nil {
		t.Fatalf("json.Unmarshal() wait_task error = %v", err)
	}
	if waitPayload.Result.IsError {
		t.Fatal("wait_task must succeed")
	}
	if waitPayload.Result.StructuredContent.TaskID != taskID {
		t.Fatalf("wait_task task_id = %q, want %q", waitPayload.Result.StructuredContent.TaskID, taskID)
	}
	if waitPayload.Result.StructuredContent.EventSeq == 0 {
		t.Fatal("wait_task must return event_seq")
	}
	if waitPayload.Result.StructuredContent.Status == "" {
		t.Fatal("wait_task must return status")
	}
	if waitPayload.Result.StructuredContent.HeartbeatAt == "" {
		t.Fatal("wait_task must return heartbeat_at")
	}
	if waitPayload.Result.StructuredContent.RecommendedPollStrategy == "" {
		t.Fatal("wait_task must return recommended_poll_strategy")
	}
	if waitPayload.Result.StructuredContent.NextPollAfterSeconds < 0 {
		t.Fatal("wait_task must return non-negative next_poll_after_seconds")
	}

	var getPayload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				TaskID                  string `json:"task_id"`
				Status                  string `json:"status"`
				EventSeq                int64  `json:"event_seq"`
				ProgressPercent         *int   `json:"progress_percent"`
				ProgressMessage         string `json:"progress_message"`
				HeartbeatAt             string `json:"heartbeat_at"`
				RecommendedPollStrategy string `json:"recommended_poll_strategy"`
				NextPollAfterSeconds    int    `json:"next_poll_after_seconds"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getPayload); err != nil {
		t.Fatalf("json.Unmarshal() get_task error = %v", err)
	}
	if getPayload.Result.IsError {
		t.Fatal("get_task must succeed")
	}
	if getPayload.Result.StructuredContent.TaskID != taskID {
		t.Fatalf("get_task task_id = %q, want %q", getPayload.Result.StructuredContent.TaskID, taskID)
	}
	if getPayload.Result.StructuredContent.Status == "" {
		t.Fatal("get_task must return status")
	}
	if getPayload.Result.StructuredContent.EventSeq == 0 {
		t.Fatal("get_task must return event_seq")
	}
	if getPayload.Result.StructuredContent.HeartbeatAt == "" {
		t.Fatal("get_task must return heartbeat_at")
	}
	if getPayload.Result.StructuredContent.RecommendedPollStrategy == "" {
		t.Fatal("get_task must return recommended_poll_strategy")
	}
	if getPayload.Result.StructuredContent.NextPollAfterSeconds < 0 {
		t.Fatal("get_task must return non-negative next_poll_after_seconds")
	}

	var logsPayload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success    bool   `json:"success"`
				TaskID     string `json:"task_id"`
				Stream     string `json:"stream"`
				Offset     int64  `json:"offset"`
				NextOffset int64  `json:"next_offset"`
				Truncated  bool   `json:"truncated"`
				Content    string `json:"content"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(logsRec.Body.Bytes(), &logsPayload); err != nil {
		t.Fatalf("json.Unmarshal() get_task_logs error = %v", err)
	}
	if logsPayload.Result.IsError {
		t.Fatal("get_task_logs must succeed")
	}
	if logsPayload.Result.StructuredContent.TaskID != taskID {
		t.Fatalf("get_task_logs task_id = %q, want %q", logsPayload.Result.StructuredContent.TaskID, taskID)
	}
	if logsPayload.Result.StructuredContent.Stream != "stdout" {
		t.Fatalf("get_task_logs stream = %q, want stdout", logsPayload.Result.StructuredContent.Stream)
	}
	if logsPayload.Result.StructuredContent.Success && logsPayload.Result.StructuredContent.NextOffset < logsPayload.Result.StructuredContent.Offset {
		t.Fatal("get_task_logs next_offset must be >= offset")
	}
}

func TestToolsCallSetAndGetDefaultCWDOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	workspace := t.TempDir()
	server.cfg.WorkspaceRoot = workspace
	targetDir := filepath.Join(workspace, "CTF")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	setReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"set_default_cwd","path":"` + strings.ReplaceAll(targetDir, `\`, `\\`) + `"}}`)
	setRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(setRec, setReq)
	if setRec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call set_default_cwd status = %d, want %d", setRec.Code, http.StatusOK)
	}

	getReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_default_cwd"}}`)
	getRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call get_default_cwd status = %d, want %d", getRec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				SessionCWD   string `json:"session_cwd"`
				EffectiveCWD string `json:"effective_cwd"`
				Source       string `json:"source"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("get_default_cwd must succeed")
	}
	if payload.Result.StructuredContent.SessionCWD != targetDir {
		t.Fatalf("session_cwd = %q, want %q", payload.Result.StructuredContent.SessionCWD, targetDir)
	}
	if payload.Result.StructuredContent.EffectiveCWD != targetDir {
		t.Fatalf("effective_cwd = %q, want %q", payload.Result.StructuredContent.EffectiveCWD, targetDir)
	}
	if payload.Result.StructuredContent.Source != "session" {
		t.Fatalf("source = %q, want session", payload.Result.StructuredContent.Source)
	}
}

func TestToolsCallSetDefaultCWDRejectsPathOutsideWorkspaceOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	server.cfg.WorkspaceRoot = t.TempDir()
	outside := t.TempDir()

	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"set_default_cwd","path":"` + strings.ReplaceAll(outside, `\`, `\\`) + `"}}`)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call set_default_cwd status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !payload.Result.IsError {
		t.Fatal("set_default_cwd outside workspace must fail")
	}
	if len(payload.Result.Content) == 0 || !strings.Contains(payload.Result.Content[0].Text, "path escapes workspace") {
		t.Fatalf("error text = %#v, want path escapes workspace", payload.Result.Content)
	}
}

func TestToolsCallGetTaskLogsOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	stateDir := t.TempDir()
	server.cfg.StateDir = stateDir

	store := taskstore.NewFSStore(stateDir)
	task, err := store.Create(taskstore.TaskInput{
		Task:     "job",
		Executor: "run_command_stream",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := store.WriteLogs(task.ID, "hello\nworld\n", ""); err != nil {
		t.Fatalf("WriteLogs() error = %v", err)
	}

	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_task_logs","task_id":"` + task.ID + `","stream":"stdout","offset":6,"limit":5}}`)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call get_task_logs status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success    bool   `json:"success"`
				TaskID     string `json:"task_id"`
				Stream     string `json:"stream"`
				Offset     int64  `json:"offset"`
				NextOffset int64  `json:"next_offset"`
				Truncated  bool   `json:"truncated"`
				Content    string `json:"content"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("get_task_logs must succeed")
	}
	if !payload.Result.StructuredContent.Success {
		t.Fatal("get_task_logs success = false, want true")
	}
	if payload.Result.StructuredContent.TaskID != task.ID {
		t.Fatalf("task_id = %q, want %q", payload.Result.StructuredContent.TaskID, task.ID)
	}
	if payload.Result.StructuredContent.Stream != "stdout" {
		t.Fatalf("stream = %q, want stdout", payload.Result.StructuredContent.Stream)
	}
	if payload.Result.StructuredContent.Offset != 6 {
		t.Fatalf("offset = %d, want 6", payload.Result.StructuredContent.Offset)
	}
	if payload.Result.StructuredContent.NextOffset != 11 {
		t.Fatalf("next_offset = %d, want 11", payload.Result.StructuredContent.NextOffset)
	}
	if !payload.Result.StructuredContent.Truncated {
		t.Fatal("truncated = false, want true")
	}
	if payload.Result.StructuredContent.Content != "world" {
		t.Fatalf("content = %q, want world", payload.Result.StructuredContent.Content)
	}
}

func TestToolsCallListRecentTasksOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	stateDir := t.TempDir()
	server.cfg.StateDir = stateDir

	store := taskstore.NewFSStore(stateDir)
	runningTask, err := store.Create(taskstore.TaskInput{
		Task:     "running job",
		Executor: "run_command_stream",
	})
	if err != nil {
		t.Fatalf("Create() running task error = %v", err)
	}
	runningTask.Status = "running"
	runningTask.EventSeq = 4
	runningTask.ProgressMessage = "still working"
	if _, err := store.Update(runningTask.ID, runningTask); err != nil {
		t.Fatalf("Update() running task error = %v", err)
	}
	if err := store.WriteSummary(runningTask.ID, "running summary"); err != nil {
		t.Fatalf("WriteSummary() running task error = %v", err)
	}

	doneTask, err := store.Create(taskstore.TaskInput{
		Task:     "done job",
		Executor: "run_command_stream",
	})
	if err != nil {
		t.Fatalf("Create() done task error = %v", err)
	}
	doneTask.Status = "succeeded"
	doneTask.EventSeq = 8
	if _, err := store.Update(doneTask.ID, doneTask); err != nil {
		t.Fatalf("Update() done task error = %v", err)
	}
	if err := store.WriteSummary(doneTask.ID, "done summary"); err != nil {
		t.Fatalf("WriteSummary() done task error = %v", err)
	}

	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_recent_tasks","status":"running","limit":5}}`)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call list_recent_tasks status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success bool `json:"success"`
				Tasks   []struct {
					TaskID          string `json:"task_id"`
					Status          string `json:"status"`
					EventSeq        int64  `json:"event_seq"`
					ProgressMessage string `json:"progress_message"`
					Summary         string `json:"summary"`
				} `json:"tasks"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("list_recent_tasks must succeed")
	}
	if !payload.Result.StructuredContent.Success {
		t.Fatal("list_recent_tasks success = false, want true")
	}
	if len(payload.Result.StructuredContent.Tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(payload.Result.StructuredContent.Tasks))
	}
	task := payload.Result.StructuredContent.Tasks[0]
	if task.TaskID != runningTask.ID {
		t.Fatalf("task_id = %q, want %q", task.TaskID, runningTask.ID)
	}
	if task.Status != "running" {
		t.Fatalf("status = %q, want running", task.Status)
	}
	if task.EventSeq != 4 {
		t.Fatalf("event_seq = %d, want 4", task.EventSeq)
	}
	if task.ProgressMessage != "still working" {
		t.Fatalf("progress_message = %q, want still working", task.ProgressMessage)
	}
	if task.Summary != "running summary" {
		t.Fatalf("summary = %q, want running summary", task.Summary)
	}
}

func TestToolsCallAwaitTaskOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	stateDir := t.TempDir()
	server.cfg.StateDir = stateDir

	store := taskstore.NewFSStore(stateDir)
	task, err := store.Create(taskstore.TaskInput{
		Task:     "done job",
		Executor: "run_command_stream",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	task.Status = "succeeded"
	task.EventSeq = 5
	if _, err := store.Update(task.ID, task); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := store.WriteSummary(task.ID, "done summary"); err != nil {
		t.Fatalf("WriteSummary() error = %v", err)
	}

	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"await_task","task_id":"` + task.ID + `","max_wait_seconds":1}}`)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call await_task status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success               bool   `json:"success"`
				TaskID                string `json:"task_id"`
				Status                string `json:"status"`
				Terminal              bool   `json:"terminal"`
				RecommendedNextAction string `json:"recommended_next_action"`
				ResumeToken           string `json:"resume_token"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("await_task must succeed")
	}
	if !payload.Result.StructuredContent.Success {
		t.Fatal("success = false, want true")
	}
	if payload.Result.StructuredContent.TaskID != task.ID {
		t.Fatalf("task_id = %q, want %q", payload.Result.StructuredContent.TaskID, task.ID)
	}
	if payload.Result.StructuredContent.Status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", payload.Result.StructuredContent.Status)
	}
	if !payload.Result.StructuredContent.Terminal {
		t.Fatal("terminal = false, want true")
	}
	if payload.Result.StructuredContent.RecommendedNextAction != "stop" {
		t.Fatalf("recommended_next_action = %q, want stop", payload.Result.StructuredContent.RecommendedNextAction)
	}
	if payload.Result.StructuredContent.ResumeToken == "" {
		t.Fatal("resume_token = empty, want non-empty")
	}
}

func TestToolsCallAwaitTaskOverMCPRouteReturnsStructuredFailureForMissingTask(t *testing.T) {
	server := newTask3TestServer()
	stateDir := t.TempDir()
	server.cfg.StateDir = stateDir

	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"await_task","task_id":"missing-task","max_wait_seconds":1,"last_event_seq":7}}`)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call await_task missing task status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success               bool   `json:"success"`
				TaskID                string `json:"task_id"`
				Status                string `json:"status"`
				Terminal              bool   `json:"terminal"`
				RecommendedNextAction string `json:"recommended_next_action"`
				ResumeToken           string `json:"resume_token"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("await_task missing task must stay a structured tool result, not RPC error")
	}
	if payload.Result.StructuredContent.Success {
		t.Fatal("success = true, want false for missing task")
	}
	if payload.Result.StructuredContent.TaskID != "missing-task" {
		t.Fatalf("task_id = %q, want missing-task", payload.Result.StructuredContent.TaskID)
	}
	if payload.Result.StructuredContent.Status != "failed" {
		t.Fatalf("status = %q, want failed", payload.Result.StructuredContent.Status)
	}
	if !payload.Result.StructuredContent.Terminal {
		t.Fatal("terminal = false, want true for missing task")
	}
	if payload.Result.StructuredContent.RecommendedNextAction != "stop" {
		t.Fatalf("recommended_next_action = %q, want stop", payload.Result.StructuredContent.RecommendedNextAction)
	}
	if payload.Result.StructuredContent.ResumeToken != "missing-task:0" {
		t.Fatalf("resume_token = %q, want missing-task:0", payload.Result.StructuredContent.ResumeToken)
	}
}

func TestToolsCallAwaitTaskOverMCPRouteWaitsForEventAfterLastEventSeq(t *testing.T) {
	server := newTask3TestServer()
	stateDir := t.TempDir()
	server.cfg.StateDir = stateDir

	store := taskstore.NewFSStore(stateDir)
	task, err := store.Create(taskstore.TaskInput{
		Task:     "running job",
		Executor: "run_command_stream",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	task.Status = "running"
	task.EventSeq = 3
	task.ProgressMessage = "warming up"
	if _, err := store.Update(task.ID, task); err != nil {
		t.Fatalf("Update() initial error = %v", err)
	}
	if err := store.WriteSummary(task.ID, "initial summary"); err != nil {
		t.Fatalf("WriteSummary() initial error = %v", err)
	}

	go func(taskID string) {
		time.Sleep(150 * time.Millisecond)

		next, err := store.Get(taskID)
		if err != nil {
			return
		}
		next.Status = "running"
		next.EventSeq = 4
		next.ProgressMessage = "still working"
		if _, err := store.Update(taskID, next); err != nil {
			return
		}
		_ = store.WriteSummary(taskID, "updated summary")
	}(task.ID)

	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"await_task","task_id":"` + task.ID + `","max_wait_seconds":2,"last_event_seq":3}}`)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call await_task running task status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success               bool   `json:"success"`
				TaskID                string `json:"task_id"`
				Status                string `json:"status"`
				Terminal              bool   `json:"terminal"`
				EventSeq              int64  `json:"event_seq"`
				ProgressMessage       string `json:"progress_message"`
				Summary               string `json:"summary"`
				RecommendedNextAction string `json:"recommended_next_action"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("await_task running task must succeed")
	}
	if !payload.Result.StructuredContent.Success {
		t.Fatal("success = false, want true")
	}
	if payload.Result.StructuredContent.TaskID != task.ID {
		t.Fatalf("task_id = %q, want %q", payload.Result.StructuredContent.TaskID, task.ID)
	}
	if payload.Result.StructuredContent.Status != "running" {
		t.Fatalf("status = %q, want running", payload.Result.StructuredContent.Status)
	}
	if payload.Result.StructuredContent.Terminal {
		t.Fatal("terminal = true, want false for running task")
	}
	if payload.Result.StructuredContent.EventSeq != 4 {
		t.Fatalf("event_seq = %d, want 4 after waiting past last_event_seq", payload.Result.StructuredContent.EventSeq)
	}
	if payload.Result.StructuredContent.ProgressMessage != "still working" {
		t.Fatalf("progress_message = %q, want still working", payload.Result.StructuredContent.ProgressMessage)
	}
	if !strings.Contains(payload.Result.StructuredContent.Summary, "task running; use wait_task") {
		t.Fatalf("summary = %q, want running wait_task guidance", payload.Result.StructuredContent.Summary)
	}
	if !strings.Contains(payload.Result.StructuredContent.Summary, "get_task_logs") {
		t.Fatalf("summary = %q, want get_task_logs guidance", payload.Result.StructuredContent.Summary)
	}
	if !strings.Contains(payload.Result.StructuredContent.Summary, "still working") {
		t.Fatalf("summary = %q, want progress hint", payload.Result.StructuredContent.Summary)
	}
	if payload.Result.StructuredContent.RecommendedNextAction != "await_task" {
		t.Fatalf("recommended_next_action = %q, want await_task", payload.Result.StructuredContent.RecommendedNextAction)
	}
}

func TestToolsCallAwaitTaskOverMCPRouteDefaultsLogStreamToStdout(t *testing.T) {
	server := newTask3TestServer()
	stateDir := t.TempDir()
	server.cfg.StateDir = stateDir

	store := taskstore.NewFSStore(stateDir)
	task, err := store.Create(taskstore.TaskInput{
		Task:     "done job",
		Executor: "run_command_stream",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	task.Status = "succeeded"
	task.EventSeq = 9
	if _, err := store.Update(task.ID, task); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := store.WriteSummary(task.ID, "done summary"); err != nil {
		t.Fatalf("WriteSummary() error = %v", err)
	}
	if err := store.WriteLogs(task.ID, "hello from stdout\n", "ignored stderr\n"); err != nil {
		t.Fatalf("WriteLogs() error = %v", err)
	}

	req := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"await_task","task_id":"` + task.ID + `","include_logs":true,"max_wait_seconds":1,"last_event_seq":7,"log_limit":5}}`)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call await_task status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success               bool `json:"success"`
				TaskID                string `json:"task_id"`
				Status                string `json:"status"`
				Terminal              bool   `json:"terminal"`
				EventSeq              int64  `json:"event_seq"`
				Summary               string `json:"summary"`
				RecommendedNextAction string `json:"recommended_next_action"`
				Logs                  *struct {
					Stream     string `json:"stream"`
					Offset     int64  `json:"offset"`
					NextOffset int64  `json:"next_offset"`
					Truncated  bool   `json:"truncated"`
					Content    string `json:"content"`
				} `json:"logs"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("await_task must succeed")
	}
	if !payload.Result.StructuredContent.Success {
		t.Fatal("await_task success = false, want true")
	}
	if payload.Result.StructuredContent.TaskID != task.ID {
		t.Fatalf("task_id = %q, want %q", payload.Result.StructuredContent.TaskID, task.ID)
	}
	if payload.Result.StructuredContent.Status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", payload.Result.StructuredContent.Status)
	}
	if !payload.Result.StructuredContent.Terminal {
		t.Fatal("terminal = false, want true")
	}
	if payload.Result.StructuredContent.EventSeq != 9 {
		t.Fatalf("event_seq = %d, want 9", payload.Result.StructuredContent.EventSeq)
	}
	if payload.Result.StructuredContent.RecommendedNextAction != "stop" {
		t.Fatalf("recommended_next_action = %q, want stop", payload.Result.StructuredContent.RecommendedNextAction)
	}
	if payload.Result.StructuredContent.Logs == nil {
		t.Fatal("logs = nil, want stdout excerpt")
	}
	if payload.Result.StructuredContent.Logs.Stream != "stdout" {
		t.Fatalf("logs.stream = %q, want stdout", payload.Result.StructuredContent.Logs.Stream)
	}
	if payload.Result.StructuredContent.Logs.Offset != 0 {
		t.Fatalf("logs.offset = %d, want 0", payload.Result.StructuredContent.Logs.Offset)
	}
	if payload.Result.StructuredContent.Logs.NextOffset != 5 {
		t.Fatalf("logs.next_offset = %d, want 5", payload.Result.StructuredContent.Logs.NextOffset)
	}
	if !payload.Result.StructuredContent.Logs.Truncated {
		t.Fatal("logs.truncated = false, want true after log_limit=5")
	}
	if payload.Result.StructuredContent.Logs.Content != "hello" {
		t.Fatalf("logs.content = %q, want hello", payload.Result.StructuredContent.Logs.Content)
	}
}

func TestToolsCallWriteFileDryRunAndRealWriteOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	workspace := t.TempDir()
	server.cfg.WorkspaceRoot = workspace

	dryRunReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_file","path":"notes/demo.txt","content":"hello","dry_run":true}}`)
	dryRunRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(dryRunRec, dryRunReq)
	if dryRunRec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call write_file dry run status = %d, want %d", dryRunRec.Code, http.StatusOK)
	}
	if _, err := os.Stat(filepath.Join(workspace, "notes", "demo.txt")); !os.IsNotExist(err) {
		t.Fatalf("dry run should not create file, stat error = %v", err)
	}

	writeReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_file","path":"notes/demo.txt","content":"hello"}}`)
	writeRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(writeRec, writeReq)
	if writeRec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call write_file status = %d, want %d", writeRec.Code, http.StatusOK)
	}
	content, err := os.ReadFile(filepath.Join(workspace, "notes", "demo.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("content = %q, want hello", string(content))
	}
}

func TestToolsCallPurgeTasksOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	stateDir := t.TempDir()
	server.cfg.StateDir = stateDir

	purgeReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"purge_tasks","older_than_hours":168,"dry_run":true}}`)
	purgeRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(purgeRec, purgeReq)
	if purgeRec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call purge_tasks status = %d, want %d", purgeRec.Code, http.StatusOK)
	}

	var purgePayload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success bool     `json:"success"`
				DryRun  bool     `json:"dry_run"`
				TaskIDs []string `json:"task_ids"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(purgeRec.Body.Bytes(), &purgePayload); err != nil {
		t.Fatalf("json.Unmarshal() purge error = %v", err)
	}
	if purgePayload.Result.IsError {
		t.Fatal("purge_tasks must succeed")
	}
	if !purgePayload.Result.StructuredContent.Success || !purgePayload.Result.StructuredContent.DryRun {
		t.Fatal("purge_tasks dry run must succeed")
	}
}
