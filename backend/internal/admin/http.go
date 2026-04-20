package admin

import (
	"errors"
	"net/http"

	"github.com/aolda/aods-backend/internal/core"
)

type Handler struct {
	Service *Service
	Users   core.UserProvider
}

func (h Handler) GetFleetResourceOverview(w http.ResponseWriter, r *http.Request) {
	user, err := h.Users.CurrentUser(r)
	if err != nil {
		if errors.Is(err, core.ErrUnauthorized) {
			core.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required.", nil, false)
			return
		}
		core.WriteError(w, r, http.StatusInternalServerError, "AUTH_PROVIDER_ERROR", "Could not resolve the current user.", nil, true)
		return
	}

	response, err := h.Service.GetFleetResourceOverview(r.Context(), user)
	if err != nil {
		switch {
		case errors.Is(err, ErrForbidden):
			core.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Platform admin permissions are required.", nil, false)
		default:
			core.WriteError(w, r, http.StatusInternalServerError, "ADMIN_RESOURCE_OVERVIEW_FAILED", "Could not calculate the fleet resource overview.", map[string]any{"error": err.Error()}, true)
		}
		return
	}

	core.WriteJSON(w, http.StatusOK, response)
}
