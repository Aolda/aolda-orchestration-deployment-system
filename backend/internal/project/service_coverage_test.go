package project

import (
	"context"
	"errors"
	"testing"

	"github.com/aolda/aods-backend/internal/cluster"
	"github.com/aolda/aods-backend/internal/core"
)

type catalogStoreStub struct {
	items                []CatalogProject
	listErr              error
	createErr            error
	deleteErr            error
	updatePoliciesErr    error
	lastCreatedRequest   CreateRequest
	lastDeletedProjectID string
	lastUpdateProjectID  string
	lastUpdatedPolicies  PolicySet
}

func (s *catalogStoreStub) ListProjects(context.Context) ([]CatalogProject, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return append([]CatalogProject(nil), s.items...), nil
}

func (s *catalogStoreStub) CreateProject(_ context.Context, input CreateRequest) (CatalogProject, error) {
	s.lastCreatedRequest = input
	if s.createErr != nil {
		return CatalogProject{}, s.createErr
	}

	created := applyProjectDefaults(CatalogProject{
		ID:           input.ID,
		Name:         input.Name,
		Description:  input.Description,
		Namespace:    input.Namespace,
		Access:       input.Access,
		Environments: input.Environments,
		Repositories: input.Repositories,
		Policies:     input.Policies,
	})
	s.items = append(s.items, created)
	return created, nil
}

func (s *catalogStoreStub) DeleteProject(_ context.Context, projectID string) (LifecycleResponse, error) {
	s.lastDeletedProjectID = projectID
	if s.deleteErr != nil {
		return LifecycleResponse{}, s.deleteErr
	}

	for index, item := range s.items {
		if item.ID != projectID {
			continue
		}

		s.items = append(s.items[:index], s.items[index+1:]...)
		return LifecycleResponse{
			ProjectID: projectID,
			Name:      item.Name,
			Namespace: item.Namespace,
			Status:    "deleted",
			secretPaths: []string{
				"secret/aods/apps/" + projectID + "/api/prod",
			},
		}, nil
	}

	return LifecycleResponse{}, ErrNotFound
}

func (s *catalogStoreStub) UpdatePolicies(_ context.Context, projectID string, policies PolicySet) (CatalogProject, error) {
	s.lastUpdateProjectID = projectID
	s.lastUpdatedPolicies = policies
	if s.updatePoliciesErr != nil {
		return CatalogProject{}, s.updatePoliciesErr
	}

	for index, item := range s.items {
		if item.ID != projectID {
			continue
		}
		item.Policies = policies
		s.items[index] = item
		return item, nil
	}

	return CatalogProject{}, ErrNotFound
}

type clusterSourceStub struct {
	items []cluster.Summary
	err   error
}

func (s clusterSourceStub) ListClusters(context.Context) ([]cluster.Summary, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]cluster.Summary(nil), s.items...), nil
}

type secretStoreStub struct {
	paths []string
	err   error
}

func (s *secretStoreStub) Delete(_ context.Context, logicalPath string) error {
	if s.err != nil {
		return s.err
	}
	s.paths = append(s.paths, logicalPath)
	return nil
}

func TestServiceListAuthorizedAndGetAuthorized(t *testing.T) {
	t.Parallel()

	service := Service{
		Source: &catalogStoreStub{
			items: []CatalogProject{
				{
					ID:          "project-a",
					Name:        "Project A",
					Description: "Payments",
					Namespace:   "project-a",
					Access: Access{
						ViewerGroups:   []string{"aods:project-a:view"},
						DeployerGroups: []string{"aods:project-a:deploy"},
						AdminGroups:    []string{"aods:project-a:admin"},
					},
				},
				{
					ID:        "project-b",
					Name:      "Project B",
					Namespace: "project-b",
					Access: Access{
						ViewerGroups: []string{"aods:project-b:view"},
					},
				},
			},
		},
	}

	user := core.User{Groups: []string{"aods:project-a:deploy"}}

	items, err := service.ListAuthorized(context.Background(), user)
	if err != nil {
		t.Fatalf("list authorized projects: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected one authorized project, got %d", len(items))
	}
	if items[0].ID != "project-a" || items[0].Role != RoleDeployer {
		t.Fatalf("unexpected authorized project summary: %#v", items[0])
	}

	authorized, err := service.GetAuthorized(context.Background(), user, "project-a")
	if err != nil {
		t.Fatalf("get authorized project: %v", err)
	}
	if authorized.Role != RoleDeployer {
		t.Fatalf("expected deployer role, got %s", authorized.Role)
	}

	if _, err := service.GetAuthorized(context.Background(), user, "project-b"); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for inaccessible project, got %v", err)
	}
	if _, err := service.GetAuthorized(context.Background(), user, "missing"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing project, got %v", err)
	}
}

