package cluster

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/aolda/aods-backend/internal/core"
)

var (
	ErrForbidden = errors.New("cluster forbidden")
	ErrConflict  = errors.New("cluster conflict")
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

const platformAdminGroup = "aods:platform:admin"

type Summary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Default     bool   `json:"default"`
}

type CreateRequest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Default     bool   `json:"default,omitempty"`
}

type ValidationError struct {
	Message string
	Details map[string]any
}

func (e ValidationError) Error() string {
	return e.Message
}

type Source interface {
	ListClusters(ctx context.Context) ([]Summary, error)
}

type Store interface {
	Source
	CreateCluster(ctx context.Context, input CreateRequest) (Summary, error)
}

type Service struct {
	Source                   Source
	PlatformAdminAuthorities []string
}

func (s Service) List(ctx context.Context) ([]Summary, error) {
	if s.Source == nil {
		return []Summary{defaultCluster()}, nil
	}

	items, err := s.Source.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []Summary{defaultCluster()}, nil
	}
	return items, nil
}

func (s Service) Create(ctx context.Context, user core.User, input CreateRequest) (Summary, error) {
	if !isPlatformAdmin(user, s.platformAdminAuthorities()) {
		return Summary{}, ErrForbidden
	}
	if err := validateCreateRequest(input); err != nil {
		return Summary{}, err
	}

	store, ok := s.Source.(Store)
	if !ok {
		return Summary{}, errors.New("cluster catalog is not writable")
	}

	return store.CreateCluster(ctx, input)
}

func defaultCluster() Summary {
	return Summary{
		ID:          "default",
		Name:        "Default Cluster",
		Description: "기본 클러스터 타겟",
		Default:     true,
	}
}

func validateCreateRequest(input CreateRequest) error {
	if !slugPattern.MatchString(strings.TrimSpace(input.ID)) {
		return ValidationError{
			Message: "cluster id must be a lowercase slug",
			Details: map[string]any{"field": "id"},
		}
	}
	if strings.TrimSpace(input.Name) == "" {
		return ValidationError{
			Message: "cluster name is required",
			Details: map[string]any{"field": "name"},
		}
	}
	return nil
}

func isPlatformAdmin(user core.User, platformAdminAuthorities []string) bool {
	groupSet := make(map[string]struct{}, len(user.Groups))
	for _, group := range user.Groups {
		groupSet[group] = struct{}{}
	}
	for _, authority := range platformAdminAuthorities {
		if _, ok := groupSet[authority]; ok {
			return true
		}
	}
	return false
}

func (s Service) platformAdminAuthorities() []string {
	items := make([]string, 0, len(s.PlatformAdminAuthorities))
	seen := make(map[string]struct{}, len(s.PlatformAdminAuthorities))
	for _, authority := range s.PlatformAdminAuthorities {
		trimmed := strings.TrimSpace(authority)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	if len(items) == 0 {
		return []string{platformAdminGroup}
	}
	return items
}
