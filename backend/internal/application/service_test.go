package application

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

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

func (s stubStore) ArchiveApplication(ctx context.Context, applicationID string, archivedBy string) (ApplicationLifecycleResponse, error) {
	return ApplicationLifecycleResponse{}, nil
}

func (s stubStore) DeleteApplication(ctx context.Context, applicationID string) (ApplicationLifecycleResponse, error) {
	return ApplicationLifecycleResponse{}, nil
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

type patchSpyStore struct {
	stubStore
	lastPatch UpdateApplicationRequest
}

func (s *patchSpyStore) PatchApplication(ctx context.Context, project ProjectContext, applicationID string, input UpdateApplicationRequest) (Record, error) {
	s.lastPatch = input
	record, err := s.GetApplication(ctx, applicationID)
	if err != nil {
		return Record{}, err
	}
	if input.RepositoryPollIntervalSeconds != nil {
		record.RepositoryPollIntervalSeconds = *input.RepositoryPollIntervalSeconds
	}
	record.UpdatedAt = time.Now().UTC()
	return record, nil
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

type staticStatusReader struct {
	info SyncInfo
}

func (s staticStatusReader) Read(ctx context.Context, record Record) (SyncInfo, error) {
	return s.info, nil
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
	createInputs []CreateRequest
	secretPaths  []string
}

func (s *createSpyStore) CreateApplication(ctx context.Context, project ProjectContext, input CreateRequest, secretPath string) (Record, error) {
	s.createCalls++
	s.createInputs = append(s.createInputs, input)
	s.secretPaths = append(s.secretPaths, secretPath)
	if s.createErr != nil {
		return Record{}, s.createErr
	}
	return s.createRecord, nil
}

type secretsSpy struct {
	stageCalls    int
	stageAtCalls  int
	finalizeCalls int
	staged        StagedSecret
	stagedPaths   []string
	finalizedPath []string
}

func (s *secretsSpy) Stage(ctx context.Context, requestID string, projectID string, appName string, createdBy string, data map[string]string) (StagedSecret, error) {
	s.stageCalls++
	if s.staged == (StagedSecret{}) {
		s.staged = StagedSecret{
			StagingPath: "secret/aods/staging/" + requestID,
			FinalPath:   "secret/aods/apps/" + projectID + "/" + appName + "/prod",
		}
	}
	s.stagedPaths = append(s.stagedPaths, s.staged.FinalPath)
	return s.staged, nil
}

func (s *secretsSpy) StageAt(ctx context.Context, requestID string, finalPath string, metadata map[string]string, data map[string]string) (StagedSecret, error) {
	s.stageAtCalls++
	staged := StagedSecret{
		StagingPath: "secret/aods/staging/" + requestID,
		FinalPath:   finalPath,
	}
	s.stagedPaths = append(s.stagedPaths, staged.FinalPath)
	return staged, nil
}

func (s *secretsSpy) Finalize(ctx context.Context, staged StagedSecret, data map[string]string) error {
	s.finalizeCalls++
	s.finalizedPath = append(s.finalizedPath, staged.FinalPath)
	return nil
}

func (s *secretsSpy) Get(ctx context.Context, logicalPath string) (map[string]string, error) {
	return nil, nil
}

func (s *secretsSpy) Delete(ctx context.Context, logicalPath string) error {
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
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

func TestGetSyncStatusIncludesRepositoryPollMetadata(t *testing.T) {
	t.Parallel()

	record := Record{
		ID:            "project-a__demo",
		ProjectID:     "project-a",
		Name:          "demo",
		RepositoryURL: "https://github.com/Aolda/demo.git",
	}
	tracker := NewRepositoryPollTracker(5 * time.Minute)
	checkedAt := time.Date(2026, 4, 18, 5, 0, 0, 0, time.UTC)
	tracker.MarkSuccess(record, project.Repository{Name: "demo"}, checkedAt)

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
			records: []Record{record},
		},
		StatusReader: staticStatusReader{
			info: SyncInfo{
				Status:     SyncStatusSynced,
				Message:    "ReconciliationSucceeded",
				ObservedAt: checkedAt,
			},
		},
		PollTracker: tracker,
	}

	response, err := service.GetSyncStatus(context.Background(), core.User{
		Groups: []string{"aods:project-a:deploy"},
	}, record.ID)
	if err != nil {
		t.Fatalf("get sync status: %v", err)
	}

	if response.RepositoryPoll == nil {
		t.Fatal("expected repository poll metadata in sync status response")
	}
	if response.RepositoryPoll.LastResult != RepositoryPollResultSuccess {
		t.Fatalf("expected success poll result, got %s", response.RepositoryPoll.LastResult)
	}
	if response.RepositoryPoll.LastCheckedAt == nil || !response.RepositoryPoll.LastCheckedAt.Equal(checkedAt) {
		t.Fatalf("expected last checked at %s, got %v", checkedAt, response.RepositoryPoll.LastCheckedAt)
	}
	if response.RepositoryPoll.IntervalSeconds != 300 {
		t.Fatalf("expected intervalSeconds=300, got %d", response.RepositoryPoll.IntervalSeconds)
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

func TestCreateApplicationNormalizesRepositoryMetadataFromConnectedProjectRepository(t *testing.T) {
	t.Parallel()

	store := &createSpyStore{
		createRecord: Record{
			ID:                 "project-a__repo-app",
			ProjectID:          "project-a",
			Name:               "repo-app",
			Image:              "ghcr.io/aolda/repo-app:v1",
			ServicePort:        8080,
			Replicas:           1,
			DeploymentStrategy: DeploymentStrategyRollout,
			DefaultEnvironment: "prod",
		},
	}
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
						Repositories: []project.Repository{
							{
								ID:         "payment-api",
								Name:       "Payment API",
								URL:        "https://github.com/aolda/payment-api",
								ConfigFile: "deploy/aolda_deploy.json",
							},
						},
						Policies: project.PolicySet{
							MinReplicas:                 1,
							AllowedEnvironments:         []string{"prod"},
							AllowedDeploymentStrategies: []string{"Rollout"},
							AllowedClusterTargets:       []string{"default"},
							RequiredProbes:              true,
						},
					},
				},
			},
		},
		Store:        store,
		StatusReader: &batchStatusReaderStub{},
	}

	created, err := service.CreateApplication(context.Background(), core.User{
		Username: "deployer",
		Groups:   []string{"aods:project-a:deploy"},
	}, "project-a", CreateRequest{
		Name:                "repo-app",
		Image:               "ghcr.io/aolda/repo-app:v1",
		ServicePort:         8080,
		DeploymentStrategy:  DeploymentStrategyRollout,
		Environment:         "prod",
		RepositoryID:        "payment-api",
		RepositoryServiceID: "api",
	}, "req_2")
	if err != nil {
		t.Fatalf("create application with repository metadata: %v", err)
	}

	if store.createCalls != 1 {
		t.Fatalf("expected manifest store create to be called once, got %d", store.createCalls)
	}
	if len(store.createInputs) != 1 {
		t.Fatalf("expected one create input, got %d", len(store.createInputs))
	}
	if store.createInputs[0].ConfigPath != "deploy/aolda_deploy.json" {
		t.Fatalf("expected repository config path to be defaulted, got %q", store.createInputs[0].ConfigPath)
	}
	if created.RepositoryID != "payment-api" {
		t.Fatalf("expected application repository id to be preserved, got %q", created.RepositoryID)
	}
	if created.RepositoryServiceID != "api" {
		t.Fatalf("expected application repository service id to be preserved, got %q", created.RepositoryServiceID)
	}
	if created.ConfigPath != "deploy/aolda_deploy.json" {
		t.Fatalf("expected application config path to be returned, got %q", created.ConfigPath)
	}
}