func TestServiceProjectDetailViewsMapEnvironmentRepositoryAndPolicyData(t *testing.T) {
	t.Parallel()

	service := Service{
		Source: &catalogStoreStub{
			items: []CatalogProject{
				{
					ID:        "project-a",
					Name:      "Project A",
					Namespace: "project-a",
					Access: Access{
						AdminGroups: []string{"aods:project-a:admin"},
					},
					Environments: []Environment{
						{
							ID:        "prod",
							Name:      "Production",
							ClusterID: "edge",
							WriteMode: WriteModePullRequest,
							Default:   true,
						},
					},
					Repositories: []Repository{
						{
							ID:             "payments-api",
							Name:           "Payments API",
							URL:            "https://github.com/aolda/payments",
							Description:    "main service",
							Branch:         "main",
							AuthSecretPath: "secret/aods/repos/payments",
							ConfigFile:     "deploy.yaml",
						},
						{
							ID:   "docs",
							Name: "Docs",
							URL:  "https://github.com/aolda/docs",
						},
					},
					Policies: PolicySet{
						MinReplicas:                 2,
						AllowedEnvironments:         []string{"prod"},
						AllowedDeploymentStrategies: []string{"Rollout", "Canary"},
						AllowedClusterTargets:       []string{"edge"},
						ProdPRRequired:              true,
						AutoRollbackEnabled:         true,
						RequiredProbes:              true,
					},
				},
			},
		},
	}

	user := core.User{Groups: []string{"aods:project-a:admin"}}

	environments, err := service.ListEnvironments(context.Background(), user, "project-a")
	if err != nil {
		t.Fatalf("list environments: %v", err)
	}
	if len(environments) != 1 || environments[0].ClusterID != "edge" || !environments[0].Default {
		t.Fatalf("unexpected environments: %#v", environments)
	}

	repositories, err := service.ListRepositories(context.Background(), user, "project-a")
	if err != nil {
		t.Fatalf("list repositories: %v", err)
	}
	if len(repositories) != 2 {
		t.Fatalf("expected two repositories, got %d", len(repositories))
	}
	if repositories[0].Access != "private" || repositories[1].Access != "public" {
		t.Fatalf("unexpected repository access mapping: %#v", repositories)
	}

	policies, err := service.GetPolicies(context.Background(), user, "project-a")
	if err != nil {
		t.Fatalf("get policies: %v", err)
	}
	if !policies.ProdPRRequired || !policies.AutoRollbackEnabled || policies.MinReplicas != 2 {
		t.Fatalf("unexpected policy summary: %#v", policies)
	}
}

