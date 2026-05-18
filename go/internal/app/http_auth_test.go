package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"notion-local-ops-mcp-go/internal/config"
	"notion-local-ops-mcp-go/internal/tools"
)

func TestHeadMCPAllowedWithoutAuth(t *testing.T) {
	server := NewServer(config.Config{
		Host:          "127.0.0.1",
		Port:          8766,
		WorkspaceRoot: "/workspace",
		StateDir:      "/state",
		AuthToken:     "secret-token",
	})

	req := httptest.NewRequest(http.MethodHead, "http://example.test"+tools.MCPPath, nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("HEAD /mcp status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD /mcp body length = %d, want 0", rec.Body.Len())
	}
}

func TestRootRequiresBearerWhenConfigured(t *testing.T) {
	server := NewServer(config.Config{
		Host:          "127.0.0.1",
		Port:          8766,
		WorkspaceRoot: "/workspace",
		StateDir:      "/state",
		AuthToken:     "secret-token",
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if rec.Header().Get("WWW-Authenticate") != `Bearer realm="mcp"` {
		t.Fatalf("WWW-Authenticate = %q, want %q", rec.Header().Get("WWW-Authenticate"), `Bearer realm="mcp"`)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["error"] != "unauthorized" {
		t.Fatalf("error = %v, want unauthorized", payload["error"])
	}
	if payload["message"] != "Missing or invalid bearer token." {
		t.Fatalf("message = %v, want missing or invalid token message", payload["message"])
	}
}

func TestRootAllowsBearerWhenConfigured(t *testing.T) {
	server := NewServer(config.Config{
		Host:          "127.0.0.1",
		Port:          8766,
		WorkspaceRoot: "/workspace",
		StateDir:      "/state",
		AuthToken:     "secret-token",
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetMCPRequiresBearerWhenConfigured(t *testing.T) {
	server := NewServer(config.Config{
		Host:          "127.0.0.1",
		Port:          8766,
		WorkspaceRoot: "/workspace",
		StateDir:      "/state",
		AuthToken:     "secret-token",
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.test"+tools.MCPPath, nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /mcp status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetMCPReturnsDiscoveryDocumentWhenAuthorized(t *testing.T) {
	server := NewServer(config.Config{
		Host:          "127.0.0.1",
		Port:          8766,
		WorkspaceRoot: "/workspace",
		StateDir:      "/state",
		AuthToken:     "secret-token",
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.test"+tools.MCPPath, nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /mcp status = %d, want %d", rec.Code, http.StatusOK)
	}
}
