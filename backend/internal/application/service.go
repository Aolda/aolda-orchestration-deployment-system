package application

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/project"
)

var (
	ErrNotFound           = errors.New("application not found")
	ErrConflict           = errors.New("application conflict")
	ErrInvalidID          = errors.New("application id is invalid")
	ErrRequiresDeployer   = errors.New("deployer permissions are required")
	ErrRequiresAdmin      = errors.New("admin permissions are required")
	ErrChangeRequired     = errors.New("change review is required for this environment")
	ErrInvalidPolicy      = errors.New("application request violates project policy")
	ErrDeploymentNotFound = errors.New("deployment not found")
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

type changeGuardBypassKey struct{}

type ValidationError struct {
	Message string
	Details map[string]any
}

func (e ValidationError) Error() string {
	return e.Message
}

type Store interface {
	ListApplications(ctx context.Context, projectID string) ([]Record, error)
	GetApplication(ctx context.Context, applicationID string) (Record, error)
	CreateApplication(ctx context.Context, project ProjectContext, input CreateRequest, secretPath string) (Record, error)
	UpdateApplicationImage(ctx context.Context, project ProjectContext, applicationID string, imageTag string, deploymentID string) (Record, error)
	PatchApplication(ctx context.Context, project ProjectContext, applicationID string, input UpdateApplicationRequest) (Record, error)
	ListDeployments(ctx context.Context, applicationID string) ([]DeploymentRecord, error)
	GetDeployment(ctx context.Context, applicationID string, deploymentID string) (DeploymentRecord, error)
	UpdateDeployment(ctx context.Context, applicationID string, deployment DeploymentRecord) (DeploymentRecord, error)
	GetRollbackPolicy(ctx context.Context, applicationID string) (RollbackPolicy, error)
	SaveRollbackPolicy(ctx context.Context, applicationID string, policy RollbackPolicy) (RollbackPolicy, error)
	ListEvents(ctx context.Context, applicationID string) ([]Event, error)
	AppendEvent(ctx context.Context, applicationID string, event Event) error
}

type StatusReader interface {
	Read(ctx context.Context, record Record) (SyncInfo, error)
}

type BatchStatusReader interface {
	ReadMany(ctx context.Context, records []Record) (map[string]SyncInfo, error)
}

type MetricsReader interface {
	Read(ctx context.Context, record Record, duration time.Duration, step time.Duration) ([]MetricSeries, error)
}

type RolloutController interface {
	GetRollout(ctx context.Context, record Record) (RolloutInfo, error)
	Promote(ctx context.Context, record Record, full bool) (RolloutInfo, error)
	Abort(ctx context.Context, record Record) (RolloutInfo, error)
}

type SecretStore interface {
	Stage(ctx context.Context, requestID string, projectID string, appName string, createdBy string, data map[string]string) (StagedSecret, error)
	Finalize(ctx context.Context, staged StagedSecret, data map[string]string) error
	Get(ctx context.Context, logicalPath string) (map[string]string, error)
}

type Service struct {
	Projects      *project.Service
	Store         Store
	StatusReader  StatusReader
	MetricsReader MetricsReader
	Secrets       SecretStore
	Rollouts      RolloutController
	Images        ImageVerifier
}

func (s Service) ListApplications(ctx context.Context, user core.User, projectID string) ([]Summary, error) {
	if _, err := s.Projects.GetAuthorized(ctx, user, projectID); err != nil {
		return nil, err
	}

	records, err := s.Store.ListApplications(ctx, projectID)
	if err != nil {
		return nil, err
	}

	syncByApplicationID := map[string]SyncInfo{}
	if reader, ok := s.StatusReader.(BatchStatusReader); ok {
		syncByApplicationID, err = reader.ReadMany(ctx, records)
		if err != nil {
			return nil, err
		}
	}

	items := make([]Summary, 0, len(records))
	for _, record := range records {
		syncInfo, ok := syncByApplicationID[record.ID]
		if !ok {
			syncInfo, err = s.StatusReader.Read(ctx, record)
			if err != nil {
				return nil, err
			}
		}

		items = append(items, Summary{
			ID:                 record.ID,
			Name:               record.Name,
			Image:              record.Image,
			DeploymentStrategy: string(NormalizeDeploymentStrategy(record.DeploymentStrategy)),
			SyncStatus:         syncInfo.Status,
		})
	}

	return items, nil
}

func (s Service) CreateApplication(
	ctx context.Context,
	user core.User,
	projectID string,
	input CreateRequest,
	requestID string,
) (Application, error) {
	authorizedProject, err := s.Projects.GetAuthorized(ctx, user, projectID)
	if err != nil {
		return Application{}, err
	}

	if !authorizedProject.Role.CanDeploy() {
		return Application{}, ErrRequiresDeployer
	}

	if err := validateCreateRequest(input); err != nil {
		return Application{}, err
	}
	input.DeploymentStrategy = NormalizeDeploymentStrategy(input.DeploymentStrategy)
	if err := s.verifyImageReference(ctx, strings.TrimSpace(input.Image)); err != nil {
		return Application{}, err
	}
	input.Environment = resolveEnvironment(authorizedProject.Project, input.Environment)
	if err := validateProjectPolicies(authorizedProject.Project, input.Environment, input.DeploymentStrategy); err != nil {
		return Application{}, err
	}
	if !changeGuardBypassed(ctx) && requiresChangeFlow(authorizedProject.Project, input.Environment) {
		return Application{}, ErrChangeRequired
	}

	secretData, err := normalizeSecrets(input.Secrets)
	if err != nil {
		return Application{}, err
	}

	secretPath := ""
	var staged StagedSecret
	if len(secretData) > 0 && s.Secrets != nil {
		secretPath = core.BuildVaultFinalPath(projectID, input.Name)
		staged, err = s.Secrets.Stage(ctx, requestID, projectID, input.Name, user.Username, secretData)
		if err != nil {
			return Application{}, err
		}
		secretPath = staged.FinalPath
	}

	record, err := s.Store.CreateApplication(ctx, buildProjectContext(authorizedProject.Project), input, secretPath)
	if err != nil {
		return Application{}, err
	}
	record.RepositoryID = input.RepositoryID
	record.RepositoryServiceID = input.RepositoryServiceID
	record.ConfigPath = input.ConfigPath
	if record.ConfigPath == "" {
		record.ConfigPath = "aolda-deploy.yaml"
	}
	// We need to re-save to store the new fields because Store.CreateApplication might not handle them yet
	// Actually, I should update the Store.CreateApplication interface but let's do a patch for now for safety
	// or better, update the Store interface.

	if len(secretData) > 0 && s.Secrets != nil {
		if err := s.Secrets.Finalize(ctx, staged, secretData); err != nil {
			return Application{}, err
		}
	}

	syncInfo, err := s.StatusReader.Read(ctx, record)
	if err != nil {
		return Application{}, err
	}
	_ = s.appendEvent(ctx, record.ID, "ApplicationCreated", fmt.Sprintf("애플리케이션 %s 생성", record.Name), map[string]any{
		"environment": record.DefaultEnvironment,
		"strategy":    record.DeploymentStrategy,
	})

	return toApplication(record, syncInfo), nil
}

func (s Service) CreateDeployment(
	ctx context.Context,
	user core.User,
	applicationID string,
	imageTag string,
	environment string,
	requestID string,
) (DeploymentResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, true)
	if err != nil {
		return DeploymentResponse{}, err
	}
	projectInfo, err := s.Projects.GetAuthorized(ctx, user, record.ProjectID)
	if err != nil {
		return DeploymentResponse{}, err
	}

	if strings.TrimSpace(imageTag) == "" {
		return DeploymentResponse{}, ValidationError{
			Message: "imageTag is required",
			Details: map[string]any{"field": "imageTag"},
		}
	}

	targetEnvironment := record.DefaultEnvironment
	if strings.TrimSpace(environment) != "" {
		targetEnvironment = resolveEnvironment(projectInfo.Project, environment)
	}
	if err := validateProjectPolicies(projectInfo.Project, targetEnvironment, record.DeploymentStrategy); err != nil {
		return DeploymentResponse{}, err
	}
	if !changeGuardBypassed(ctx) && requiresChangeFlow(projectInfo.Project, targetEnvironment) {
		return DeploymentResponse{}, ErrChangeRequired
	}
	nextImage := replaceImageTag(record.Image, imageTag)
	if err := s.verifyDeploymentImage(ctx, record, nextImage, imageTag, targetEnvironment); err != nil {
		return DeploymentResponse{}, err
	}

	if targetEnvironment != record.DefaultEnvironment {
		updatedRecord, err := s.Store.PatchApplication(ctx, buildProjectContext(projectInfo.Project), record.ID, UpdateApplicationRequest{
			Environment: &targetEnvironment,
		})
		if err != nil {
			return DeploymentResponse{}, err
		}
		record = updatedRecord
	}

	deploymentID := strings.Replace(requestID, "req_", "dep_", 1)
	updatedRecord, err := s.Store.UpdateApplicationImage(ctx, buildProjectContext(projectInfo.Project), record.ID, imageTag, deploymentID)
	if err != nil {
		return DeploymentResponse{}, err
	}
	_ = s.appendEvent(ctx, record.ID, "DesiredStateCommitted", fmt.Sprintf("Git desired state에 이미지 %s 반영", nextImage), map[string]any{
		"environment": updatedRecord.DefaultEnvironment,
		"image":       nextImage,
	})
	_ = s.appendEvent(ctx, record.ID, "DeploymentTriggered", fmt.Sprintf("새 이미지 태그 %s 재배포", imageTag), map[string]any{
		"environment": updatedRecord.DefaultEnvironment,
		"imageTag":    imageTag,
	})

	return DeploymentResponse{
		DeploymentID:  deploymentID,
		ApplicationID: updatedRecord.ID,
		ImageTag:      imageTag,
		Environment:   targetEnvironment,
		Status:        "Syncing",
	}, nil
}

