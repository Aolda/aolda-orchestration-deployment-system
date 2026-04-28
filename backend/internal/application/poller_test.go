package application

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/project"
)

func TestParseRepositoryDescriptorAndResolveService(t *testing.T) {
	t.Parallel()

	descriptor, err := parseRepositoryDescriptor([]byte(`{
		"services": [
			{"serviceId":"api","image":"ghcr.io/aolda/demo-api:v3","port":8080,"replicas":3},
			{"serviceId":"worker","image":"ghcr.io/aolda/demo-worker:v5","port":9090,"replicas":2}
		]
	}`))
	if err != nil {
		t.Fatalf("parse repository descriptor: %v", err)
	}

	service, ok := descriptor.resolveService(Record{
		Name:                "ignored",
		RepositoryServiceID: "worker",
	})
	if !ok {
		t.Fatal("expected to resolve repository service by repositoryServiceId")
	}
	if service.ServiceID != "worker" {
		t.Fatalf("expected worker service, got %s", service.ServiceID)
	}
	if service.Replicas != 2 {
		t.Fatalf("expected 2 replicas, got %d", service.Replicas)
	}
}

func TestParseRepositoryDescriptorRejectsInvalidReplicas(t *testing.T) {
	t.Parallel()

	_, err := parseRepositoryDescriptor([]byte(`{
		"services": [
			{"serviceId":"api","image":"ghcr.io/aolda/demo-api:v3","port":8080,"replicas":0}
		]
	}`))
	if err == nil {
		t.Fatal("expected invalid descriptor to fail")
	}
}

func TestRepositoryDescriptorResolvesSingleServiceWithoutExplicitID(t *testing.T) {
	t.Parallel()

	descriptor, err := parseRepositoryDescriptor([]byte(`{
		"services": [
			{"serviceId":"only","image":"ghcr.io/aolda/demo-only:v1","port":8080,"replicas":1}
		]
	}`))
	if err != nil {
		t.Fatalf("parse repository descriptor: %v", err)
	}

	service, ok := descriptor.resolveService(Record{Name: "different-name"})
	if !ok {
		t.Fatal("expected single-service descriptor to resolve without explicit id")
	}
	if service.ServiceID != "only" {
		t.Fatalf("expected only service, got %s", service.ServiceID)
	}
}

func TestImageRepositoryRefIgnoresTagAndDigest(t *testing.T) {
	t.Parallel()

	left := imageRepositoryRef("ghcr.io/aolda/service-api:v1")
	right := imageRepositoryRef("ghcr.io/aolda/service-api@sha256:abc123")
	if left != right {
		t.Fatalf("expected repository ref to ignore tag/digest, got %q and %q", left, right)
	}
}

func TestResolveRepositoryTokenSupportsPatAliases(t *testing.T) {
	t.Parallel()

	token := resolveRepositoryToken(map[string]string{
		"github_pat": "github_pat_123",
	})
	if token != "github_pat_123" {
		t.Fatalf("expected github_pat token, got %q", token)
	}
}

func TestResolveRepositoryFileTargetUsesGitHubContentsAPIWhenTokenPresent(t *testing.T) {
	t.Parallel()

	poller := AutoUpdatePoller{}
	target, err := poller.resolveRepositoryFileTarget(project.Repository{
		URL:    "https://github.com/Aolda/private-repo.git",
		Branch: "release",
	}, "aolda_deploy.json", "", "github_pat_123")
	if err != nil {
		t.Fatalf("resolve repository file target: %v", err)
	}

	if !target.UseGitHubContents {
		t.Fatal("expected GitHub contents API target when token is present")
	}
	expected := "https://api.github.com/repos/Aolda/private-repo/contents/aolda_deploy.json?ref=release"
	if target.URL != expected {
		t.Fatalf("expected %s, got %s", expected, target.URL)
	}
}

func TestResolveRepositoryFileTargetUsesRawGitHubURLWithoutToken(t *testing.T) {
	t.Parallel()

	poller := AutoUpdatePoller{}
	target, err := poller.resolveRepositoryFileTarget(project.Repository{
		URL:    "https://github.com/Aolda/public-repo.git",
		Branch: "main",
	}, "configs/aolda_deploy.json", "", "")
	if err != nil {
		t.Fatalf("resolve repository file target: %v", err)
	}

	if target.UseGitHubContents {
		t.Fatal("expected raw GitHub target without token")
	}
	expected := "https://raw.githubusercontent.com/Aolda/public-repo/main/configs/aolda_deploy.json"
	if target.URL != expected {
		t.Fatalf("expected %s, got %s", expected, target.URL)
	}
}

