package cluster

import (
	"errors"
	"net/http"

	"github.com/aolda/aods-backend/internal/core"
)

type Handler struct {
	Service *Service
	Users   core.UserProvider
}

func (h Handler) ListClusters(w http.ResponseWriter, r *http.Request) {
	items, err := h.Service.List(r.Context())
	if err != nil {
		core.WriteError(
			w,
			r,
			http.StatusInternalServerError,
			"CLUSTER_CATALOG_READ_FAILED",
			"Could not read the cluster catalog.",
			map[string]any{"error": err.Error()},
			true,
		)
		return
	}

	core.WriteJSON(w, http.StatusOK, struct {
		Items []Summary `json:"items"`
	}{Items: items})
}

func (h Handler) CreateCluster(w http.ResponseWriter, r *http.Request) {
	user, err := h.Users.CurrentUser(r)
	if err != nil {
		if errors.Is(err, core.ErrUnauthorized) {
			core.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required.", nil, false)
			return
		}
		core.WriteError(w, r, http.StatusInternalServerError, "AUTH_PROVIDER_ERROR", "Could not resolve the current user.", nil, true)
		return
	}

	var request CreateRequest
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

	cluster, err := h.Service.Create(r.Context(), user, request)
	if err != nil {
		switch typed := err.(type) {
		case ValidationError:
			core.WriteError(w, r, http.StatusBadRequest, "INVALID_CLUSTER_REQUEST", typed.Message, typed.Details, false)
			return
		}
		switch {
		case errors.Is(err, ErrForbidden):
			core.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Platform admin permissions are required.", nil, false)
		case errors.Is(err, ErrConflict):
			core.WriteError(w, r, http.StatusConflict, "CLUSTER_CONFLICT", "Cluster already exists.", map[string]any{"id": request.ID}, false)
		default:
			core.WriteError(w, r, http.StatusInternalServerError, "CLUSTER_CATALOG_WRITE_FAILED", "Could not update the cluster catalog.", map[string]any{"error": err.Error()}, true)
		}
		return
	}

	core.WriteJSON(w, http.StatusCreated, cluster)
}
