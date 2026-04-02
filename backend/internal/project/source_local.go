package project

import (
	"context"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type LocalCatalogSource struct {
	Path string
}

type catalogFile struct {
	Projects []catalogProject `yaml:"projects"`
}

type catalogProject struct {
	ID           string               `yaml:"id"`
	Name         string               `yaml:"name"`
	Description  string               `yaml:"description"`
	Namespace    string               `yaml:"namespace"`
	Access       catalogAccess        `yaml:"access"`
	Environments []catalogEnvironment `yaml:"environments"`
	Policies     catalogPolicies      `yaml:"policies"`
}

type catalogAccess struct {
	ViewerGroups   []string `yaml:"viewerGroups"`
	DeployerGroups []string `yaml:"deployerGroups"`
	AdminGroups    []string `yaml:"adminGroups"`
}

type catalogEnvironment struct {
	ID        string    `yaml:"id"`
	Name      string    `yaml:"name"`
	ClusterID string    `yaml:"clusterId"`
	WriteMode WriteMode `yaml:"writeMode"`
	Default   bool      `yaml:"default"`
}

type catalogPolicies struct {
	MinReplicas                 int      `yaml:"minReplicas"`
	AllowedEnvironments         []string `yaml:"allowedEnvironments"`
	AllowedDeploymentStrategies []string `yaml:"allowedDeploymentStrategies"`
	AllowedClusterTargets       []string `yaml:"allowedClusterTargets"`
	ProdPRRequired              bool     `yaml:"prodPRRequired"`
	AutoRollbackEnabled         bool     `yaml:"autoRollbackEnabled"`
	RequiredProbes              bool     `yaml:"requiredProbes"`
}

type CatalogNotFoundError struct {
	Path string
}

func (e CatalogNotFoundError) Error() string {
	return fmt.Sprintf("project catalog was not found at %s", e.Path)
}

func (s LocalCatalogSource) ListProjects(ctx context.Context) ([]CatalogProject, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, CatalogNotFoundError{Path: s.Path}
	}
	if err != nil {
		return nil, fmt.Errorf("read project catalog: %w", err)
	}

	var file catalogFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode project catalog: %w", err)
	}

	projects := make([]CatalogProject, 0, len(file.Projects))
	for _, item := range file.Projects {
		project := CatalogProject{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Description,
			Namespace:   item.Namespace,
			Access: Access{
				ViewerGroups:   item.Access.ViewerGroups,
				DeployerGroups: item.Access.DeployerGroups,
				AdminGroups:    item.Access.AdminGroups,
			},
			Policies: PolicySet{
				MinReplicas:                 item.Policies.MinReplicas,
				AllowedEnvironments:         append([]string(nil), item.Policies.AllowedEnvironments...),
				AllowedDeploymentStrategies: append([]string(nil), item.Policies.AllowedDeploymentStrategies...),
				AllowedClusterTargets:       append([]string(nil), item.Policies.AllowedClusterTargets...),
				ProdPRRequired:              item.Policies.ProdPRRequired,
				AutoRollbackEnabled:         item.Policies.AutoRollbackEnabled,
				RequiredProbes:              item.Policies.RequiredProbes,
			},
		}
		for _, environment := range item.Environments {
			project.Environments = append(project.Environments, Environment{
				ID:        environment.ID,
				Name:      environment.Name,
				ClusterID: environment.ClusterID,
				WriteMode: environment.WriteMode,
				Default:   environment.Default,
			})
		}
		projects = append(projects, applyProjectDefaults(project))
	}

	return projects, nil
}

func (s LocalCatalogSource) UpdatePolicies(ctx context.Context, projectID string, policies PolicySet) (CatalogProject, error) {
	if err := ctx.Err(); err != nil {
		return CatalogProject{}, err
	}

	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return CatalogProject{}, CatalogNotFoundError{Path: s.Path}
	}
	if err != nil {
		return CatalogProject{}, fmt.Errorf("read project catalog: %w", err)
	}

	var file catalogFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return CatalogProject{}, fmt.Errorf("decode project catalog: %w", err)
	}

	for index, item := range file.Projects {
		if item.ID != projectID {
			continue
		}

		item.Policies = catalogPolicies{
			MinReplicas:                 policies.MinReplicas,
			AllowedEnvironments:         append([]string(nil), policies.AllowedEnvironments...),
			AllowedDeploymentStrategies: append([]string(nil), policies.AllowedDeploymentStrategies...),
			AllowedClusterTargets:       append([]string(nil), policies.AllowedClusterTargets...),
			ProdPRRequired:              policies.ProdPRRequired,
			AutoRollbackEnabled:         policies.AutoRollbackEnabled,
			RequiredProbes:              policies.RequiredProbes,
		}
		file.Projects[index] = item

		rendered, err := yaml.Marshal(&file)
		if err != nil {
			return CatalogProject{}, fmt.Errorf("encode project catalog: %w", err)
		}
		if err := os.WriteFile(s.Path, rendered, 0o644); err != nil {
			return CatalogProject{}, fmt.Errorf("write project catalog: %w", err)
		}

		projects, err := s.ListProjects(ctx)
		if err != nil {
			return CatalogProject{}, err
		}
		for _, project := range projects {
			if project.ID == projectID {
				return project, nil
			}
		}
		break
	}

	return CatalogProject{}, ErrNotFound
}
