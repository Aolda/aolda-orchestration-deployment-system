package project

import (
	"context"
	"errors"
	"strings"

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

type WriteMode string

const (
	WriteModeDirect      WriteMode = "direct"
	WriteModePullRequest WriteMode = "pull_request"
)

type Access struct {
	ViewerGroups   []string
	DeployerGroups []string
	AdminGroups    []string
}

type Environment struct {
	ID        string
	Name      string
	ClusterID string
	WriteMode WriteMode
	Default   bool
}

type PolicySet struct {
	MinReplicas                 int
	AllowedEnvironments         []string
	AllowedDeploymentStrategies []string
	AllowedClusterTargets       []string
	ProdPRRequired              bool
	AutoRollbackEnabled         bool
	RequiredProbes              bool
}

type Repository struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	Description    string `json:"description,omitempty"`
	Branch         string `json:"branch,omitempty"`
	AuthSecretPath string `json:"authSecretPath,omitempty"`
	ConfigFile     string `json:"configFile,omitempty"`
}

type CatalogProject struct {
	ID           string
	Name         string
	Description  string
	Namespace    string
	Access       Access
	Environments []Environment
	Repositories []Repository
	Policies     PolicySet
}

type Summary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Namespace   string `json:"namespace"`
	Role        Role   `json:"role"`
}

type EnvironmentSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ClusterID string    `json:"clusterId"`
	WriteMode WriteMode `json:"writeMode"`
	Default   bool      `json:"default"`
}

type RepositorySummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
	Access      string `json:"access"`
	Branch      string `json:"branch,omitempty"`
	ConfigFile  string `json:"configFile,omitempty"`
}

type PolicySummary struct {
	MinReplicas                 int      `json:"minReplicas"`
	AllowedEnvironments         []string `json:"allowedEnvironments"`
	AllowedDeploymentStrategies []string `json:"allowedDeploymentStrategies"`
	AllowedClusterTargets       []string `json:"allowedClusterTargets"`
	ProdPRRequired              bool     `json:"prodPRRequired"`
	AutoRollbackEnabled         bool     `json:"autoRollbackEnabled"`
	RequiredProbes              bool     `json:"requiredProbes"`
}

type AuthorizedProject struct {
	Project CatalogProject
	Role    Role
}

type CatalogSource interface {
	ListProjects(ctx context.Context) ([]CatalogProject, error)
}

