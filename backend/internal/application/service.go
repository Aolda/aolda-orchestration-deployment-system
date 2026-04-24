package application

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/project"
)

var (
	ErrNotFound           = errors.New("application not found")
	ErrConflict           = errors.New("application conflict")
	ErrArchived           = errors.New("application archived")
	ErrAlreadyArchived    = errors.New("application already archived")
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
	ArchiveApplication(ctx context.Context, applicationID string, archivedBy string) (ApplicationLifecycleResponse, error)
	DeleteApplication(ctx context.Context, applicationID string) (ApplicationLifecycleResponse, error)
	UpdateApplicationImage(ctx context.Context, project ProjectContext, applicationID string, imageTag string, deploymentID string) (Record, error)
	PatchApplication(ctx context.Context, project ProjectContext, applicationID string, input UpdateApplicationRequest) (Record, error)
	SaveApplicationSecretPath(ctx context.Context, project ProjectContext, applicationID string, secretPath string) (Record, error)
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

type NetworkExposureReader interface {
	Read(ctx context.Context, record Record) (NetworkExposureInfo, error)
}

type LogsReader interface {
	ListTargets(ctx context.Context, record Record) ([]ContainerLogTarget, error)
	Read(ctx context.Context, record Record, tailLines int) ([]ContainerLogStream, error)
	Stream(ctx context.Context, record Record, podName string, containerName string, tailLines int, emit func(ContainerLogEvent) error) error
}

type RolloutController interface {
	GetRollout(ctx context.Context, record Record) (RolloutInfo, error)
	Promote(ctx context.Context, record Record, full bool) (RolloutInfo, error)
	Abort(ctx context.Context, record Record) (RolloutInfo, error)
}

type SecretStore interface {
	Stage(ctx context.Context, requestID string, projectID string, appName string, createdBy string, data map[string]string) (StagedSecret, error)
	StageAt(ctx context.Context, requestID string, finalPath string, metadata map[string]string, data map[string]string) (StagedSecret, error)
	Finalize(ctx context.Context, staged StagedSecret, data map[string]string) error
	Get(ctx context.Context, logicalPath string) (map[string]string, error)
	Delete(ctx context.Context, logicalPath string) error
}

type VersionedSecretStore interface {
	ListVersions(ctx context.Context, logicalPath string) (ApplicationSecretVersionsResponse, error)
	GetVersion(ctx context.Context, logicalPath string, version int) (map[string]string, SecretVersionSummary, error)
}

type Service struct {
	Projects              *project.Service
	Store                 Store
	StatusReader          StatusReader
	MetricsReader         MetricsReader
	NetworkExposureReader NetworkExposureReader
	LogsReader            LogsReader
	Secrets               SecretStore
	Rollouts              RolloutController
	Images                ImageVerifier
	HTTPClient            *http.Client
	PollTracker           *RepositoryPollTracker
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
			ID:                  record.ID,
			Name:                record.Name,
			Image:               record.Image,
			DeploymentStrategy:  string(NormalizeDeploymentStrategy(record.DeploymentStrategy)),
			SyncStatus:          syncInfo.Status,
			Resources:           record.Resources,
			MeshEnabled:         record.MeshEnabled,
			LoadBalancerEnabled: record.LoadBalancerEnabled,
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

	if err := s.hydrateRepositorySource(ctx, &input); err != nil {
		return Application{}, err
	}
	if err := validateCreateRequest(input); err != nil {
		return Application{}, err
	}
	if err := normalizeRepositoryMetadata(authorizedProject.Project, &input); err != nil {
		return Application{}, err
	}
	input.DeploymentStrategy = NormalizeDeploymentStrategy(input.DeploymentStrategy)
	normalizeCreateNetworkProfile(&input)
	if err := validateNetworkProfile(input.DeploymentStrategy, input.MeshEnabled, input.LoadBalancerEnabled); err != nil {
		return Application{}, err
	}
	registryCredential, dockerConfigJSON, err := normalizeRegistryCredentialInput(
		strings.TrimSpace(input.Image),
		input.RegistryServer,
		input.RegistryUsername,
		input.RegistryToken,
	)
	if err != nil {
		return Application{}, err
	}
	if registryCredential != nil {
		input.RegistryServer = registryCredential.Server
		input.RegistryUsername = registryCredential.Username
	}
	if err := s.verifyImageReference(ctx, strings.TrimSpace(input.Image), registryCredential); err != nil {
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
	stagedSecrets := make([]StagedSecret, 0, 3)
	if len(secretData) > 0 && s.Secrets != nil {
		secretPath = core.BuildVaultFinalPath(projectID, input.Name)
		staged, err := s.Secrets.Stage(ctx, requestID, projectID, input.Name, user.Username, secretData)
		if err != nil {
			return Application{}, err
		}
		secretPath = staged.FinalPath
		stagedSecrets = append(stagedSecrets, staged)
	}

	repositoryToken := strings.TrimSpace(input.RepositoryToken)
	if repositoryToken != "" && s.Secrets != nil {
		repositoryTokenPath := core.BuildVaultRepositoryTokenPath(projectID, input.Name)
		staged, err := s.Secrets.StageAt(ctx, requestID+"_repository", repositoryTokenPath, map[string]string{
			"projectId": projectID,
			"appName":   input.Name,
			"createdBy": user.Username,
			"kind":      "repository-token",
		}, map[string]string{
			"token": repositoryToken,
		})
		if err != nil {
			return Application{}, err
		}
		input.RepositoryTokenPath = staged.FinalPath
		stagedSecrets = append(stagedSecrets, staged)
	}

	if registryCredential != nil && s.Secrets != nil {
		registrySecretPath := core.BuildVaultRegistryCredentialPath(projectID, input.Name)
		staged, err := s.Secrets.StageAt(ctx, requestID+"_registry", registrySecretPath, map[string]string{
			"projectId": projectID,
			"appName":   input.Name,
			"createdBy": user.Username,
			"kind":      "registry-credential",
		}, map[string]string{
			"server":           registryCredential.Server,
			"username":         registryCredential.Username,
			"password":         registryCredential.Password,
			"dockerconfigjson": dockerConfigJSON,
		})
		if err != nil {
			return Application{}, err
		}
		input.RegistrySecretPath = staged.FinalPath
		stagedSecrets = append(stagedSecrets, staged)
	}

	record, err := s.Store.CreateApplication(ctx, buildProjectContext(authorizedProject.Project), input, secretPath)
	if err != nil {
		return Application{}, err
	}
	record.RepositoryID = input.RepositoryID
	record.RepositoryURL = input.RepositoryURL
	record.RepositoryBranch = input.RepositoryBranch
	record.RepositoryServiceID = input.RepositoryServiceID
	record.ConfigPath = input.ConfigPath
	record.RepositoryTokenPath = input.RepositoryTokenPath
	record.RegistrySecretPath = input.RegistrySecretPath

	for _, staged := range stagedSecrets {
		payload := secretData
		if staged.FinalPath == input.RepositoryTokenPath {
			payload = map[string]string{"token": repositoryToken}
		}
		if staged.FinalPath == input.RegistrySecretPath {
			payload = map[string]string{
				"server":           registryCredential.Server,
				"username":         registryCredential.Username,
				"password":         registryCredential.Password,
				"dockerconfigjson": dockerConfigJSON,
			}
		}
		if err := s.Secrets.Finalize(ctx, staged, payload); err != nil {
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

func (s Service) PreviewRepositorySource(
	ctx context.Context,
	user core.User,
	projectID string,
	input PreviewRepositorySourceRequest,
) (PreviewRepositorySourceResponse, error) {
	authorizedProject, err := s.Projects.GetAuthorized(ctx, user, projectID)
	if err != nil {
		return PreviewRepositorySourceResponse{}, err
	}
	if !authorizedProject.Role.CanDeploy() {
		return PreviewRepositorySourceResponse{}, ErrRequiresDeployer
	}

	createInput := CreateRequest{
		Name:                strings.TrimSpace(input.Name),
		RepositoryURL:       strings.TrimSpace(input.RepositoryURL),
		RepositoryBranch:    strings.TrimSpace(input.RepositoryBranch),
		RepositoryToken:     strings.TrimSpace(input.RepositoryToken),
		RepositoryServiceID: strings.TrimSpace(input.RepositoryServiceID),
		ConfigPath:          strings.TrimSpace(input.ConfigPath),
	}
	if err := normalizeRepositoryMetadata(authorizedProject.Project, &createInput); err != nil {
		return PreviewRepositorySourceResponse{}, err
	}

	descriptor, err := s.readRepositoryDescriptor(ctx, createInput)
	if err != nil {
		return PreviewRepositorySourceResponse{}, err
	}

	items := make([]PreviewRepositoryService, 0, len(descriptor.Services))
	for _, service := range descriptor.Services {
		items = append(items, PreviewRepositoryService{
			ServiceID: service.ServiceID,
			Image:     service.Image,
			Port:      service.Port,
			Replicas:  service.Replicas,
			Strategy:  NormalizeDeploymentStrategy(service.Strategy),
		})
	}

	selectedServiceID := ""
	if resolved, ok := descriptor.resolveService(Record{
		Name:                createInput.Name,
		RepositoryServiceID: createInput.RepositoryServiceID,
	}); ok {
		selectedServiceID = resolved.ServiceID
	}

	return PreviewRepositorySourceResponse{
		ConfigPath:               createInput.ConfigPath,
		Services:                 items,
		SelectedServiceID:        selectedServiceID,
		RequiresServiceSelection: len(items) > 1 && selectedServiceID == "",
	}, nil
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
	return s.verifyImageReference(ctx, strings.TrimSpace(image), nil)
}

func (s Service) ValidateImageReferenceWithCredential(
	ctx context.Context,
	image string,
	registryServer string,
	registryUsername string,
	registryToken string,
) error {
	credential, _, err := normalizeRegistryCredentialInput(image, registryServer, registryUsername, registryToken)
	if err != nil {
		return err
	}
	return s.verifyImageReference(ctx, strings.TrimSpace(image), credential)
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
	networkSettingsChanged := input.MeshEnabled != nil || input.LoadBalancerEnabled != nil
	if input.Resources != nil || networkSettingsChanged {
		if _, _, err := s.requireProjectAdmin(ctx, user, applicationID); err != nil {
			return Application{}, err
		}
	}
	if input.Resources != nil {
		normalizedResources, err := normalizeResourceRequirements(*input.Resources, false)
		if err != nil {
			return Application{}, err
		}
		input.Resources = &normalizedResources
	}
	if input.DeploymentStrategy != nil {
		normalized := NormalizeDeploymentStrategy(*input.DeploymentStrategy)
		input.DeploymentStrategy = &normalized
	}
	nextStrategy := record.DeploymentStrategy
	if input.DeploymentStrategy != nil {
		nextStrategy = NormalizeDeploymentStrategy(*input.DeploymentStrategy)
	}
	nextMeshEnabled := record.MeshEnabled
	if input.MeshEnabled != nil {
		nextMeshEnabled = *input.MeshEnabled
	}
	nextLoadBalancerEnabled := record.LoadBalancerEnabled
	if input.LoadBalancerEnabled != nil {
		nextLoadBalancerEnabled = *input.LoadBalancerEnabled
	}
	if err := validateNetworkProfile(nextStrategy, nextMeshEnabled, nextLoadBalancerEnabled); err != nil {
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
		strategy = *input.DeploymentStrategy
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
		registryCredential, err := s.resolveRecordRegistryCredential(ctx, record)
		if err != nil {
			return Application{}, err
		}
		if err := s.verifyImageReference(ctx, strings.TrimSpace(*input.Image), registryCredential); err != nil {
			return Application{}, err
		}
	}
	if input.Replicas != nil && *input.Replicas < 1 {
		return Application{}, ValidationError{
			Message: "replicas must be at least 1",
			Details: map[string]any{"field": "replicas"},
		}
	}
	if input.RepositoryPollIntervalSeconds != nil {
		if !appHasRepositorySource(record) {
			return Application{}, ValidationError{
				Message: "repositoryPollIntervalSeconds requires a repository-backed application",
				Details: map[string]any{"field": "repositoryPollIntervalSeconds"},
			}
		}
		if err := validateRepositoryPollIntervalSeconds(*input.RepositoryPollIntervalSeconds); err != nil {
			return Application{}, err
		}
	}

	updatedRecord, err := s.Store.PatchApplication(ctx, buildProjectContext(projectInfo.Project), applicationID, input)
	if err != nil {
		return Application{}, err
	}
	if s.PollTracker != nil && input.RepositoryPollIntervalSeconds != nil {
		s.PollTracker.Reschedule(updatedRecord, timeNowUTC())
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

func (s Service) GetApplicationSecrets(ctx context.Context, user core.User, applicationID string) (ApplicationSecretsResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, true)
	if err != nil {
		return ApplicationSecretsResponse{}, err
	}
	if s.Secrets == nil {
		return ApplicationSecretsResponse{}, ValidationError{
			Message: "vault secret store is not configured",
			Details: map[string]any{"applicationId": applicationID},
		}
	}

	secretPath := strings.TrimSpace(record.SecretPath)
	configured := secretPath != ""
	if secretPath == "" {
		secretPath = core.BuildVaultFinalPath(record.ProjectID, record.Name)
	}

	values := map[string]string{}
	if configured {
		stored, err := s.Secrets.Get(ctx, secretPath)
		if err != nil {
			return ApplicationSecretsResponse{}, fmt.Errorf("read application secrets: %w", err)
		}
		values = cloneSecretData(stored)
	}

	response, err := s.buildApplicationSecretsResponse(ctx, record.ID, secretPath, configured, values)
	if err != nil {
		return ApplicationSecretsResponse{}, err
	}
	return response, nil
}

func (s Service) UpdateApplicationSecrets(
	ctx context.Context,
	user core.User,
	applicationID string,
	input UpdateSecretsRequest,
	requestID string,
) (ApplicationSecretsResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, true)
	if err != nil {
		return ApplicationSecretsResponse{}, err
	}
	if s.Secrets == nil {
		return ApplicationSecretsResponse{}, ValidationError{
			Message: "vault secret store is not configured",
			Details: map[string]any{"applicationId": applicationID},
		}
	}

	projectInfo, err := s.Projects.GetAuthorized(ctx, user, record.ProjectID)
	if err != nil {
		return ApplicationSecretsResponse{}, err
	}
	if !changeGuardBypassed(ctx) && requiresChangeFlow(projectInfo.Project, record.DefaultEnvironment) {
		return ApplicationSecretsResponse{}, ErrChangeRequired
	}

	setValues, err := normalizeSecrets(input.Set)
	if err != nil {
		return ApplicationSecretsResponse{}, err
	}
	deleteKeys, err := normalizeSecretDeleteKeys(input.Delete, setValues)
	if err != nil {
		return ApplicationSecretsResponse{}, err
	}

	currentPath := strings.TrimSpace(record.SecretPath)
	secretPath := currentPath
	if secretPath == "" {
		secretPath = core.BuildVaultFinalPath(record.ProjectID, record.Name)
	}

	currentValues := map[string]string{}
	if currentPath != "" {
		stored, err := s.Secrets.Get(ctx, currentPath)
		if err != nil {
			return ApplicationSecretsResponse{}, fmt.Errorf("read application secrets: %w", err)
		}
		currentValues = cloneSecretData(stored)
	}

	nextValues := cloneSecretData(currentValues)
	for _, key := range deleteKeys {
		delete(nextValues, key)
	}
	for key, value := range setValues {
		nextValues[key] = value
	}

	nextSecretPath := ""
	if len(nextValues) > 0 {
		nextSecretPath = secretPath
	}
	if nextSecretPath != currentPath {
		record, err = s.Store.SaveApplicationSecretPath(ctx, buildProjectContext(projectInfo.Project), applicationID, nextSecretPath)
		if err != nil {
			return ApplicationSecretsResponse{}, err
		}
	}

	if len(nextValues) > 0 {
		staged, err := s.Secrets.StageAt(ctx, requestID+"_env", nextSecretPath, map[string]string{
			"projectId": record.ProjectID,
			"appName":   record.Name,
			"updatedBy": user.Username,
			"kind":      "application-env",
		}, nextValues)
		if err != nil {
			return ApplicationSecretsResponse{}, err
		}
		if err := s.Secrets.Finalize(ctx, staged, nextValues); err != nil {
			return ApplicationSecretsResponse{}, err
		}
	} else if currentPath != "" {
		if err := s.Secrets.Delete(ctx, currentPath); err != nil {
			return ApplicationSecretsResponse{}, err
		}
	}

	_ = s.appendEvent(ctx, record.ID, "ApplicationSecretsUpdated", "애플리케이션 환경 변수를 갱신했습니다.", map[string]any{
		"secretPath": nextSecretPath,
		"keyCount":   len(nextValues),
	})

	response, err := s.buildApplicationSecretsResponse(ctx, record.ID, secretPath, nextSecretPath != "", nextValues)
	if err != nil {
		return ApplicationSecretsResponse{}, err
	}
	return response, nil
}

func (s Service) ListApplicationSecretVersions(ctx context.Context, user core.User, applicationID string) (ApplicationSecretVersionsResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, true)
	if err != nil {
		return ApplicationSecretVersionsResponse{}, err
	}
	secretPath := strings.TrimSpace(record.SecretPath)
	if secretPath == "" {
		return ApplicationSecretVersionsResponse{
			ApplicationID: applicationID,
			SecretPath:    core.BuildVaultFinalPath(record.ProjectID, record.Name),
			Items:         []SecretVersionSummary{},
		}, nil
	}
	versioned, ok := s.Secrets.(VersionedSecretStore)
	if !ok {
		return ApplicationSecretVersionsResponse{
			ApplicationID: applicationID,
			SecretPath:    secretPath,
			Items:         []SecretVersionSummary{},
		}, nil
	}

	response, err := versioned.ListVersions(ctx, secretPath)
	if err != nil {
		return ApplicationSecretVersionsResponse{}, fmt.Errorf("read application secret versions: %w", err)
	}
	response.ApplicationID = applicationID
	response.SecretPath = secretPath
	if response.Items == nil {
		response.Items = []SecretVersionSummary{}
	}
	return response, nil
}

func (s Service) RestoreApplicationSecretVersion(
	ctx context.Context,
	user core.User,
	applicationID string,
	version int,
	requestID string,
) (ApplicationSecretsResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, true)
	if err != nil {
		return ApplicationSecretsResponse{}, err
	}
	if version <= 0 {
		return ApplicationSecretsResponse{}, ValidationError{
			Message: "secret version must be a positive integer",
			Details: map[string]any{"field": "version"},
		}
	}
	secretPath := strings.TrimSpace(record.SecretPath)
	if secretPath == "" {
		return ApplicationSecretsResponse{}, ValidationError{
			Message: "application has no configured environment secret",
			Details: map[string]any{"applicationId": applicationID},
		}
	}
	versioned, ok := s.Secrets.(VersionedSecretStore)
	if !ok {
		return ApplicationSecretsResponse{}, ValidationError{
			Message: "vault version history is not available",
			Details: map[string]any{"applicationId": applicationID},
		}
	}

	projectInfo, err := s.Projects.GetAuthorized(ctx, user, record.ProjectID)
	if err != nil {
		return ApplicationSecretsResponse{}, err
	}
	if !changeGuardBypassed(ctx) && requiresChangeFlow(projectInfo.Project, record.DefaultEnvironment) {
		return ApplicationSecretsResponse{}, ErrChangeRequired
	}

	values, info, err := versioned.GetVersion(ctx, secretPath, version)
	if err != nil {
		return ApplicationSecretsResponse{}, fmt.Errorf("read application secret version: %w", err)
	}
	if info.Deleted || info.Destroyed {
		return ApplicationSecretsResponse{}, ValidationError{
			Message: "secret version cannot be restored because it is deleted or destroyed",
			Details: map[string]any{"version": version},
		}
	}

	staged, err := s.Secrets.StageAt(ctx, requestID+"_env_restore", secretPath, map[string]string{
		"projectId":       record.ProjectID,
		"appName":         record.Name,
		"updatedBy":       user.Username,
		"kind":            "application-env-restore",
		"restoredVersion": fmt.Sprintf("%d", version),
	}, values)
	if err != nil {
		return ApplicationSecretsResponse{}, err
	}
	if err := s.Secrets.Finalize(ctx, staged, values); err != nil {
		return ApplicationSecretsResponse{}, err
	}

	_ = s.appendEvent(ctx, record.ID, "ApplicationSecretsRestored", fmt.Sprintf("환경 변수를 Vault version %d 기준으로 복원했습니다.", version), map[string]any{
		"secretPath":       secretPath,
		"restoredVersion":  version,
		"restoredKeyCount": len(values),
	})

	response, err := s.buildApplicationSecretsResponse(ctx, record.ID, secretPath, true, values)
	if err != nil {
		return ApplicationSecretsResponse{}, err
	}
	return response, nil
}

func (s Service) ArchiveApplication(ctx context.Context, user core.User, applicationID string) (ApplicationLifecycleResponse, error) {
	if _, _, err := s.requireProjectAdmin(ctx, user, applicationID); err != nil {
		return ApplicationLifecycleResponse{}, err
	}

	result, err := s.Store.ArchiveApplication(ctx, applicationID, user.Username)
	if err != nil {
		return ApplicationLifecycleResponse{}, err
	}
	archivedAt := ""
	if result.ArchivedAt != nil {
		archivedAt = result.ArchivedAt.UTC().Format(time.RFC3339)
	}
	_ = s.appendEvent(ctx, applicationID, "ApplicationArchived", "애플리케이션을 보관 처리했습니다.", map[string]any{
		"archivedAt": archivedAt,
	})
	return result, nil
}

func (s Service) DeleteApplication(ctx context.Context, user core.User, applicationID string) (ApplicationLifecycleResponse, error) {
	if _, _, err := s.requireProjectAdmin(ctx, user, applicationID); err != nil {
		return ApplicationLifecycleResponse{}, err
	}

	result, err := s.Store.DeleteApplication(ctx, applicationID)
	if err != nil {
		return ApplicationLifecycleResponse{}, err
	}

	if s.Secrets != nil {
		for _, secretPath := range result.secretPaths {
			if strings.TrimSpace(secretPath) == "" {
				continue
			}
			if err := s.Secrets.Delete(ctx, secretPath); err != nil {
				return ApplicationLifecycleResponse{}, err
			}
		}
	}

	return result, nil
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
		ApplicationID:  record.ID,
		Status:         syncInfo.Status,
		Message:        syncInfo.Message,
		ObservedAt:     syncInfo.ObservedAt,
		RepositoryPoll: s.PollTracker.Snapshot(record),
	}, nil
}

func (s Service) SyncRepositoryNow(ctx context.Context, user core.User, applicationID string) (RepositorySyncResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, true)
	if err != nil {
		return RepositorySyncResponse{}, err
	}

	projectInfo, err := s.Projects.GetAuthorized(ctx, user, record.ProjectID)
	if err != nil {
		return RepositorySyncResponse{}, err
	}

	repoMap := make(map[string]project.Repository, len(projectInfo.Project.Repositories))
	for _, repo := range projectInfo.Project.Repositories {
		repoMap[repo.ID] = repo
	}

	poller := AutoUpdatePoller{
		Service:  &s,
		Projects: s.Projects,
		Interval: s.defaultRepositoryPollInterval(),
		Client:   s.httpClient(),
	}

	repo, ok := poller.repositoryForApp(record, repoMap)
	if !ok {
		return RepositorySyncResponse{}, ValidationError{
			Message: "repository sync is only available for repository-backed applications",
			Details: map[string]any{"applicationId": applicationID},
		}
	}

	return poller.SyncRepositoryNow(ctx, user, projectInfo.Project, record, repo)
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

func (s Service) GetProjectHealth(ctx context.Context, user core.User, projectID string) (ProjectHealthResponse, error) {
	if _, err := s.Projects.GetAuthorized(ctx, user, projectID); err != nil {
		return ProjectHealthResponse{}, err
	}

	records, err := s.Store.ListApplications(ctx, projectID)
	if err != nil {
		return ProjectHealthResponse{}, err
	}

	observedAt := timeNowUTC()
	syncInfos, syncErr := s.readSyncInfoMap(ctx, records)
	items := make([]ApplicationHealthSnapshot, 0, len(records))
	for _, record := range records {
		signals := make([]HealthSignal, 0, 3)

		if syncErr != nil {
			signals = append(signals, HealthSignal{
				Key:        "sync",
				Status:     HealthSignalUnavailable,
				Message:    syncErr.Error(),
				ObservedAt: observedAt,
			})
		} else {
			syncInfo, ok := syncInfos[record.ID]
			if !ok {
				syncInfo = SyncInfo{
					Status:     SyncStatusUnknown,
					Message:    "sync 상태를 읽지 못했습니다.",
					ObservedAt: observedAt,
				}
			}
			signals = append(signals, syncHealthSignal(syncInfo))
		}

		diagnostics, metrics := s.readMetricsDiagnostics(ctx, record, 15*time.Minute, time.Minute)
		signals = append(signals, HealthSignal{
			Key:        "metrics",
			Status:     diagnostics.Status,
			Message:    diagnostics.Message,
			ObservedAt: diagnostics.CheckedAt,
			Details: map[string]any{
				"series":        len(diagnostics.Series),
				"scrapeTargets": len(diagnostics.ScrapeTargets),
			},
		})

		latestDeployment, deploymentSignal := s.latestDeploymentHealthSignal(ctx, record, observedAt)
		signals = append(signals, deploymentSignal)

		syncStatus := SyncStatusUnknown
		if syncInfo, ok := syncInfos[record.ID]; ok && syncErr == nil {
			syncStatus = syncInfo.Status
		}

		items = append(items, ApplicationHealthSnapshot{
			ApplicationID:      record.ID,
			Name:               record.Name,
			Namespace:          record.Namespace,
			Status:             deriveHealthStatus(signals),
			SyncStatus:         syncStatus,
			DeploymentStrategy: string(record.DeploymentStrategy),
			Metrics:            metrics,
			LatestDeployment:   latestDeployment,
			Signals:            signals,
		})
	}

	return ProjectHealthResponse{
		ProjectID:  projectID,
		ObservedAt: observedAt,
		Items:      items,
	}, nil
}

func (s Service) GetMetricsDiagnostics(ctx context.Context, user core.User, applicationID string, duration time.Duration, step time.Duration) (MetricsDiagnosticsResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, false)
	if err != nil {
		return MetricsDiagnosticsResponse{}, err
	}
	diagnostics, _ := s.readMetricsDiagnostics(ctx, record, duration, step)
	return diagnostics, nil
}

func (s Service) readSyncInfoMap(ctx context.Context, records []Record) (map[string]SyncInfo, error) {
	items := make(map[string]SyncInfo, len(records))
	if len(records) == 0 {
		return items, nil
	}
	if s.StatusReader == nil {
		return items, fmt.Errorf("sync status reader is not configured")
	}
	if batchReader, ok := s.StatusReader.(BatchStatusReader); ok {
		return batchReader.ReadMany(ctx, records)
	}
	for _, record := range records {
		info, err := s.StatusReader.Read(ctx, record)
		if err != nil {
			return items, err
		}
		items[record.ID] = info
	}
	return items, nil
}

func syncHealthSignal(info SyncInfo) HealthSignal {
	status := HealthSignalUnknown
	switch info.Status {
	case SyncStatusSynced:
		status = HealthSignalOK
	case SyncStatusDegraded:
		status = HealthSignalCritical
	case SyncStatusSyncing, SyncStatusUnknown:
		status = HealthSignalWarning
	}
	message := strings.TrimSpace(info.Message)
	if message == "" {
		message = "Flux sync 상태를 읽었습니다."
	}
	return HealthSignal{
		Key:        "sync",
		Status:     status,
		Message:    message,
		ObservedAt: info.ObservedAt,
		Details: map[string]any{
			"syncStatus": info.Status,
		},
	}
}

func (s Service) readMetricsDiagnostics(ctx context.Context, record Record, duration time.Duration, step time.Duration) (MetricsDiagnosticsResponse, []MetricSeries) {
	checkedAt := timeNowUTC()
	response := MetricsDiagnosticsResponse{
		ApplicationID: record.ID,
		CheckedAt:     checkedAt,
		Status:        HealthSignalUnavailable,
		Message:       "metrics reader is not configured",
		MeshEnabled:   record.MeshEnabled,
		ScrapeTargets: metricsScrapeTargets(record),
		Series:        []MetricSeriesDiagnostic{},
	}
	if s.MetricsReader == nil {
		return response, nil
	}

	metrics, err := s.MetricsReader.Read(ctx, record, duration, step)
	if err != nil {
		response.Message = err.Error()
		return response, nil
	}

	response.Series = diagnoseMetricSeries(metrics)
	response.Status, response.Message = summarizeMetricDiagnostics(response.Series)
	return response, metrics
}

func (s Service) latestDeploymentHealthSignal(ctx context.Context, record Record, observedAt time.Time) (*DeploymentRecord, HealthSignal) {
	deployments, err := s.Store.ListDeployments(ctx, record.ID)
	if err != nil {
		return nil, HealthSignal{
			Key:        "deployment",
			Status:     HealthSignalUnavailable,
			Message:    err.Error(),
			ObservedAt: observedAt,
		}
	}
	if len(deployments) == 0 {
		return nil, HealthSignal{
			Key:        "deployment",
			Status:     HealthSignalOK,
			Message:    "아직 기록된 배포 이력이 없습니다.",
			ObservedAt: observedAt,
		}
	}

	latest := deployments[0]
	status := HealthSignalOK
	message := "최근 배포 이력을 읽었습니다."
	switch strings.ToLower(strings.TrimSpace(latest.Status)) {
	case "failed", "degraded":
		status = HealthSignalCritical
		message = "최근 배포가 실패 상태입니다."
	case "aborted", "autorollbacktriggered":
		status = HealthSignalWarning
		message = "최근 배포가 중단되었거나 자동 롤백을 트리거했습니다."
	}

	return &latest, HealthSignal{
		Key:        "deployment",
		Status:     status,
		Message:    message,
		ObservedAt: latest.UpdatedAt,
		Details: map[string]any{
			"deploymentId": latest.DeploymentID,
			"status":       latest.Status,
			"imageTag":     latest.ImageTag,
		},
	}
}

func metricsScrapeTargets(record Record) []MetricsScrapeTarget {
	targets := []MetricsScrapeTarget{
		{
			Name:     "application-http",
			Port:     "http",
			Path:     defaultMetricsPath,
			Required: true,
		},
	}
	if record.MeshEnabled {
		targets = append(targets, MetricsScrapeTarget{
			Name:     "istio-envoy",
			Port:     "envoy-metrics",
			Path:     defaultEnvoyMetricsPath,
			Required: true,
		})
	}
	return targets
}

func diagnoseMetricSeries(metrics []MetricSeries) []MetricSeriesDiagnostic {
	items := make([]MetricSeriesDiagnostic, 0, len(metrics))
	for _, series := range metrics {
		valueCount := 0
		var latestValue *float64
		for _, point := range series.Points {
			if point.Value == nil {
				continue
			}
			valueCount++
			valueCopy := *point.Value
			latestValue = &valueCopy
		}
		status := HealthSignalOK
		message := "series has recent values"
		if len(series.Points) == 0 {
			status = HealthSignalWarning
			message = "series has no sampled points"
		} else if valueCount == 0 {
			status = HealthSignalWarning
			message = "series is present but all sampled values are empty"
		}
		items = append(items, MetricSeriesDiagnostic{
			Key:         series.Key,
			Label:       series.Label,
			Unit:        series.Unit,
			Status:      status,
			Message:     message,
			PointCount:  len(series.Points),
			ValueCount:  valueCount,
			LatestValue: latestValue,
		})
	}
	return items
}

func summarizeMetricDiagnostics(series []MetricSeriesDiagnostic) (HealthSignalStatus, string) {
	if len(series) == 0 {
		return HealthSignalUnavailable, "수집된 metric series가 없습니다."
	}
	valueSeries := 0
	warningSeries := 0
	for _, item := range series {
		if item.ValueCount > 0 {
			valueSeries++
		}
		if item.Status != HealthSignalOK {
			warningSeries++
		}
	}
	switch {
	case valueSeries == 0:
		return HealthSignalWarning, "metric series는 있으나 값이 비어 있습니다. ServiceMonitor target, /metrics endpoint, label 매칭을 확인해야 합니다."
	case warningSeries > 0:
		return HealthSignalWarning, "일부 metric series가 비어 있습니다."
	default:
		return HealthSignalOK, "metric series가 정상적으로 수집되고 있습니다."
	}
}

func deriveHealthStatus(signals []HealthSignal) HealthStatus {
	if len(signals) == 0 {
		return HealthStatusUnknown
	}
	hasWarning := false
	hasUnknown := false
	for _, signal := range signals {
		switch signal.Status {
		case HealthSignalCritical:
			return HealthStatusCritical
		case HealthSignalWarning, HealthSignalUnavailable:
			hasWarning = true
		case HealthSignalUnknown:
			hasUnknown = true
		}
	}
	if hasWarning {
		return HealthStatusWarning
	}
	if hasUnknown {
		return HealthStatusUnknown
	}
	return HealthStatusHealthy
}

func (s Service) GetNetworkExposure(ctx context.Context, user core.User, applicationID string) (NetworkExposureResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, false)
	if err != nil {
		return NetworkExposureResponse{}, err
	}
	if s.NetworkExposureReader == nil {
		return NetworkExposureResponse{
			ApplicationID: record.ID,
			Enabled:       record.LoadBalancerEnabled,
			Status:        NetworkExposureStatusPending,
			Message:       "LoadBalancer 노출 상태 리더가 설정되지 않았습니다.",
			ObservedAt:    timeNowUTC(),
		}, nil
	}

	info, err := s.NetworkExposureReader.Read(ctx, record)
	if err != nil {
		return NetworkExposureResponse{
			ApplicationID: record.ID,
			Enabled:       record.LoadBalancerEnabled,
			Status:        NetworkExposureStatusError,
			Message:       err.Error(),
			ServiceType:   "LoadBalancer",
			ObservedAt:    timeNowUTC(),
		}, nil
	}

	return NetworkExposureResponse{
		ApplicationID: record.ID,
		Enabled:       record.LoadBalancerEnabled,
		Status:        info.Status,
		Message:       info.Message,
		ServiceType:   info.ServiceType,
		Addresses:     append([]string(nil), info.Addresses...),
		Ports:         append([]NetworkExposurePort(nil), info.Ports...),
		LastEvent:     info.LastEvent,
		ObservedAt:    info.ObservedAt,
	}, nil
}

