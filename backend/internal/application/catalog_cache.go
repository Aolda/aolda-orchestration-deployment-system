package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type ApplicationCatalogCache interface {
	Ensure(ctx context.Context) error
	ListApplications(ctx context.Context, projectID string, maxAge time.Duration) ([]Record, bool, error)
	ReplaceProjectApplications(ctx context.Context, projectID string, records []Record) error
	InvalidateProject(ctx context.Context, projectID string) error
}

type ApplicationCatalogRefreshStore interface {
	RefreshProject(ctx context.Context, projectID string) ([]Record, error)
}

type CachedManifestStore struct {
	Source    Store
	Cache     ApplicationCatalogCache
	Freshness time.Duration
}

func (s CachedManifestStore) ListApplications(ctx context.Context, projectID string) ([]Record, error) {
	if s.Cache != nil && s.Freshness > 0 {
		records, ok, err := s.Cache.ListApplications(ctx, projectID, s.Freshness)
		if err == nil && ok {
			return records, nil
		}
		if err != nil {
			slog.Warn("application catalog cache read failed", "projectID", projectID, "error", err)
		}
	}
	return s.RefreshProject(ctx, projectID)
}

func (s CachedManifestStore) RefreshProject(ctx context.Context, projectID string) ([]Record, error) {
	source, err := s.source()
	if err != nil {
		return nil, err
	}
	records, err := source.ListApplications(ctx, projectID)
	if err != nil {
		return nil, err
	}
	s.replaceProjectApplications(ctx, projectID, records)
	return records, nil
}

func (s CachedManifestStore) GetApplication(ctx context.Context, applicationID string) (Record, error) {
	if s.Cache != nil && s.Freshness > 0 {
		projectID, _, err := splitApplicationID(applicationID)
		if err == nil {
			records, ok, cacheErr := s.Cache.ListApplications(ctx, projectID, s.Freshness)
			if cacheErr == nil && ok {
				for _, record := range records {
					if record.ID == applicationID {
						return record, nil
					}
				}
			} else if cacheErr != nil {
				slog.Warn("application catalog cache read failed", "projectID", projectID, "error", cacheErr)
			}
		}
	}
	source, err := s.source()
	if err != nil {
		return Record{}, err
	}
	return source.GetApplication(ctx, applicationID)
}

func (s CachedManifestStore) CreateApplication(ctx context.Context, project ProjectContext, input CreateRequest, secretPath string) (Record, error) {
	source, err := s.source()
	if err != nil {
		return Record{}, err
	}
	record, err := source.CreateApplication(ctx, project, input, secretPath)
	if err != nil {
		return Record{}, err
	}
	s.refreshProjectAfterWrite(ctx, record.ProjectID)
	return record, nil
}

func (s CachedManifestStore) ArchiveApplication(ctx context.Context, applicationID string, archivedBy string) (ApplicationLifecycleResponse, error) {
	source, err := s.source()
	if err != nil {
		return ApplicationLifecycleResponse{}, err
	}
	response, err := source.ArchiveApplication(ctx, applicationID, archivedBy)
	if err != nil {
		return ApplicationLifecycleResponse{}, err
	}
	s.refreshProjectAfterWrite(ctx, response.ProjectID)
	return response, nil
}

func (s CachedManifestStore) DeleteApplication(ctx context.Context, applicationID string) (ApplicationLifecycleResponse, error) {
	source, err := s.source()
	if err != nil {
		return ApplicationLifecycleResponse{}, err
	}
	response, err := source.DeleteApplication(ctx, applicationID)
	if err != nil {
		return ApplicationLifecycleResponse{}, err
	}
	s.refreshProjectAfterWrite(ctx, response.ProjectID)
	return response, nil
}

func (s CachedManifestStore) UpdateApplicationImage(ctx context.Context, project ProjectContext, applicationID string, imageTag string, deploymentID string) (Record, error) {
	source, err := s.source()
	if err != nil {
		return Record{}, err
	}
	record, err := source.UpdateApplicationImage(ctx, project, applicationID, imageTag, deploymentID)
	if err != nil {
		return Record{}, err
	}
	s.refreshProjectAfterWrite(ctx, record.ProjectID)
	return record, nil
}

