package server

import (
	"net/http"
	"path/filepath"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/kubernetes"
	"github.com/aolda/aods-backend/internal/project"
	"github.com/aolda/aods-backend/internal/vault"
)

func New(cfg core.Config) http.Handler {
	userProvider := core.HeaderUserProvider{
		AllowDevFallback: cfg.AllowDevFallback,
		DevUser:          cfg.DevUser,
	}

	projectService := &project.Service{
		Source: project.LocalCatalogSource{
			Path: filepath.Join(cfg.RepoRoot, "platform", "projects.yaml"),
		},
	}

	applicationService := &application.Service{
		Projects:      projectService,
		Store:         application.LocalManifestStore{RepoRoot: cfg.RepoRoot},
		StatusReader:  kubernetes.LocalSyncStatusReader{},
		MetricsReader: application.LocalMetricsReader{},
		Secrets:       vault.LocalStore{RootDir: cfg.LocalVaultDir},
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
