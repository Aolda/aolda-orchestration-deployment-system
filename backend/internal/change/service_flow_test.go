package change

import (
	"context"
	"errors"
	"testing"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/project"
)

type memoryStore struct {
	records map[string]Record
}

func (s *memoryStore) Create(_ context.Context, record Record) (Record, error) {
	if s.records == nil {
		s.records = map[string]Record{}
	}
	s.records[record.ID] = record
	return record, nil
}

func (s *memoryStore) Get(_ context.Context, changeID string) (Record, error) {
	record, ok := s.records[changeID]
	if !ok {
		return Record{}, ErrNotFound
	}
	return record, nil
}

func (s *memoryStore) Update(_ context.Context, record Record) (Record, error) {
	if s.records == nil {
		s.records = map[string]Record{}
	}
	s.records[record.ID] = record
	return record, nil
}

type projectCatalogStoreStub struct {
	items               []project.CatalogProject
	updatePoliciesErr   error
	lastUpdateProjectID string
	lastUpdatedPolicies project.PolicySet
}

func (s *projectCatalogStoreStub) ListProjects(context.Context) ([]project.CatalogProject, error) {
	return append([]project.CatalogProject(nil), s.items...), nil
}

func (s *projectCatalogStoreStub) CreateProject(context.Context, project.CreateRequest) (project.CatalogProject, error) {
	return project.CatalogProject{}, errors.New("not implemented")
}

func (s *projectCatalogStoreStub) DeleteProject(context.Context, string) (project.LifecycleResponse, error) {
	return project.LifecycleResponse{}, errors.New("not implemented")
}

func (s *projectCatalogStoreStub) UpdatePolicies(_ context.Context, projectID string, policies project.PolicySet) (project.CatalogProject, error) {
	s.lastUpdateProjectID = projectID
	s.lastUpdatedPolicies = policies
	if s.updatePoliciesErr != nil {
		return project.CatalogProject{}, s.updatePoliciesErr
	}

	for index, item := range s.items {
		if item.ID != projectID {
			continue
		}
		item.Policies = policies
		s.items[index] = item
		return item, nil
	}

	return project.CatalogProject{}, project.ErrNotFound
}

func TestServiceUpdatePoliciesChangeLifecyclePullRequest(t *testing.T) {
	t.Parallel()

	projectStore := &projectCatalogStoreStub{
		items: []project.CatalogProject{
			{
				ID:        "project-a",
				Name:      "Project A",
				Namespace: "project-a",
				Access: project.Access{
					DeployerGroups: []string{"aods:project-a:deploy"},
					AdminGroups:    []string{"aods:project-a:admin"},
				},
				Environments: []project.Environment{
					{
						ID:        "prod",
						Name:      "Production",
						ClusterID: "default",
						WriteMode: project.WriteModePullRequest,
						Default:   true,
					},
				},
				Policies: project.PolicySet{
					MinReplicas:                 1,
					AllowedEnvironments:         []string{"prod"},
					AllowedDeploymentStrategies: []string{"Rollout", "Canary"},
					AllowedClusterTargets:       []string{"default"},
					RequiredProbes:              true,
				},
			},
		},
	}

	service := Service{
		Projects: &project.Service{Source: projectStore},
		Store:    &memoryStore{},
	}

	createUser := core.User{Username: "deployer", Groups: []string{"aods:project-a:deploy"}}
	record, err := service.Create(context.Background(), createUser, "project-a", Request{
		Operation:   OperationUpdatePolicies,
		Environment: "prod",
		Policies: &project.PolicySummary{
			MinReplicas:                 3,
			AllowedEnvironments:         []string{"prod"},
			AllowedDeploymentStrategies: []string{"Rollout", "Canary"},
			AllowedClusterTargets:       []string{"default"},
			RequiredProbes:              true,
		},
	}, "req_123")
	if err != nil {
		t.Fatalf("create change: %v", err)
	}

	if record.ID != "chg_123" || record.Status != StatusDraft || record.WriteMode != project.WriteModePullRequest {
		t.Fatalf("unexpected created record: %#v", record)
	}
	if record.Summary != "프로젝트 정책 갱신" || len(record.DiffPreview) != 1 {
		t.Fatalf("expected generated summary and diff preview, got %#v", record)
	}

	record, err = service.Submit(context.Background(), createUser, record.ID)
	if err != nil {
		t.Fatalf("submit change: %v", err)
	}
	if record.Status != StatusSubmitted {
		t.Fatalf("expected submitted status, got %s", record.Status)
	}

	record, err = service.Approve(context.Background(), core.User{
		Username: "admin",
		Groups:   []string{"aods:project-a:admin"},
	}, record.ID)
	if err != nil {
		t.Fatalf("approve change: %v", err)
	}
	if record.Status != StatusApproved || record.ApprovedBy != "admin" {
		t.Fatalf("unexpected approved record: %#v", record)
	}

	record, err = service.Merge(context.Background(), core.User{
		Username: "admin",
		Groups:   []string{"aods:project-a:admin"},
	}, record.ID)
	if err != nil {
		t.Fatalf("merge change: %v", err)
	}
	if record.Status != StatusMerged || record.MergedBy != "admin" {
		t.Fatalf("unexpected merged record: %#v", record)
	}
	if projectStore.lastUpdateProjectID != "project-a" {
		t.Fatalf("expected project-a policy update, got %q", projectStore.lastUpdateProjectID)
	}
	if projectStore.lastUpdatedPolicies.MinReplicas != 3 {
		t.Fatalf("expected merged min replicas to be applied, got %#v", projectStore.lastUpdatedPolicies)
	}
}

