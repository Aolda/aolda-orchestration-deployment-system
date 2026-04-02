package change

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/project"
)

var (
	ErrNotFound         = errors.New("change not found")
	ErrInvalidOperation = errors.New("change operation is invalid")
	ErrApprovalRequired = errors.New("approved change is required before merge")
)

type Store interface {
	Create(ctx context.Context, record Record) (Record, error)
	Get(ctx context.Context, changeID string) (Record, error)
	Update(ctx context.Context, record Record) (Record, error)
}

type Service struct {
	Projects     *project.Service
	Applications *application.Service
	Store        Store
}

func (s Service) Create(ctx context.Context, user core.User, projectID string, input Request, requestID string) (Record, error) {
	authorized, err := s.Projects.GetAuthorized(ctx, user, projectID)
	if err != nil {
		return Record{}, err
	}
	if !authorized.Role.CanDeploy() {
		return Record{}, application.ErrRequiresDeployer
	}

	environment := resolveEnvironment(authorized.Project, input.Environment)
	writeMode := resolveWriteMode(authorized.Project, environment)
	now := time.Now().UTC()
	record := Record{
		ID:            strings.Replace(requestID, "req_", "chg_", 1),
		ProjectID:     projectID,
		ApplicationID: strings.TrimSpace(input.ApplicationID),
		Operation:     input.Operation,
		Environment:   environment,
		WriteMode:     writeMode,
		Status:        StatusDraft,
		Summary:       resolveSummary(input),
		DiffPreview:   buildDiffPreview(projectID, environment, input),
		Request:       input,
		CreatedBy:     user.Username,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if !isSupportedOperation(record.Operation) {
		return Record{}, ErrInvalidOperation
	}
	return s.Store.Create(ctx, record)
}

func (s Service) Get(ctx context.Context, user core.User, changeID string) (Record, error) {
	record, err := s.Store.Get(ctx, changeID)
	if err != nil {
		return Record{}, err
	}
	if _, err := s.Projects.GetAuthorized(ctx, user, record.ProjectID); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s Service) Submit(ctx context.Context, user core.User, changeID string) (Record, error) {
	record, err := s.Get(ctx, user, changeID)
	if err != nil {
		return Record{}, err
	}
	authorized, err := s.Projects.GetAuthorized(ctx, user, record.ProjectID)
	if err != nil {
		return Record{}, err
	}
	if !authorized.Role.CanDeploy() {
		return Record{}, application.ErrRequiresDeployer
	}
	record.Status = StatusSubmitted
	record.UpdatedAt = time.Now().UTC()
	return s.Store.Update(ctx, record)
}

func (s Service) Approve(ctx context.Context, user core.User, changeID string) (Record, error) {
	record, err := s.Get(ctx, user, changeID)
	if err != nil {
		return Record{}, err
	}
	authorized, err := s.Projects.GetAuthorized(ctx, user, record.ProjectID)
	if err != nil {
		return Record{}, err
	}
	if !authorized.Role.CanAdmin() {
		return Record{}, application.ErrRequiresAdmin
	}
	record.Status = StatusApproved
	record.ApprovedBy = user.Username
	record.UpdatedAt = time.Now().UTC()
	return s.Store.Update(ctx, record)
}

func (s Service) Merge(ctx context.Context, user core.User, changeID string) (Record, error) {
	record, err := s.Get(ctx, user, changeID)
	if err != nil {
		return Record{}, err
	}
	authorized, err := s.Projects.GetAuthorized(ctx, user, record.ProjectID)
	if err != nil {
		return Record{}, err
	}
	if !authorized.Role.CanDeploy() {
		return Record{}, application.ErrRequiresDeployer
	}
	if record.WriteMode == project.WriteModePullRequest && record.Status != StatusApproved {
		return Record{}, ErrApprovalRequired
	}

	applyCtx := application.WithChangeGuardBypass(ctx)
	switch record.Operation {
	case OperationCreateApplication:
		_, err = s.Applications.CreateApplication(applyCtx, user, record.ProjectID, application.CreateRequest{
			Name:               record.Request.Name,
			Description:        record.Request.Description,
			Image:              record.Request.Image,
			ServicePort:        record.Request.ServicePort,
			DeploymentStrategy: record.Request.DeploymentStrategy,
			Environment:        record.Request.Environment,
			Secrets:            record.Request.Secrets,
		}, record.ID)
	case OperationUpdateApplication:
		_, err = s.Applications.PatchApplication(applyCtx, user, record.Request.ApplicationID, application.UpdateApplicationRequest{
			Description:        optionalString(record.Request.Description),
			ServicePort:        optionalInt(record.Request.ServicePort),
			DeploymentStrategy: optionalStrategy(record.Request.DeploymentStrategy),
			Environment:        optionalString(record.Request.Environment),
		})
	case OperationRedeploy:
		_, err = s.Applications.CreateDeployment(applyCtx, user, record.Request.ApplicationID, record.Request.ImageTag, record.Request.Environment, record.ID)
	case OperationUpdatePolicies:
		if record.Request.Policies == nil {
			return Record{}, ErrInvalidOperation
		}
		_, err = s.Projects.UpdatePolicies(applyCtx, user, record.ProjectID, project.PolicySet{
			MinReplicas:                 record.Request.Policies.MinReplicas,
			AllowedEnvironments:         append([]string(nil), record.Request.Policies.AllowedEnvironments...),
			AllowedDeploymentStrategies: append([]string(nil), record.Request.Policies.AllowedDeploymentStrategies...),
			AllowedClusterTargets:       append([]string(nil), record.Request.Policies.AllowedClusterTargets...),
			ProdPRRequired:              record.Request.Policies.ProdPRRequired,
			AutoRollbackEnabled:         record.Request.Policies.AutoRollbackEnabled,
			RequiredProbes:              record.Request.Policies.RequiredProbes,
		})
	default:
		return Record{}, ErrInvalidOperation
	}
	if err != nil {
		return Record{}, err
	}

	record.Status = StatusMerged
	record.MergedBy = user.Username
	record.UpdatedAt = time.Now().UTC()
	return s.Store.Update(ctx, record)
}

func resolveEnvironment(projectInfo project.CatalogProject, requested string) string {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		return requested
	}
	for _, environment := range projectInfo.Environments {
		if environment.Default {
			return environment.ID
		}
	}
	return "prod"
}

