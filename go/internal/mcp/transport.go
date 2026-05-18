package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"

	"notion-local-ops-mcp-go/internal/config"
	"notion-local-ops-mcp-go/internal/tools"
)

type Request struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
}

type response struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *responseError `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ServeHTTPWithConfig(w, r, config.Config{
		Host:          "127.0.0.1",
		Port:          8766,
		WorkspaceRoot: ".",
		StateDir:      ".notion-local-ops-mcp",
	})
}

func ServeHTTPWithConfig(w http.ResponseWriter, r *http.Request, cfg config.Config) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, req.ID, http.StatusBadRequest, -32700, "invalid JSON body")
		return
	}

	session := NewSession(req)
	switch session.Method() {
	case "initialize":
		writeSessionResult(w, session, initializeResult())
	case "notifications/initialized":
		writeNotificationAck(w)
	case "tools/list":
		writeSessionResult(w, session, map[string]any{
			"tools": CoreTools(),
		})
	case "tools/call":
		handleToolCall(w, session, req, cfg)
	default:
		writeSessionError(w, session, http.StatusNotImplemented, -32601, "method not implemented")
	}
}

func ServeDiscovery(w http.ResponseWriter, cfg config.Config) {
	authSchemes := []string{}
	if cfg.AuthToken != "" {
		authSchemes = append(authSchemes, "Bearer")
	}

	writeDiscoveryJSON(w, http.StatusOK, map[string]any{
		"$schema":         "https://static.modelcontextprotocol.io/schemas/mcp-server-card/v1.json",
		"protocolVersion": ProtocolVersion,
		"serverInfo": map[string]any{
			"name":    tools.AppName,
			"title":   tools.AppName,
			"version": "go-phase1",
		},
		"description": "Local MCP server for filesystem, shell, git, and delegated coding tasks.",
		"transport": map[string]any{
			"type":     tools.Transport,
			"endpoint": tools.MCPPath,
		},
		"capabilities": map[string]any{
			"tools": map[string]any{
				"listChanged": true,
			},
		},
		"authentication": map[string]any{
			"required": len(authSchemes) > 0,
			"schemes":  authSchemes,
		},
		"instructions": fmt.Sprintf(
			"Use %s for JSON-RPC requests. The phase 1 Go entry currently validates initialize, tools/list, server_info, list_files, run_command_stream, wait_task, and get_task flows.",
			tools.MCPPath,
		),
	})
}

func initializeResult() map[string]any {
	return map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "notion-local-ops-mcp-go",
			"version": "phase1-task3",
		},
	}
}

func writeResult(w http.ResponseWriter, id any, result map[string]any) {
	writeJSON(w, http.StatusOK, response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func writeNotificationAck(w http.ResponseWriter) {
	w.WriteHeader(http.StatusAccepted)
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeError(w, nil, http.StatusMethodNotAllowed, -32600, "method not allowed")
}

func handleToolCall(w http.ResponseWriter, session *Session, req Request, cfg config.Config) {
	name, ok := req.Params["name"].(string)
	if !ok || name == "" {
		writeSessionError(w, session, http.StatusBadRequest, -32602, "tool name is required")
		return
	}
	if !HasTool(name) {
		writeSessionError(w, session, http.StatusBadRequest, -32602, "unknown tool")
		return
	}

	result, err := CallTool(cfg, name, normalizeToolParams(req.Params))
	if err != nil {
		writeSessionResult(w, session, toolErrorResult(err.Error()))
		return
	}
	writeSessionResult(w, session, result)
}

func normalizeToolParams(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}

	if nested, ok := nestedToolParams(params, "toolArguments"); ok {
		merged := cloneParams(nested)
		if name, ok := params["name"]; ok {
			merged["name"] = name
		}
		return merged
	}
	if nested, ok := nestedToolParams(params, "arguments"); ok {
		merged := cloneParams(nested)
		if name, ok := params["name"]; ok {
			merged["name"] = name
		}
		return merged
	}
	return params
}

func nestedToolParams(params map[string]any, key string) (map[string]any, bool) {
	raw, ok := params[key]
	if !ok {
		return nil, false
	}
	nested, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	return nested, true
}

func cloneParams(params map[string]any) map[string]any {
	cloned := make(map[string]any, len(params))
	for key, value := range params {
		cloned[key] = value
	}
	return cloned
}

func writeSessionResult(w http.ResponseWriter, session *Session, result map[string]any) {
	writeJSON(w, http.StatusOK, session.Response(result))
}

func writeSessionError(w http.ResponseWriter, session *Session, status, code int, message string) {
	writeJSON(w, status, session.Error(code, message))
}

func writeError(w http.ResponseWriter, id any, status, code int, message string) {
	writeJSON(w, status, response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &responseError{
			Code:    code,
			Message: message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeDiscoveryJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func toolErrorResult(message string) map[string]any {
	return map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": message,
			},
		},
		"isError": true,
	}
}
