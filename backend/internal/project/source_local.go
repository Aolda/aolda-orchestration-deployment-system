package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/fluxscaffold"
	"gopkg.in/yaml.v3"
)

type LocalCatalogSource struct {
	Path                       string
	RepoRoot                   string
	FluxKustomizationNamespace string
	FluxSourceName             string
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
	Repositories []catalogRepository  `yaml:"repositories,omitempty"`
	Policies     catalogPolicies      `yaml:"policies"`
}

type catalogRepository struct {
	ID             string `yaml:"id"`
	Name           string `yaml:"name"`
	URL            string `yaml:"url"`
	Description    string `yaml:"description,omitempty"`
	Branch         string `yaml:"branch,omitempty"`
	AuthSecretPath string `yaml:"authSecretPath,omitempty"`
	ConfigFile     string `yaml:"configFile,omitempty"`
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

type applicationMetadata struct {
	SecretPath string `yaml:"secretPath,omitempty"`
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

	file, _, _, err := s.readCatalogFile()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, CatalogNotFoundError{Path: s.Path}
		}
		return nil, err
	}

	projects := make([]CatalogProject, 0, len(file.Projects))
	for _, item := range file.Projects {
		projects = append(projects, projectFromCatalog(item))
	}

	return projects, nil
}

func (s LocalCatalogSource) CreateProject(ctx context.Context, input CreateRequest) (CatalogProject, error) {
	if err := ctx.Err(); err != nil {
		return CatalogProject{}, err
	}

	file, originalData, existed, err := s.readCatalogFile()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return CatalogProject{}, err
	}

	for _, item := range file.Projects {
		if strings.TrimSpace(item.ID) == input.ID {
			return CatalogProject{}, ErrConflict
		}
	}

	created := CatalogProject{
		ID:           input.ID,
		Name:         input.Name,
		Description:  input.Description,
		Namespace:    input.Namespace,
		Access:       input.Access,
		Environments: append([]Environment(nil), input.Environments...),
		Repositories: append([]Repository(nil), input.Repositories...),
		Policies:     input.Policies,
	}

	file.Projects = append(file.Projects, catalogProjectFromProject(created))
	rendered, err := yaml.Marshal(&file)
	if err != nil {
		return CatalogProject{}, fmt.Errorf("encode project catalog: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return CatalogProject{}, fmt.Errorf("create project catalog directory: %w", err)
	}
	if err := os.WriteFile(s.Path, rendered, 0o644); err != nil {
		return CatalogProject{}, fmt.Errorf("write project catalog: %w", err)
	}

	if err := s.ensureClusterScaffolds(created); err != nil {
		_ = s.rollbackCatalogWrite(originalData, existed)
		return CatalogProject{}, err
	}

	return applyProjectDefaults(created), nil
}

func (s LocalCatalogSource) DeleteProject(ctx context.Context, projectID string) (LifecycleResponse, error) {
	if err := ctx.Err(); err != nil {
		return LifecycleResponse{}, err
	}

	file, _, _, err := s.readCatalogFile()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LifecycleResponse{}, CatalogNotFoundError{Path: s.Path}
		}
		return LifecycleResponse{}, err
	}

	projectIndex := -1
	var target catalogProject
	for index, item := range file.Projects {
		if item.ID == projectID {
			projectIndex = index
			target = item
			break
		}
	}
	if projectIndex < 0 {
		return LifecycleResponse{}, ErrNotFound
	}

	secretPaths, err := s.collectProjectSecretPaths(projectID)
	if err != nil {
		return LifecycleResponse{}, err
	}
	if err := s.removeProjectFluxChildren(projectID); err != nil {
		return LifecycleResponse{}, err
	}
	if err := os.RemoveAll(filepath.Join(s.repoRoot(), "apps", projectID)); err != nil {
		return LifecycleResponse{}, fmt.Errorf("delete project application directory: %w", err)
	}

	file.Projects = append(file.Projects[:projectIndex], file.Projects[projectIndex+1:]...)
	rendered, err := yaml.Marshal(&file)
	if err != nil {
		return LifecycleResponse{}, fmt.Errorf("encode project catalog: %w", err)
	}
	if err := os.WriteFile(s.Path, rendered, 0o644); err != nil {
		return LifecycleResponse{}, fmt.Errorf("write project catalog: %w", err)
	}

	deletedAt := time.Now().UTC()
	return LifecycleResponse{
		ProjectID:   target.ID,
		Name:        target.Name,
		Namespace:   target.Namespace,
		Status:      "deleted",
		DeletedAt:   &deletedAt,
		secretPaths: secretPaths,
	}, nil
}

