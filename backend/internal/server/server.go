package server

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/aolda/aods-backend/internal/admin"
	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/change"
	"github.com/aolda/aods-backend/internal/cluster"
	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/gitops"
	"github.com/aolda/aods-backend/internal/kubernetes"
	"github.com/aolda/aods-backend/internal/project"
	"github.com/aolda/aods-backend/internal/vault"
)

func New(cfg core.Config) (http.Handler, *application.Service, *project.Service) {
	userProvider := core.NewUserProvider(cfg)

	projectSource := project.CatalogSource(project.LocalCatalogSource{
		Path:                       filepath.Join(cfg.RepoRoot, "platform", "projects.yaml"),
		RepoRoot:                   cfg.RepoRoot,
		FluxKustomizationNamespace: cfg.FluxKustomizationNamespace,
		FluxSourceName:             cfg.FluxSourceName,
	})
	clusterSource := cluster.Source(cluster.LocalSource{
		Path:                       filepath.Join(cfg.RepoRoot, "platform", "clusters.yaml"),
		RepoRoot:                   cfg.RepoRoot,
		FluxKustomizationNamespace: cfg.FluxKustomizationNamespace,
		FluxSourceName:             cfg.FluxSourceName,
	})
	applicationStore := application.Store(application.LocalManifestStore{
		RepoRoot:                   cfg.RepoRoot,
		FluxKustomizationNamespace: cfg.FluxKustomizationNamespace,
		FluxSourceName:             cfg.FluxSourceName,
	})
	changeStore := change.Store(change.LocalStore{RepoRoot: cfg.RepoRoot})

	if cfg.UseGitRepo() {
		repository := &gitops.Repository{
			Dir:         cfg.GitRepoDir,
			Remote:      cfg.GitRemote,
			Branch:      cfg.GitBranch,
			AuthorName:  cfg.GitAuthorName,
			AuthorEmail: cfg.GitAuthorEmail,
			Timeout:     maxDuration(cfg.GitCommandTimeout, 15*time.Second),
			SyncTTL:     cfg.GitSyncTTL,
		}

		projectSource = project.GitCatalogSource{
			Repository:                 repository,
			FluxKustomizationNamespace: cfg.FluxKustomizationNamespace,
			FluxSourceName:             cfg.FluxSourceName,
		}
		clusterSource = cluster.GitSource{
			Repository:                 repository,
			FluxKustomizationNamespace: cfg.FluxKustomizationNamespace,
			FluxSourceName:             cfg.FluxSourceName,
		}
		applicationStore = application.GitManifestStore{
			Repository:                 repository,
			FluxKustomizationNamespace: cfg.FluxKustomizationNamespace,
			FluxSourceName:             cfg.FluxSourceName,
		}
		changeStore = change.GitStore{Repository: repository}
	}

	metricsReader := application.MetricsReader(application.LocalMetricsReader{})
	networkExposureReader := application.NetworkExposureReader(kubernetes.LocalNetworkExposureReader{})
	var prometheusReader application.MetricsReader
	if cfg.UsePrometheusAPI() {
		prometheusReader = application.PrometheusMetricsReader{
			BaseURL: cfg.PrometheusURL,
			Client:  &http.Client{Timeout: maxDuration(cfg.PrometheusRequestTimeout, 5*time.Second)},
			Range:   cfg.PrometheusRange,
			Step:    cfg.PrometheusStep,
		}
	}
	var kubernetesMetricsReader application.MetricsReader
	if cfg.UseKubernetesAPI() {
		reader, err := kubernetes.NewPodMetricsReader(cfg)
		if err != nil {
			if prometheusReader == nil {
				metricsReader = application.ErrorMetricsReader{Err: err}
			}
		} else {
			kubernetesMetricsReader = reader
		}
	}
	var kubernetesLogsReader application.LogsReader
	if cfg.UseKubernetesAPI() {
		reader, err := kubernetes.NewPodLogReader(cfg)
		if err == nil {
			kubernetesLogsReader = reader
		}
		networkExposureReader = kubernetes.NewNetworkExposureReader(cfg)
	}
	switch {
	case prometheusReader != nil && kubernetesMetricsReader != nil:
		metricsReader = application.CompositeMetricsReader{
			Primary:  prometheusReader,
			Fallback: kubernetesMetricsReader,
		}
	case prometheusReader != nil:
		metricsReader = prometheusReader
	case kubernetesMetricsReader != nil:
		metricsReader = kubernetesMetricsReader
	}

	secretStore := application.SecretStore(vault.LocalStore{RootDir: cfg.LocalVaultDir})
	if cfg.UseVaultAPI() {
		secretStore = vault.RealStore{
			Address:   cfg.VaultAddress,
			Token:     cfg.VaultToken,
			Namespace: cfg.VaultNamespace,
			Client:    &http.Client{Timeout: maxDuration(cfg.VaultRequestTimeout, 5*time.Second)},
		}
	}

	projectService := &project.Service{
		Source:                   projectSource,
		Clusters:                 clusterSource,
		Secrets:                  secretStore,
		PlatformAdminAuthorities: cfg.PlatformAdminAuthorities,
	}
	clusterService := &cluster.Service{
		Source:                   clusterSource,
		PlatformAdminAuthorities: cfg.PlatformAdminAuthorities,
	}

	imageVerifier := application.ImageVerifier(application.NoopImageVerifier{})
	if cfg.UseImageVerification() {
		imageVerifier = application.RegistryImageVerifier{
			Client: &http.Client{Timeout: maxDuration(cfg.ImageVerificationTimeout, 5*time.Second)},
		}
	}

	applicationService := &application.Service{
		Projects:              projectService,
		Store:                 applicationStore,
		StatusReader:          kubernetes.NewSyncStatusReader(cfg),
		MetricsReader:         metricsReader,
		NetworkExposureReader: networkExposureReader,
		LogsReader:            kubernetesLogsReader,
		Secrets:               secretStore,
		Rollouts:              kubernetes.NewRolloutController(cfg),
		Images:                imageVerifier,
		PollTracker:           application.NewRepositoryPollTracker(cfg.RepositoryPollInterval),
	}
	resourceOverviewReader := admin.ResourceOverviewReader(admin.LocalResourceOverviewReader{})
	if cfg.UseKubernetesAPI() {
		reader, err := kubernetes.NewFleetResourceReader(cfg)
		if err != nil {
			resourceOverviewReader = admin.ErrorResourceOverviewReader{Err: err}
		} else {
			resourceOverviewReader = reader
		}
	}
	adminService := &admin.Service{
		Projects:                 projectSource,
		Clusters:                 clusterSource,
		Applications:             applicationStore,
		ResourceOverviewReader:   resourceOverviewReader,
		PlatformAdminAuthorities: cfg.PlatformAdminAuthorities,
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
	clusterHandler := cluster.Handler{
		Service: clusterService,
		Users:   userProvider,
	}
	changeHandler := change.Handler{
		Service: changeService,
		Users:   userProvider,
	}
	adminHandler := admin.Handler{
		Service: adminService,
		Users:   userProvider,
	}

	applicationHandler := application.Handler{
		Service: applicationService,
		Users:   userProvider,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/me", core.CurrentUserHandler(userProvider))
	mux.HandleFunc("GET /api/v1/clusters", clusterHandler.ListClusters)
	mux.HandleFunc("POST /api/v1/clusters", clusterHandler.CreateCluster)
	mux.HandleFunc("GET /api/v1/admin/resource-overview", adminHandler.GetFleetResourceOverview)
	mux.HandleFunc("GET /api/v1/projects", projectHandler.ListProjects)
	mux.HandleFunc("POST /api/v1/projects", projectHandler.CreateProject)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}", projectHandler.DeleteProject)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/environments", projectHandler.ListEnvironments)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/repositories", projectHandler.ListRepositories)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/policies", projectHandler.GetPolicies)
	mux.HandleFunc("PATCH /api/v1/projects/{projectId}/policies", projectHandler.UpdatePolicies)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/changes", changeHandler.Create)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/applications", applicationHandler.ListApplications)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/health", applicationHandler.GetProjectHealth)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/applications/source-preview", applicationHandler.PreviewRepositorySource)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/applications", applicationHandler.CreateApplication)
	mux.HandleFunc("DELETE /api/v1/applications/{applicationId}", applicationHandler.DeleteApplication)
	mux.HandleFunc("PATCH /api/v1/applications/{applicationId}", applicationHandler.PatchApplication)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/secrets", applicationHandler.GetApplicationSecrets)
	mux.HandleFunc("PUT /api/v1/applications/{applicationId}/secrets", applicationHandler.UpdateApplicationSecrets)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/secrets/versions", applicationHandler.ListApplicationSecretVersions)
	mux.HandleFunc("POST /api/v1/applications/{applicationId}/secrets/versions/{version}/restore", applicationHandler.RestoreApplicationSecretVersion)
	mux.HandleFunc("POST /api/v1/applications/{applicationId}/archive", applicationHandler.ArchiveApplication)
	mux.HandleFunc("POST /api/v1/applications/{applicationId}/deployments", applicationHandler.CreateDeployment)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/deployments", applicationHandler.ListDeployments)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/deployments/{deploymentId}", applicationHandler.GetDeployment)
	mux.HandleFunc("POST /api/v1/applications/{applicationId}/deployments/{deploymentId}/promote", applicationHandler.PromoteDeployment)
	mux.HandleFunc("POST /api/v1/applications/{applicationId}/deployments/{deploymentId}/abort", applicationHandler.AbortDeployment)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/sync-status", applicationHandler.GetSyncStatus)
	mux.HandleFunc("POST /api/v1/applications/{applicationId}/sync", applicationHandler.SyncRepositoryNow)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/network-exposure", applicationHandler.GetNetworkExposure)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/metrics", applicationHandler.GetMetrics)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/metrics/diagnostics", applicationHandler.GetMetricsDiagnostics)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/logs", applicationHandler.GetContainerLogs)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/logs/targets", applicationHandler.GetContainerLogTargets)
	mux.HandleFunc("GET /api/v1/applications/{applicationId}/logs/stream", applicationHandler.StreamContainerLogs)
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

	return core.WithRequestID(core.WithCORS(mux, cfg.AllowedOrigin, cfg.AllowDevFallback)), applicationService, projectService
}

func maxDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}