func TestSyncRepositoryNowFallsBackToPublicAccessWhenTokenSecretIsEmpty(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Authorization"); got != "" {
			t.Fatalf("expected no authorization header for empty repository token, got %q", got)
		}
		if !strings.HasPrefix(req.URL.String(), "https://raw.githubusercontent.com/Aolda/public-repo/main/") {
			t.Fatalf("expected raw GitHub URL, got %s", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("{}")),
		}, nil
	})
	poller := AutoUpdatePoller{
		Client: &http.Client{Transport: transport},
		Service: &Service{
			Secrets: emptyRepositoryTokenStore{},
		},
	}

	response, err := poller.SyncRepositoryNow(
		context.Background(),
		core.User{ID: "system", Username: "system", Groups: []string{"aods:platform:admin"}},
		project.CatalogProject{ID: "shared", Namespace: "shared"},
		Record{
			ID:                  "shared__public-app",
			ProjectID:           "shared",
			Name:                "public-app",
			Image:               "ghcr.io/aolda/public-app:v1",
			ConfigPath:          DefaultRepositoryConfigPath,
			RepositoryTokenPath: "secret/aods/apps/shared/public-app/repository",
		},
		project.Repository{
			URL:    "https://github.com/Aolda/public-repo.git",
			Branch: "main",
		},
	)
	if err != nil {
		t.Fatalf("expected public fallback to succeed, got %v", err)
	}
	if response.DeploymentTriggered {
		t.Fatal("expected no deployment to be triggered when update config has no imageTag")
	}
}

type autoRollbackStore struct {
	record             Record
	deployments        []DeploymentRecord
	policy             RollbackPolicy
	events             []Event
	updatedImageTags   []string
	updatedDeployments []DeploymentRecord
}

type emptyRepositoryTokenStore struct{}

func (emptyRepositoryTokenStore) Stage(ctx context.Context, requestID string, projectID string, appName string, createdBy string, data map[string]string) (StagedSecret, error) {
	return StagedSecret{}, nil
}

func (emptyRepositoryTokenStore) StageAt(ctx context.Context, requestID string, finalPath string, metadata map[string]string, data map[string]string) (StagedSecret, error) {
	return StagedSecret{}, nil
}

func (emptyRepositoryTokenStore) Finalize(ctx context.Context, staged StagedSecret, data map[string]string) error {
	return nil
}

func (emptyRepositoryTokenStore) Get(ctx context.Context, logicalPath string) (map[string]string, error) {
	return map[string]string{"token": ""}, nil
}

func (emptyRepositoryTokenStore) Delete(ctx context.Context, logicalPath string) error {
	return nil
}

func (s *autoRollbackStore) ListApplications(ctx context.Context, projectID string) ([]Record, error) {
	return []Record{s.record}, nil
}

func (s *autoRollbackStore) GetApplication(ctx context.Context, applicationID string) (Record, error) {
	return s.record, nil
}

func (s *autoRollbackStore) CreateApplication(ctx context.Context, project ProjectContext, input CreateRequest, secretPath string) (Record, error) {
	return s.record, nil
}

func (s *autoRollbackStore) ArchiveApplication(ctx context.Context, applicationID string, archivedBy string) (ApplicationLifecycleResponse, error) {
	return ApplicationLifecycleResponse{}, nil
}

func (s *autoRollbackStore) DeleteApplication(ctx context.Context, applicationID string) (ApplicationLifecycleResponse, error) {
	return ApplicationLifecycleResponse{}, nil
}

