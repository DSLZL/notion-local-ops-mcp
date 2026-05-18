package bootstrap

import (
	"errors"
	"log"
	"net/http"

	"notion-local-ops-mcp-go/internal/app"
)

func Run() {
	server, envPath, err := app.BuildDefaultServer()
	if err != nil {
		log.Fatal(err)
	}

	listener, err := server.Listen()
	if err != nil {
		log.Fatal(err)
	}

	if envPath == "" {
		envPath = "not found"
	}
	info := server.ServerInfo()
	log.Printf(
		"Serving notion-local-ops-mcp-go addr=%s workspace_root=%v state_dir=%v env_file=%s config_precedence=.env>shell>default",
		listener.Addr().String(),
		info["workspace_root"],
		info["state_dir"],
		envPath,
	)
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