func (s LocalCatalogSource) UpdatePolicies(ctx context.Context, projectID string, policies PolicySet) (CatalogProject, error) {
	if err := ctx.Err(); err != nil {
		return CatalogProject{}, err
	}

	file, _, _, err := s.readCatalogFile()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return CatalogProject{}, CatalogNotFoundError{Path: s.Path}
		}
		return CatalogProject{}, err
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

func (s LocalCatalogSource) collectProjectSecretPaths(projectID string) ([]string, error) {
	appRoot := filepath.Join(s.repoRoot(), "apps", projectID)
	entries, err := os.ReadDir(appRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read project application directory: %w", err)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metadataPath := filepath.Join(appRoot, entry.Name(), ".aods", "metadata.yaml")
		data, err := os.ReadFile(metadataPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read application metadata: %w", err)
		}
		var metadata applicationMetadata
		if err := yaml.Unmarshal(data, &metadata); err != nil {
			return nil, fmt.Errorf("decode application metadata: %w", err)
		}
		if secretPath := strings.TrimSpace(metadata.SecretPath); secretPath != "" {
			paths = append(paths, secretPath)
		}
	}

	sort.Strings(paths)
	return paths, nil
}

func (s LocalCatalogSource) removeProjectFluxChildren(projectID string) error {
	clustersRoot := filepath.Join(s.repoRoot(), "platform", "flux", "clusters")
	entries, err := os.ReadDir(clustersRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read flux clusters directory: %w", err)
	}

	childPrefix := projectID + "-"
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		clusterID := entry.Name()
		appDir := filepath.Join(clustersRoot, clusterID, "applications")
		appEntries, err := os.ReadDir(appDir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read flux applications directory: %w", err)
		}

		removedAny := false
		for _, appEntry := range appEntries {
			if appEntry.IsDir() {
				continue
			}
			fileName := appEntry.Name()
			if !strings.HasPrefix(fileName, childPrefix) || !strings.HasSuffix(fileName, ".yaml") {
				continue
			}
			if err := os.Remove(filepath.Join(appDir, fileName)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove project flux child kustomization: %w", err)
			}
			removedAny = true
		}

		if removedAny {
			if err := fluxscaffold.RewriteClusterRoot(fluxscaffold.Config{
				RepoRoot:               s.repoRoot(),
				ClusterID:              clusterID,
				KustomizationNamespace: s.fluxKustomizationNamespace(),
				SourceName:             s.fluxSourceName(),
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s LocalCatalogSource) readCatalogFile() (catalogFile, []byte, bool, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return catalogFile{}, nil, false, err
		}
		return catalogFile{}, nil, false, fmt.Errorf("read project catalog: %w", err)
	}

	var file catalogFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return catalogFile{}, nil, false, fmt.Errorf("decode project catalog: %w", err)
	}
	return file, data, true, nil
}