func (s *autoRollbackStore) UpdateApplicationImage(ctx context.Context, project ProjectContext, applicationID string, imageTag string, deploymentID string) (Record, error) {
	s.updatedImageTags = append(s.updatedImageTags, imageTag)
	s.record.Image = replaceImageTag(s.record.Image, imageTag)
	now := timeNowUTC()
	deployment := DeploymentRecord{
		DeploymentID:       deploymentID,
		ApplicationID:      applicationID,
		ProjectID:          s.record.ProjectID,
		ApplicationName:    s.record.Name,
		Environment:        s.record.DefaultEnvironment,
		Image:              s.record.Image,
		ImageTag:           imageTag,
		DeploymentStrategy: s.record.DeploymentStrategy,
		Status:             "Syncing",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	s.deployments = append([]DeploymentRecord{deployment}, s.deployments...)
	return s.record, nil
}

func (s *autoRollbackStore) PatchApplication(ctx context.Context, project ProjectContext, applicationID string, input UpdateApplicationRequest) (Record, error) {
	return s.record, nil
}

func (s *autoRollbackStore) SaveApplicationSecretPath(ctx context.Context, project ProjectContext, applicationID string, secretPath string) (Record, error) {
	s.record.SecretPath = secretPath
	return s.record, nil
}

func (s *autoRollbackStore) ListDeployments(ctx context.Context, applicationID string) ([]DeploymentRecord, error) {
	return append([]DeploymentRecord(nil), s.deployments...), nil
}

func (s *autoRollbackStore) GetDeployment(ctx context.Context, applicationID string, deploymentID string) (DeploymentRecord, error) {
	for _, deployment := range s.deployments {
		if deployment.DeploymentID == deploymentID {
			return deployment, nil
		}
	}
	return DeploymentRecord{}, ErrDeploymentNotFound
}

func (s *autoRollbackStore) UpdateDeployment(ctx context.Context, applicationID string, deployment DeploymentRecord) (DeploymentRecord, error) {
	s.updatedDeployments = append(s.updatedDeployments, deployment)
	for idx := range s.deployments {
		if s.deployments[idx].DeploymentID == deployment.DeploymentID {
			s.deployments[idx] = deployment
			return deployment, nil
		}
	}
	s.deployments = append(s.deployments, deployment)
	return deployment, nil
}

func (s *autoRollbackStore) GetRollbackPolicy(ctx context.Context, applicationID string) (RollbackPolicy, error) {
	return s.policy, nil
}

func (s *autoRollbackStore) SaveRollbackPolicy(ctx context.Context, applicationID string, policy RollbackPolicy) (RollbackPolicy, error) {
	s.policy = policy
	return policy, nil
}

func (s *autoRollbackStore) ListEvents(ctx context.Context, applicationID string) ([]Event, error) {
	return append([]Event(nil), s.events...), nil
}

func (s *autoRollbackStore) AppendEvent(ctx context.Context, applicationID string, event Event) error {
	s.events = append(s.events, event)
	return nil
}

type rollbackMetricsReader struct {
	metrics []MetricSeries
}

func (r rollbackMetricsReader) Read(ctx context.Context, record Record, duration time.Duration, step time.Duration) ([]MetricSeries, error) {
	return append([]MetricSeries(nil), r.metrics...), nil
}

func TestPollAutoRollbackTriggersRedeployWhenThresholdBreached(t *testing.T) {
	t.Parallel()

	store := &autoRollbackStore{
		record: Record{
			ID:                 "project-a__auto-rollback",
			ProjectID:          "project-a",
			Namespace:          "project-a",
			Name:               "auto-rollback",
			Image:              "ghcr.io/aolda/auto-rollback:v2",
			ServicePort:        8080,
			Replicas:           2,
			DeploymentStrategy: DeploymentStrategyCanary,
			DefaultEnvironment: "prod",
		},
		deployments: []DeploymentRecord{
			{
				DeploymentID:       "dep-new",
				ApplicationID:      "project-a__auto-rollback",
				ProjectID:          "project-a",
				ApplicationName:    "auto-rollback",
				Environment:        "prod",
				Image:              "ghcr.io/aolda/auto-rollback:v2",
				ImageTag:           "v2",
				DeploymentStrategy: DeploymentStrategyCanary,
				Status:             "Syncing",
				CreatedAt:          timeNowUTC().Add(-30 * time.Minute),
				UpdatedAt:          timeNowUTC().Add(-30 * time.Minute),
			},
			{
				DeploymentID:       "dep-old",
				ApplicationID:      "project-a__auto-rollback",
				ProjectID:          "project-a",
				ApplicationName:    "auto-rollback",
				Environment:        "prod",
				Image:              "ghcr.io/aolda/auto-rollback:v1",
				ImageTag:           "v1",
				DeploymentStrategy: DeploymentStrategyCanary,
				Status:             "Synced",
				CreatedAt:          timeNowUTC().Add(-2 * time.Hour),
				UpdatedAt:          timeNowUTC().Add(-2 * time.Hour),
			},
		},
		policy: RollbackPolicy{
			Enabled:         true,
			MaxErrorRate:    floatPointer(3.0),
			MaxLatencyP95Ms: intPointerValue(1200),
			MinRequestRate:  floatPointer(10),
		},
	}

	service := &Service{
		Projects: &project.Service{
			Source: staticCatalogSource{
				items: []project.CatalogProject{
					{
						ID:        "project-a",
						Name:      "Project A",
						Namespace: "project-a",
						Access: project.Access{
							AdminGroups: []string{"aods:platform:admin"},
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
							AllowedDeploymentStrategies: []string{"Canary"},
							AllowedClusterTargets:       []string{"default"},
							AutoRollbackEnabled:         true,
							RequiredProbes:              true,
						},
					},
				},
			},
		},
		Store:         store,
		StatusReader:  &batchStatusReaderStub{},
		MetricsReader: rollbackMetricsReader{metrics: rollbackMetrics(24, 4.5, 1800)},
	}

	poller := AutoUpdatePoller{
		Service:                 service,
		RollbackEvaluationDelay: 5 * time.Minute,
	}

	poller.pollAutoRollback(context.Background(), core.User{
		Username: "system.poller",
		Groups:   []string{"aods:platform:admin"},
	}, project.CatalogProject{
		ID:        "project-a",
		Name:      "Project A",
		Namespace: "project-a",
		Policies: project.PolicySet{
			AutoRollbackEnabled: true,
		},
	}, store.record)

	if len(store.updatedImageTags) != 1 || store.updatedImageTags[0] != "v1" {
		t.Fatalf("expected rollback redeploy to target image tag v1, got %#v", store.updatedImageTags)
	}
	if len(store.updatedDeployments) == 0 || store.updatedDeployments[0].Status != "AutoRollbackTriggered" {
		t.Fatalf("expected source deployment status to be updated after auto rollback, got %#v", store.updatedDeployments)
	}
	if !hasEventType(store.events, "AutoRollbackTriggered") {
		t.Fatalf("expected AutoRollbackTriggered event, got %#v", store.events)
	}
}

func TestPollAutoRollbackSkipsWhenTrafficIsBelowThreshold(t *testing.T) {
	t.Parallel()

	store := &autoRollbackStore{
		record: Record{
			ID:                 "project-a__traffic-gated",
			ProjectID:          "project-a",
			Namespace:          "project-a",
			Name:               "traffic-gated",
			Image:              "ghcr.io/aolda/traffic-gated:v2",
			ServicePort:        8080,
			Replicas:           2,
			DeploymentStrategy: DeploymentStrategyCanary,
			DefaultEnvironment: "prod",
		},
		deployments: []DeploymentRecord{
			{
				DeploymentID:       "dep-new",
				ApplicationID:      "project-a__traffic-gated",
				ProjectID:          "project-a",
				ApplicationName:    "traffic-gated",
				Environment:        "prod",
				Image:              "ghcr.io/aolda/traffic-gated:v2",
				ImageTag:           "v2",
				DeploymentStrategy: DeploymentStrategyCanary,
				Status:             "Syncing",
				CreatedAt:          timeNowUTC().Add(-30 * time.Minute),
				UpdatedAt:          timeNowUTC().Add(-30 * time.Minute),
			},
			{
				DeploymentID:       "dep-old",
				ApplicationID:      "project-a__traffic-gated",
				ProjectID:          "project-a",
				ApplicationName:    "traffic-gated",
				Environment:        "prod",
				Image:              "ghcr.io/aolda/traffic-gated:v1",
				ImageTag:           "v1",
				DeploymentStrategy: DeploymentStrategyCanary,
				Status:             "Synced",
				CreatedAt:          timeNowUTC().Add(-2 * time.Hour),
				UpdatedAt:          timeNowUTC().Add(-2 * time.Hour),
			},
		},
		policy: RollbackPolicy{
			Enabled:        true,
			MaxErrorRate:   floatPointer(1.5),
			MinRequestRate: floatPointer(50),
		},
	}

	service := &Service{
		Projects: &project.Service{
			Source: staticCatalogSource{
				items: []project.CatalogProject{
					{
						ID:        "project-a",
						Name:      "Project A",
						Namespace: "project-a",
						Access: project.Access{
							AdminGroups: []string{"aods:platform:admin"},
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
							AllowedDeploymentStrategies: []string{"Canary"},
							AllowedClusterTargets:       []string{"default"},
							AutoRollbackEnabled:         true,
							RequiredProbes:              true,
						},
					},
				},
			},
		},
		Store:         store,
		StatusReader:  &batchStatusReaderStub{},
		MetricsReader: rollbackMetricsReader{metrics: rollbackMetrics(5, 4.5, 1800)},
	}

	poller := AutoUpdatePoller{
		Service:                 service,
		RollbackEvaluationDelay: 5 * time.Minute,
	}

	poller.pollAutoRollback(context.Background(), core.User{
		Username: "system.poller",
		Groups:   []string{"aods:platform:admin"},
	}, project.CatalogProject{
		ID:        "project-a",
		Name:      "Project A",
		Namespace: "project-a",
		Policies: project.PolicySet{
			AutoRollbackEnabled: true,
		},
	}, store.record)

	if len(store.updatedImageTags) != 0 {
		t.Fatalf("expected no auto rollback when traffic gate is not met, got %#v", store.updatedImageTags)
	}
	if hasEventType(store.events, "AutoRollbackTriggered") {
		t.Fatalf("expected no AutoRollbackTriggered event, got %#v", store.events)
	}
}

func TestPollAutoRollbackDoesNotTriggerTwiceForSameDeployment(t *testing.T) {
	t.Parallel()

	store := &autoRollbackStore{
		record: Record{
			ID:                 "project-a__dedupe",
			ProjectID:          "project-a",
			Namespace:          "project-a",
			Name:               "dedupe",
			Image:              "ghcr.io/aolda/dedupe:v2",
			ServicePort:        8080,
			Replicas:           2,
			DeploymentStrategy: DeploymentStrategyCanary,
			DefaultEnvironment: "prod",
		},
		deployments: []DeploymentRecord{
			{
				DeploymentID:       "dep-new",
				ApplicationID:      "project-a__dedupe",
				ProjectID:          "project-a",
				ApplicationName:    "dedupe",
				Environment:        "prod",
				Image:              "ghcr.io/aolda/dedupe:v2",
				ImageTag:           "v2",
				DeploymentStrategy: DeploymentStrategyCanary,
				Status:             "Syncing",
				CreatedAt:          timeNowUTC().Add(-30 * time.Minute),
				UpdatedAt:          timeNowUTC().Add(-30 * time.Minute),
			},
			{
				DeploymentID:       "dep-old",
				ApplicationID:      "project-a__dedupe",
				ProjectID:          "project-a",
				ApplicationName:    "dedupe",
				Environment:        "prod",
				Image:              "ghcr.io/aolda/dedupe:v1",
				ImageTag:           "v1",
				DeploymentStrategy: DeploymentStrategyCanary,
				Status:             "Synced",
				CreatedAt:          timeNowUTC().Add(-2 * time.Hour),
				UpdatedAt:          timeNowUTC().Add(-2 * time.Hour),
			},
		},
		policy: RollbackPolicy{
			Enabled:        true,
			MaxErrorRate:   floatPointer(1.5),
			MinRequestRate: floatPointer(10),
		},
		events: []Event{
			{
				ID:        "evt-existing",
				Type:      "AutoRollbackTriggered",
				Message:   "이미 자동 롤백을 수행했습니다.",
				CreatedAt: timeNowUTC().Add(-10 * time.Minute),
				Metadata: map[string]any{
					"sourceDeploymentId": "dep-new",
				},
			},
		},
	}

	service := &Service{
		Projects: &project.Service{
			Source: staticCatalogSource{
				items: []project.CatalogProject{
					{
						ID:        "project-a",
						Name:      "Project A",
						Namespace: "project-a",
						Access: project.Access{
							AdminGroups: []string{"aods:platform:admin"},
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
							AllowedDeploymentStrategies: []string{"Canary"},
							AllowedClusterTargets:       []string{"default"},
							AutoRollbackEnabled:         true,
							RequiredProbes:              true,
						},
					},
				},
			},
		},
		Store:         store,
		StatusReader:  &batchStatusReaderStub{},
		MetricsReader: rollbackMetricsReader{metrics: rollbackMetrics(24, 4.5, 1800)},
	}

	poller := AutoUpdatePoller{
		Service:                 service,
		RollbackEvaluationDelay: 5 * time.Minute,
	}

	poller.pollAutoRollback(context.Background(), core.User{
		Username: "system.poller",
		Groups:   []string{"aods:platform:admin"},
	}, project.CatalogProject{
		ID:        "project-a",
		Name:      "Project A",
		Namespace: "project-a",
		Policies: project.PolicySet{
			AutoRollbackEnabled: true,
		},
	}, store.record)

	if len(store.updatedImageTags) != 0 {
		t.Fatalf("expected no duplicate auto rollback, got %#v", store.updatedImageTags)
	}
}

func rollbackMetrics(requestRate float64, errorRate float64, latencyP95 float64) []MetricSeries {
	now := timeNowUTC()
	return []MetricSeries{
		singlePointMetric("request_rate", requestRate, now),
		singlePointMetric("error_rate", errorRate, now),
		singlePointMetric("latency_p95", latencyP95, now),
	}
}

func singlePointMetric(key string, value float64, timestamp time.Time) MetricSeries {
	return MetricSeries{
		Key: key,
		Points: []MetricPoint{
			{
				Timestamp: timestamp,
				Value:     floatPointer(value),
			},
		},
	}
}

func floatPointer(value float64) *float64 {
	return &value
}

func intPointerValue(value int) *int {
	return &value
}

func hasEventType(events []Event, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}
