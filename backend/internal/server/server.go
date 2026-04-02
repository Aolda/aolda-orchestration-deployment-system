package server

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/gitops"
	"github.com/aolda/aods-backend/internal/kubernetes"
	"github.com/aolda/aods-backend/internal/project"
	"github.com/aolda/aods-backend/internal/vault"
)

func New(cfg core.Config) http.Handler {
	userProvider := core.NewUserProvider(cfg)

	projectSource := project.CatalogSource(project.LocalCatalogSource{
		Path: filepath.Join(cfg.RepoRoot, "platform", "projects.yaml"),
	})
	applicationStore := application.Store(application.LocalManifestStore{RepoRoot: cfg.RepoRoot})

	if cfg.UseGitRepo() {
		repository := &gitops.Repository{
			Dir:         cfg.GitRepoDir,
			Remote:      cfg.GitRemote,
			Branch:      cfg.GitBranch,
			AuthorName:  cfg.GitAuthorName,
			AuthorEmail: cfg.GitAuthorEmail,
			Timeout:     maxDuration(cfg.GitCommandTimeout, 15*time.Second),
		}

		projectSource = project.GitCatalogSource{Repository: repository}
		applicationStore = application.GitManifestStore{Repository: repository}
	}

	projectService := &project.Service{
		Source: projectSource,
	}

	metricsReader := application.MetricsReader(application.LocalMetricsReader{})
	if cfg.UsePrometheusAPI() {
		metricsReader = application.PrometheusMetricsReader{
			BaseURL: cfg.PrometheusURL,
			Client:  &http.Client{Timeout: maxDuration(cfg.PrometheusRequestTimeout, 5*time.Second)},
			Range:   cfg.PrometheusRange,
			Step:    cfg.PrometheusStep,
		}
	}

	secretStore := application.SecretsStager(vault.LocalStore{RootDir: cfg.LocalVaultDir})
	if cfg.UseVaultAPI() {
		secretStore = vault.RealStore{
			Address:   cfg.VaultAddress,
			Token:     cfg.VaultToken,
			Namespace: cfg.VaultNamespace,
			Client:    &http.Client{Timeout: maxDuration(cfg.VaultRequestTimeout, 5*time.Second)},
		}
	}

	applicationService := &application.Service{
		Projects:      projectService,
		Store:         applicationStore,
		StatusReader:  kubernetes.NewSyncStatusReader(cfg),
		MetricsReader: metricsReader,
		Secrets:       secretStore,
	}

	projectHandler := project.Handler{
		Service: projectService,
		Users:   userProvider,
	}

	applicationHandler := application.Handler{
		Service: applicationService,
		Users:   userProvider,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/me", core.CurrentUserHandler(userProvider))
	mux.HandleFunc("GET /api/v1/projects", projectHandler.ListProjects)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/applications", applicationHandler.ListApplications)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/applications", applicationHandler.CreateApplication)
	mux.HandleFunc("POST /api/v1/applications/{applicationId}/deployments", applicationHandler.CreateDeployment)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/sync-status", applicationHandler.GetSyncStatus)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/metrics", applicationHandler.GetMetrics)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		core.WriteError(
			w,
			r,
			http.StatusNotFound,
			"ROUTE_NOT_FOUND",
			"Route was not found.",
			map[string]any{"path": r.URL.Path},
			false,
		)
	})

	return core.WithRequestID(core.WithCORS(mux, cfg.AllowedOrigin))
}

func maxDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}