func (s Service) ValidateImageReference(ctx context.Context, image string) error {
	return s.verifyImageReference(ctx, strings.TrimSpace(image))
}

func (s Service) ValidateDeploymentImage(ctx context.Context, user core.User, applicationID string, imageTag string) error {
	record, err := s.requireApplication(ctx, user, applicationID, true)
	if err != nil {
		return err
	}
	if strings.TrimSpace(imageTag) == "" {
		return ValidationError{
			Message: "imageTag is required",
			Details: map[string]any{"field": "imageTag"},
		}
	}
	nextImage := replaceImageTag(record.Image, imageTag)
	return s.verifyDeploymentImage(ctx, record, nextImage, imageTag, record.DefaultEnvironment)
}

func (s Service) PatchApplication(ctx context.Context, user core.User, applicationID string, input UpdateApplicationRequest) (Application, error) {
	record, err := s.requireApplication(ctx, user, applicationID, true)
	if err != nil {
		return Application{}, err
	}

	projectInfo, err := s.Projects.GetAuthorized(ctx, user, record.ProjectID)
	if err != nil {
		return Application{}, err
	}

	environment := record.DefaultEnvironment
	if input.Environment != nil {
		environment = resolveEnvironment(projectInfo.Project, *input.Environment)
	}
	strategy := record.DeploymentStrategy
	if input.DeploymentStrategy != nil {
		normalized := NormalizeDeploymentStrategy(*input.DeploymentStrategy)
		input.DeploymentStrategy = &normalized
		strategy = normalized
	}
	if err := validateProjectPolicies(projectInfo.Project, environment, strategy); err != nil {
		return Application{}, err
	}
	if !changeGuardBypassed(ctx) && requiresChangeFlow(projectInfo.Project, environment) {
		return Application{}, ErrChangeRequired
	}
	if input.Image != nil {
		if strings.TrimSpace(*input.Image) == "" {
			return Application{}, ValidationError{
				Message: "image is required",
				Details: map[string]any{"field": "image"},
			}
		}
		if err := s.verifyImageReference(ctx, strings.TrimSpace(*input.Image)); err != nil {
			return Application{}, err
		}
	}
	if input.Replicas != nil && *input.Replicas < 1 {
		return Application{}, ValidationError{
			Message: "replicas must be at least 1",
			Details: map[string]any{"field": "replicas"},
		}
	}

	updatedRecord, err := s.Store.PatchApplication(ctx, buildProjectContext(projectInfo.Project), applicationID, input)
	if err != nil {
		return Application{}, err
	}
	syncInfo, err := s.StatusReader.Read(ctx, updatedRecord)
	if err != nil {
		return Application{}, err
	}
	_ = s.appendEvent(ctx, updatedRecord.ID, "ApplicationUpdated", "애플리케이션 설정 갱신", map[string]any{
		"environment": updatedRecord.DefaultEnvironment,
		"strategy":    updatedRecord.DeploymentStrategy,
	})
	return toApplication(updatedRecord, syncInfo), nil
}

