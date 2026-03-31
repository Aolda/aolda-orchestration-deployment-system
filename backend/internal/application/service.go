package application

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/project"
)

var (
	ErrNotFound         = errors.New("application not found")
	ErrConflict         = errors.New("application conflict")
	ErrInvalidID        = errors.New("application id is invalid")
	ErrRequiresDeployer = errors.New("deployer permissions are required")
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

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
	UpdateApplicationImage(ctx context.Context, applicationID string, imageTag string) (Record, error)
}

type StatusReader interface {
	Read(ctx context.Context, record Record) (SyncInfo, error)
}

type MetricsReader interface {
	Read(ctx context.Context, record Record) ([]MetricSeries, error)
}

type SecretsStager interface {
	Stage(ctx context.Context, requestID string, projectID string, appName string, createdBy string, data map[string]string) (StagedSecret, error)
	Finalize(ctx context.Context, staged StagedSecret, data map[string]string) error
}

type Service struct {
	Projects      *project.Service
	Store         Store
	StatusReader  StatusReader
	MetricsReader MetricsReader
	Secrets       SecretsStager
}

func (s Service) ListApplications(ctx context.Context, user core.User, projectID string) ([]Summary, error) {
	if _, err := s.Projects.GetAuthorized(ctx, user, projectID); err != nil {
		return nil, err
	}

	records, err := s.Store.ListApplications(ctx, projectID)
	if err != nil {
		return nil, err
	}

	items := make([]Summary, 0, len(records))
	for _, record := range records {
		syncInfo, err := s.StatusReader.Read(ctx, record)
		if err != nil {
			return nil, err
		}

		items = append(items, Summary{
			ID:                 record.ID,
			Name:               record.Name,
			Image:              record.Image,
			DeploymentStrategy: string(record.DeploymentStrategy),
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

	secretData, err := normalizeSecrets(input.Secrets)
	if err != nil {
		return Application{}, err
	}

	secretPath := core.BuildVaultFinalPath(projectID, input.Name)
	var staged StagedSecret
	if len(secretData) > 0 && s.Secrets != nil {
		staged, err = s.Secrets.Stage(ctx, requestID, projectID, input.Name, user.Username, secretData)
		if err != nil {
			return Application{}, err
		}
		secretPath = staged.FinalPath
	}

	record, err := s.Store.CreateApplication(ctx, ProjectContext{
		ID:        authorizedProject.Project.ID,
		Namespace: authorizedProject.Project.Namespace,
	}, input, secretPath)
	if err != nil {
		return Application{}, err
	}

	if len(secretData) > 0 && s.Secrets != nil {
		if err := s.Secrets.Finalize(ctx, staged, secretData); err != nil {
			return Application{}, err
		}
	}

	syncInfo, err := s.StatusReader.Read(ctx, record)
	if err != nil {
		return Application{}, err
	}

	return toApplication(record, syncInfo), nil
}

func (s Service) CreateDeployment(
	ctx context.Context,
	user core.User,
	applicationID string,
	imageTag string,
	requestID string,
) (DeploymentResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, true)
	if err != nil {
		return DeploymentResponse{}, err
	}

	if strings.TrimSpace(imageTag) == "" {
		return DeploymentResponse{}, ValidationError{
			Message: "imageTag is required",
			Details: map[string]any{"field": "imageTag"},
		}
	}

	updatedRecord, err := s.Store.UpdateApplicationImage(ctx, record.ID, imageTag)
	if err != nil {
		return DeploymentResponse{}, err
	}

	return DeploymentResponse{
		DeploymentID:  strings.Replace(requestID, "req_", "dep_", 1),
		ApplicationID: updatedRecord.ID,
		ImageTag:      imageTag,
		Status:        "Syncing",
	}, nil
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

func (s Service) GetMetrics(ctx context.Context, user core.User, applicationID string) (MetricsResponse, error) {
	record, err := s.requireApplication(ctx, user, applicationID, false)
	if err != nil {
		return MetricsResponse{}, err
	}

	metrics, err := s.MetricsReader.Read(ctx, record)
	if err != nil {
		return MetricsResponse{}, err
	}

	return MetricsResponse{
		ApplicationID: record.ID,
		Metrics:       metrics,
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

	if input.DeploymentStrategy != DeploymentStrategyStandard {
		return ValidationError{
			Message: "Phase 1 only supports Standard deployment strategy",
			Details: map[string]any{"field": "deploymentStrategy"},
		}
	}

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

func toApplication(record Record, syncInfo SyncInfo) Application {
	return Application{
		ID:                 record.ID,
		ProjectID:          record.ProjectID,
		Name:               record.Name,
		Description:        record.Description,
		Image:              record.Image,
		ServicePort:        record.ServicePort,
		DeploymentStrategy: string(record.DeploymentStrategy),
		SyncStatus:         syncInfo.Status,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
	}
}
