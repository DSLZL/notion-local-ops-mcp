package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInitializeResultIncludesProtocolVersion(t *testing.T) {
	res := initializeResult()
	if res["protocolVersion"] == "" {
		t.Fatal("protocolVersion must be present")
	}
}

func TestCoreToolsIncludesServerInfo(t *testing.T) {
	tools := CoreTools()
	if len(tools) == 0 {
		t.Fatal("expected at least one core tool")
	}
	if tools[0].Name != "server_info" {
		t.Fatalf("first tool = %q, want server_info", tools[0].Name)
	}
}

func TestServeHTTPRejectsGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.test/mcp", nil)
	rec := httptest.NewRecorder()

	ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /mcp status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestServeHTTPRejectsInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://example.test/mcp", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServeHTTPAcknowledgesInitializedNotification(t *testing.T) {
	body := `{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`
	req := httptest.NewRequest(http.MethodPost, "http://example.test/mcp", strings.NewReader(body))
	rec := httptest.NewRecorder()

	ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("initialized notification status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestServeHTTPReturnsSkeletonForKnownToolCall(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"server_info"}}`
	req := httptest.NewRequest(http.MethodPost, "http://example.test/mcp", strings.NewReader(body))
	rec := httptest.NewRecorder()

	ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("tools/call status = %d, want %d", rec.Code, http.StatusOK)
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
		t.Fatal("server_info call must succeed")
	}
	if payload.Result.StructuredContent["mcp_path"] != "/mcp" {
		t.Fatalf("structuredContent.mcp_path = %v, want /mcp", payload.Result.StructuredContent["mcp_path"])
	}
	if len(payload.Result.Content) == 0 || !strings.Contains(payload.Result.Content[0].Text, "\"transport\":\"streamable-http\"") {
		t.Fatal("expected server_info response to include transport metadata")
	}
}

func TestServeHTTPRejectsUnknownToolCall(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"missing_tool"}}`
	req := httptest.NewRequest(http.MethodPost, "http://example.test/mcp", strings.NewReader(body))
	rec := httptest.NewRecorder()

	ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown tool status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServeHTTPAcceptsWrappedToolArguments(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"server_info","toolArguments":{"ignored":"value"}}}`
	req := httptest.NewRequest(http.MethodPost, "http://example.test/mcp", strings.NewReader(body))
	rec := httptest.NewRecorder()

	ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("wrapped tools/call status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Result struct {
			IsError           bool           `json:"isError"`
			StructuredContent map[string]any `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Result.IsError {
		t.Fatal("wrapped server_info call must succeed")
	}
	if payload.Result.StructuredContent["mcp_path"] != "/mcp" {
		t.Fatalf("structuredContent.mcp_path = %v, want /mcp", payload.Result.StructuredContent["mcp_path"])
	}
}