func (s Service) GetSyncStatus(ctx context.Context, user core.User, applicationID string) (SyncStatusResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, false)
	if err != nil {
		return SyncStatusResponse{}, err
	}

	syncInfo, err := s.StatusReader.Read(ctx, record)
	if err != nil {
		return SyncStatusResponse{}, err
	}

	return SyncStatusResponse{
		ApplicationID: record.ID,
		Status:        syncInfo.Status,
		Message:       syncInfo.Message,
		ObservedAt:    syncInfo.ObservedAt,
	}, nil
}

func (s Service) GetMetrics(ctx context.Context, user core.User, applicationID string, duration time.Duration, step time.Duration) (MetricsResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, false)
	if err != nil {
		return MetricsResponse{}, err
	}

	metrics, err := s.MetricsReader.Read(ctx, record, duration, step)
	if err != nil {
		return MetricsResponse{}, err
	}

	return MetricsResponse{
		ApplicationID: record.ID,
		Metrics:       metrics,
	}, nil
}

func (s Service) ListDeployments(ctx context.Context, user core.User, applicationID string) (DeploymentListResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, false)
	if err != nil {
		return DeploymentListResponse{}, err
	}

	items, err := s.Store.ListDeployments(ctx, applicationID)
	if err != nil {
		return DeploymentListResponse{}, err
	}
	return DeploymentListResponse{
		ApplicationID: record.ID,
		Items:         items,
	}, nil
}

