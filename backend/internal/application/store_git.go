package application

import (
	"context"
	"fmt"

	"github.com/aolda/aods-backend/internal/gitops"
)

type GitManifestStore struct {
	Repository                 *gitops.Repository
	FluxKustomizationNamespace string
	FluxSourceName             string
}

func (s GitManifestStore) ListApplications(ctx context.Context, projectID string) ([]Record, error) {
	if s.Repository == nil {
		return nil, fmt.Errorf("git manifest repository is not configured")
	}

	var records []Record
	err := s.Repository.WithRead(ctx, func(repoDir string) error {
		items, err := s.localStore(repoDir).ListApplications(ctx, projectID)
		if err != nil {
			return err
		}
		records = items
		return nil
	})
	if err != nil {
		return nil, err
	}

	return records, nil
}

func (s GitManifestStore) GetApplication(ctx context.Context, applicationID string) (Record, error) {
	if s.Repository == nil {
		return Record{}, fmt.Errorf("git manifest repository is not configured")
	}

	var record Record
	err := s.Repository.WithRead(ctx, func(repoDir string) error {
		item, err := s.localStore(repoDir).GetApplication(ctx, applicationID)
		if err != nil {
			return err
		}
		record = item
		return nil
	})
	if err != nil {
		return Record{}, err
	}

	return record, nil
}

func (s GitManifestStore) CreateApplication(
	ctx context.Context,
	project ProjectContext,
	input CreateRequest,
	secretPath string,
) (Record, error) {
	if s.Repository == nil {
		return Record{}, fmt.Errorf("git manifest repository is not configured")
	}

	var record Record
	err := s.Repository.WithWrite(
		ctx,
		fmt.Sprintf("feat: create application %s in %s", input.Name, project.ID),
		func(repoDir string) error {
			item, err := s.localStore(repoDir).CreateApplication(ctx, project, input, secretPath)
			if err != nil {
				return err
			}
			record = item
			return nil
		},
	)
	if err != nil {
		return Record{}, err
	}

	return record, nil
}

func (s GitManifestStore) ArchiveApplication(ctx context.Context, applicationID string, archivedBy string) (ApplicationLifecycleResponse, error) {
	if s.Repository == nil {
		return ApplicationLifecycleResponse{}, fmt.Errorf("git manifest repository is not configured")
	}

	var response ApplicationLifecycleResponse
	err := s.Repository.WithWrite(ctx, fmt.Sprintf("feat: archive application %s", applicationID), func(repoDir string) error {
		item, err := s.localStore(repoDir).ArchiveApplication(ctx, applicationID, archivedBy)
		if err != nil {
			return err
		}
		response = item
		return nil
	})
	if err != nil {
		return ApplicationLifecycleResponse{}, err
	}
	return response, nil
}

func (s GitManifestStore) DeleteApplication(ctx context.Context, applicationID string) (ApplicationLifecycleResponse, error) {
	if s.Repository == nil {
		return ApplicationLifecycleResponse{}, fmt.Errorf("git manifest repository is not configured")
	}

	var response ApplicationLifecycleResponse
	err := s.Repository.WithWrite(ctx, fmt.Sprintf("feat: delete application %s", applicationID), func(repoDir string) error {
		item, err := s.localStore(repoDir).DeleteApplication(ctx, applicationID)
		if err != nil {
			return err
		}
		response = item
		return nil
	})
	if err != nil {
		return ApplicationLifecycleResponse{}, err
	}
	return response, nil
}

func (s GitManifestStore) UpdateApplicationImage(
	ctx context.Context,
	project ProjectContext,
	applicationID string,
	imageTag string,
	deploymentID string,
) (Record, error) {
	if s.Repository == nil {
		return Record{}, fmt.Errorf("git manifest repository is not configured")
	}

	var record Record
	err := s.Repository.WithWrite(
		ctx,
		fmt.Sprintf("feat: redeploy %s with image tag %s", applicationID, imageTag),
		func(repoDir string) error {
			item, err := s.localStore(repoDir).UpdateApplicationImage(ctx, project, applicationID, imageTag, deploymentID)
			if err != nil {
				return err
			}
			record = item
			return nil
		},
	)
	if err != nil {
		return Record{}, err
	}

	return record, nil
}