func (s Service) GetContainerLogs(ctx context.Context, user core.User, applicationID string, tailLines int) (ContainerLogsResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, false)
	if err != nil {
		return ContainerLogsResponse{}, err
	}
	if s.LogsReader == nil {
		return ContainerLogsResponse{}, ValidationError{
			Message: "container logs are unavailable because kubernetes api is not configured",
			Details: map[string]any{"applicationId": applicationID},
		}
	}

	if tailLines <= 0 {
		tailLines = 120
	}
	if tailLines > 500 {
		tailLines = 500
	}

	items, err := s.LogsReader.Read(ctx, record, tailLines)
	if err != nil {
		return ContainerLogsResponse{}, err
	}

	return ContainerLogsResponse{
		ApplicationID: record.ID,
		CollectedAt:   timeNowUTC(),
		TailLines:     tailLines,
		Items:         items,
	}, nil
}

func (s Service) GetContainerLogTargets(ctx context.Context, user core.User, applicationID string) (ContainerLogTargetsResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, false)
	if err != nil {
		return ContainerLogTargetsResponse{}, err
	}
	if s.LogsReader == nil {
		return ContainerLogTargetsResponse{}, ValidationError{
			Message: "container logs are unavailable because kubernetes api is not configured",
			Details: map[string]any{"applicationId": applicationID},
		}
	}

	items, err := s.LogsReader.ListTargets(ctx, record)
	if err != nil {
		return ContainerLogTargetsResponse{}, err
	}

	return ContainerLogTargetsResponse{
		ApplicationID: record.ID,
		CollectedAt:   timeNowUTC(),
		Items:         items,
	}, nil
}

