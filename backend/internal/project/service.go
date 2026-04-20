package project

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/cluster"
	"github.com/aolda/aods-backend/internal/core"
)

var (
	ErrNotFound  = errors.New("project not found")
	ErrForbidden = errors.New("project forbidden")
	ErrConflict  = errors.New("project conflict")
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

const platformAdminGroup = "aods:platform:admin"
const sharedProjectNamespace = "shared"

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
	ViewerGroups   []string `json:"viewerGroups"`
	DeployerGroups []string `json:"deployerGroups"`
	AdminGroups    []string `json:"adminGroups"`
}

type Environment struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ClusterID string    `json:"clusterId"`
	WriteMode WriteMode `json:"writeMode"`
	Default   bool      `json:"default"`
}

type PolicySet struct {
	MinReplicas                 int      `json:"minReplicas"`
	AllowedEnvironments         []string `json:"allowedEnvironments"`
	AllowedDeploymentStrategies []string `json:"allowedDeploymentStrategies"`
	AllowedClusterTargets       []string `json:"allowedClusterTargets"`
	ProdPRRequired              bool     `json:"prodPRRequired"`
	AutoRollbackEnabled         bool     `json:"autoRollbackEnabled"`
	RequiredProbes              bool     `json:"requiredProbes"`
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

type LifecycleResponse struct {
	ProjectID string     `json:"projectId"`
	Name      string     `json:"name"`
	Namespace string     `json:"namespace"`
	Status    string     `json:"status"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`

	secretPaths []string
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

type CreateRequest struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	Namespace    string        `json:"namespace,omitempty"`
	Access       Access        `json:"access,omitempty"`
	Environments []Environment `json:"environments,omitempty"`
	Repositories []Repository  `json:"repositories,omitempty"`
	Policies     PolicySet     `json:"policies,omitempty"`
}

type ValidationError struct {
	Message string
	Details map[string]any
}

func (e ValidationError) Error() string {
	return e.Message
}

type ProtectedProjectError struct {
	ProjectID  string
	Namespace  string
	ReasonCode string
}

func (e ProtectedProjectError) Error() string {
	return "protected project cannot be deleted"
}

type CatalogSource interface {
	ListProjects(ctx context.Context) ([]CatalogProject, error)
}

type CatalogStore interface {
	CatalogSource
	CreateProject(ctx context.Context, input CreateRequest) (CatalogProject, error)
	DeleteProject(ctx context.Context, projectID string) (LifecycleResponse, error)
	UpdatePolicies(ctx context.Context, projectID string, policies PolicySet) (CatalogProject, error)
}

type SecretStore interface {
	Delete(ctx context.Context, logicalPath string) error
}

type Service struct {
	Source                   CatalogSource
	Clusters                 cluster.Source
	Secrets                  SecretStore
	PlatformAdminAuthorities []string
}

func (s Service) ListAuthorized(ctx context.Context, user core.User) ([]Summary, error) {
	projects, err := s.Source.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	groupSet := makeGroupSet(user.Groups)
	items := make([]Summary, 0, len(projects))
	for _, project := range projects {
		role, ok := resolveRoleWithPlatformAdmins(groupSet, project.Access, s.PlatformAdminAuthoritiesOrDefault())
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

		role, ok := resolveRoleWithPlatformAdmins(groupSet, project.Access, s.PlatformAdminAuthoritiesOrDefault())
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

func (s Service) Create(ctx context.Context, user core.User, input CreateRequest) (Summary, error) {
	if !isPlatformAdmin(user, s.PlatformAdminAuthoritiesOrDefault()) {
		return Summary{}, ErrForbidden
	}

	store, ok := s.Source.(CatalogStore)
	if !ok {
		return Summary{}, errors.New("project catalog is not writable")
	}

	normalized, err := s.normalizeCreateRequest(ctx, input)
	if err != nil {
		return Summary{}, err
	}

	created, err := store.CreateProject(ctx, normalized)
	if err != nil {
		return Summary{}, err
	}

	role, ok := resolveRoleWithPlatformAdmins(makeGroupSet(user.Groups), created.Access, s.PlatformAdminAuthoritiesOrDefault())
	if !ok {
		role = RoleAdmin
	}

	return Summary{
		ID:          created.ID,
		Name:        created.Name,
		Description: created.Description,
		Namespace:   created.Namespace,
		Role:        role,
	}, nil
}

func (s Service) Delete(ctx context.Context, user core.User, projectID string) (LifecycleResponse, error) {
	if !isPlatformAdmin(user, s.PlatformAdminAuthoritiesOrDefault()) {
		return LifecycleResponse{}, ErrForbidden
	}

	store, ok := s.Source.(CatalogStore)
	if !ok {
		return LifecycleResponse{}, errors.New("project catalog is not writable")
	}

	project, err := s.getProject(ctx, projectID)
	if err != nil {
		return LifecycleResponse{}, err
	}
	if isProtectedProject(project) {
		return LifecycleResponse{}, ProtectedProjectError{
			ProjectID:  project.ID,
			Namespace:  project.Namespace,
			ReasonCode: "shared_namespace",
		}
	}

	result, err := store.DeleteProject(ctx, projectID)
	if err != nil {
		return LifecycleResponse{}, err
	}

	if s.Secrets != nil {
		for _, secretPath := range dedupeStrings(result.secretPaths) {
			if secretPath == "" {
				continue
			}
			if err := s.Secrets.Delete(ctx, secretPath); err != nil {
				return LifecycleResponse{}, err
			}
		}
	}

	return result, nil
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

func (s Service) getProject(ctx context.Context, projectID string) (CatalogProject, error) {
	projects, err := s.Source.ListProjects(ctx)
	if err != nil {
		return CatalogProject{}, err
	}
	for _, project := range projects {
		if project.ID == projectID {
			return project, nil
		}
	}
	return CatalogProject{}, ErrNotFound
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

func resolveRoleWithPlatformAdmins(groupSet map[string]struct{}, access Access, platformAdminAuthorities []string) (Role, bool) {
	if hasAnyGroup(groupSet, effectivePlatformAdminAuthorities(platformAdminAuthorities)) {
		return RoleAdmin, true
	}
	return resolveRole(groupSet, access)
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

func isProtectedProject(project CatalogProject) bool {
	return strings.EqualFold(strings.TrimSpace(project.Namespace), sharedProjectNamespace)
}

func applyProjectDefaults(project CatalogProject) CatalogProject {
	project.Environments = applyEnvironmentDefaults(project.Environments)
	if strings.TrimSpace(project.Namespace) == "" {
		project.Namespace = project.ID
	}
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
	defaultIndex := -1
	for index, environment := range environments {
		if environment.ID != "" && environment.Default {
			defaultIndex = index
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
		if defaultIndex == -1 {
			item.Default = index == 0
		} else {
			item.Default = index == defaultIndex
		}
		items = append(items, item)
	}

	if defaultIndex == -1 && len(items) > 0 {
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

func (s Service) normalizeCreateRequest(ctx context.Context, input CreateRequest) (CreateRequest, error) {
	projectID := strings.TrimSpace(input.ID)
	if !slugPattern.MatchString(projectID) {
		return CreateRequest{}, ValidationError{
			Message: "project id must be a lowercase slug",
			Details: map[string]any{"field": "id"},
		}
	}
	projectName := strings.TrimSpace(input.Name)
	if projectName == "" {
		return CreateRequest{}, ValidationError{
			Message: "project name is required",
			Details: map[string]any{"field": "name"},
		}
	}
	if !slugPattern.MatchString(projectName) {
		return CreateRequest{}, ValidationError{
			Message: "project name must be a lowercase slug",
			Details: map[string]any{"field": "name"},
		}
	}
	if projectName != projectID {
		return CreateRequest{}, ValidationError{
			Message: "project name must match project id",
			Details: map[string]any{"field": "name", "id": projectID, "name": projectName},
		}
	}

	environments, err := normalizeCreateEnvironments(input.Environments)
	if err != nil {
		return CreateRequest{}, err
	}
	repositories, err := normalizeCreateRepositories(input.Repositories)
	if err != nil {
		return CreateRequest{}, err
	}
	policies, err := normalizeCreatePolicies(input.Policies)
	if err != nil {
		return CreateRequest{}, err
	}

	normalized := CreateRequest{
		ID:           projectID,
		Name:         projectName,
		Description:  strings.TrimSpace(input.Description),
		Namespace:    strings.TrimSpace(input.Namespace),
		Access:       normalizeAccessWithPlatformAdmins(input.Access, projectID, s.PlatformAdminAuthoritiesOrDefault()),
		Environments: applyEnvironmentDefaults(environments),
		Repositories: repositories,
		Policies:     policies,
	}
	if normalized.Namespace == "" {
		normalized.Namespace = normalized.Name
	}
	if normalized.Namespace != normalized.Name {
		return CreateRequest{}, ValidationError{
			Message: "namespace must match project name",
			Details: map[string]any{"field": "namespace", "name": normalized.Name, "namespace": normalized.Namespace},
		}
	}
	normalized.Policies = applyPolicyDefaults(normalized.Policies, normalized.Environments)

	if err := s.validateClusterTargets(ctx, normalized.Environments, normalized.Policies); err != nil {
		return CreateRequest{}, err
	}
	if err := validateCreatePolicyTargets(normalized.Policies, normalized.Environments); err != nil {
		return CreateRequest{}, err
	}

	return normalized, nil
}

func normalizeAccess(access Access, projectID string) Access {
	return normalizeAccessWithPlatformAdmins(access, projectID, nil)
}

func normalizeAccessWithPlatformAdmins(access Access, projectID string, platformAdminAuthorities []string) Access {
	viewerGroups := dedupeStrings(access.ViewerGroups)
	if len(viewerGroups) == 0 {
		viewerGroups = []string{fmt.Sprintf("aods:%s:view", projectID)}
	}

	deployerGroups := dedupeStrings(access.DeployerGroups)
	if len(deployerGroups) == 0 {
		deployerGroups = []string{fmt.Sprintf("aods:%s:deploy", projectID)}
	}

	platformAdmins := effectivePlatformAdminAuthorities(platformAdminAuthorities)
	adminGroups := dedupeStrings(access.AdminGroups)
	if len(adminGroups) == 0 {
		adminGroups = append([]string{fmt.Sprintf("aods:%s:admin", projectID)}, platformAdmins...)
	} else {
		for _, authority := range platformAdmins {
			if !containsString(adminGroups, authority) {
				adminGroups = append(adminGroups, authority)
			}
		}
	}

	return Access{
		ViewerGroups:   viewerGroups,
		DeployerGroups: deployerGroups,
		AdminGroups:    adminGroups,
	}
}

func (s Service) PlatformAdminAuthoritiesOrDefault() []string {
	return effectivePlatformAdminAuthorities(s.PlatformAdminAuthorities)
}

func normalizeCreateEnvironments(items []Environment) ([]Environment, error) {
	if len(items) == 0 {
		return nil, nil
	}

	seen := map[string]struct{}{}
	environments := make([]Environment, 0, len(items))
	for _, item := range items {
		environmentID := strings.TrimSpace(item.ID)
		if !slugPattern.MatchString(environmentID) {
			return nil, ValidationError{
				Message: "environment id must be a lowercase slug",
				Details: map[string]any{"field": "environments.id", "id": environmentID},
			}
		}
		if _, ok := seen[environmentID]; ok {
			return nil, ValidationError{
				Message: "environment ids must be unique",
				Details: map[string]any{"field": "environments.id", "id": environmentID},
			}
		}
		seen[environmentID] = struct{}{}

		writeMode := item.WriteMode
		switch writeMode {
		case "", WriteModeDirect, WriteModePullRequest:
		default:
			return nil, ValidationError{
				Message: "writeMode must be direct or pull_request",
				Details: map[string]any{"field": "environments.writeMode", "id": environmentID},
			}
		}

		clusterID := strings.TrimSpace(item.ClusterID)
		if clusterID != "" && !slugPattern.MatchString(clusterID) {
			return nil, ValidationError{
				Message: "clusterId must be a lowercase slug",
				Details: map[string]any{"field": "environments.clusterId", "id": environmentID},
			}
		}

		environments = append(environments, Environment{
			ID:        environmentID,
			Name:      strings.TrimSpace(item.Name),
			ClusterID: clusterID,
			WriteMode: writeMode,
			Default:   item.Default,
		})
	}

	return environments, nil
}

func normalizeCreateRepositories(items []Repository) ([]Repository, error) {
	repositories := make([]Repository, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		repositoryID := strings.TrimSpace(item.ID)
		if !slugPattern.MatchString(repositoryID) {
			return nil, ValidationError{
				Message: "repository id must be a lowercase slug",
				Details: map[string]any{"field": "repositories.id", "id": repositoryID},
			}
		}
		if _, ok := seen[repositoryID]; ok {
			return nil, ValidationError{
				Message: "repository ids must be unique",
				Details: map[string]any{"field": "repositories.id", "id": repositoryID},
			}
		}
		seen[repositoryID] = struct{}{}
		if strings.TrimSpace(item.Name) == "" {
			return nil, ValidationError{
				Message: "repository name is required",
				Details: map[string]any{"field": "repositories.name", "id": repositoryID},
			}
		}
		if strings.TrimSpace(item.URL) == "" {
			return nil, ValidationError{
				Message: "repository url is required",
				Details: map[string]any{"field": "repositories.url", "id": repositoryID},
			}
		}

		repositories = append(repositories, Repository{
			ID:             repositoryID,
			Name:           strings.TrimSpace(item.Name),
			URL:            strings.TrimSpace(item.URL),
			Description:    strings.TrimSpace(item.Description),
			Branch:         strings.TrimSpace(item.Branch),
			AuthSecretPath: strings.TrimSpace(item.AuthSecretPath),
			ConfigFile:     strings.TrimSpace(item.ConfigFile),
		})
	}
	return repositories, nil
}

func normalizeCreatePolicies(policy PolicySet) (PolicySet, error) {
	if policy.MinReplicas < 0 {
		return PolicySet{}, ValidationError{
			Message: "minReplicas must be zero or greater",
			Details: map[string]any{"field": "policies.minReplicas"},
		}
	}
	for _, strategy := range policy.AllowedDeploymentStrategies {
		switch NormalizeDeploymentStrategy(strategy) {
		case "Rollout", "Canary", "":
		default:
			return PolicySet{}, ValidationError{
				Message: "allowedDeploymentStrategies may only include Rollout or Canary",
				Details: map[string]any{"field": "policies.allowedDeploymentStrategies"},
			}
		}
	}

	return PolicySet{
		MinReplicas:                 policy.MinReplicas,
		AllowedEnvironments:         dedupeStrings(policy.AllowedEnvironments),
		AllowedDeploymentStrategies: normalizeAllowedStrategies(policy.AllowedDeploymentStrategies),
		AllowedClusterTargets:       dedupeStrings(policy.AllowedClusterTargets),
		ProdPRRequired:              policy.ProdPRRequired,
		AutoRollbackEnabled:         policy.AutoRollbackEnabled,
		RequiredProbes:              policy.RequiredProbes,
	}, nil
}

func validateCreatePolicyTargets(policy PolicySet, environments []Environment) error {
	environmentSet := map[string]struct{}{}
	for _, environment := range environments {
		environmentSet[environment.ID] = struct{}{}
	}

	for _, environmentID := range policy.AllowedEnvironments {
		if _, ok := environmentSet[environmentID]; !ok {
			return ValidationError{
				Message: "allowedEnvironments must reference configured environments",
				Details: map[string]any{"field": "policies.allowedEnvironments", "environment": environmentID},
			}
		}
	}

	return nil
}

func (s Service) validateClusterTargets(ctx context.Context, environments []Environment, policies PolicySet) error {
	available := map[string]struct{}{
		"default": {},
	}

	if s.Clusters != nil {
		clusters, err := s.Clusters.ListClusters(ctx)
		if err != nil {
			return err
		}
		for _, item := range clusters {
			available[item.ID] = struct{}{}
		}
	}

	for _, environment := range environments {
		clusterID := strings.TrimSpace(environment.ClusterID)
		if clusterID == "" {
			clusterID = "default"
		}
		if _, ok := available[clusterID]; !ok {
			return ValidationError{
				Message: "environment clusterId must reference an existing cluster",
				Details: map[string]any{"field": "environments.clusterId", "clusterId": clusterID, "environment": environment.ID},
			}
		}
	}

	for _, clusterID := range policies.AllowedClusterTargets {
		if _, ok := available[clusterID]; !ok {
			return ValidationError{
				Message: "allowedClusterTargets must reference existing clusters",
				Details: map[string]any{"field": "policies.allowedClusterTargets", "clusterId": clusterID},
			}
		}
	}

	return nil
}

func dedupeStrings(values []string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || slices.Contains(items, value) {
			continue
		}
		items = append(items, value)
	}
	return items
}

func NormalizeDeploymentStrategy(value string) string {
	switch strings.TrimSpace(value) {
	case "Standard", "Rollout":
		return "Rollout"
	case "Canary":
		return "Canary"
	default:
		return strings.TrimSpace(value)
	}
}

func isPlatformAdmin(user core.User, platformAdminAuthorities []string) bool {
	return hasAnyGroup(makeGroupSet(user.Groups), effectivePlatformAdminAuthorities(platformAdminAuthorities))
}

func effectivePlatformAdminAuthorities(values []string) []string {
	items := dedupeStrings(values)
	if len(items) == 0 {
		return []string{platformAdminGroup}
	}
	return items
}
