package core

import (
	"errors"
	"fmt"
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

type ErrorUserProvider struct {
	Err error
}

func (p ErrorUserProvider) CurrentUser(r *http.Request) (User, error) {
	if p.Err == nil {
		return User{}, ErrUnauthorized
	}
	return User{}, p.Err
}

type HeaderUserProvider struct {
	AllowDevFallback bool
	DevUser          User
}

type DevFallbackUserProvider struct {
	AllowDevFallback bool
	DevUser          User
}

func (p HeaderUserProvider) CurrentUser(r *http.Request) (User, error) {
	user, hasExplicitHeaders, err := userFromHeaders(r)
	if err == nil {
		return user, nil
	}
	if errors.Is(err, ErrUnauthorized) && !hasExplicitHeaders && p.AllowDevFallback {
		return p.DevUser, nil
	}
	return User{}, err
}

type CompositeUserProvider struct {
	Primary     UserProvider
	DevFallback DevFallbackUserProvider
}

func (p CompositeUserProvider) CurrentUser(r *http.Request) (User, error) {
	if authorizationHeaderProvided(r) {
		if p.Primary == nil {
			return User{}, ErrUnauthorized
		}
		return p.Primary.CurrentUser(r)
	}
	return p.DevFallback.CurrentUser(r)
}

func (p DevFallbackUserProvider) CurrentUser(r *http.Request) (User, error) {
	if authorizationHeaderProvided(r) || headerIdentityProvided(r) {
		return User{}, ErrUnauthorized
	}
	if p.AllowDevFallback {
		return p.DevUser, nil
	}
	return User{}, ErrUnauthorized
}

func NewUserProvider(cfg Config) UserProvider {
	header := HeaderUserProvider{
		AllowDevFallback: cfg.AllowDevFallback,
		DevUser:          cfg.DevUser,
	}

	var provider UserProvider
	switch strings.ToLower(strings.TrimSpace(cfg.AuthMode)) {
	case "", "header":
		provider = header
	case "oidc":
		oidcProvider, err := NewOIDCUserProvider(cfg)
		if err != nil {
			provider = ErrorUserProvider{Err: fmt.Errorf("configure oidc auth provider: %w", err)}
			break
		}
		provider = CompositeUserProvider{
			Primary: oidcProvider,
			DevFallback: DevFallbackUserProvider{
				AllowDevFallback: cfg.AllowDevFallback,
				DevUser:          cfg.DevUser,
			},
		}
	default:
		provider = ErrorUserProvider{Err: fmt.Errorf("unsupported auth mode %q", cfg.AuthMode)}
	}

	if len(cfg.OIDCRoleMappings) > 0 {
		provider = AuthorityMappingUserProvider{
			Base:     provider,
			Mappings: cfg.OIDCRoleMappings,
		}
	}

	return provider
}

func userFromHeaders(r *http.Request) (User, bool, error) {
	userID := strings.TrimSpace(r.Header.Get("X-AODS-User-Id"))
	username := strings.TrimSpace(r.Header.Get("X-AODS-Username"))
	displayName := strings.TrimSpace(r.Header.Get("X-AODS-Display-Name"))
	groupsHeader := strings.TrimSpace(r.Header.Get("X-AODS-Groups"))

	hasExplicitHeaders := userID != "" || username != "" || displayName != "" || groupsHeader != ""
	if userID == "" || username == "" {
		return User{}, hasExplicitHeaders, ErrUnauthorized
	}

	return User{
		ID:          userID,
		Username:    username,
		DisplayName: displayName,
		Groups:      splitCommaSeparated(groupsHeader),
	}, hasExplicitHeaders, nil
}

func bearerTokenFromRequest(r *http.Request) (string, bool) {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	if raw == "" {
		return "", false
	}
	scheme, token, ok := strings.Cut(raw, " ")
	if !ok || !strings.EqualFold(strings.TrimSpace(scheme), "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}

func authorizationHeaderProvided(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get("Authorization")) != ""
}

func headerIdentityProvided(r *http.Request) bool {
	_, hasExplicitHeaders, _ := userFromHeaders(r)
	return hasExplicitHeaders
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
