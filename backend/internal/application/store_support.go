package application

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

type appMetadata struct {
	ID                  string             `yaml:"id"`
	ProjectID           string             `yaml:"projectId"`
	Namespace           string             `yaml:"namespace"`
	Name                string             `yaml:"name"`
	Description         string             `yaml:"description,omitempty"`
	Image               string             `yaml:"image"`
	ServicePort         int                `yaml:"servicePort"`
	Replicas            int                `yaml:"replicas"`
	RequiredProbes      *bool              `yaml:"requiredProbes,omitempty"`
	DeploymentStrategy  DeploymentStrategy `yaml:"deploymentStrategy"`
	DefaultEnvironment  string             `yaml:"defaultEnvironment"`
	CreatedAt           time.Time          `yaml:"createdAt"`
	UpdatedAt           time.Time          `yaml:"updatedAt"`
	SecretPath          string             `yaml:"secretPath,omitempty"`
	Environments        []string           `yaml:"environments,omitempty"`
	RepositoryID        string             `yaml:"repositoryId,omitempty"`
	RepositoryServiceID string             `yaml:"repositoryServiceId,omitempty"`
	ConfigPath          string             `yaml:"configPath,omitempty"`
}

func metadataFromRecord(record Record, environments []string) appMetadata {
	return appMetadata{
		ID:                  record.ID,
		ProjectID:           record.ProjectID,
		Namespace:           record.Namespace,
		Name:                record.Name,
		Description:         record.Description,
		Image:               record.Image,
		ServicePort:         record.ServicePort,
		Replicas:            record.Replicas,
		RequiredProbes:      boolPointer(record.RequiredProbes),
		DeploymentStrategy:  record.DeploymentStrategy,
		DefaultEnvironment:  record.DefaultEnvironment,
		CreatedAt:           record.CreatedAt,
		UpdatedAt:           record.UpdatedAt,
		SecretPath:          record.SecretPath,
		Environments:        append([]string(nil), environments...),
		RepositoryID:        record.RepositoryID,
		RepositoryServiceID: record.RepositoryServiceID,
		ConfigPath:          record.ConfigPath,
	}
}

func (m appMetadata) toRecord() Record {
	return Record{
		ID:                  m.ID,
		ProjectID:           m.ProjectID,
		Namespace:           m.Namespace,
		Name:                m.Name,
		Description:         m.Description,
		Image:               m.Image,
		ServicePort:         m.ServicePort,
		Replicas:            m.Replicas,
		RequiredProbes:      m.requiredProbesOrDefault(),
		DeploymentStrategy:  m.DeploymentStrategy,
		DefaultEnvironment:  m.DefaultEnvironment,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
		SecretPath:          m.SecretPath,
		RepositoryID:        m.RepositoryID,
		RepositoryServiceID: m.RepositoryServiceID,
		ConfigPath:          m.ConfigPath,
	}
}

func metadataPath(repoRoot string, projectID string, appName string) string {
	return filepath.Join(repoRoot, "apps", projectID, appName, ".aods", "metadata.yaml")
}

func writeMetadata(repoRoot string, record Record, environments []string) error {
	data, err := yaml.Marshal(metadataFromRecord(record, environments))
	if err != nil {
		return fmt.Errorf("encode application metadata: %w", err)
	}

	path := metadataPath(repoRoot, record.ProjectID, record.Name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create metadata directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write application metadata: %w", err)
	}
	return nil
}

func readMetadata(repoRoot string, projectID string, appName string) (appMetadata, error) {
	data, err := os.ReadFile(metadataPath(repoRoot, projectID, appName))
	if err != nil {
		return appMetadata{}, err
	}

	var metadata appMetadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return appMetadata{}, fmt.Errorf("decode application metadata: %w", err)
	}
	return metadata, nil
}

func (m appMetadata) requiredProbesOrDefault() bool {
	if m.RequiredProbes == nil {
		return true
	}
	return *m.RequiredProbes
}

func boolPointer(value bool) *bool {
	return &value
}

func deploymentHistoryDir(repoRoot string, projectID string, appName string) string {
	return filepath.Join(repoRoot, "apps", projectID, appName, ".aods", "deployments")
}

