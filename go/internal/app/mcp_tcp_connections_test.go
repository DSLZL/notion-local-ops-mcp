package app

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type tcpRouteTestServer struct {
	Host string
	Port int
}

func startTCPRouteTestServer(t *testing.T) *tcpRouteTestServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	addr := ln.Addr().String()
	host, portRaw, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("net.SplitHostPort() error = %v", err)
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		t.Fatalf("strconv.Atoi() error = %v", err)
	}

	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = c.Write([]byte("name: "))
				buf := make([]byte, 64)
				n, _ := c.Read(buf)
				_, _ = c.Write([]byte("hello " + strings.TrimSpace(string(buf[:n])) + "\n> "))
			}(conn)
		}
	}()

	return &tcpRouteTestServer{
		Host: host,
		Port: port,
	}
}

func decodeToolCallResultMap(t *testing.T, rec *httptest.ResponseRecorder, action string) map[string]any {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp tools/call %s status = %d, want %d", action, rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool           `json:"isError"`
			StructuredContent map[string]any `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", action, err)
	}
	if payload.Result.IsError {
		t.Fatalf("%s must succeed", action)
	}
	return payload.Result.StructuredContent
}

func TestToolsListIncludesTCPSchemas(t *testing.T) {
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
		"tcp_connect": {"host", "port", "timeout_seconds"},
		"tcp_send":    {"connection_id", "text", "content_base64", "append_newline"},
		"tcp_read":    {"connection_id", "timeout_seconds", "max_bytes", "read_until", "read_until_base64", "output_mode"},
		"tcp_close":   {"connection_id"},
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
	for toolName := range required {
		if !seen[toolName] {
			t.Fatalf("%s must be listed", toolName)
		}
	}
}

func TestToolsCallTCPLifecycleOverMCPRoute(t *testing.T) {
	testTCP := startTCPRouteTestServer(t)
	server := newTask3TestServer()
	server.cfg.StateDir = t.TempDir()
	server.cfg.WorkspaceRoot = t.TempDir()

	connectReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"tcp_connect","host":"` + testTCP.Host + `","port":` + strconv.Itoa(testTCP.Port) + `,"timeout_seconds":2}}`)
	connectRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(connectRec, connectReq)
	connect := decodeToolCallResultMap(t, connectRec, "tcp_connect")

	connectionObj, ok := connect["connection"].(map[string]any)
	if !ok {
		t.Fatal("tcp_connect structuredContent.connection must be present")
	}
	connectionID, _ := connectionObj["connection_id"].(string)
	if connectionID == "" {
		t.Fatal("tcp_connect connection_id must not be empty")
	}

	readPromptReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"tcp_read","connection_id":"` + connectionID + `","timeout_seconds":2,"read_until":"name: ","max_bytes":1024}}`)
	readPromptRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(readPromptRec, readPromptReq)
	prompt := decodeToolCallResultMap(t, readPromptRec, "tcp_read prompt")
	promptContent, _ := prompt["content"].(string)
	if !strings.Contains(promptContent, "name: ") {
		t.Fatalf("tcp_read prompt content = %q, want name prompt", promptContent)
	}

	sendReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"tcp_send","connection_id":"` + connectionID + `","text":"alice","append_newline":true}}`)
	sendRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(sendRec, sendReq)
	_ = decodeToolCallResultMap(t, sendRec, "tcp_send")

	readReplyReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"tcp_read","connection_id":"` + connectionID + `","timeout_seconds":2,"read_until":"> ","max_bytes":1024}}`)
	readReplyRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(readReplyRec, readReplyReq)
	reply := decodeToolCallResultMap(t, readReplyRec, "tcp_read reply")
	replyContent, _ := reply["content"].(string)
	if !strings.Contains(replyContent, "hello alice") {
		t.Fatalf("tcp_read reply content = %q, want greeting", replyContent)
	}

	closeReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"tcp_close","connection_id":"` + connectionID + `"}}`)
	closeRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(closeRec, closeReq)
	closed := decodeToolCallResultMap(t, closeRec, "tcp_close")
	active, _ := closed["active"].(bool)
	if active {
		t.Fatal("tcp_close active = true, want false")
	}
}