func TestCreateApplicationHydratesFromRepositoryDescriptorAndStagesRepositoryToken(t *testing.T) {
	t.Parallel()

	store := &createSpyStore{
		createRecord: Record{
			ID:                 "shared__ops",
			ProjectID:          "shared",
			Namespace:          "shared",
			Name:               "ops",
			Image:              "ghcr.io/aolda/ops:v2",
			ServicePort:        9090,
			Replicas:           2,
			DeploymentStrategy: DeploymentStrategyCanary,
			DefaultEnvironment: "shared",
		},
	}
	secrets := &secretsSpy{}
	service := Service{
		Projects: &project.Service{
			Source: staticCatalogSource{
				items: []project.CatalogProject{
					{
						ID:        "shared",
						Name:      "shared",
						Namespace: "shared",
						Access: project.Access{
							DeployerGroups: []string{"aods:shared:deploy"},
						},
						Environments: []project.Environment{
							{
								ID:        "shared",
								Name:      "Shared",
								ClusterID: "default",
								WriteMode: project.WriteModeDirect,
								Default:   true,
							},
						},
						Policies: project.PolicySet{
							MinReplicas:                 1,
							AllowedEnvironments:         []string{"shared"},
							AllowedDeploymentStrategies: []string{"Rollout", "Canary"},
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
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://api.github.com/repos/aolda/ops/contents/aolda_deploy.json?ref=main" {
					t.Fatalf("unexpected descriptor request: %s", req.URL.String())
				}
				if got := req.Header.Get("Authorization"); got != "Bearer github_pat_123" {
					t.Fatalf("expected bearer token, got %q", got)
				}
				body := `{"services":[{"serviceId":"ops","image":"ghcr.io/aolda/ops:v2","port":9090,"replicas":2,"strategy":"Canary"}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		},
	}

	created, err := service.CreateApplication(context.Background(), core.User{
		Username: "deployer",
		Groups:   []string{"aods:shared:deploy"},
	}, "shared", CreateRequest{
		RepositoryURL:    "https://github.com/aolda/ops",
		RepositoryBranch: "main",
		RepositoryToken:  "github_pat_123",
		RegistryUsername: "octocat",
		RegistryToken:    "ghcr_pat_456",
		Environment:      "shared",
	}, "req_4")
	if err != nil {
		t.Fatalf("create application with repository descriptor: %v", err)
	}

	if len(store.createInputs) != 1 {
		t.Fatalf("expected one create input, got %d", len(store.createInputs))
	}
	createInput := store.createInputs[0]
	if createInput.Name != "ops" {
		t.Fatalf("expected service id to hydrate name, got %q", createInput.Name)
	}
	if createInput.Image != "ghcr.io/aolda/ops:v2" {
		t.Fatalf("expected image from descriptor, got %q", createInput.Image)
	}
	if createInput.ServicePort != 9090 {
		t.Fatalf("expected service port from descriptor, got %d", createInput.ServicePort)
	}
	if createInput.Replicas != 2 {
		t.Fatalf("expected replicas from descriptor, got %d", createInput.Replicas)
	}
	if createInput.DeploymentStrategy != DeploymentStrategyCanary {
		t.Fatalf("expected canary strategy from descriptor, got %q", createInput.DeploymentStrategy)
	}
	if createInput.RepositoryServiceID != "ops" {
		t.Fatalf("expected repository service id to hydrate from descriptor, got %q", createInput.RepositoryServiceID)
	}
	if createInput.RepositoryTokenPath != "secret/aods/apps/shared/ops/repository" {
		t.Fatalf("expected repository token path to be staged separately, got %q", createInput.RepositoryTokenPath)
	}
	if createInput.RegistrySecretPath != "secret/aods/apps/shared/ops/registry" {
		t.Fatalf("expected registry credential path to be staged separately, got %q", createInput.RegistrySecretPath)
	}
	if secrets.stageCalls != 0 {
		t.Fatalf("expected app env secrets not to be staged, got %d", secrets.stageCalls)
	}
	if secrets.stageAtCalls != 2 {
		t.Fatalf("expected repository and registry credentials to be staged, got %d", secrets.stageAtCalls)
	}
	if len(secrets.finalizedPath) != 2 {
		t.Fatalf("expected two finalized secret paths, got %#v", secrets.finalizedPath)
	}
	if !containsString(secrets.finalizedPath, "secret/aods/apps/shared/ops/repository") {
		t.Fatalf("expected repository token finalization path, got %#v", secrets.finalizedPath)
	}
	if !containsString(secrets.finalizedPath, "secret/aods/apps/shared/ops/registry") {
		t.Fatalf("expected registry credential finalization path, got %#v", secrets.finalizedPath)
	}
	if created.RepositoryURL != "https://github.com/aolda/ops" {
		t.Fatalf("expected repository url on response, got %q", created.RepositoryURL)
	}
	if created.RepositoryBranch != "main" {
		t.Fatalf("expected repository branch on response, got %q", created.RepositoryBranch)
	}
	if created.RepositoryServiceID != "ops" {
		t.Fatalf("expected repository service id on response, got %q", created.RepositoryServiceID)
	}
	if created.ConfigPath != "aolda_deploy.json" {
		t.Fatalf("expected default config path on response, got %q", created.ConfigPath)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func TestCreateApplicationNormalizesRepositoryServiceIDFromResolvedDescriptorService(t *testing.T) {
	t.Parallel()

	store := &createSpyStore{}
	service := Service{
		Projects: &project.Service{
			Source: staticCatalogSource{
				items: []project.CatalogProject{
					{
						ID:        "shared",
						Name:      "Shared",
						Namespace: "shared",
						Access: project.Access{
							DeployerGroups: []string{"aods:shared:deploy"},
						},
						Environments: []project.Environment{
							{
								ID:        "shared",
								Name:      "Shared",
								ClusterID: "default",
								WriteMode: project.WriteModeDirect,
								Default:   true,
							},
						},
						Policies: project.PolicySet{
							MinReplicas:                 1,
							AllowedEnvironments:         []string{"shared"},
							AllowedDeploymentStrategies: []string{"Rollout", "Canary"},
							AllowedClusterTargets:       []string{"default"},
							RequiredProbes:              true,
						},
					},
				},
			},
		},
		Store:        store,
		StatusReader: &batchStatusReaderStub{},
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				body := `{"services":[{"serviceId":"ops","image":"ghcr.io/aolda/ops:v3","port":8080,"replicas":1,"strategy":"Rollout"}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		},
	}

	created, err := service.CreateApplication(context.Background(), core.User{
		Username: "deployer",
		Groups:   []string{"aods:shared:deploy"},
	}, "shared", CreateRequest{
		Name:                "custom-name",
		RepositoryURL:       "https://github.com/aolda/ops",
		RepositoryBranch:    "main",
		RepositoryServiceID: "custom-name",
		Environment:         "shared",
	}, "req_norm_repo_service_id")
	if err != nil {
		t.Fatalf("create application with normalized repository service id: %v", err)
	}

	if len(store.createInputs) != 1 {
		t.Fatalf("expected one create input, got %d", len(store.createInputs))
	}
	if store.createInputs[0].RepositoryServiceID != "ops" {
		t.Fatalf("expected repository service id to normalize to descriptor service id, got %q", store.createInputs[0].RepositoryServiceID)
	}
	if created.RepositoryServiceID != "ops" {
		t.Fatalf("expected response repository service id to normalize to descriptor service id, got %q", created.RepositoryServiceID)
	}
}

func TestCreateApplicationWithPublicRepositoryDescriptorDoesNotRequireToken(t *testing.T) {
	t.Parallel()

	store := &createSpyStore{}
	secrets := &secretsSpy{}
	service := Service{
		Projects: &project.Service{
			Source: staticCatalogSource{
				items: []project.CatalogProject{
					{
						ID:        "shared",
						Name:      "Shared",
						Namespace: "shared",
						Access: project.Access{
							DeployerGroups: []string{"aods:shared:deploy"},
						},
						Environments: []project.Environment{
							{
								ID:        "shared",
								Name:      "Shared",
								ClusterID: "default",
								WriteMode: project.WriteModeDirect,
								Default:   true,
							},
						},
						Policies: project.PolicySet{
							MinReplicas:                 1,
							AllowedEnvironments:         []string{"shared"},
							AllowedDeploymentStrategies: []string{"Rollout", "Canary"},
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
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				expectedURL := "https://raw.githubusercontent.com/aolda/ops/main/aolda_deploy.json"
				if req.URL.String() != expectedURL {
					t.Fatalf("unexpected descriptor request: %s", req.URL.String())
				}
				if got := req.Header.Get("Authorization"); got != "" {
					t.Fatalf("expected no authorization header for public repository, got %q", got)
				}
				body := `{"services":[{"serviceId":"ops","image":"ghcr.io/aolda/ops:v3","port":8080,"replicas":1,"strategy":"Rollout"}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		},
	}

	created, err := service.CreateApplication(context.Background(), core.User{
		Username: "deployer",
		Groups:   []string{"aods:shared:deploy"},
	}, "shared", CreateRequest{
		RepositoryURL:    "https://github.com/aolda/ops",
		RepositoryBranch: "main",
		Environment:      "shared",
	}, "req_public_repo")
	if err != nil {
		t.Fatalf("create application with public repository descriptor: %v", err)
	}

	if len(store.createInputs) != 1 {
		t.Fatalf("expected one create input, got %d", len(store.createInputs))
	}
	createInput := store.createInputs[0]
	if createInput.Name != "ops" {
		t.Fatalf("expected service id to hydrate name, got %q", createInput.Name)
	}
	if createInput.Image != "ghcr.io/aolda/ops:v3" {
		t.Fatalf("expected image from descriptor, got %q", createInput.Image)
	}
	if createInput.RepositoryTokenPath != "" {
		t.Fatalf("expected no repository token path for public repository, got %q", createInput.RepositoryTokenPath)
	}
	if secrets.stageAtCalls != 0 {
		t.Fatalf("expected no staged repository credentials, got %d", secrets.stageAtCalls)
	}
	if len(secrets.finalizedPath) != 0 {
		t.Fatalf("expected no finalized repository credentials, got %#v", secrets.finalizedPath)
	}
	if created.RepositoryURL != "https://github.com/aolda/ops" {
		t.Fatalf("expected repository url on response, got %q", created.RepositoryURL)
	}
	if created.ConfigPath != "aolda_deploy.json" {
		t.Fatalf("expected default config path on response, got %q", created.ConfigPath)
	}
}

func TestCreateApplicationRejectsUnknownProjectRepository(t *testing.T) {
	t.Parallel()

	store := &createSpyStore{}
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
						Repositories: []project.Repository{
							{
								ID:   "payment-api",
								Name: "Payment API",
								URL:  "https://github.com/aolda/payment-api",
							},
						},
						Policies: project.PolicySet{
							MinReplicas:                 1,
							AllowedEnvironments:         []string{"prod"},
							AllowedDeploymentStrategies: []string{"Rollout"},
							AllowedClusterTargets:       []string{"default"},
							RequiredProbes:              true,
						},
					},
				},
			},
		},
		Store:        store,
		StatusReader: &batchStatusReaderStub{},
	}

	_, err := service.CreateApplication(context.Background(), core.User{
		Username: "deployer",
		Groups:   []string{"aods:project-a:deploy"},
	}, "project-a", CreateRequest{
		Name:               "repo-app",
		Image:              "ghcr.io/aolda/repo-app:v1",
		ServicePort:        8080,
		DeploymentStrategy: DeploymentStrategyRollout,
		Environment:        "prod",
		RepositoryID:       "unknown-repo",
	}, "req_3")
	if err == nil {
		t.Fatal("expected unknown repository to be rejected")
	}

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationErr.Details["repositoryId"] != "unknown-repo" {
		t.Fatalf("expected repository id detail to be preserved, got %#v", validationErr.Details)
	}
	if store.createCalls != 0 {
		t.Fatalf("expected manifest store create not to run, got %d", store.createCalls)
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

func TestPatchApplicationResourcesRequireAdmin(t *testing.T) {
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
							AdminGroups:    []string{"aods:project-a:admin"},
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

	_, err := service.PatchApplication(context.Background(), core.User{
		Groups: []string{"aods:project-a:deploy"},
	}, "project-a__demo", UpdateApplicationRequest{
		Resources: &ResourceRequirements{
			Requests: ResourceQuantity{CPU: "250m", Memory: "256Mi"},
			Limits:   ResourceQuantity{CPU: "500m", Memory: "512Mi"},
		},
	})
	if err == nil {
		t.Fatal("expected resource patch to require admin")
	}
	if !errors.Is(err, ErrRequiresAdmin) {
		t.Fatalf("expected ErrRequiresAdmin, got %v", err)
	}
}

func TestPatchApplicationNetworkSettingsRequireAdmin(t *testing.T) {
	t.Parallel()

	meshEnabled := true
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
							AdminGroups:    []string{"aods:project-a:admin"},
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
							AllowedDeploymentStrategies: []string{"Standard", "Rollout", "Canary"},
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
					ID:                  "project-a__demo",
					ProjectID:           "project-a",
					Name:                "demo",
					Image:               "ghcr.io/aolda/demo:v1",
					ServicePort:         8080,
					Replicas:            1,
					DeploymentStrategy:  DeploymentStrategyRollout,
					DefaultEnvironment:  "prod",
					MeshEnabled:         false,
					LoadBalancerEnabled: false,
				},
			},
		},
		StatusReader: &batchStatusReaderStub{},
	}

	_, err := service.PatchApplication(context.Background(), core.User{
		Groups: []string{"aods:project-a:deploy"},
	}, "project-a__demo", UpdateApplicationRequest{
		MeshEnabled: &meshEnabled,
	})
	if err == nil {
		t.Fatal("expected network patch to require admin")
	}
	if !errors.Is(err, ErrRequiresAdmin) {
		t.Fatalf("expected ErrRequiresAdmin, got %v", err)
	}
}

func TestPatchApplicationRejectsLoadBalancerForCanary(t *testing.T) {
	t.Parallel()

	loadBalancerEnabled := true
	service := Service{
		Projects: &project.Service{
			Source: staticCatalogSource{
				items: []project.CatalogProject{
					{
						ID:        "project-a",
						Name:      "Project A",
						Namespace: "project-a",
						Access: project.Access{
							AdminGroups: []string{"aods:project-a:admin"},
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
							AllowedDeploymentStrategies: []string{"Standard", "Rollout", "Canary"},
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
					ID:                  "project-a__demo",
					ProjectID:           "project-a",
					Name:                "demo",
					Image:               "ghcr.io/aolda/demo:v1",
					ServicePort:         8080,
					Replicas:            1,
					DeploymentStrategy:  DeploymentStrategyCanary,
					DefaultEnvironment:  "prod",
					MeshEnabled:         true,
					LoadBalancerEnabled: false,
				},
			},
		},
		StatusReader: &batchStatusReaderStub{},
	}

	_, err := service.PatchApplication(context.Background(), core.User{
		Groups: []string{"aods:project-a:admin"},
	}, "project-a__demo", UpdateApplicationRequest{
		LoadBalancerEnabled: &loadBalancerEnabled,
	})
	if err == nil {
		t.Fatal("expected canary load balancer patch to be rejected")
	}

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Message != "loadBalancerEnabled cannot be true when deploymentStrategy is Canary" {
		t.Fatalf("unexpected validation message: %s", validationErr.Message)
	}
}

func TestPatchApplicationRejectsUnsupportedRepositoryPollInterval(t *testing.T) {
	t.Parallel()

	interval := 120
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
								ID:        "shared",
								Name:      "Shared",
								ClusterID: "default",
								WriteMode: project.WriteModeDirect,
								Default:   true,
							},
						},
						Policies: project.PolicySet{
							MinReplicas:                 1,
							AllowedEnvironments:         []string{"shared"},
							AllowedDeploymentStrategies: []string{"Standard", "Rollout"},
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
					DeploymentStrategy: DeploymentStrategyRollout,
					DefaultEnvironment: "shared",
					RepositoryURL:      "https://github.com/aolda/demo.git",
				},
			},
		},
		StatusReader: &batchStatusReaderStub{},
	}

	_, err := service.PatchApplication(context.Background(), core.User{
		Groups: []string{"aods:project-a:deploy"},
	}, "project-a__demo", UpdateApplicationRequest{
		RepositoryPollIntervalSeconds: &interval,
	})
	if err == nil {
		t.Fatal("expected unsupported repository poll interval to be rejected")
	}

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Message != "repositoryPollIntervalSeconds must be one of 60, 300, or 600" {
		t.Fatalf("unexpected validation message: %s", validationErr.Message)
	}
}

func TestPatchApplicationReschedulesRepositoryPollInterval(t *testing.T) {
	t.Parallel()

	store := &patchSpyStore{
		stubStore: stubStore{
			records: []Record{
				{
					ID:                 "project-a__demo",
					ProjectID:          "project-a",
					Name:               "demo",
					Image:              "ghcr.io/aolda/demo:v1",
					ServicePort:        8080,
					Replicas:           1,
					DeploymentStrategy: DeploymentStrategyRollout,
					DefaultEnvironment: "shared",
					RepositoryURL:      "https://github.com/aolda/demo.git",
				},
			},
		},
	}
	tracker := NewRepositoryPollTracker(5 * time.Minute)
	checkedAt := time.Date(2026, 4, 18, 5, 0, 0, 0, time.UTC)
	tracker.MarkSuccess(store.records[0], project.Repository{Name: "demo"}, checkedAt)

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
								ID:        "shared",
								Name:      "Shared",
								ClusterID: "default",
								WriteMode: project.WriteModeDirect,
								Default:   true,
							},
						},
						Policies: project.PolicySet{
							MinReplicas:                 1,
							AllowedEnvironments:         []string{"shared"},
							AllowedDeploymentStrategies: []string{"Standard", "Rollout"},
							AllowedClusterTargets:       []string{"default"},
							RequiredProbes:              true,
						},
					},
				},
			},
		},
		Store:        store,
		StatusReader: staticStatusReader{info: SyncInfo{Status: SyncStatusSynced}},
		PollTracker:  tracker,
	}

	interval := 600
	updated, err := service.PatchApplication(context.Background(), core.User{
		Groups: []string{"aods:project-a:deploy"},
	}, "project-a__demo", UpdateApplicationRequest{
		RepositoryPollIntervalSeconds: &interval,
	})
	if err != nil {
		t.Fatalf("patch application: %v", err)
	}
	if updated.RepositoryPollIntervalSeconds != 600 {
		t.Fatalf("expected updated interval to be 600, got %d", updated.RepositoryPollIntervalSeconds)
	}

	snapshot := tracker.Snapshot(Record{
		ID:                            updated.ID,
		RepositoryURL:                 "https://github.com/aolda/demo.git",
		RepositoryPollIntervalSeconds: updated.RepositoryPollIntervalSeconds,
	})
	if snapshot == nil {
		t.Fatal("expected repository poll snapshot after patch")
	}
	if snapshot.IntervalSeconds != 600 {
		t.Fatalf("expected tracker intervalSeconds=600, got %d", snapshot.IntervalSeconds)
	}
}