func TestServiceNormalizeCreateRequestAppliesDefaultsAndValidatesTargets(t *testing.T) {
	t.Parallel()

	service := Service{
		Source:   &catalogStoreStub{},
		Clusters: clusterSourceStub{items: []cluster.Summary{{ID: "edge"}, {ID: "analytics"}}},
	}

	normalized, err := service.normalizeCreateRequest(context.Background(), CreateRequest{
		ID:          "project-a",
		Name:        "  project-a  ",
		Description: "  Payment platform  ",
		Environments: []Environment{
			{ID: "dev", ClusterID: "edge"},
			{ID: "prod", WriteMode: WriteModePullRequest, Default: true},
		},
		Repositories: []Repository{
			{
				ID:             " payments-api ",
				Name:           " Payments API ",
				URL:            " https://github.com/aolda/payments ",
				Description:    " main repo ",
				Branch:         " main ",
				AuthSecretPath: " secret/aods/repos/payments ",
				ConfigFile:     " deploy.yaml ",
			},
		},
		Policies: PolicySet{
			AllowedDeploymentStrategies: []string{"Standard", "Canary", "Canary"},
		},
	})
	if err != nil {
		t.Fatalf("normalize create request: %v", err)
	}

	if normalized.Namespace != "project-a" {
		t.Fatalf("expected namespace to default to project name, got %q", normalized.Namespace)
	}
	if normalized.Name != "project-a" || normalized.Description != "Payment platform" {
		t.Fatalf("expected trimmed name/description, got %#v", normalized)
	}
	if len(normalized.Access.ViewerGroups) != 1 || normalized.Access.ViewerGroups[0] != "aods:project-a:view" {
		t.Fatalf("unexpected default viewer groups: %#v", normalized.Access.ViewerGroups)
	}
	if len(normalized.Access.AdminGroups) != 2 || normalized.Access.AdminGroups[1] != platformAdminGroup {
		t.Fatalf("expected platform admin group to be appended, got %#v", normalized.Access.AdminGroups)
	}
	if len(normalized.Environments) != 2 {
		t.Fatalf("expected two normalized environments, got %d", len(normalized.Environments))
	}
	if normalized.Environments[0].Name != "dev" || normalized.Environments[0].WriteMode != WriteModeDirect {
		t.Fatalf("expected defaulted dev environment values, got %#v", normalized.Environments[0])
	}
	if normalized.Environments[1].ClusterID != "default" || !normalized.Environments[1].Default {
		t.Fatalf("expected prod environment defaults, got %#v", normalized.Environments[1])
	}
	if len(normalized.Repositories) != 1 || normalized.Repositories[0].ID != "payments-api" || normalized.Repositories[0].URL != "https://github.com/aolda/payments" {
		t.Fatalf("unexpected normalized repositories: %#v", normalized.Repositories)
	}
	if normalized.Policies.MinReplicas != 1 {
		t.Fatalf("expected minReplicas default of 1, got %d", normalized.Policies.MinReplicas)
	}
	if len(normalized.Policies.AllowedEnvironments) != 2 || normalized.Policies.AllowedEnvironments[0] != "dev" || normalized.Policies.AllowedEnvironments[1] != "prod" {
		t.Fatalf("unexpected allowed environments: %#v", normalized.Policies.AllowedEnvironments)
	}
	if len(normalized.Policies.AllowedDeploymentStrategies) != 2 || normalized.Policies.AllowedDeploymentStrategies[0] != "Rollout" || normalized.Policies.AllowedDeploymentStrategies[1] != "Canary" {
		t.Fatalf("unexpected normalized strategies: %#v", normalized.Policies.AllowedDeploymentStrategies)
	}
	if len(normalized.Policies.AllowedClusterTargets) != 2 || normalized.Policies.AllowedClusterTargets[0] != "edge" || normalized.Policies.AllowedClusterTargets[1] != "default" {
		t.Fatalf("unexpected allowed cluster targets: %#v", normalized.Policies.AllowedClusterTargets)
	}
	if normalized.Policies.RequiredProbes {
		t.Fatal("expected explicit policy customization to keep required probes disabled")
	}
}

func TestServiceNormalizeCreateRequestRejectsUnknownClustersAndPolicyTargets(t *testing.T) {
	t.Parallel()

	service := Service{
		Source:   &catalogStoreStub{},
		Clusters: clusterSourceStub{items: []cluster.Summary{{ID: "edge"}}},
	}

	_, err := service.normalizeCreateRequest(context.Background(), CreateRequest{
		ID:   "project-a",
		Name: "project-a",
		Environments: []Environment{
			{ID: "dev", ClusterID: "missing"},
		},
	})
	if err == nil {
		t.Fatal("expected missing cluster validation error")
	}
	validationErr, ok := err.(ValidationError)
	if !ok || validationErr.Details["field"] != "environments.clusterId" {
		t.Fatalf("expected environment cluster validation error, got %T %#v", err, err)
	}

	_, err = service.normalizeCreateRequest(context.Background(), CreateRequest{
		ID:   "project-a",
		Name: "project-a",
		Environments: []Environment{
			{ID: "dev", ClusterID: "edge"},
		},
		Policies: PolicySet{
			AllowedEnvironments: []string{"prod"},
		},
	})
	if err == nil {
		t.Fatal("expected policy target validation error")
	}
	validationErr, ok = err.(ValidationError)
	if !ok || validationErr.Details["field"] != "policies.allowedEnvironments" {
		t.Fatalf("expected allowed environment validation error, got %T %#v", err, err)
	}
}