func (s Service) StreamContainerLogs(
	ctx context.Context,
	user core.User,
	applicationID string,
	podName string,
	containerName string,
	tailLines int,
	emit func(ContainerLogEvent) error,
) error {
	record, err := s.requireApplication(ctx, user, applicationID, false)
	if err != nil {
		return err
	}
	if s.LogsReader == nil {
		return ValidationError{
			Message: "container logs are unavailable because kubernetes api is not configured",
			Details: map[string]any{"applicationId": applicationID},
		}
	}
	if strings.TrimSpace(podName) == "" {
		return ValidationError{
			Message: "podName is required to stream container logs",
			Details: map[string]any{"field": "podName"},
		}
	}
	if strings.TrimSpace(containerName) == "" {
		return ValidationError{
			Message: "containerName is required to stream container logs",
			Details: map[string]any{"field": "containerName"},
		}
	}

	if tailLines <= 0 {
		tailLines = 120
	}
	if tailLines > 500 {
		tailLines = 500
	}

	targets, err := s.LogsReader.ListTargets(ctx, record)
	if err != nil {
		return err
	}
	if !hasContainerLogTarget(targets, podName, containerName) {
		return ValidationError{
			Message: "selected pod or container was not found",
			Details: map[string]any{
				"podName":       podName,
				"containerName": containerName,
			},
		}
	}

	return s.LogsReader.Stream(ctx, record, podName, containerName, tailLines, emit)
}