func writeDeploymentRecord(repoRoot string, deployment DeploymentRecord) error {
	path := filepath.Join(deploymentHistoryDir(repoRoot, deployment.ProjectID, deployment.ApplicationName), deployment.DeploymentID+".yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create deployment history directory: %w", err)
	}

	data, err := yaml.Marshal(deployment)
	if err != nil {
		return fmt.Errorf("encode deployment history: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write deployment history: %w", err)
	}
	return nil
}

func readDeploymentRecord(repoRoot string, projectID string, appName string, deploymentID string) (DeploymentRecord, error) {
	path := filepath.Join(deploymentHistoryDir(repoRoot, projectID, appName), deploymentID+".yaml")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DeploymentRecord{}, ErrDeploymentNotFound
	}
	if err != nil {
		return DeploymentRecord{}, fmt.Errorf("read deployment history: %w", err)
	}

	var deployment DeploymentRecord
	if err := yaml.Unmarshal(data, &deployment); err != nil {
		return DeploymentRecord{}, fmt.Errorf("decode deployment history: %w", err)
	}
	return deployment, nil
}

func listDeploymentRecords(repoRoot string, projectID string, appName string) ([]DeploymentRecord, error) {
	entries, err := os.ReadDir(deploymentHistoryDir(repoRoot, projectID, appName))
	if errors.Is(err, os.ErrNotExist) {
		return []DeploymentRecord{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read deployment history directory: %w", err)
	}

	items := make([]DeploymentRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		deployment, err := readDeploymentRecord(repoRoot, projectID, appName, trimExtension(entry.Name()))
		if err != nil {
			return nil, err
		}
		items = append(items, deployment)
	}

	sort.Slice(items, func(i int, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items, nil
}

func rollbackPolicyPath(repoRoot string, projectID string, appName string) string {
	return filepath.Join(repoRoot, "apps", projectID, appName, ".aods", "rollback-policy.yaml")
}

func readRollbackPolicy(repoRoot string, projectID string, appName string) (RollbackPolicy, error) {
	data, err := os.ReadFile(rollbackPolicyPath(repoRoot, projectID, appName))
	if errors.Is(err, os.ErrNotExist) {
		return RollbackPolicy{}, nil
	}
	if err != nil {
		return RollbackPolicy{}, fmt.Errorf("read rollback policy: %w", err)
	}

	var policy RollbackPolicy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return RollbackPolicy{}, fmt.Errorf("decode rollback policy: %w", err)
	}
	return policy, nil
}

func writeRollbackPolicy(repoRoot string, record Record, policy RollbackPolicy) error {
	path := rollbackPolicyPath(repoRoot, record.ProjectID, record.Name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create rollback policy directory: %w", err)
	}
	data, err := yaml.Marshal(policy)
	if err != nil {
		return fmt.Errorf("encode rollback policy: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write rollback policy: %w", err)
	}
	return nil
}

func eventsDir(repoRoot string, projectID string, appName string) string {
	return filepath.Join(repoRoot, "apps", projectID, appName, ".aods", "events")
}

func writeEventRecord(repoRoot string, projectID string, appName string, event Event) error {
	path := filepath.Join(eventsDir(repoRoot, projectID, appName), event.ID+".yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create events directory: %w", err)
	}
	data, err := yaml.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write event: %w", err)
	}
	return nil
}

func listEventRecords(repoRoot string, projectID string, appName string) ([]Event, error) {
	entries, err := os.ReadDir(eventsDir(repoRoot, projectID, appName))
	if errors.Is(err, os.ErrNotExist) {
		return []Event{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read events directory: %w", err)
	}

	items := make([]Event, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(eventsDir(repoRoot, projectID, appName), entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read event: %w", err)
		}
		var event Event
		if err := yaml.Unmarshal(data, &event); err != nil {
			return nil, fmt.Errorf("decode event: %w", err)
		}
		items = append(items, event)
	}

	sort.Slice(items, func(i int, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items, nil
}

func trimExtension(value string) string {
	return value[:len(value)-len(filepath.Ext(value))]
}
