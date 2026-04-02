package project

import (
	"errors"
	"net/http"

	"github.com/aolda/aods-backend/internal/core"
)

type Handler struct {
	Service *Service
	Users   core.UserProvider
}

type policiesPatchRequest struct {
	MinReplicas                 int      `json:"minReplicas"`
	AllowedEnvironments         []string `json:"allowedEnvironments"`
	AllowedDeploymentStrategies []string `json:"allowedDeploymentStrategies"`
	AllowedClusterTargets       []string `json:"allowedClusterTargets"`
	ProdPRRequired              bool     `json:"prodPRRequired"`
	AutoRollbackEnabled         bool     `json:"autoRollbackEnabled"`
	RequiredProbes              bool     `json:"requiredProbes"`
}

func (h Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	user, err := h.Users.CurrentUser(r)
	if err != nil {
		if errors.Is(err, core.ErrUnauthorized) {
			core.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required.", nil, false)
			return
		}

		core.WriteError(w, r, http.StatusInternalServerError, "AUTH_PROVIDER_ERROR", "Could not resolve the current user.", nil, true)
		return
	}

	projects, err := h.Service.ListAuthorized(r.Context(), user)
	if err != nil {
		var catalogMissing CatalogNotFoundError
		if errors.As(err, &catalogMissing) {
			core.WriteError(
				w,
				r,
				http.StatusInternalServerError,
				"PROJECT_CATALOG_NOT_BOOTSTRAPPED",
				"Project catalog is missing from the GitOps repository.",
				map[string]any{
					"path": catalogMissing.Path,
					"hint": "Seed platform/projects.yaml in the GitOps repository before enabling git mode.",
				},
				false,
			)
			return
		}

		core.WriteError(
			w,
			r,
			http.StatusInternalServerError,
			"PROJECT_CATALOG_READ_FAILED",
			"Could not read the project catalog.",
			map[string]any{"error": err.Error()},
			true,
		)
		return
	}

	core.WriteJSON(w, http.StatusOK, struct {
		Items []Summary `json:"items"`
	}{
		Items: projects,
	})
}

func (h Handler) ListEnvironments(w http.ResponseWriter, r *http.Request) {
	user, err := h.Users.CurrentUser(r)
	if err != nil {
		if errors.Is(err, core.ErrUnauthorized) {
			core.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required.", nil, false)
			return
		}
		core.WriteError(w, r, http.StatusInternalServerError, "AUTH_PROVIDER_ERROR", "Could not resolve the current user.", nil, true)
		return
	}

	items, err := h.Service.ListEnvironments(r.Context(), user, r.PathValue("projectId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, struct {
		Items []EnvironmentSummary `json:"items"`
	}{Items: items})
}

func (h Handler) GetPolicies(w http.ResponseWriter, r *http.Request) {
	user, err := h.Users.CurrentUser(r)
	if err != nil {
		if errors.Is(err, core.ErrUnauthorized) {
			core.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required.", nil, false)
			return
		}
		core.WriteError(w, r, http.StatusInternalServerError, "AUTH_PROVIDER_ERROR", "Could not resolve the current user.", nil, true)
		return
	}

	policies, err := h.Service.GetPolicies(r.Context(), user, r.PathValue("projectId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, policies)
}

func (h Handler) UpdatePolicies(w http.ResponseWriter, r *http.Request) {
	user, err := h.Users.CurrentUser(r)
	if err != nil {
		if errors.Is(err, core.ErrUnauthorized) {
			core.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required.", nil, false)
			return
		}
		core.WriteError(w, r, http.StatusInternalServerError, "AUTH_PROVIDER_ERROR", "Could not resolve the current user.", nil, true)
		return
	}

	var request policiesPatchRequest
	if err := core.DecodeJSON(r, &request); err != nil {
		core.WriteError(
			w,
			r,
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"Request body is invalid.",
			map[string]any{"error": err.Error()},
			false,
		)
		return
	}

	policies, err := h.Service.UpdatePolicies(r.Context(), user, r.PathValue("projectId"), PolicySet{
		MinReplicas:                 request.MinReplicas,
		AllowedEnvironments:         request.AllowedEnvironments,
		AllowedDeploymentStrategies: request.AllowedDeploymentStrategies,
		AllowedClusterTargets:       request.AllowedClusterTargets,
		ProdPRRequired:              request.ProdPRRequired,
		AutoRollbackEnabled:         request.AutoRollbackEnabled,
		RequiredProbes:              request.RequiredProbes,
	})
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, policies)
}

func (h Handler) writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		core.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this project.", nil, false)
	case errors.Is(err, ErrNotFound):
		core.WriteError(w, r, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project was not found.", map[string]any{"projectId": r.PathValue("projectId")}, false)
	default:
		core.WriteError(w, r, http.StatusInternalServerError, "PROJECT_CATALOG_WRITE_FAILED", "Could not update the project catalog.", map[string]any{"error": err.Error()}, true)
	}
}
