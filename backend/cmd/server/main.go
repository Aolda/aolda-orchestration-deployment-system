package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/gitops"
	"github.com/aolda/aods-backend/internal/server"
	"github.com/aolda/aods-backend/internal/vault"
	"time"
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
			SyncTTL:     cfg.GitSyncTTL,
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

	if cfg.UseOIDCAuth() {
		if _, err := core.NewOIDCUserProvider(cfg); err != nil {
			slog.Error(
				"oidc auth preflight failed",
				"authMode", cfg.AuthMode,
				"oidcIssuerURL", cfg.OIDCIssuerURL,
				"oidcJWKSURL", cfg.OIDCJWKSURL,
				"error", err,
			)
			os.Exit(1)
		}
	}

	handler, applicationService, projectService, cleanup, err := server.NewWithResources(cfg)
	if err != nil {
		slog.Error("failed to initialize backend dependencies", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	slog.Info(
		"starting AODS backend",
		"address", cfg.Address,
		"repoRoot", cfg.RepoRoot,
		"authMode", cfg.AuthMode,
		"oidcIssuerURL", cfg.OIDCIssuerURL,
		"oidcJWKSURL", cfg.OIDCJWKSURL,
		"gitMode", cfg.GitMode,
		"gitRepoDir", cfg.GitRepoDir,
		"gitRemote", redactRemote(cfg.GitRemote),
		"gitBranch", cfg.GitBranch,
		"repositoryPollInterval", cfg.RepositoryPollInterval,
		"kubernetesMode", cfg.KubernetesMode,
		"kubernetesAPIURL", cfg.KubernetesAPIURL,
		"fluxKustomizationNamespace", cfg.FluxKustomizationNamespace,
		"prometheusMode", cfg.PrometheusMode,
		"prometheusURL", cfg.PrometheusURL,
		"vaultMode", cfg.VaultMode,
		"vaultAddress", cfg.VaultAddress,
		"vaultNamespace", cfg.VaultNamespace,
		"vaultStagingCleanupInterval", cfg.VaultStagingCleanupInterval,
		"vaultStagingMaxAge", cfg.VaultStagingMaxAge,
		"orphanFluxCleanupInterval", cfg.OrphanFluxCleanupInterval,
		"mariadbOperationsEnabled", cfg.UseMariaDBOperations(),
		"deploymentOperationInterval", cfg.DeploymentOperationInterval,
		"deploymentOperationLease", cfg.DeploymentOperationLease,
		"deploymentOperationMaxAttempts", cfg.DeploymentOperationMaxAttempts,
		"devAuthFallback", cfg.AllowDevFallback,
		"localVaultDir", cfg.LocalVaultDir,
	)

	poller := &application.AutoUpdatePoller{
		Service:  applicationService,
		Projects: projectService,
		Interval: cfg.RepositoryPollInterval,
	}
	go poller.Start(context.Background())

	if applicationService.DeploymentOperations != nil {
		worker := &application.DeploymentOperationWorker{
			Service:       applicationService,
			Store:         applicationService.DeploymentOperations,
			WorkerID:      hostnameWorkerID(),
			Interval:      cfg.DeploymentOperationInterval,
			LeaseDuration: cfg.DeploymentOperationLease,
		}
		go worker.Start(context.Background())
	}

	if cleaner, ok := applicationService.Secrets.(interface {
		CleanupStale(context.Context, time.Time) (int, error)
	}); ok && cfg.VaultStagingCleanupInterval > 0 && cfg.VaultStagingMaxAge > 0 {
		cleanupWorker := &vault.StagingSecretCleanupWorker{
			Cleaner:  cleaner,
			Interval: cfg.VaultStagingCleanupInterval,
			MaxAge:   cfg.VaultStagingMaxAge,
		}
		go cleanupWorker.Start(context.Background())
	}

	if cleaner, ok := applicationService.Store.(interface {
		CleanupOrphanFluxManifests(context.Context) (int, error)
	}); ok && cfg.OrphanFluxCleanupInterval > 0 {
		cleanupWorker := &application.OrphanFluxManifestCleanupWorker{
			Cleaner:  cleaner,
			Interval: cfg.OrphanFluxCleanupInterval,
		}
		go cleanupWorker.Start(context.Background())
	}

	if err := http.ListenAndServe(cfg.Address, handler); err != nil {
		slog.Error("backend server stopped", "error", err)
		os.Exit(1)
	}
}

func hostnameWorkerID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "aods-backend"
	}
	return hostname
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