func TestServiceNormalizeCreateRequestRejectsNonSlugProjectNameAndMismatchedNamespace(t *testing.T) {
	t.Parallel()

	service := Service{}

	_, err := service.normalizeCreateRequest(context.Background(), CreateRequest{
		ID:   "project-a",
		Name: "Project A",
	})
	if err == nil {
		t.Fatal("expected non-slug project name validation error")
	}
	validationErr, ok := err.(ValidationError)
	if !ok || validationErr.Message != "project name must be a lowercase slug" {
		t.Fatalf("expected project name slug validation error, got %T %#v", err, err)
	}

	_, err = service.normalizeCreateRequest(context.Background(), CreateRequest{
		ID:        "project-a",
		Name:      "project-a",
		Namespace: "team-a",
	})
	if err == nil {
		t.Fatal("expected mismatched namespace validation error")
	}
	validationErr, ok = err.(ValidationError)
	if !ok || validationErr.Message != "namespace must match project name" {
		t.Fatalf("expected namespace match validation error, got %T %#v", err, err)
	}
}

func TestServiceCreateAndUpdatePoliciesUseCatalogStore(t *testing.T) {
	t.Parallel()

	store := &catalogStoreStub{
		items: []CatalogProject{
			applyProjectDefaults(CatalogProject{
				ID:        "project-a",
				Name:      "Project A",
				Namespace: "project-a",
				Access: Access{
					AdminGroups: []string{"aods:project-a:admin", platformAdminGroup},
				},
				Environments: []Environment{
					{ID: "dev", ClusterID: "edge", Default: true},
					{ID: "prod", ClusterID: "default", WriteMode: WriteModePullRequest},
				},
				Policies: PolicySet{
					AllowedEnvironments:         []string{"dev", "prod"},
					AllowedDeploymentStrategies: []string{"Rollout", "Canary"},
					AllowedClusterTargets:       []string{"edge", "default"},
					RequiredProbes:              true,
				},
			}),
		},
	}
	service := Service{
		Source:   store,
		Clusters: clusterSourceStub{items: []cluster.Summary{{ID: "edge"}}},
	}

	admin := core.User{Groups: []string{platformAdminGroup}}

	created, err := service.Create(context.Background(), admin, CreateRequest{
		ID:   "project-b",
		Name: "project-b",
		Environments: []Environment{
			{ID: "dev", ClusterID: "edge", Default: true},
		},
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if created.Role != RoleAdmin {
		t.Fatalf("expected created summary role admin, got %s", created.Role)
	}
	if store.lastCreatedRequest.Access.ViewerGroups[0] != "aods:project-b:view" {
		t.Fatalf("expected create path to normalize access, got %#v", store.lastCreatedRequest.Access)
	}

	summary, err := service.UpdatePolicies(context.Background(), core.User{Groups: []string{"aods:project-a:admin"}}, "project-a", PolicySet{
		AllowedDeploymentStrategies: []string{"Standard", "Canary"},
	})
	if err != nil {
		t.Fatalf("update policies: %v", err)
	}
	if store.lastUpdateProjectID != "project-a" {
		t.Fatalf("expected project-a update call, got %q", store.lastUpdateProjectID)
	}
	if len(store.lastUpdatedPolicies.AllowedClusterTargets) != 2 {
		t.Fatalf("expected default cluster targets to be preserved, got %#v", store.lastUpdatedPolicies.AllowedClusterTargets)
	}
	if len(summary.AllowedDeploymentStrategies) != 2 || summary.AllowedDeploymentStrategies[0] != "Rollout" {
		t.Fatalf("expected normalized policy summary, got %#v", summary)
	}

	if _, err := service.Create(context.Background(), core.User{Groups: []string{"aods:project-a:deploy"}}, CreateRequest{ID: "x", Name: "X"}); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for non-admin create, got %v", err)
	}
	if _, err := service.UpdatePolicies(context.Background(), core.User{Groups: []string{"aods:project-a:view"}}, "project-a", PolicySet{}); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for non-admin policy update, got %v", err)
	}
}

