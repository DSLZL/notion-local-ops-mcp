package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestToolsListIncludesShellSessionSchemas(t *testing.T) {
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

	required := map[string][]string{
		"open_shell_session":  {"cwd", "shell"},
		"get_shell_session":   {"session_id"},
		"send_shell_input":    {"session_id", "input"},
		"read_shell_output":   {"session_id", "offset", "limit"},
		"close_shell_session": {"session_id"},
	}

	seen := map[string]bool{}
	for _, tool := range payload.Result.Tools {
		fields, ok := required[tool.Name]
		if !ok {
			continue
		}
		seen[tool.Name] = true
		props, _ := tool.InputSchema["properties"].(map[string]any)
		for _, field := range fields {
			if _, ok := props[field]; !ok {
				t.Fatalf("%s schema must expose %s", tool.Name, field)
			}
		}
	}

	for name := range required {
		if !seen[name] {
			t.Fatalf("%s must be listed", name)
		}
	}
}

func TestToolsCallShellSessionLifecycleOverMCPRoute(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("PTY MCP lifecycle test only runs on linux")
	}

	server := newTask3TestServer()
	stateDir := t.TempDir()
	workspace := t.TempDir()
	server.cfg.StateDir = stateDir
	server.cfg.WorkspaceRoot = workspace

	openReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"open_shell_session"}}`)
	openRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(openRec, openReq)
	if openRec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call open_shell_session status = %d, want %d", openRec.Code, http.StatusOK)
	}

	var openPayload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success bool `json:"success"`
				Session struct {
					ID string `json:"session_id"`
				} `json:"session"`
				Active bool `json:"active"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(openRec.Body.Bytes(), &openPayload); err != nil {
		t.Fatalf("json.Unmarshal(open) error = %v", err)
	}
	if openPayload.Result.IsError {
		t.Fatal("open_shell_session must succeed")
	}
	if !openPayload.Result.StructuredContent.Success {
		t.Fatal("open_shell_session success = false, want true")
	}
	if !openPayload.Result.StructuredContent.Active {
		t.Fatal("open_shell_session active = false, want true")
	}
	sessionID := openPayload.Result.StructuredContent.Session.ID
	if sessionID == "" {
		t.Fatal("session_id must not be empty")
	}

	sendReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"send_shell_input","session_id":"` + sessionID + `","input":"printf 'mcp-shell\\n'\n"}}`)
	sendRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(sendRec, sendReq)
	if sendRec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call send_shell_input status = %d, want %d", sendRec.Code, http.StatusOK)
	}

	deadline := time.Now().Add(5 * time.Second)
	var readPayload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success    bool   `json:"success"`
				SessionID  string `json:"session_id"`
				Content    string `json:"content"`
				NextOffset int64  `json:"next_offset"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	for time.Now().Before(deadline) {
		readReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_shell_output","session_id":"` + sessionID + `","offset":0,"limit":4096}}`)
		readRec := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(readRec, readReq)
		if readRec.Code != http.StatusOK {
			t.Fatalf("POST /mcp tools/call read_shell_output status = %d, want %d", readRec.Code, http.StatusOK)
		}
		if err := json.Unmarshal(readRec.Body.Bytes(), &readPayload); err != nil {
			t.Fatalf("json.Unmarshal(read) error = %v", err)
		}
		if readPayload.Result.IsError {
			t.Fatal("read_shell_output must succeed")
		}
		if strings.Contains(readPayload.Result.StructuredContent.Content, "mcp-shell") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(readPayload.Result.StructuredContent.Content, "mcp-shell") {
		t.Fatalf("content = %q, want to contain mcp-shell", readPayload.Result.StructuredContent.Content)
	}
	if readPayload.Result.StructuredContent.SessionID != sessionID {
		t.Fatalf("session_id = %q, want %q", readPayload.Result.StructuredContent.SessionID, sessionID)
	}

	closeReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"close_shell_session","session_id":"` + sessionID + `"}}`)
	closeRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(closeRec, closeReq)
	if closeRec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call close_shell_session status = %d, want %d", closeRec.Code, http.StatusOK)
	}

	var closePayload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success bool `json:"success"`
				Active  bool `json:"active"`
				Session struct {
					Status string `json:"status"`
				} `json:"session"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(closeRec.Body.Bytes(), &closePayload); err != nil {
		t.Fatalf("json.Unmarshal(close) error = %v", err)
	}
	if closePayload.Result.IsError {
		t.Fatal("close_shell_session must succeed")
	}
	if closePayload.Result.StructuredContent.Active {
		t.Fatal("close_shell_session active = true, want false")
	}
	if closePayload.Result.StructuredContent.Session.Status != "closed" && closePayload.Result.StructuredContent.Session.Status != "exited" {
		t.Fatalf("status = %q, want closed or exited", closePayload.Result.StructuredContent.Session.Status)
	}
}
