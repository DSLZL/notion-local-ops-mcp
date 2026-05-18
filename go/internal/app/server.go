package app

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"notion-local-ops-mcp-go/internal/config"
	"notion-local-ops-mcp-go/internal/mcp"
	"notion-local-ops-mcp-go/internal/tools"
)

type Server struct {
	cfg        config.Config
	httpServer *http.Server
}

func NewServer(cfg config.Config) *Server {
	server := &Server{cfg: cfg}
	mux := http.NewServeMux()
	RegisterBaseRoutes(mux, server)
	server.httpServer = &http.Server{
		Addr:    server.Addr(),
		Handler: mux,
	}
	return server
}

func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
}

func (s *Server) Listen() (net.Listener, error) {
	return net.Listen("tcp", s.Addr())
}

func (s *Server) Serve(listener net.Listener) error {
	return s.httpServer.Serve(listener)
}

func (s *Server) ServerInfo() map[string]any {
	registered := mcp.CoreTools()
	names := make([]string, 0, len(registered))
	for _, tool := range registered {
		names = append(names, tool.Name)
	}
	return tools.BuildServerInfo(s.cfg, names)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"status":      "bootstrap-active",
		"server_info": s.ServerInfo(),
	})
}

func (s *Server) handleMCP(w http.ResponseWriter, _ *http.Request) {
	mcp.ServeDiscovery(w, s.cfg)
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