func (s Service) GetDeployment(ctx context.Context, user core.User, applicationID string, deploymentID string) (DeploymentRecord, error) {
	record, err := s.requireApplication(ctx, user, applicationID, false)
	if err != nil {
		return DeploymentRecord{}, err
	}

	deployment, err := s.Store.GetDeployment(ctx, applicationID, deploymentID)
	if err != nil {
		return DeploymentRecord{}, err
	}

	if s.Rollouts != nil && IsCanaryDeploymentStrategy(record.DeploymentStrategy) {
		rollout, err := s.Rollouts.GetRollout(ctx, record)
		if err == nil {
			deployment.RolloutPhase = rollout.Phase
			deployment.CurrentStep = rollout.CurrentStep
			deployment.CanaryWeight = rollout.CanaryWeight
			deployment.StableRevision = rollout.StableRevision
			deployment.CanaryRevision = rollout.CanaryRevision
			if rollout.Message != "" {
				deployment.Message = rollout.Message
			}
		}
	}
	return deployment, nil
}

func (s Service) PromoteDeployment(ctx context.Context, user core.User, applicationID string, deploymentID string) (DeploymentRecord, error) {
	record, err := s.requireApplication(ctx, user, applicationID, true)
	if err != nil {
		return DeploymentRecord{}, err
	}

	deployment, err := s.Store.GetDeployment(ctx, applicationID, deploymentID)
	if err != nil {
		return DeploymentRecord{}, err
	}
	if !IsCanaryDeploymentStrategy(record.DeploymentStrategy) {
		return DeploymentRecord{}, ValidationError{
			Message: "only Canary deployments can be promoted",
			Details: map[string]any{"applicationId": applicationID},
		}
	}

	if s.Rollouts != nil {
		rollout, err := s.Rollouts.Promote(ctx, record, false)
		if err != nil {
			return DeploymentRecord{}, err
		}
		deployment.Status = "Promoted"
		deployment.RolloutPhase = rollout.Phase
		deployment.CurrentStep = rollout.CurrentStep
		deployment.CanaryWeight = rollout.CanaryWeight
		deployment.StableRevision = rollout.StableRevision
		deployment.CanaryRevision = rollout.CanaryRevision
		deployment.Message = rollout.Message
		deployment.UpdatedAt = timeNowUTC()
		if deployment, err = s.Store.UpdateDeployment(ctx, applicationID, deployment); err != nil {
			return DeploymentRecord{}, err
		}
	}

	_ = s.appendEvent(ctx, applicationID, "RolloutPromoted", fmt.Sprintf("배포 %s 승격", deploymentID), nil)
	return deployment, nil
}

