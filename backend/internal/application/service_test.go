package application

import (
	"context"
	"errors"
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
	for _, record := range s.records {
		if record.ID == applicationID {
			return record, nil
		}
	}
	return Record{}, nil
}

func (s stubStore) CreateApplication(ctx context.Context, project ProjectContext, input CreateRequest, secretPath string) (Record, error) {
	return Record{}, nil
}

func (s stubStore) UpdateApplicationImage(ctx context.Context, project ProjectContext, applicationID string, imageTag string, deploymentID string) (Record, error) {
	return Record{}, nil
}

func (s stubStore) PatchApplication(ctx context.Context, project ProjectContext, applicationID string, input UpdateApplicationRequest) (Record, error) {
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

type createSpyStore struct {
	stubStore
	createErr    error
	createRecord Record
	createCalls  int
	secretPaths  []string
}

func (s *createSpyStore) CreateApplication(ctx context.Context, project ProjectContext, input CreateRequest, secretPath string) (Record, error) {
	s.createCalls++
	s.secretPaths = append(s.secretPaths, secretPath)
	if s.createErr != nil {
		return Record{}, s.createErr
	}
	return s.createRecord, nil
}

type secretsSpy struct {
	stageCalls    int
	finalizeCalls int
	staged        StagedSecret
	finalizedPath string
}

func (s *secretsSpy) Stage(ctx context.Context, requestID string, projectID string, appName string, createdBy string, data map[string]string) (StagedSecret, error) {
	s.stageCalls++
	if s.staged == (StagedSecret{}) {
		s.staged = StagedSecret{
			StagingPath: "secret/aods/staging/" + requestID,
			FinalPath:   "secret/aods/apps/" + projectID + "/" + appName + "/prod",
		}
	}
	return s.staged, nil
}

func (s *secretsSpy) Finalize(ctx context.Context, staged StagedSecret, data map[string]string) error {
	s.finalizeCalls++
	s.finalizedPath = staged.FinalPath
	return nil
}

func (s *secretsSpy) Get(ctx context.Context, logicalPath string) (map[string]string, error) {
	return nil, nil
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

func TestCreateApplicationDoesNotFinalizeSecretsWhenStoreWriteFails(t *testing.T) {
	t.Parallel()

	store := &createSpyStore{
		createErr: errors.New("manifest write failed"),
	}
	secrets := &secretsSpy{}
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
						Environments: []project.Environment{
							{
								ID:        "prod",
								Name:      "Production",
								ClusterID: "default",
								WriteMode: project.WriteModeDirect,
								Default:   true,
							},
						},
						Policies: project.PolicySet{
							MinReplicas:                 1,
							AllowedEnvironments:         []string{"prod"},
							AllowedDeploymentStrategies: []string{"Standard"},
							AllowedClusterTargets:       []string{"default"},
							RequiredProbes:              true,
						},
					},
				},
			},
		},
		Store:        store,
		Secrets:      secrets,
		StatusReader: &batchStatusReaderStub{},
	}

	_, err := service.CreateApplication(context.Background(), core.User{
		Username: "deployer",
		Groups:   []string{"aods:project-a:deploy"},
	}, "project-a", CreateRequest{
		Name:               "secret-app",
		Image:              "repo/secret-app:v1",
		ServicePort:        8080,
		DeploymentStrategy: DeploymentStrategyStandard,
		Environment:        "prod",
		Secrets: []SecretEntry{
			{Key: "DATABASE_URL", Value: "postgres://db"},
		},
	}, "req_1")
	if err == nil {
		t.Fatal("expected create application to fail when manifest store write fails")
	}

	if secrets.stageCalls != 1 {
		t.Fatalf("expected secrets to be staged once, got %d", secrets.stageCalls)
	}
	if secrets.finalizeCalls != 0 {
		t.Fatalf("expected secrets not to be finalized on store failure, got %d", secrets.finalizeCalls)
	}
	if store.createCalls != 1 {
		t.Fatalf("expected manifest store create to be attempted once, got %d", store.createCalls)
	}
	if len(store.secretPaths) != 1 || store.secretPaths[0] != "secret/aods/apps/project-a/secret-app/prod" {
		t.Fatalf("expected final vault path to be passed to manifest store, got %#v", store.secretPaths)
	}
}

func TestPatchApplicationRejectsZeroReplicas(t *testing.T) {
	t.Parallel()

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
						Environments: []project.Environment{
							{
								ID:        "prod",
								Name:      "Production",
								ClusterID: "default",
								WriteMode: project.WriteModeDirect,
								Default:   true,
							},
						},
						Policies: project.PolicySet{
							MinReplicas:                 1,
							AllowedEnvironments:         []string{"prod"},
							AllowedDeploymentStrategies: []string{"Standard"},
							AllowedClusterTargets:       []string{"default"},
							RequiredProbes:              true,
						},
					},
				},
			},
		},
		Store: stubStore{
			records: []Record{
				{
					ID:                 "project-a__demo",
					ProjectID:          "project-a",
					Name:               "demo",
					Image:              "ghcr.io/aolda/demo:v1",
					ServicePort:        8080,
					Replicas:           1,
					DeploymentStrategy: DeploymentStrategyStandard,
					DefaultEnvironment: "prod",
				},
			},
		},
		StatusReader: &batchStatusReaderStub{},
	}

	zero := 0
	_, err := service.PatchApplication(context.Background(), core.User{
		Groups: []string{"aods:project-a:deploy"},
	}, "project-a__demo", UpdateApplicationRequest{
		Replicas: &zero,
	})
	if err == nil {
		t.Fatal("expected zero replicas to be rejected")
	}
}