func TestServicePlatformAdminAuthorityOverrideAppliesAcrossProjectActions(t *testing.T) {
	t.Parallel()

	const keycloakAdminRole = "aods:platform:admin"

	store := &catalogStoreStub{
		items: []CatalogProject{
			applyProjectDefaults(CatalogProject{
				ID:        "project-a",
				Name:      "Project A",
				Namespace: "project-a",
				Access: Access{
					ViewerGroups:   []string{"/Ajou_Univ/Aolda_Member/project-a/view"},
					DeployerGroups: []string{"/Ajou_Univ/Aolda_Member/project-a/ops"},
					AdminGroups:    []string{"/Ajou_Univ/Aolda_Member/project-a/admin"},
				},
			}),
		},
	}
	service := Service{
		Source:                   store,
		Clusters:                 clusterSourceStub{items: []cluster.Summary{{ID: "default"}}},
		PlatformAdminAuthorities: []string{keycloakAdminRole},
	}
	admin := core.User{Groups: []string{keycloakAdminRole}}

	items, err := service.ListAuthorized(context.Background(), admin)
	if err != nil {
		t.Fatalf("list authorized projects: %v", err)
	}
	if len(items) != 1 || items[0].Role != RoleAdmin {
		t.Fatalf("expected configured platform admin to see project as admin, got %#v", items)
	}

	authorized, err := service.GetAuthorized(context.Background(), admin, "project-a")
	if err != nil {
		t.Fatalf("get authorized project: %v", err)
	}
	if authorized.Role != RoleAdmin {
		t.Fatalf("expected platform admin override to resolve admin role, got %s", authorized.Role)
	}

	created, err := service.Create(context.Background(), admin, CreateRequest{
		ID:   "project-b",
		Name: "project-b",
	})
	if err != nil {
		t.Fatalf("create project with configured platform admin: %v", err)
	}
	if created.Role != RoleAdmin {
		t.Fatalf("expected created project role admin, got %s", created.Role)
	}
	if !containsString(store.lastCreatedRequest.Access.AdminGroups, keycloakAdminRole) {
		t.Fatalf("expected normalized admin groups to contain configured platform admin, got %#v", store.lastCreatedRequest.Access.AdminGroups)
	}
}

func TestServiceDeleteProjectUsesCatalogStoreAndProtectsSharedNamespace(t *testing.T) {
	t.Parallel()

	store := &catalogStoreStub{
		items: []CatalogProject{
			applyProjectDefaults(CatalogProject{
				ID:        "project-a",
				Name:      "Project A",
				Namespace: "project-a",
				Access: Access{
					AdminGroups: []string{platformAdminGroup},
				},
			}),
			applyProjectDefaults(CatalogProject{
				ID:        "shared-project",
				Name:      "Shared",
				Namespace: "shared",
				Access: Access{
					AdminGroups: []string{platformAdminGroup},
				},
			}),
		},
	}
	secrets := &secretStoreStub{}
	service := Service{
		Source:   store,
		Secrets:  secrets,
		Clusters: clusterSourceStub{},
	}

	deleted, err := service.Delete(context.Background(), core.User{Groups: []string{platformAdminGroup}}, "project-a")
	if err != nil {
		t.Fatalf("delete project: %v", err)
	}
	if deleted.ProjectID != "project-a" || deleted.Status != "deleted" {
		t.Fatalf("unexpected delete response: %#v", deleted)
	}
	if store.lastDeletedProjectID != "project-a" {
		t.Fatalf("expected delete call for project-a, got %q", store.lastDeletedProjectID)
	}
	if len(secrets.paths) != 1 || secrets.paths[0] != "secret/aods/apps/project-a/api/prod" {
		t.Fatalf("expected project secrets to be deleted, got %#v", secrets.paths)
	}

	if _, err := service.Delete(context.Background(), core.User{Groups: []string{platformAdminGroup}}, "shared-project"); err == nil {
		t.Fatal("expected shared namespace project deletion to be blocked")
	} else {
		var protectedErr ProtectedProjectError
		if !errors.As(err, &protectedErr) {
			t.Fatalf("expected ProtectedProjectError, got %T %#v", err, err)
		}
	}

	if _, err := service.Delete(context.Background(), core.User{Groups: []string{"aods:project-a:admin"}}, "project-a"); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for non-platform-admin delete, got %v", err)
	}
}

