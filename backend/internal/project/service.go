package project

import (
	"context"
	"errors"

	"github.com/aolda/aods-backend/internal/core"
)

var (
	ErrNotFound  = errors.New("project not found")
	ErrForbidden = errors.New("project forbidden")
)

type Role string

const (
	RoleViewer   Role = "viewer"
	RoleDeployer Role = "deployer"
	RoleAdmin    Role = "admin"
)

type Access struct {
	ViewerGroups   []string
	DeployerGroups []string
	AdminGroups    []string
}

type CatalogProject struct {
	ID          string
	Name        string
	Description string
	Namespace   string
	Access      Access
}

type Summary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Namespace   string `json:"namespace"`
	Role        Role   `json:"role"`
}

type AuthorizedProject struct {
	Project CatalogProject
	Role    Role
}

type CatalogSource interface {
	ListProjects(ctx context.Context) ([]CatalogProject, error)
}

type Service struct {
	Source CatalogSource
}

func (s Service) ListAuthorized(ctx context.Context, user core.User) ([]Summary, error) {
	projects, err := s.Source.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	groupSet := makeGroupSet(user.Groups)
	items := make([]Summary, 0, len(projects))
	for _, project := range projects {
		role, ok := resolveRole(groupSet, project.Access)
		if !ok {
			continue
		}

		items = append(items, Summary{
			ID:          project.ID,
			Name:        project.Name,
			Description: project.Description,
			Namespace:   project.Namespace,
			Role:        role,
		})
	}

	return items, nil
}

func (s Service) GetAuthorized(ctx context.Context, user core.User, projectID string) (AuthorizedProject, error) {
	projects, err := s.Source.ListProjects(ctx)
	if err != nil {
		return AuthorizedProject{}, err
	}

	groupSet := makeGroupSet(user.Groups)
	for _, project := range projects {
		if project.ID != projectID {
			continue
		}

		role, ok := resolveRole(groupSet, project.Access)
		if !ok {
			return AuthorizedProject{}, ErrForbidden
		}

		return AuthorizedProject{
			Project: project,
			Role:    role,
		}, nil
	}

	return AuthorizedProject{}, ErrNotFound
}

func (r Role) CanDeploy() bool {
	return r == RoleDeployer || r == RoleAdmin
}

func resolveRole(groupSet map[string]struct{}, access Access) (Role, bool) {
	if hasAnyGroup(groupSet, access.AdminGroups) {
		return RoleAdmin, true
	}
	if hasAnyGroup(groupSet, access.DeployerGroups) {
		return RoleDeployer, true
	}
	if hasAnyGroup(groupSet, access.ViewerGroups) {
		return RoleViewer, true
	}
	return "", false
}

func makeGroupSet(groups []string) map[string]struct{} {
	set := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		set[group] = struct{}{}
	}
	return set
}

func hasAnyGroup(groupSet map[string]struct{}, required []string) bool {
	for _, group := range required {
		if _, ok := groupSet[group]; ok {
			return true
		}
	}
	return false
}
