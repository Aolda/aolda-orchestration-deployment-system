package application

import (
	"context"
	"testing"

	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/project"
)

type stubStore struct {
	records []Record
}

func (s stubStore) ListApplications(ctx context.Context, projectID string) ([]Record, error) {
	return s.records, nil
}

func (s stubStore) GetApplication(ctx context.Context, applicationID string) (Record, error) {
	return Record{}, nil
}

func (s stubStore) CreateApplication(ctx context.Context, project ProjectContext, input CreateRequest, secretPath string) (Record, error) {
	return Record{}, nil
}

func (s stubStore) UpdateApplicationImage(ctx context.Context, applicationID string, imageTag string, deploymentID string) (Record, error) {
	return Record{}, nil
}

func (s stubStore) PatchApplication(ctx context.Context, applicationID string, input UpdateApplicationRequest) (Record, error) {
	return Record{}, nil
}

func (s stubStore) ListDeployments(ctx context.Context, applicationID string) ([]DeploymentRecord, error) {
	return nil, nil
}

func (s stubStore) GetDeployment(ctx context.Context, applicationID string, deploymentID string) (DeploymentRecord, error) {
	return DeploymentRecord{}, nil
}

func (s stubStore) UpdateDeployment(ctx context.Context, applicationID string, deployment DeploymentRecord) (DeploymentRecord, error) {
	return deployment, nil
}

func (s stubStore) GetRollbackPolicy(ctx context.Context, applicationID string) (RollbackPolicy, error) {
	return RollbackPolicy{}, nil
}

func (s stubStore) SaveRollbackPolicy(ctx context.Context, applicationID string, policy RollbackPolicy) (RollbackPolicy, error) {
	return policy, nil
}

func (s stubStore) ListEvents(ctx context.Context, applicationID string) ([]Event, error) {
	return nil, nil
}

func (s stubStore) AppendEvent(ctx context.Context, applicationID string, event Event) error {
	return nil
}

type batchStatusReaderStub struct {
	readCalls     int
	readManyCalls int
}

func (s *batchStatusReaderStub) Read(ctx context.Context, record Record) (SyncInfo, error) {
	s.readCalls++
	return SyncInfo{Status: SyncStatusSynced}, nil
}

func (s *batchStatusReaderStub) ReadMany(ctx context.Context, records []Record) (map[string]SyncInfo, error) {
	s.readManyCalls++
	items := make(map[string]SyncInfo, len(records))
	for _, record := range records {
		items[record.ID] = SyncInfo{Status: SyncStatusSynced}
	}
	return items, nil
}

type staticCatalogSource struct {
	items []project.CatalogProject
}

func (s staticCatalogSource) ListProjects(ctx context.Context) ([]project.CatalogProject, error) {
	return s.items, nil
}

func TestServiceListApplicationsUsesBatchStatusReaderWhenAvailable(t *testing.T) {
	t.Parallel()

	reader := &batchStatusReaderStub{}
	service := Service{
		Projects: &project.Service{
			Source: staticCatalogSource{
				items: []project.CatalogProject{
					{
						ID:        "project-a",
						Name:      "Project A",
						Namespace: "project-a",
						Access: project.Access{
							DeployerGroups: []string{"aods:project-a:deploy"},
						},
					},
				},
			},
		},
		Store: stubStore{
			records: []Record{
				{ID: "project-a__one", ProjectID: "project-a", Name: "one"},
				{ID: "project-a__two", ProjectID: "project-a", Name: "two"},
			},
		},
		StatusReader: reader,
	}

	items, err := service.ListApplications(context.Background(), core.User{
		Groups: []string{"aods:project-a:deploy"},
	}, "project-a")
	if err != nil {
		t.Fatalf("list applications: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if reader.readManyCalls != 1 {
		t.Fatalf("expected batch reader to be used once, got %d", reader.readManyCalls)
	}
	if reader.readCalls != 0 {
		t.Fatalf("expected single-item Read not to be used, got %d", reader.readCalls)
	}
}