func TestServiceChangeStateValidationForDirectAndPullRequestModes(t *testing.T) {
	t.Parallel()

	directStore := &projectCatalogStoreStub{
		items: []project.CatalogProject{
			{
				ID:        "project-a",
				Name:      "Project A",
				Namespace: "project-a",
				Access: project.Access{
					DeployerGroups: []string{"aods:project-a:deploy"},
					AdminGroups:    []string{"aods:project-a:admin"},
				},
				Environments: []project.Environment{
					{
						ID:        "dev",
						Name:      "Development",
						ClusterID: "default",
						WriteMode: project.WriteModeDirect,
						Default:   true,
					},
					{
						ID:        "prod",
						Name:      "Production",
						ClusterID: "default",
						WriteMode: project.WriteModePullRequest,
					},
				},
				Policies: project.PolicySet{
					MinReplicas:                 1,
					AllowedEnvironments:         []string{"dev", "prod"},
					AllowedDeploymentStrategies: []string{"Rollout", "Canary"},
					AllowedClusterTargets:       []string{"default"},
					RequiredProbes:              true,
				},
			},
		},
	}

	service := Service{
		Projects: &project.Service{Source: directStore},
		Store:    &memoryStore{},
	}

	deployer := core.User{Username: "deployer", Groups: []string{"aods:project-a:deploy"}}
	admin := core.User{Username: "admin", Groups: []string{"aods:project-a:admin"}}

	draft, err := service.Create(context.Background(), deployer, "project-a", Request{
		Operation:   OperationUpdatePolicies,
		Environment: "dev",
		Policies: &project.PolicySummary{
			AllowedEnvironments:         []string{"dev", "prod"},
			AllowedDeploymentStrategies: []string{"Rollout", "Canary"},
			AllowedClusterTargets:       []string{"default"},
			RequiredProbes:              true,
		},
	}, "req_direct")
	if err != nil {
		t.Fatalf("create direct change: %v", err)
	}

	if _, err := service.Approve(context.Background(), admin, draft.ID); err != ErrSubmissionRequired {
		t.Fatalf("expected ErrSubmissionRequired when approving draft, got %v", err)
	}

	submitted, err := service.Submit(context.Background(), deployer, draft.ID)
	if err != nil {
		t.Fatalf("submit direct change: %v", err)
	}
	if _, err := service.Submit(context.Background(), deployer, submitted.ID); err != ErrStateConflict {
		t.Fatalf("expected ErrStateConflict on resubmit, got %v", err)
	}

	merged, err := service.Merge(context.Background(), admin, submitted.ID)
	if err != nil {
		t.Fatalf("merge direct change: %v", err)
	}
	if merged.Status != StatusMerged {
		t.Fatalf("expected merged direct change, got %s", merged.Status)
	}

	prDraft, err := service.Create(context.Background(), deployer, "project-a", Request{
		Operation:   OperationUpdatePolicies,
		Environment: "prod",
		Policies: &project.PolicySummary{
			AllowedEnvironments:         []string{"dev", "prod"},
			AllowedDeploymentStrategies: []string{"Rollout", "Canary"},
			AllowedClusterTargets:       []string{"default"},
			RequiredProbes:              true,
		},
	}, "req_pr")
	if err != nil {
		t.Fatalf("create pr change: %v", err)
	}
	prSubmitted, err := service.Submit(context.Background(), deployer, prDraft.ID)
	if err != nil {
		t.Fatalf("submit pr change: %v", err)
	}
	if _, err := service.Merge(context.Background(), deployer, prSubmitted.ID); err != ErrApprovalRequired {
		t.Fatalf("expected ErrApprovalRequired for pull_request merge, got %v", err)
	}
}

func TestServiceCreateRejectsInvalidOperationAndPermissionFailures(t *testing.T) {
	t.Parallel()

	projectStore := &projectCatalogStoreStub{
		items: []project.CatalogProject{
			{
				ID:        "project-a",
				Name:      "Project A",
				Namespace: "project-a",
				Access: project.Access{
					ViewerGroups: []string{"aods:project-a:view"},
				},
				Environments: []project.Environment{
					{ID: "shared", Default: true, WriteMode: project.WriteModeDirect},
				},
				Policies: project.PolicySet{
					AllowedEnvironments:         []string{"shared"},
					AllowedDeploymentStrategies: []string{"Rollout"},
					AllowedClusterTargets:       []string{"default"},
					RequiredProbes:              true,
				},
			},
		},
	}

	service := Service{
		Projects: &project.Service{Source: projectStore},
		Store:    &memoryStore{},
	}

	if _, err := service.Create(context.Background(), core.User{Groups: []string{"aods:project-a:view"}}, "project-a", Request{
		Operation: OperationUpdatePolicies,
		Policies: &project.PolicySummary{
			AllowedEnvironments:         []string{"shared"},
			AllowedDeploymentStrategies: []string{"Rollout"},
			AllowedClusterTargets:       []string{"default"},
			RequiredProbes:              true,
		},
	}, "req_viewer"); err != application.ErrRequiresDeployer {
		t.Fatalf("expected deployer permission error, got %v", err)
	}

	projectStore.items[0].Access.DeployerGroups = []string{"aods:project-a:deploy"}
	if _, err := service.Create(context.Background(), core.User{Groups: []string{"aods:project-a:deploy"}}, "project-a", Request{
		Operation: Operation("DeleteApplication"),
	}, "req_invalid"); err != ErrInvalidOperation {
		t.Fatalf("expected invalid operation error, got %v", err)
	}
}
