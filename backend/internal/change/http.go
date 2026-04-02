package change

import (
	"errors"
	"net/http"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/project"
)

type Handler struct {
	Service *Service
	Users   core.UserProvider
}

func (h Handler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	var request Request
	if err := core.DecodeJSON(r, &request); err != nil {
		core.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Request body is invalid.", map[string]any{"error": err.Error()}, false)
		return
	}

	change, err := h.Service.Create(r.Context(), user, r.PathValue("projectId"), request, core.RequestID(r.Context()))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusCreated, change)
}

func (h Handler) Get(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	change, err := h.Service.Get(r.Context(), user, r.PathValue("changeId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, change)
}

func (h Handler) Submit(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	change, err := h.Service.Submit(r.Context(), user, r.PathValue("changeId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, change)
}

func (h Handler) Approve(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	change, err := h.Service.Approve(r.Context(), user, r.PathValue("changeId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, change)
}

func (h Handler) Merge(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	change, err := h.Service.Merge(r.Context(), user, r.PathValue("changeId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, change)
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
	switch {
	case errors.Is(err, project.ErrForbidden), errors.Is(err, application.ErrRequiresDeployer), errors.Is(err, application.ErrRequiresAdmin):
		core.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "You do not have permission to perform this action.", nil, false)
	case errors.Is(err, project.ErrNotFound):
		core.WriteError(w, r, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project was not found.", map[string]any{"projectId": r.PathValue("projectId")}, false)
	case errors.Is(err, ErrNotFound):
		core.WriteError(w, r, http.StatusNotFound, "CHANGE_NOT_FOUND", "Change was not found.", map[string]any{"changeId": r.PathValue("changeId")}, false)
	case errors.Is(err, ErrApprovalRequired):
		core.WriteError(w, r, http.StatusConflict, "CHANGE_APPROVAL_REQUIRED", "This change must be approved before merge.", map[string]any{"changeId": r.PathValue("changeId")}, false)
	default:
		core.WriteError(w, r, http.StatusInternalServerError, "CHANGE_PROCESSING_FAILED", "Could not process the change.", map[string]any{"error": err.Error()}, true)
	}
}
