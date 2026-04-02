package server

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/change"
	"github.com/aolda/aods-backend/internal/cluster"
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
	clusterSource := cluster.Source(cluster.LocalSource{
		Path: filepath.Join(cfg.RepoRoot, "platform", "clusters.yaml"),
	})
	applicationStore := application.Store(application.LocalManifestStore{RepoRoot: cfg.RepoRoot})
	changeStore := change.Store(change.LocalStore{RepoRoot: cfg.RepoRoot})

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
		clusterSource = cluster.GitSource{Repository: repository}
		applicationStore = application.GitManifestStore{Repository: repository}
		changeStore = change.GitStore{Repository: repository}
	}

	projectService := &project.Service{
		Source: projectSource,
	}
	clusterService := &cluster.Service{Source: clusterSource}

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
		Rollouts:      kubernetes.NewRolloutController(cfg),
	}
	changeService := &change.Service{
		Projects:     projectService,
		Applications: applicationService,
		Store:        changeStore,
	}

	projectHandler := project.Handler{
		Service: projectService,
		Users:   userProvider,
	}
	clusterHandler := cluster.Handler{Service: clusterService}
	changeHandler := change.Handler{
		Service: changeService,
		Users:   userProvider,
	}

	applicationHandler := application.Handler{
		Service: applicationService,
		Users:   userProvider,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/me", core.CurrentUserHandler(userProvider))
	mux.HandleFunc("GET /api/v1/clusters", clusterHandler.ListClusters)
	mux.HandleFunc("GET /api/v1/projects", projectHandler.ListProjects)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/environments", projectHandler.ListEnvironments)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/policies", projectHandler.GetPolicies)
	mux.HandleFunc("PATCH /api/v1/projects/{projectId}/policies", projectHandler.UpdatePolicies)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/changes", changeHandler.Create)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/applications", applicationHandler.ListApplications)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/applications", applicationHandler.CreateApplication)
	mux.HandleFunc("PATCH /api/v1/applications/{applicationId}", applicationHandler.PatchApplication)
	mux.HandleFunc("POST /api/v1/applications/{applicationId}/deployments", applicationHandler.CreateDeployment)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/deployments", applicationHandler.ListDeployments)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/deployments/{deploymentId}", applicationHandler.GetDeployment)
	mux.HandleFunc("POST /api/v1/applications/{applicationId}/deployments/{deploymentId}/promote", applicationHandler.PromoteDeployment)
	mux.HandleFunc("POST /api/v1/applications/{applicationId}/deployments/{deploymentId}/abort", applicationHandler.AbortDeployment)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/sync-status", applicationHandler.GetSyncStatus)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/metrics", applicationHandler.GetMetrics)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/rollback-policies", applicationHandler.GetRollbackPolicy)
	mux.HandleFunc("POST /api/v1/applications/{applicationId}/rollback-policies", applicationHandler.SaveRollbackPolicy)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/events", applicationHandler.GetEvents)
	mux.HandleFunc("GET /api/v1/changes/{changeId}", changeHandler.Get)
	mux.HandleFunc("POST /api/v1/changes/{changeId}/submit", changeHandler.Submit)
	mux.HandleFunc("POST /api/v1/changes/{changeId}/approve", changeHandler.Approve)
	mux.HandleFunc("POST /api/v1/changes/{changeId}/merge", changeHandler.Merge)
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
