package core

import (
	"errors"
	"net/http"
	"strings"
)

var ErrUnauthorized = errors.New("unauthorized")

type User struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName,omitempty"`
	Groups      []string `json:"groups,omitempty"`
}

type UserProvider interface {
	CurrentUser(r *http.Request) (User, error)
}

type HeaderUserProvider struct {
	AllowDevFallback bool
	DevUser          User
}

func (p HeaderUserProvider) CurrentUser(r *http.Request) (User, error) {
	userID := strings.TrimSpace(r.Header.Get("X-AODS-User-Id"))
	username := strings.TrimSpace(r.Header.Get("X-AODS-Username"))
	displayName := strings.TrimSpace(r.Header.Get("X-AODS-Display-Name"))
	groupsHeader := strings.TrimSpace(r.Header.Get("X-AODS-Groups"))

	hasExplicitHeaders := userID != "" || username != "" || displayName != "" || groupsHeader != ""
	if userID == "" && username == "" && !hasExplicitHeaders && p.AllowDevFallback {
		return p.DevUser, nil
	}

	if userID == "" || username == "" {
		return User{}, ErrUnauthorized
	}

	return User{
		ID:          userID,
		Username:    username,
		DisplayName: displayName,
		Groups:      splitCommaSeparated(groupsHeader),
	}, nil
}

func splitCommaSeparated(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}

	return items
}