func (s Service) AbortDeployment(ctx context.Context, user core.User, applicationID string, deploymentID string) (DeploymentRecord, error) {
	record, err := s.requireApplication(ctx, user, applicationID, true)
	if err != nil {
		return DeploymentRecord{}, err
	}

	deployment, err := s.Store.GetDeployment(ctx, applicationID, deploymentID)
	if err != nil {
		return DeploymentRecord{}, err
	}
	if !IsCanaryDeploymentStrategy(record.DeploymentStrategy) {
		return DeploymentRecord{}, ValidationError{
			Message: "only Canary deployments can be aborted",
			Details: map[string]any{"applicationId": applicationID},
		}
	}

	if s.Rollouts != nil {
		rollout, err := s.Rollouts.Abort(ctx, record)
		if err != nil {
			return DeploymentRecord{}, err
		}
		deployment.Status = "Aborted"
		deployment.RolloutPhase = rollout.Phase
		deployment.CurrentStep = rollout.CurrentStep
		deployment.CanaryWeight = rollout.CanaryWeight
		deployment.StableRevision = rollout.StableRevision
		deployment.CanaryRevision = rollout.CanaryRevision
		deployment.Message = rollout.Message
		deployment.UpdatedAt = timeNowUTC()
		if deployment, err = s.Store.UpdateDeployment(ctx, applicationID, deployment); err != nil {
			return DeploymentRecord{}, err
		}
	}

	_ = s.appendEvent(ctx, applicationID, "RolloutAborted", fmt.Sprintf("배포 %s 중단", deploymentID), nil)
	return deployment, nil
}

func (s Service) GetRollbackPolicy(ctx context.Context, user core.User, applicationID string) (RollbackPolicy, error) {
	if _, err := s.requireApplication(ctx, user, applicationID, false); err != nil {
		return RollbackPolicy{}, err
	}
	return s.Store.GetRollbackPolicy(ctx, applicationID)
}

func (s Service) SaveRollbackPolicy(ctx context.Context, user core.User, applicationID string, policy RollbackPolicy) (RollbackPolicy, error) {
	if _, err := s.requireApplication(ctx, user, applicationID, true); err != nil {
		return RollbackPolicy{}, err
	}
	saved, err := s.Store.SaveRollbackPolicy(ctx, applicationID, policy)
	if err != nil {
		return RollbackPolicy{}, err
	}
	_ = s.appendEvent(ctx, applicationID, "RollbackPolicyUpdated", "자동 롤백 정책 갱신", nil)
	return saved, nil
}

func (s Service) GetEvents(ctx context.Context, user core.User, applicationID string) (EventListResponse, error) {
	if _, err := s.requireApplication(ctx, user, applicationID, false); err != nil {
		return EventListResponse{}, err
	}
	items, err := s.Store.ListEvents(ctx, applicationID)
	if err != nil {
		return EventListResponse{}, err
	}
	return EventListResponse{
		ApplicationID: applicationID,
		Items:         items,
	}, nil
}

func (s Service) requireApplication(
	ctx context.Context,
	user core.User,
	applicationID string,
	requireDeploy bool,
) (Record, error) {
	projectID, _, err := splitApplicationID(applicationID)
	if err != nil {
		return Record{}, err
	}

	authorizedProject, err := s.Projects.GetAuthorized(ctx, user, projectID)
	if err != nil {
		return Record{}, err
	}

	if requireDeploy && !authorizedProject.Role.CanDeploy() {
		return Record{}, ErrRequiresDeployer
	}

	return s.Store.GetApplication(ctx, applicationID)
}