func (s GitManifestStore) PatchApplication(ctx context.Context, project ProjectContext, applicationID string, input UpdateApplicationRequest) (Record, error) {
	if s.Repository == nil {
		return Record{}, fmt.Errorf("git manifest repository is not configured")
	}

	var record Record
	err := s.Repository.WithWrite(ctx, fmt.Sprintf("feat: update application %s", applicationID), func(repoDir string) error {
		item, err := s.localStore(repoDir).PatchApplication(ctx, project, applicationID, input)
		if err != nil {
			return err
		}
		record = item
		return nil
	})
	if err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s GitManifestStore) ListDeployments(ctx context.Context, applicationID string) ([]DeploymentRecord, error) {
	if s.Repository == nil {
		return nil, fmt.Errorf("git manifest repository is not configured")
	}
	var items []DeploymentRecord
	err := s.Repository.WithRead(ctx, func(repoDir string) error {
		deployments, err := s.localStore(repoDir).ListDeployments(ctx, applicationID)
		if err != nil {
			return err
		}
		items = deployments
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s GitManifestStore) GetDeployment(ctx context.Context, applicationID string, deploymentID string) (DeploymentRecord, error) {
	if s.Repository == nil {
		return DeploymentRecord{}, fmt.Errorf("git manifest repository is not configured")
	}
	var deployment DeploymentRecord
	err := s.Repository.WithRead(ctx, func(repoDir string) error {
		item, err := s.localStore(repoDir).GetDeployment(ctx, applicationID, deploymentID)
		if err != nil {
			return err
		}
		deployment = item
		return nil
	})
	if err != nil {
		return DeploymentRecord{}, err
	}
	return deployment, nil
}

func (s GitManifestStore) UpdateDeployment(ctx context.Context, applicationID string, deployment DeploymentRecord) (DeploymentRecord, error) {
	if s.Repository == nil {
		return DeploymentRecord{}, fmt.Errorf("git manifest repository is not configured")
	}
	var updated DeploymentRecord
	err := s.Repository.WithWrite(ctx, fmt.Sprintf("feat: update deployment %s", deployment.DeploymentID), func(repoDir string) error {
		item, err := s.localStore(repoDir).UpdateDeployment(ctx, applicationID, deployment)
		if err != nil {
			return err
		}
		updated = item
		return nil
	})
	if err != nil {
		return DeploymentRecord{}, err
	}
	return updated, nil
}

func (s GitManifestStore) GetRollbackPolicy(ctx context.Context, applicationID string) (RollbackPolicy, error) {
	if s.Repository == nil {
		return RollbackPolicy{}, fmt.Errorf("git manifest repository is not configured")
	}
	var policy RollbackPolicy
	err := s.Repository.WithRead(ctx, func(repoDir string) error {
		item, err := s.localStore(repoDir).GetRollbackPolicy(ctx, applicationID)
		if err != nil {
			return err
		}
		policy = item
		return nil
	})
	if err != nil {
		return RollbackPolicy{}, err
	}
	return policy, nil
}

func (s GitManifestStore) SaveRollbackPolicy(ctx context.Context, applicationID string, policy RollbackPolicy) (RollbackPolicy, error) {
	if s.Repository == nil {
		return RollbackPolicy{}, fmt.Errorf("git manifest repository is not configured")
	}
	var saved RollbackPolicy
	err := s.Repository.WithWrite(ctx, fmt.Sprintf("feat: update rollback policy for %s", applicationID), func(repoDir string) error {
		item, err := s.localStore(repoDir).SaveRollbackPolicy(ctx, applicationID, policy)
		if err != nil {
			return err
		}
		saved = item
		return nil
	})
	if err != nil {
		return RollbackPolicy{}, err
	}
	return saved, nil
}

func (s GitManifestStore) ListEvents(ctx context.Context, applicationID string) ([]Event, error) {
	if s.Repository == nil {
		return nil, fmt.Errorf("git manifest repository is not configured")
	}
	var items []Event
	err := s.Repository.WithRead(ctx, func(repoDir string) error {
		events, err := s.localStore(repoDir).ListEvents(ctx, applicationID)
		if err != nil {
			return err
		}
		items = events
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s GitManifestStore) AppendEvent(ctx context.Context, applicationID string, event Event) error {
	if s.Repository == nil {
		return fmt.Errorf("git manifest repository is not configured")
	}
	return s.Repository.WithWrite(ctx, fmt.Sprintf("feat: append application event %s", event.ID), func(repoDir string) error {
		return s.localStore(repoDir).AppendEvent(ctx, applicationID, event)
	})
}

func (s GitManifestStore) localStore(repoDir string) LocalManifestStore {
	return LocalManifestStore{
		RepoRoot:                   repoDir,
		FluxKustomizationNamespace: s.FluxKustomizationNamespace,
		FluxSourceName:             s.FluxSourceName,
	}
}