func (s LocalCatalogSource) rollbackCatalogWrite(originalData []byte, existed bool) error {
	if !existed {
		if err := os.Remove(s.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return os.WriteFile(s.Path, originalData, 0o644)
}

func (s LocalCatalogSource) ensureClusterScaffolds(project CatalogProject) error {
	for _, clusterID := range projectClusterIDs(project) {
		if err := fluxscaffold.EnsureCluster(fluxscaffold.Config{
			RepoRoot:               s.repoRoot(),
			ClusterID:              clusterID,
			KustomizationNamespace: s.fluxKustomizationNamespace(),
			SourceName:             s.fluxSourceName(),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s LocalCatalogSource) repoRoot() string {
	if root := strings.TrimSpace(s.RepoRoot); root != "" {
		return root
	}
	return filepath.Dir(filepath.Dir(s.Path))
}

func (s LocalCatalogSource) fluxKustomizationNamespace() string {
	if namespace := strings.TrimSpace(s.FluxKustomizationNamespace); namespace != "" {
		return namespace
	}
	return fluxscaffold.DefaultKustomizationNamespace
}

func (s LocalCatalogSource) fluxSourceName() string {
	if sourceName := strings.TrimSpace(s.FluxSourceName); sourceName != "" {
		return sourceName
	}
	return fluxscaffold.DefaultSourceName
}

func projectFromCatalog(item catalogProject) CatalogProject {
	project := CatalogProject{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		Namespace:   item.Namespace,
		Access: Access{
			ViewerGroups:   append([]string(nil), item.Access.ViewerGroups...),
			DeployerGroups: append([]string(nil), item.Access.DeployerGroups...),
			AdminGroups:    append([]string(nil), item.Access.AdminGroups...),
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
	for _, repo := range item.Repositories {
		project.Repositories = append(project.Repositories, Repository{
			ID:             repo.ID,
			Name:           repo.Name,
			URL:            repo.URL,
			Description:    repo.Description,
			Branch:         repo.Branch,
			AuthSecretPath: repo.AuthSecretPath,
			ConfigFile:     repo.ConfigFile,
		})
	}
	return applyProjectDefaults(project)
}

func catalogProjectFromProject(project CatalogProject) catalogProject {
	item := catalogProject{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		Namespace:   project.Namespace,
		Access: catalogAccess{
			ViewerGroups:   append([]string(nil), project.Access.ViewerGroups...),
			DeployerGroups: append([]string(nil), project.Access.DeployerGroups...),
			AdminGroups:    append([]string(nil), project.Access.AdminGroups...),
		},
		Policies: catalogPolicies{
			MinReplicas:                 project.Policies.MinReplicas,
			AllowedEnvironments:         append([]string(nil), project.Policies.AllowedEnvironments...),
			AllowedDeploymentStrategies: append([]string(nil), project.Policies.AllowedDeploymentStrategies...),
			AllowedClusterTargets:       append([]string(nil), project.Policies.AllowedClusterTargets...),
			ProdPRRequired:              project.Policies.ProdPRRequired,
			AutoRollbackEnabled:         project.Policies.AutoRollbackEnabled,
			RequiredProbes:              project.Policies.RequiredProbes,
		},
	}
	for _, environment := range project.Environments {
		item.Environments = append(item.Environments, catalogEnvironment{
			ID:        environment.ID,
			Name:      environment.Name,
			ClusterID: environment.ClusterID,
			WriteMode: environment.WriteMode,
			Default:   environment.Default,
		})
	}
	for _, repo := range project.Repositories {
		item.Repositories = append(item.Repositories, catalogRepository{
			ID:             repo.ID,
			Name:           repo.Name,
			URL:            repo.URL,
			Description:    repo.Description,
			Branch:         repo.Branch,
			AuthSecretPath: repo.AuthSecretPath,
			ConfigFile:     repo.ConfigFile,
		})
	}
	return item
}

func projectClusterIDs(project CatalogProject) []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(project.Environments))
	for _, environment := range project.Environments {
		clusterID := strings.TrimSpace(environment.ClusterID)
		if clusterID == "" {
			clusterID = "default"
		}
		if _, ok := seen[clusterID]; ok {
			continue
		}
		seen[clusterID] = struct{}{}
		items = append(items, clusterID)
	}
	if len(items) == 0 {
		return []string{"default"}
	}
	return items
}
