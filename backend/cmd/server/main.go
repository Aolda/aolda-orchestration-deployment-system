package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/server"
)

func main() {
	cfg, err := core.LoadConfig()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	handler := server.New(cfg)

	slog.Info(
		"starting AODS backend",
		"address", cfg.Address,
		"repoRoot", cfg.RepoRoot,
		"devAuthFallback", cfg.AllowDevFallback,
		"localVaultDir", cfg.LocalVaultDir,
	)

	if err := http.ListenAndServe(cfg.Address, handler); err != nil {
		slog.Error("backend server stopped", "error", err)
		os.Exit(1)
	}
}