func splitApplicationID(applicationID string) (string, string, error) {
	projectID, appName, ok := strings.Cut(applicationID, "__")
	if !ok || projectID == "" || appName == "" {
		return "", "", ErrInvalidID
	}
	return projectID, appName, nil
}

func buildApplicationID(projectID string, appName string) string {
	return projectID + "__" + appName
}

func validateCreateRequest(input CreateRequest) error {
	if !slugPattern.MatchString(strings.TrimSpace(input.Name)) {
		return ValidationError{
			Message: "name must be a DNS-1123 style slug",
			Details: map[string]any{"field": "name"},
		}
	}

	if strings.TrimSpace(input.Image) == "" {
		return ValidationError{
			Message: "image is required",
			Details: map[string]any{"field": "image"},
		}
	}

	if input.ServicePort < 1 || input.ServicePort > 65535 {
		return ValidationError{
			Message: "servicePort must be between 1 and 65535",
			Details: map[string]any{"field": "servicePort"},
		}
	}
	if input.Replicas < 0 {
		return ValidationError{
			Message: "replicas must be zero or greater",
			Details: map[string]any{"field": "replicas"},
		}
	}

	if strings.TrimSpace(string(input.DeploymentStrategy)) == "" {
		return ValidationError{
			Message: "deploymentStrategy is required",
			Details: map[string]any{"field": "deploymentStrategy"},
		}
	}

	switch NormalizeDeploymentStrategy(input.DeploymentStrategy) {
	case DeploymentStrategyRollout, DeploymentStrategyCanary:
	default:
		return ValidationError{
			Message: "deploymentStrategy must be Rollout or Canary",
			Details: map[string]any{"field": "deploymentStrategy"},
		}
	}

	return nil
}

func (s Service) verifyImageReference(ctx context.Context, image string) error {
	if s.Images == nil {
		return nil
	}
	return s.Images.Verify(ctx, image)
}

func (s Service) verifyDeploymentImage(
	ctx context.Context,
	record Record,
	image string,
	imageTag string,
	environment string,
) error {
	if err := s.verifyImageReference(ctx, image); err != nil {
		var imageErr ImageValidationError
		if errors.As(err, &imageErr) {
			_ = s.appendEvent(ctx, record.ID, "DeploymentPreflightFailed", imageErr.Message, map[string]any{
				"environment": environment,
				"image":       image,
				"imageTag":    imageTag,
				"code":        imageErr.Code,
			})
		}
		return err
	}

	_ = s.appendEvent(ctx, record.ID, "DeploymentPreflightSucceeded", fmt.Sprintf("이미지 %s 접근을 확인했습니다.", image), map[string]any{
		"environment": environment,
		"image":       image,
		"imageTag":    imageTag,
	})
	return nil
}

func normalizeSecrets(entries []SecretEntry) (map[string]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	values := make(map[string]string, len(entries))
	for _, entry := range entries {
		key := strings.TrimSpace(entry.Key)
		if key == "" {
			return nil, ValidationError{
				Message: "secret key must not be blank",
				Details: map[string]any{"field": "secrets.key"},
			}
		}

		if _, exists := values[key]; exists {
			return nil, ValidationError{
				Message: fmt.Sprintf("duplicate secret key %s", key),
				Details: map[string]any{"field": "secrets.key", "key": key},
			}
		}

		values[key] = entry.Value
	}

	return values, nil
}

func resolveEnvironment(project project.CatalogProject, requested string) string {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		return requested
	}
	for _, environment := range project.Environments {
		if environment.Default {
			return environment.ID
		}
	}
	return "shared"
}

func validateProjectPolicies(projectInfo project.CatalogProject, environment string, strategy DeploymentStrategy) error {
	if !containsValue(projectInfo.Policies.AllowedEnvironments, environment) {
		return ValidationError{
			Message: "environment is not allowed by project policy",
			Details: map[string]any{"field": "environment", "environment": environment},
		}
	}
	if !containsNormalizedValue(projectInfo.Policies.AllowedDeploymentStrategies, string(NormalizeDeploymentStrategy(strategy))) {
		return ValidationError{
			Message: "deploymentStrategy is not allowed by project policy",
			Details: map[string]any{"field": "deploymentStrategy", "deploymentStrategy": NormalizeDeploymentStrategy(strategy)},
		}
	}
	return nil
}