func (s CachedManifestStore) PatchApplication(ctx context.Context, project ProjectContext, applicationID string, input UpdateApplicationRequest) (Record, error) {
	source, err := s.source()
	if err != nil {
		return Record{}, err
	}
	record, err := source.PatchApplication(ctx, project, applicationID, input)
	if err != nil {
		return Record{}, err
	}
	s.refreshProjectAfterWrite(ctx, record.ProjectID)
	return record, nil
}

func (s CachedManifestStore) SaveApplicationSecretPath(ctx context.Context, project ProjectContext, applicationID string, secretPath string) (Record, error) {
	source, err := s.source()
	if err != nil {
		return Record{}, err
	}
	record, err := source.SaveApplicationSecretPath(ctx, project, applicationID, secretPath)
	if err != nil {
		return Record{}, err
	}
	s.refreshProjectAfterWrite(ctx, record.ProjectID)
	return record, nil
}

func (s CachedManifestStore) ListDeployments(ctx context.Context, applicationID string) ([]DeploymentRecord, error) {
	source, err := s.source()
	if err != nil {
		return nil, err
	}
	return source.ListDeployments(ctx, applicationID)
}

func (s CachedManifestStore) GetDeployment(ctx context.Context, applicationID string, deploymentID string) (DeploymentRecord, error) {
	source, err := s.source()
	if err != nil {
		return DeploymentRecord{}, err
	}
	return source.GetDeployment(ctx, applicationID, deploymentID)
}

func (s CachedManifestStore) UpdateDeployment(ctx context.Context, applicationID string, deployment DeploymentRecord) (DeploymentRecord, error) {
	source, err := s.source()
	if err != nil {
		return DeploymentRecord{}, err
	}
	return source.UpdateDeployment(ctx, applicationID, deployment)
}

func (s CachedManifestStore) GetRollbackPolicy(ctx context.Context, applicationID string) (RollbackPolicy, error) {
	source, err := s.source()
	if err != nil {
		return RollbackPolicy{}, err
	}
	return source.GetRollbackPolicy(ctx, applicationID)
}

func (s CachedManifestStore) SaveRollbackPolicy(ctx context.Context, applicationID string, policy RollbackPolicy) (RollbackPolicy, error) {
	source, err := s.source()
	if err != nil {
		return RollbackPolicy{}, err
	}
	return source.SaveRollbackPolicy(ctx, applicationID, policy)
}

func (s CachedManifestStore) ListEvents(ctx context.Context, applicationID string) ([]Event, error) {
	source, err := s.source()
	if err != nil {
		return nil, err
	}
	return source.ListEvents(ctx, applicationID)
}

func (s CachedManifestStore) AppendEvent(ctx context.Context, applicationID string, event Event) error {
	source, err := s.source()
	if err != nil {
		return err
	}
	return source.AppendEvent(ctx, applicationID, event)
}

func (s CachedManifestStore) CleanupOrphanFluxManifests(ctx context.Context) (int, error) {
	cleaner, ok := s.Source.(interface {
		CleanupOrphanFluxManifests(context.Context) (int, error)
	})
	if !ok {
		return 0, fmt.Errorf("application manifest store does not support orphan flux cleanup")
	}
	return cleaner.CleanupOrphanFluxManifests(ctx)
}

func (s CachedManifestStore) source() (Store, error) {
	if s.Source == nil {
		return nil, fmt.Errorf("cached manifest store source is not configured")
	}
	return s.Source, nil
}

func (s CachedManifestStore) refreshProjectAfterWrite(ctx context.Context, projectID string) {
	if projectID == "" {
		return
	}
	if _, err := s.RefreshProject(ctx, projectID); err != nil {
		s.invalidateProject(ctx, projectID)
		slog.Warn("application catalog cache refresh after write failed", "projectID", projectID, "error", err)
	}
}

func (s CachedManifestStore) replaceProjectApplications(ctx context.Context, projectID string, records []Record) {
	if s.Cache == nil {
		return
	}
	if err := s.Cache.ReplaceProjectApplications(ctx, projectID, records); err != nil {
		slog.Warn("application catalog cache write failed", "projectID", projectID, "error", err)
	}
}

func (s CachedManifestStore) invalidateProject(ctx context.Context, projectID string) {
	if s.Cache == nil {
		return
	}
	if err := s.Cache.InvalidateProject(ctx, projectID); err != nil {
		slog.Warn("application catalog cache invalidation failed", "projectID", projectID, "error", err)
	}
}
