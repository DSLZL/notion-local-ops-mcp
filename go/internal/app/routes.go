package app

import (
	"net/http"

	"notion-local-ops-mcp-go/internal/auth"
	"notion-local-ops-mcp-go/internal/mcp"
	"notion-local-ops-mcp-go/internal/tools"
)

func RegisterBaseRoutes(mux *http.ServeMux, server *Server) {
	mux.HandleFunc("/", server.handleRootRoute)
	mux.HandleFunc(tools.MCPPath, server.handleMCPRoute)
}

func (s *Server) handleRootRoute(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !s.authorize(w, r) {
		return
	}
	s.handleRoot(w, r)
}

func (s *Server) handleMCPRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !s.authorize(w, r) {
		return
	}
	if r.Method == http.MethodPost {
		mcp.ServeHTTPWithConfig(w, r, s.cfg)
		return
	}
	s.handleMCP(w, r)
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) bool {
	if auth.ValidateBearer(s.cfg.AuthToken, r.Header.Get("Authorization")) {
		return true
	}
	s.writeUnauthorized(w)
	return false
}

func (s *Server) writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="mcp"`)
	s.writeJSON(w, http.StatusUnauthorized, map[string]any{
		"error":   "unauthorized",
		"message": "Missing or invalid bearer token.",
	})
}