func TestProjectHelpersCoverNormalizationAndRoleUtilities(t *testing.T) {
	t.Parallel()

	groupSet := makeGroupSet([]string{"admin", "deploy", "view"})
	if role, ok := resolveRole(groupSet, Access{
		ViewerGroups:   []string{"view"},
		DeployerGroups: []string{"deploy"},
		AdminGroups:    []string{"admin"},
	}); !ok || role != RoleAdmin {
		t.Fatalf("expected admin role to win, got %s %v", role, ok)
	}
	if !RoleAdmin.CanAdmin() || !RoleAdmin.CanDeploy() || RoleViewer.CanDeploy() {
		t.Fatal("unexpected role capability mapping")
	}
	if !hasAnyGroup(groupSet, []string{"missing", "deploy"}) {
		t.Fatal("expected deploy group to be found")
	}

	if got := dedupeStrings([]string{" dev ", "", "dev", "prod"}); len(got) != 2 || got[0] != "dev" || got[1] != "prod" {
		t.Fatalf("unexpected deduped strings: %#v", got)
	}
	if NormalizeDeploymentStrategy("Standard") != "Rollout" || NormalizeDeploymentStrategy(" Canary ") != "Canary" {
		t.Fatal("expected deployment strategy normalization to map Standard and trim Canary")
	}
	if repositoryAccess(Repository{AuthSecretPath: "secret/path"}) != "private" || repositoryAccess(Repository{}) != "public" {
		t.Fatal("unexpected repository access mapping")
	}
	if !containsString([]string{"dev", "prod"}, "prod") {
		t.Fatal("expected containsString to find prod")
	}
	if !shouldDefaultRequiredProbes(PolicySet{}) || shouldDefaultRequiredProbes(PolicySet{RequiredProbes: true}) {
		t.Fatal("unexpected required probes default detection")
	}
	if got := normalizeAllowedStrategies([]string{"Standard", "Rollout", "Canary", "Canary"}); len(got) != 2 || got[0] != "Rollout" || got[1] != "Canary" {
		t.Fatalf("unexpected normalized allowed strategies: %#v", got)
	}
	if got := normalizeAccess(Access{AdminGroups: []string{"project-admin"}}, "project-z"); len(got.AdminGroups) != 2 || got.AdminGroups[1] != platformAdminGroup {
		t.Fatalf("expected platform admin to be appended, got %#v", got.AdminGroups)
	}
	if got := normalizeAccessWithPlatformAdmins(Access{AdminGroups: []string{"project-admin"}}, "project-z", []string{"aods:platform:admin"}); len(got.AdminGroups) != 2 || got.AdminGroups[1] != "aods:platform:admin" {
		t.Fatalf("expected configured platform admin to be appended, got %#v", got.AdminGroups)
	}
	if role, ok := resolveRoleWithPlatformAdmins(makeGroupSet([]string{"aods:platform:admin"}), Access{}, []string{"aods:platform:admin"}); !ok || role != RoleAdmin {
		t.Fatalf("expected configured platform admin override to resolve admin, got %s %v", role, ok)
	}
}

func TestNormalizeCreateRepositoriesAndPoliciesValidation(t *testing.T) {
	t.Parallel()

	if _, err := normalizeCreateRepositories([]Repository{{ID: "bad id", Name: "Repo", URL: "https://example.com"}}); err == nil {
		t.Fatal("expected invalid repository id to fail")
	}
	if _, err := normalizeCreateRepositories([]Repository{{ID: "repo", URL: "https://example.com"}}); err == nil {
		t.Fatal("expected missing repository name to fail")
	}
	if _, err := normalizeCreateEnvironments([]Environment{{ID: "dev", WriteMode: WriteMode("invalid")}}); err == nil {
		t.Fatal("expected invalid write mode to fail")
	}
	if _, err := normalizeCreateEnvironments([]Environment{{ID: "dev"}, {ID: "dev"}}); err == nil {
		t.Fatal("expected duplicate environment id to fail")
	}
	if _, err := normalizeCreatePolicies(PolicySet{MinReplicas: -1}); err == nil {
		t.Fatal("expected negative replicas to fail")
	}
	if _, err := normalizeCreatePolicies(PolicySet{AllowedDeploymentStrategies: []string{"BlueGreen"}}); err == nil {
		t.Fatal("expected invalid deployment strategy to fail")
	}
}

func TestServicePropagatesSourceErrors(t *testing.T) {
	t.Parallel()

	expected := errors.New("catalog unavailable")
	service := Service{Source: &catalogStoreStub{listErr: expected}}

	if _, err := service.ListAuthorized(context.Background(), core.User{}); !errors.Is(err, expected) {
		t.Fatalf("expected source error from ListAuthorized, got %v", err)
	}
	if _, err := service.GetAuthorized(context.Background(), core.User{}, "project-a"); !errors.Is(err, expected) {
		t.Fatalf("expected source error from GetAuthorized, got %v", err)
	}
}
