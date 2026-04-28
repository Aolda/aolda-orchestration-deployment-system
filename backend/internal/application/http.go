package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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

func (h Handler) GetProjectHealth(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.GetProjectHealth(r.Context(), user, r.PathValue("projectId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, response)
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

func (h Handler) PreviewRepositorySource(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	var request PreviewRepositorySourceRequest
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

	response, err := h.Service.PreviewRepositorySource(
		r.Context(),
		user,
		r.PathValue("projectId"),
		request,
	)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) VerifyImageAccess(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	var request VerifyImageAccessRequest
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

	response, err := h.Service.VerifyImageAccess(
		r.Context(),
		user,
		r.PathValue("projectId"),
		request,
	)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, response)
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

func (h Handler) GetApplicationSecrets(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.GetApplicationSecrets(r.Context(), user, r.PathValue("applicationId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) UpdateApplicationSecrets(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	var request UpdateSecretsRequest
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

	response, err := h.Service.UpdateApplicationSecrets(
		r.Context(),
		user,
		r.PathValue("applicationId"),
		request,
		core.RequestID(r.Context()),
	)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) ListApplicationSecretVersions(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.ListApplicationSecretVersions(r.Context(), user, r.PathValue("applicationId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) RestoreApplicationSecretVersion(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil {
		core.WriteError(
			w,
			r,
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"Secret version must be a valid integer.",
			map[string]any{"field": "version"},
			false,
		)
		return
	}

	response, err := h.Service.RestoreApplicationSecretVersion(
		r.Context(),
		user,
		r.PathValue("applicationId"),
		version,
		core.RequestID(r.Context()),
	)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) ArchiveApplication(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.ArchiveApplication(r.Context(), user, r.PathValue("applicationId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) DeleteApplication(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.DeleteApplication(r.Context(), user, r.PathValue("applicationId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, response)
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

func (h Handler) SyncRepositoryNow(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.SyncRepositoryNow(r.Context(), user, r.PathValue("applicationId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) GetNetworkExposure(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.GetNetworkExposure(r.Context(), user, r.PathValue("applicationId"))
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

	duration, step := parseMetricWindow(r)

	response, err := h.Service.GetMetrics(r.Context(), user, r.PathValue("applicationId"), duration, step)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) GetMetricsDiagnostics(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	duration, step := parseMetricWindow(r)

	response, err := h.Service.GetMetricsDiagnostics(r.Context(), user, r.PathValue("applicationId"), duration, step)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) GetContainerLogs(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	tailLines, ok := parseTailLines(w, r)
	if !ok {
		return
	}

	response, err := h.Service.GetContainerLogs(r.Context(), user, r.PathValue("applicationId"), tailLines)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) GetContainerLogTargets(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	response, err := h.Service.GetContainerLogTargets(r.Context(), user, r.PathValue("applicationId"))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	core.WriteJSON(w, http.StatusOK, response)
}

func (h Handler) StreamContainerLogs(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	tailLines, ok := parseTailLines(w, r)
	if !ok {
		return
	}
	podName := strings.TrimSpace(r.URL.Query().Get("podName"))
	containerName := strings.TrimSpace(r.URL.Query().Get("containerName"))

	flusher, streamSupported := w.(http.Flusher)
	if !streamSupported {
		core.WriteError(w, r, http.StatusInternalServerError, "STREAM_UNSUPPORTED", "Streaming is not supported by the current server.", nil, true)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	wroteEvent := false
	emit := func(event ContainerLogEvent) error {
		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("encode log stream event: %w", err)
		}
		if _, err := fmt.Fprintf(w, "event: log\ndata: %s\n\n", payload); err != nil {
			return err
		}
		wroteEvent = true
		flusher.Flush()
		return nil
	}

	if err := h.Service.StreamContainerLogs(
		r.Context(),
		user,
		r.PathValue("applicationId"),
		podName,
		containerName,
		tailLines,
		emit,
	); err != nil {
		if !wroteEvent {
			h.writeDomainError(w, r, err)
			return
		}

		payload, _ := json.Marshal(map[string]string{
			"message": err.Error(),
		})
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", payload)
		flusher.Flush()
		return
	}

	_, _ = fmt.Fprint(w, "event: done\ndata: {}\n\n")
	flusher.Flush()
}

func parseTailLines(w http.ResponseWriter, r *http.Request) (int, bool) {
	tailLines := 120
	if raw := r.URL.Query().Get("tailLines"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			core.WriteError(
				w,
				r,
				http.StatusBadRequest,
				"INVALID_REQUEST",
				"tailLines must be a valid integer.",
				map[string]any{"field": "tailLines"},
				false,
			)
			return 0, false
		}
		tailLines = parsed
	}
	return tailLines, true
}

func parseMetricWindow(r *http.Request) (time.Duration, time.Duration) {
	duration := 15 * time.Minute
	if raw := r.URL.Query().Get("range"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			duration = parsed
		}
	}

	step := time.Minute
	if raw := r.URL.Query().Get("step"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			step = parsed
		}
	}

	return duration, step
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
	var imageError ImageValidationError

	switch {
	case errors.As(err, &validationError):
		core.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", validationError.Message, validationError.Details, false)
	case errors.As(err, &imageError):
		core.WriteError(
			w,
			r,
			http.StatusBadRequest,
			imageError.Code,
			imageError.Message,
			map[string]any{
				"image":    imageError.Image,
				"registry": imageError.Registry,
			},
			false,
		)
	case errors.Is(err, project.ErrForbidden), errors.Is(err, ErrRequiresDeployer), errors.Is(err, ErrRequiresAdmin):
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
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrArchived):
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
	case errors.Is(err, ErrAlreadyArchived):
		core.WriteError(
			w,
			r,
			http.StatusConflict,
			"APPLICATION_ALREADY_ARCHIVED",
			"Application is already archived.",
			map[string]any{"applicationId": r.PathValue("applicationId")},
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