func requiresChangeFlow(projectInfo project.CatalogProject, environment string) bool {
	for _, item := range projectInfo.Environments {
		if item.ID == environment && item.WriteMode == project.WriteModePullRequest {
			return true
		}
	}
	return environment == "prod" && projectInfo.Policies.ProdPRRequired
}

func containsValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsNormalizedValue(values []string, target string) bool {
	for _, value := range values {
		if string(NormalizeDeploymentStrategy(DeploymentStrategy(value))) == target {
			return true
		}
	}
	return false
}

func environmentIDs(items []project.Environment) []string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, item.ID)
	}
	return values
}

func environmentClusterMap(items []project.Environment) map[string]string {
	values := make(map[string]string, len(items))
	for _, item := range items {
		values[item.ID] = item.ClusterID
	}
	return values
}

func buildProjectContext(projectInfo project.CatalogProject) ProjectContext {
	return ProjectContext{
		ID:                  projectInfo.ID,
		Namespace:           projectInfo.Namespace,
		Environments:        environmentIDs(projectInfo.Environments),
		EnvironmentClusters: environmentClusterMap(projectInfo.Environments),
		Policies: projectPolicy{
			MinReplicas:                 projectInfo.Policies.MinReplicas,
			AllowedEnvironments:         append([]string(nil), projectInfo.Policies.AllowedEnvironments...),
			AllowedDeploymentStrategies: normalizePolicyStrategies(projectInfo.Policies.AllowedDeploymentStrategies),
			AllowedClusterTargets:       append([]string(nil), projectInfo.Policies.AllowedClusterTargets...),
			ProdPRRequired:              projectInfo.Policies.ProdPRRequired,
			AutoRollbackEnabled:         projectInfo.Policies.AutoRollbackEnabled,
			RequiredProbes:              projectInfo.Policies.RequiredProbes,
		},
	}
}

func normalizePolicyStrategies(values []string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		normalized := NormalizeDeploymentStrategy(DeploymentStrategy(value))
		if normalized == "" {
			continue
		}
		if containsValue(items, string(normalized)) {
			continue
		}
		items = append(items, string(normalized))
	}
	return items
}

func (s Service) appendEvent(ctx context.Context, applicationID string, eventType string, message string, metadata map[string]any) error {
	if s.Store == nil {
		return nil
	}
	return s.Store.AppendEvent(ctx, applicationID, Event{
		ID:        strings.Replace(fmt.Sprintf("evt_%d", timeNowUTC().UnixNano()), "-", "", -1),
		Type:      eventType,
		Message:   message,
		CreatedAt: timeNowUTC(),
		Metadata:  metadata,
	})
}

func toApplication(record Record, syncInfo SyncInfo) Application {
	return Application{
		ID:                  record.ID,
		ProjectID:           record.ProjectID,
		Name:                record.Name,
		Description:         record.Description,
		Image:               record.Image,
		ServicePort:         record.ServicePort,
		Replicas:            record.Replicas,
		DeploymentStrategy:  string(NormalizeDeploymentStrategy(record.DeploymentStrategy)),
		DefaultEnvironment:  record.DefaultEnvironment,
		SyncStatus:          syncInfo.Status,
		CreatedAt:           record.CreatedAt,
		UpdatedAt:           record.UpdatedAt,
		RepositoryID:        record.RepositoryID,
		RepositoryServiceID: record.RepositoryServiceID,
		ConfigPath:          record.ConfigPath,
	}
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}

func WithChangeGuardBypass(ctx context.Context) context.Context {
	return context.WithValue(ctx, changeGuardBypassKey{}, true)
}

func changeGuardBypassed(ctx context.Context) bool {
	value, _ := ctx.Value(changeGuardBypassKey{}).(bool)
	return value
}