func hasContainerLogTarget(targets []ContainerLogTarget, podName string, containerName string) bool {
	for _, target := range targets {
		if target.PodName != podName {
			continue
		}
		for _, container := range target.Containers {
			if container.Name == containerName {
				return true
			}
		}
	}
	return false
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

func (s Service) requireProjectAdmin(ctx context.Context, user core.User, applicationID string) (string, project.AuthorizedProject, error) {
	projectID, _, err := splitApplicationID(applicationID)
	if err != nil {
		return "", project.AuthorizedProject{}, err
	}

	authorizedProject, err := s.Projects.GetAuthorized(ctx, user, projectID)
	if err != nil {
		return "", project.AuthorizedProject{}, err
	}
	if !authorizedProject.Role.CanAdmin() {
		return "", project.AuthorizedProject{}, ErrRequiresAdmin
	}

	return projectID, authorizedProject, nil
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
	if input.RepositoryPollIntervalSeconds != 0 {
		if err := validateRepositoryPollIntervalSeconds(input.RepositoryPollIntervalSeconds); err != nil {
			return err
		}
	}

	return nil
}

func validateRepositoryPollIntervalSeconds(value int) error {
	if normalizeRepositoryPollIntervalSeconds(value) > 0 {
		return nil
	}

	return ValidationError{
		Message: "repositoryPollIntervalSeconds must be one of 60, 300, or 600",
		Details: map[string]any{
			"field":   "repositoryPollIntervalSeconds",
			"allowed": AllowedRepositoryPollIntervalsSeconds,
		},
	}
}

func normalizeCreateNetworkProfile(input *CreateRequest) {
	if input == nil {
		return
	}
	if NormalizeDeploymentStrategy(input.DeploymentStrategy) == DeploymentStrategyCanary {
		input.MeshEnabled = true
	}
}

func validateNetworkProfile(strategy DeploymentStrategy, meshEnabled bool, loadBalancerEnabled bool) error {
	normalizedStrategy := NormalizeDeploymentStrategy(strategy)
	if normalizedStrategy == DeploymentStrategyCanary && !meshEnabled {
		return ValidationError{
			Message: "meshEnabled must be true when deploymentStrategy is Canary",
			Details: map[string]any{"field": "meshEnabled"},
		}
	}
	if normalizedStrategy == DeploymentStrategyCanary && loadBalancerEnabled {
		return ValidationError{
			Message: "loadBalancerEnabled cannot be true when deploymentStrategy is Canary",
			Details: map[string]any{"field": "loadBalancerEnabled"},
		}
	}
	return nil
}

func normalizeRepositoryMetadata(projectInfo project.CatalogProject, input *CreateRequest) error {
	repositoryID := strings.TrimSpace(input.RepositoryID)
	repositoryURL := strings.TrimSpace(input.RepositoryURL)
	repositoryBranch := strings.TrimSpace(input.RepositoryBranch)
	repositoryToken := strings.TrimSpace(input.RepositoryToken)
	repositoryServiceID := strings.TrimSpace(input.RepositoryServiceID)
	configPath := strings.TrimSpace(input.ConfigPath)

	if repositoryURL != "" {
		if repositoryID != "" {
			return ValidationError{
				Message: "repositoryId and repositoryUrl cannot be used together",
				Details: map[string]any{"field": "repositoryUrl"},
			}
		}
		input.RepositoryID = ""
		input.RepositoryURL = repositoryURL
		input.RepositoryBranch = repositoryBranch
		input.RepositoryServiceID = repositoryServiceID
		if configPath == "" {
			input.ConfigPath = DefaultRepositoryConfigPath
		} else {
			input.ConfigPath = configPath
		}
		return nil
	}

	if repositoryID == "" {
		if repositoryServiceID != "" || configPath != "" || repositoryBranch != "" || repositoryToken != "" {
			return ValidationError{
				Message: "repositoryUrl is required when GitHub source metadata is provided",
				Details: map[string]any{"field": "repositoryUrl"},
			}
		}
		input.RepositoryURL = ""
		input.RepositoryBranch = ""
		input.RepositoryServiceID = ""
		input.ConfigPath = ""
		return nil
	}

	for _, repository := range projectInfo.Repositories {
		if repository.ID != repositoryID {
			continue
		}

		input.RepositoryID = repositoryID
		input.RepositoryURL = repository.URL
		input.RepositoryBranch = strings.TrimSpace(repository.Branch)
		input.RepositoryServiceID = repositoryServiceID
		if configPath != "" {
			input.ConfigPath = configPath
			return nil
		}
		if strings.TrimSpace(repository.ConfigFile) != "" {
			input.ConfigPath = strings.TrimSpace(repository.ConfigFile)
			return nil
		}
		input.ConfigPath = DefaultRepositoryConfigPath
		return nil
	}

	return ValidationError{
		Message: "repositoryId is not connected to this project",
		Details: map[string]any{
			"field":        "repositoryId",
			"repositoryId": repositoryID,
		},
	}
}

func (s Service) hydrateRepositorySource(ctx context.Context, input *CreateRequest) error {
	if strings.TrimSpace(input.RepositoryURL) == "" {
		return nil
	}

	input.RepositoryURL = strings.TrimSpace(input.RepositoryURL)
	input.RepositoryBranch = strings.TrimSpace(input.RepositoryBranch)
	input.RepositoryServiceID = strings.TrimSpace(input.RepositoryServiceID)
	input.ConfigPath = strings.TrimSpace(input.ConfigPath)
	input.RepositoryToken = strings.TrimSpace(input.RepositoryToken)
	if input.ConfigPath == "" {
		input.ConfigPath = DefaultRepositoryConfigPath
	}

	descriptor, err := s.readRepositoryDescriptor(ctx, *input)
	if err != nil {
		return err
	}

	service, ok := descriptor.resolveService(Record{
		Name:                strings.TrimSpace(input.Name),
		RepositoryServiceID: input.RepositoryServiceID,
	})
	if !ok {
		return ValidationError{
			Message: "repositoryServiceId is required when the descriptor defines multiple services",
			Details: map[string]any{
				"field": "repositoryServiceId",
			},
		}
	}

	if strings.TrimSpace(input.Name) == "" {
		input.Name = service.ServiceID
	}
	if input.Image == "" {
		input.Image = service.Image
	}
	if input.ServicePort == 0 {
		input.ServicePort = service.Port
	}
	if input.Replicas == 0 {
		input.Replicas = service.Replicas
	}
	input.RepositoryServiceID = service.ServiceID
	if strings.TrimSpace(string(input.DeploymentStrategy)) == "" {
		if normalized := NormalizeDeploymentStrategy(service.Strategy); normalized != "" {
			input.DeploymentStrategy = normalized
		} else {
			input.DeploymentStrategy = DeploymentStrategyRollout
		}
	}

	return nil
}

func (s Service) readRepositoryDescriptor(ctx context.Context, input CreateRequest) (repositoryDescriptor, error) {
	poller := AutoUpdatePoller{
		Client: s.httpClient(),
	}
	repo := project.Repository{
		URL:        strings.TrimSpace(input.RepositoryURL),
		Branch:     strings.TrimSpace(input.RepositoryBranch),
		ConfigFile: strings.TrimSpace(input.ConfigPath),
	}

	target, err := poller.resolveRepositoryFileTarget(repo, repo.ConfigFile, DefaultRepositoryConfigPath, strings.TrimSpace(input.RepositoryToken))
	if err != nil {
		return repositoryDescriptor{}, ValidationError{
			Message: "repositoryUrl must point to a GitHub repository",
			Details: map[string]any{"field": "repositoryUrl"},
		}
	}

	data, err := poller.fetchRemoteFile(ctx, target, strings.TrimSpace(input.RepositoryToken))
	if err != nil {
		return repositoryDescriptor{}, ValidationError{
			Message: DefaultRepositoryConfigPath + " could not be read from the repository",
			Details: map[string]any{
				"field":          "repositoryUrl",
				"repositoryUrl":  input.RepositoryURL,
				"repositoryPath": repo.ConfigFile,
				"error":          err.Error(),
			},
		}
	}

	descriptor, err := parseRepositoryDescriptor(data)
	if err != nil {
		return repositoryDescriptor{}, ValidationError{
			Message: DefaultRepositoryConfigPath + " format is invalid",
			Details: map[string]any{
				"field": "configPath",
				"error": err.Error(),
			},
		}
	}

	return descriptor, nil
}

func (s Service) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (s Service) defaultRepositoryPollInterval() time.Duration {
	if s.PollTracker != nil {
		return s.PollTracker.DefaultInterval()
	}
	return 5 * time.Minute
}

func (s Service) verifyImageReference(ctx context.Context, image string, credential *RegistryCredential) error {
	if s.Images == nil {
		return nil
	}
	return s.Images.Verify(ctx, image, credential)
}

func (s Service) verifyDeploymentImage(
	ctx context.Context,
	record Record,
	image string,
	imageTag string,
	environment string,
) error {
	registryCredential, err := s.resolveRecordRegistryCredential(ctx, record)
	if err != nil {
		return err
	}
	if err := s.verifyImageReference(ctx, image, registryCredential); err != nil {
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

func normalizeRegistryCredentialInput(
	image string,
	registryServer string,
	registryUsername string,
	registryToken string,
) (*RegistryCredential, string, error) {
	registryServer = normalizeRegistryServer(registryServer)
	registryUsername = strings.TrimSpace(registryUsername)
	registryToken = strings.TrimSpace(registryToken)

	if registryServer == "" && registryUsername == "" && registryToken == "" {
		return nil, "", nil
	}
	if registryUsername == "" || registryToken == "" {
		return nil, "", ValidationError{
			Message: "registryUsername and registryToken must be provided together",
			Details: map[string]any{"field": "registryToken"},
		}
	}
	if registryServer == "" {
		ref, err := parseImageReference(image)
		if err != nil {
			return nil, "", ValidationError{
				Message: "registryServer could not be inferred from image",
				Details: map[string]any{"field": "registryServer"},
			}
		}
		registryServer = normalizeRegistryServer(ref.Registry)
	}

	dockerConfigJSON, err := buildDockerConfigJSON(registryServer, registryUsername, registryToken)
	if err != nil {
		return nil, "", ValidationError{
			Message: "registry credential is invalid",
			Details: map[string]any{
				"field": "registryToken",
				"error": err.Error(),
			},
		}
	}

	return &RegistryCredential{
		Server:   registryServer,
		Username: registryUsername,
		Password: registryToken,
	}, dockerConfigJSON, nil
}

func (s Service) resolveRecordRegistryCredential(ctx context.Context, record Record) (*RegistryCredential, error) {
	secretPath := strings.TrimSpace(record.RegistrySecretPath)
	if secretPath == "" || s.Secrets == nil {
		return nil, nil
	}

	values, err := s.Secrets.Get(ctx, secretPath)
	if err != nil {
		return nil, fmt.Errorf("read registry credential: %w", err)
	}
	credential, err := registryCredentialFromSecret(values)
	if err != nil {
		return nil, ValidationError{
			Message: "stored registry credential is invalid",
			Details: map[string]any{
				"field": "registryToken",
				"path":  secretPath,
				"error": err.Error(),
			},
		}
	}
	return credential, nil
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

func normalizeSecretDeleteKeys(entries []string, setValues map[string]string) ([]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(entries))
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		key := strings.TrimSpace(entry)
		if key == "" {
			return nil, ValidationError{
				Message: "secret delete key must not be blank",
				Details: map[string]any{"field": "delete"},
			}
		}
		if _, ok := seen[key]; ok {
			continue
		}
		if _, ok := setValues[key]; ok {
			return nil, ValidationError{
				Message: fmt.Sprintf("secret key %s cannot be set and deleted in the same request", key),
				Details: map[string]any{"field": "delete", "key": key},
			}
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys, nil
}

func cloneSecretData(values map[string]string) map[string]string {
	cloned := map[string]string{}
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func (s Service) buildApplicationSecretsResponse(
	ctx context.Context,
	applicationID string,
	secretPath string,
	configured bool,
	values map[string]string,
) (ApplicationSecretsResponse, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	items := make([]SecretKeySummary, 0, len(keys))
	for _, key := range keys {
		items = append(items, SecretKeySummary{Key: key})
	}

	response := ApplicationSecretsResponse{
		ApplicationID: applicationID,
		SecretPath:    secretPath,
		Configured:    configured,
		Items:         items,
	}

	versioned, ok := s.Secrets.(VersionedSecretStore)
	if !ok || strings.TrimSpace(secretPath) == "" || !configured {
		return response, nil
	}

	versions, err := versioned.ListVersions(ctx, secretPath)
	if err != nil {
		return ApplicationSecretsResponse{}, fmt.Errorf("read application secret versions: %w", err)
	}
	response.VersioningEnabled = true
	response.CurrentVersion = versions.CurrentVersion
	for _, item := range versions.Items {
		if item.Current {
			response.UpdatedAt = item.CreatedAt
			break
		}
	}
	return response, nil
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
		ID:                            record.ID,
		ProjectID:                     record.ProjectID,
		Name:                          record.Name,
		Description:                   record.Description,
		Image:                         record.Image,
		ServicePort:                   record.ServicePort,
		Replicas:                      record.Replicas,
		DeploymentStrategy:            string(NormalizeDeploymentStrategy(record.DeploymentStrategy)),
		DefaultEnvironment:            record.DefaultEnvironment,
		SyncStatus:                    syncInfo.Status,
		CreatedAt:                     record.CreatedAt,
		UpdatedAt:                     record.UpdatedAt,
		RepositoryID:                  record.RepositoryID,
		RepositoryURL:                 record.RepositoryURL,
		RepositoryBranch:              record.RepositoryBranch,
		RepositoryServiceID:           record.RepositoryServiceID,
		ConfigPath:                    record.ConfigPath,
		RepositoryPollIntervalSeconds: record.RepositoryPollIntervalSeconds,
		Resources:                     record.Resources,
		MeshEnabled:                   record.MeshEnabled,
		LoadBalancerEnabled:           record.LoadBalancerEnabled,
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
