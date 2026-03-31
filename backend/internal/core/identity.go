package core

import (
	"errors"
	"net/http"
)

func CurrentUserHandler(users UserProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := users.CurrentUser(r)
		if err != nil {
			if errors.Is(err, ErrUnauthorized) {
				WriteError(
					w,
					r,
					http.StatusUnauthorized,
					"UNAUTHORIZED",
					"Authentication is required.",
					nil,
					false,
				)
				return
			}

			WriteError(
				w,
				r,
				http.StatusInternalServerError,
				"AUTH_PROVIDER_ERROR",
				"Could not resolve the current user.",
				nil,
				true,
			)
			return
		}

		WriteJSON(w, http.StatusOK, user)
	}
}
