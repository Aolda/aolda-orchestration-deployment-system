package application

import (
	"errors"
	"net/http"

	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/project"
)

type Handler struct {
	Service *Service
	Users   core.UserProvider
}

func (h Handler) ListApplications(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	items, err := h.Service.ListApplications(r.Context(), user, r.PathValue("projectId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, struct {
		Items []Summary `json:"items"`
	}{
		Items: items,
	})
}

func (h Handler) CreateApplication(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
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

	application, err := h.Service.CreateApplication(
		r.Context(),
		user,
		r.PathValue("projectId"),
		request,
		core.RequestID(r.Context()),
	)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusCreated, application)
}

func (h Handler) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	var request CreateDeploymentRequest
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

	deployment, err := h.Service.CreateDeployment(
		r.Context(),
		user,
		r.PathValue("applicationId"),
		request.ImageTag,
		request.Environment,
		core.RequestID(r.Context()),
	)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusCreated, deployment)
}

func (h Handler) PatchApplication(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	var request UpdateApplicationRequest
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

	application, err := h.Service.PatchApplication(r.Context(), user, r.PathValue("applicationId"), request)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, application)
}

func (h Handler) GetSyncStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.GetSyncStatus(r.Context(), user, r.PathValue("applicationId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.GetMetrics(r.Context(), user, r.PathValue("applicationId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.ListDeployments(r.Context(), user, r.PathValue("applicationId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) GetDeployment(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.GetDeployment(r.Context(), user, r.PathValue("applicationId"), r.PathValue("deploymentId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) PromoteDeployment(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.PromoteDeployment(r.Context(), user, r.PathValue("applicationId"), r.PathValue("deploymentId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) AbortDeployment(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.AbortDeployment(r.Context(), user, r.PathValue("applicationId"), r.PathValue("deploymentId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) GetRollbackPolicy(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.GetRollbackPolicy(r.Context(), user, r.PathValue("applicationId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) SaveRollbackPolicy(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	var request RollbackPolicy
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

	response, err := h.Service.SaveRollbackPolicy(r.Context(), user, r.PathValue("applicationId"), request)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) GetEvents(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.GetEvents(r.Context(), user, r.PathValue("applicationId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) currentUser(w http.ResponseWriter, r *http.Request) (core.User, bool) {
	user, err := h.Users.CurrentUser(r)
	if err != nil {
		if errors.Is(err, core.ErrUnauthorized) {
			core.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required.", nil, false)
			return core.User{}, false
		}

		core.WriteError(w, r, http.StatusInternalServerError, "AUTH_PROVIDER_ERROR", "Could not resolve the current user.", nil, true)
		return core.User{}, false
	}

	return user, true
}

func (h Handler) writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	var validationError ValidationError

	switch {
	case errors.As(err, &validationError):
		core.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", validationError.Message, validationError.Details, false)
	case errors.Is(err, project.ErrForbidden), errors.Is(err, ErrRequiresDeployer):
		core.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "You do not have permission to perform this action.", nil, false)
	case errors.Is(err, ErrChangeRequired):
		core.WriteError(
			w,
			r,
			http.StatusConflict,
			"CHANGE_REVIEW_REQUIRED",
			"Direct mutation is disabled for this environment. Create a reviewed change instead.",
			map[string]any{"applicationId": r.PathValue("applicationId")},
			false,
		)
	case errors.Is(err, project.ErrNotFound):
		core.WriteError(
			w,
			r,
			http.StatusNotFound,
			"PROJECT_NOT_FOUND",
			"Project was not found.",
			map[string]any{"projectId": r.PathValue("projectId")},
			false,
		)
	case errors.Is(err, ErrInvalidID):
		core.WriteError(
			w,
			r,
			http.StatusNotFound,
			"APPLICATION_NOT_FOUND",
			"Application was not found.",
			map[string]any{"applicationId": r.PathValue("applicationId")},
			false,
		)
	case errors.Is(err, ErrNotFound):
		core.WriteError(
			w,
			r,
			http.StatusNotFound,
			"APPLICATION_NOT_FOUND",
			"Application was not found.",
			map[string]any{"applicationId": r.PathValue("applicationId")},
			false,
		)
	case errors.Is(err, ErrDeploymentNotFound):
		core.WriteError(
			w,
			r,
			http.StatusNotFound,
			"DEPLOYMENT_NOT_FOUND",
			"Deployment was not found.",
			map[string]any{
				"applicationId": r.PathValue("applicationId"),
				"deploymentId":  r.PathValue("deploymentId"),
			},
			false,
		)
	case errors.Is(err, ErrConflict):
		core.WriteError(
			w,
			r,
			http.StatusConflict,
			"DUPLICATE_APPLICATION",
			"An application with this name already exists.",
			map[string]any{"projectId": r.PathValue("projectId")},
			false,
		)
	default:
		core.WriteError(
			w,
			r,
			http.StatusInternalServerError,
			"INTEGRATION_ERROR",
			"An unexpected integration error occurred.",
			map[string]any{"error": err.Error()},
			true,
		)
	}
}