func resolveWriteMode(projectInfo project.CatalogProject, environment string) project.WriteMode {
	for _, item := range projectInfo.Environments {
		if item.ID == environment {
			if item.WriteMode != "" {
				return item.WriteMode
			}
			break
		}
	}
	if environment == "prod" && projectInfo.Policies.ProdPRRequired {
		return project.WriteModePullRequest
	}
	return project.WriteModeDirect
}

func resolveSummary(input Request) string {
	if strings.TrimSpace(input.Summary) != "" {
		return strings.TrimSpace(input.Summary)
	}
	switch input.Operation {
	case OperationCreateApplication:
		return fmt.Sprintf("애플리케이션 %s 생성", input.Name)
	case OperationUpdateApplication:
		return fmt.Sprintf("애플리케이션 %s 설정 갱신", input.ApplicationID)
	case OperationRedeploy:
		return fmt.Sprintf("애플리케이션 %s 재배포", input.ApplicationID)
	case OperationUpdatePolicies:
		return "프로젝트 정책 갱신"
	default:
		return "변경 요청"
	}
}

func buildDiffPreview(projectID string, environment string, input Request) []string {
	switch input.Operation {
	case OperationCreateApplication:
		return []string{
			fmt.Sprintf("apps/%s/%s/base/%s 생성", projectID, input.Name, workloadFileName(input.DeploymentStrategy)),
			fmt.Sprintf("apps/%s/%s/overlays/%s 생성", projectID, input.Name, environment),
		}
	case OperationUpdateApplication:
		return []string{
			fmt.Sprintf("%s 전략을 %s 로 변경", input.ApplicationID, input.DeploymentStrategy),
			fmt.Sprintf("%s 기본 환경을 %s 로 전환", input.ApplicationID, environment),
		}
	case OperationRedeploy:
		return []string{
			fmt.Sprintf("%s 이미지를 새 태그 %s 로 갱신", input.ApplicationID, input.ImageTag),
		}
	case OperationUpdatePolicies:
		return []string{
			fmt.Sprintf("%s 프로젝트 정책 갱신", projectID),
		}
	default:
		return []string{"미리보기 정보 없음"}
	}
}

func workloadFileName(strategy application.DeploymentStrategy) string {
	if strategy == application.DeploymentStrategyCanary {
		return "rollout.yaml"
	}
	return "deployment.yaml"
}

func isSupportedOperation(operation Operation) bool {
	switch operation {
	case OperationCreateApplication, OperationUpdateApplication, OperationRedeploy, OperationUpdatePolicies:
		return true
	default:
		return false
	}
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	return &trimmed
}

func optionalInt(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func optionalStrategy(value application.DeploymentStrategy) *application.DeploymentStrategy {
	if value == "" {
		return nil
	}
	return &value
}
