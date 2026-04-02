package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/gitops"
	"github.com/aolda/aods-backend/internal/server"
)

func main() {
	cfg, err := core.LoadConfig()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	if cfg.UseGitRepo() {
		repository := &gitops.Repository{
			Dir:         cfg.GitRepoDir,
			Remote:      cfg.GitRemote,
			Branch:      cfg.GitBranch,
			AuthorName:  cfg.GitAuthorName,
			AuthorEmail: cfg.GitAuthorEmail,
			Timeout:     cfg.GitCommandTimeout,
		}

		if err := repository.EnsureFile(context.Background(), "platform/projects.yaml"); err != nil {
			slog.Error(
				"git-mode preflight failed",
				"gitRepoDir", cfg.GitRepoDir,
				"gitRemote", redactRemote(cfg.GitRemote),
				"gitBranch", cfg.GitBranch,
				"requiredFile", "platform/projects.yaml",
				"error", err,
			)
			os.Exit(1)
		}
	}

	handler := server.New(cfg)

	slog.Info(
		"starting AODS backend",
		"address", cfg.Address,
		"repoRoot", cfg.RepoRoot,
		"gitMode", cfg.GitMode,
		"gitRepoDir", cfg.GitRepoDir,
		"gitRemote", redactRemote(cfg.GitRemote),
		"gitBranch", cfg.GitBranch,
		"kubernetesMode", cfg.KubernetesMode,
		"kubernetesAPIURL", cfg.KubernetesAPIURL,
		"fluxKustomizationNamespace", cfg.FluxKustomizationNamespace,
		"prometheusMode", cfg.PrometheusMode,
		"prometheusURL", cfg.PrometheusURL,
		"vaultMode", cfg.VaultMode,
		"vaultAddress", cfg.VaultAddress,
		"vaultNamespace", cfg.VaultNamespace,
		"devAuthFallback", cfg.AllowDevFallback,
		"localVaultDir", cfg.LocalVaultDir,
	)

	if err := http.ListenAndServe(cfg.Address, handler); err != nil {
		slog.Error("backend server stopped", "error", err)
		os.Exit(1)
	}
}

func redactRemote(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User == nil {
		return value
	}

	username := parsed.User.Username()
	if username == "" {
		username = "redacted"
	}

	if _, ok := parsed.User.Password(); !ok {
		return value
	}

	parsed.User = url.UserPassword(username, "<redacted>")
	return parsed.String()
}