type CatalogStore interface {
	CatalogSource
	UpdatePolicies(ctx context.Context, projectID string, policies PolicySet) (CatalogProject, error)
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

func (r Role) CanAdmin() bool {
	return r == RoleAdmin
}

func (s Service) ListEnvironments(ctx context.Context, user core.User, projectID string) ([]EnvironmentSummary, error) {
	authorized, err := s.GetAuthorized(ctx, user, projectID)
	if err != nil {
		return nil, err
	}

	items := make([]EnvironmentSummary, 0, len(authorized.Project.Environments))
	for _, environment := range authorized.Project.Environments {
		items = append(items, EnvironmentSummary{
			ID:        environment.ID,
			Name:      environment.Name,
			ClusterID: environment.ClusterID,
			WriteMode: environment.WriteMode,
			Default:   environment.Default,
		})
	}
	return items, nil
}

func (s Service) ListRepositories(ctx context.Context, user core.User, projectID string) ([]RepositorySummary, error) {
	authorized, err := s.GetAuthorized(ctx, user, projectID)
	if err != nil {
		return nil, err
	}

	items := make([]RepositorySummary, 0, len(authorized.Project.Repositories))
	for _, repo := range authorized.Project.Repositories {
		items = append(items, RepositorySummary{
			ID:          repo.ID,
			Name:        repo.Name,
			URL:         repo.URL,
			Description: repo.Description,
			Access:      repositoryAccess(repo),
			Branch:      repo.Branch,
			ConfigFile:  repo.ConfigFile,
		})
	}
	return items, nil
}

func (s Service) GetPolicies(ctx context.Context, user core.User, projectID string) (PolicySummary, error) {
	authorized, err := s.GetAuthorized(ctx, user, projectID)
	if err != nil {
		return PolicySummary{}, err
	}
	return toPolicySummary(authorized.Project.Policies), nil
}

func (s Service) UpdatePolicies(ctx context.Context, user core.User, projectID string, policies PolicySet) (PolicySummary, error) {
	authorized, err := s.GetAuthorized(ctx, user, projectID)
	if err != nil {
		return PolicySummary{}, err
	}
	if !authorized.Role.CanAdmin() {
		return PolicySummary{}, ErrForbidden
	}

	store, ok := s.Source.(CatalogStore)
	if !ok {
		return PolicySummary{}, errors.New("project catalog is not writable")
	}

	policies = applyPolicyDefaults(policies, authorized.Project.Environments)
	updated, err := store.UpdatePolicies(ctx, projectID, policies)
	if err != nil {
		return PolicySummary{}, err
	}
	return toPolicySummary(updated.Policies), nil
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

func applyProjectDefaults(project CatalogProject) CatalogProject {
	project.Environments = applyEnvironmentDefaults(project.Environments)
	project.Policies = applyPolicyDefaults(project.Policies, project.Environments)
	return project
}

func applyEnvironmentDefaults(environments []Environment) []Environment {
	if len(environments) == 0 {
		return []Environment{{
			ID:        "shared",
			Name:      "Shared",
			ClusterID: "default",
			WriteMode: WriteModeDirect,
			Default:   true,
		}}
	}

	items := make([]Environment, 0, len(environments))
	hasExplicitDefault := false
	for _, environment := range environments {
		if environment.ID != "" && environment.Default {
			hasExplicitDefault = true
			break
		}
	}

	for index, environment := range environments {
		item := environment
		if item.ID == "" {
			continue
		}
		if item.Name == "" {
			item.Name = item.ID
		}
		if item.ClusterID == "" {
			item.ClusterID = "default"
		}
		if item.WriteMode == "" {
			item.WriteMode = WriteModeDirect
		}
		if index == 0 && !hasExplicitDefault {
			item.Default = true
		}
		items = append(items, item)
	}

	if !hasExplicitDefault && len(items) > 0 {
		items[0].Default = true
	}

	return items
}

func applyPolicyDefaults(policy PolicySet, environments []Environment) PolicySet {
	defaultRequiredProbes := shouldDefaultRequiredProbes(policy)
	if policy.MinReplicas <= 0 {
		policy.MinReplicas = 1
	}
	if len(policy.AllowedEnvironments) == 0 {
		for _, environment := range environments {
			policy.AllowedEnvironments = append(policy.AllowedEnvironments, environment.ID)
		}
	}
	if len(policy.AllowedDeploymentStrategies) == 0 {
		policy.AllowedDeploymentStrategies = []string{"Rollout", "Canary"}
	} else {
		policy.AllowedDeploymentStrategies = normalizeAllowedStrategies(policy.AllowedDeploymentStrategies)
	}
	if len(policy.AllowedClusterTargets) == 0 {
		for _, environment := range environments {
			if environment.ClusterID == "" {
				continue
			}
			if !containsString(policy.AllowedClusterTargets, environment.ClusterID) {
				policy.AllowedClusterTargets = append(policy.AllowedClusterTargets, environment.ClusterID)
			}
		}
	}
	if len(policy.AllowedClusterTargets) == 0 {
		policy.AllowedClusterTargets = []string{"default"}
	}
	if defaultRequiredProbes {
		policy.RequiredProbes = true
	}
	return policy
}

func normalizeAllowedStrategies(values []string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		switch value {
		case "Standard", "Rollout":
			if !containsString(items, "Rollout") {
				items = append(items, "Rollout")
			}
		case "Canary":
			if !containsString(items, "Canary") {
				items = append(items, "Canary")
			}
		}
	}
	if len(items) == 0 {
		return []string{"Rollout", "Canary"}
	}
	return items
}

func repositoryAccess(repo Repository) string {
	if strings.TrimSpace(repo.AuthSecretPath) != "" {
		return "private"
	}
	return "public"
}

func toPolicySummary(policy PolicySet) PolicySummary {
	return PolicySummary{
		MinReplicas:                 policy.MinReplicas,
		AllowedEnvironments:         append([]string(nil), policy.AllowedEnvironments...),
		AllowedDeploymentStrategies: append([]string(nil), policy.AllowedDeploymentStrategies...),
		AllowedClusterTargets:       append([]string(nil), policy.AllowedClusterTargets...),
		ProdPRRequired:              policy.ProdPRRequired,
		AutoRollbackEnabled:         policy.AutoRollbackEnabled,
		RequiredProbes:              policy.RequiredProbes,
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func shouldDefaultRequiredProbes(policy PolicySet) bool {
	return !policy.RequiredProbes &&
		policy.MinReplicas <= 0 &&
		len(policy.AllowedEnvironments) == 0 &&
		len(policy.AllowedDeploymentStrategies) == 0 &&
		len(policy.AllowedClusterTargets) == 0 &&
		!policy.ProdPRRequired &&
		!policy.AutoRollbackEnabled
}
