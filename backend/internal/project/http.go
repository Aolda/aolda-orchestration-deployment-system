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
