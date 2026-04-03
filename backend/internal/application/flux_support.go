package application

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultFluxKustomizationNamespace = "flux-system"
	defaultFluxSourceName             = "aods-manifest"
	fluxBootstrapRootFileName         = "root-kustomization.yaml"
)

func (p ProjectContext) clusterIDForEnvironment(environment string) string {
	environment = strings.TrimSpace(environment)
	if clusterID := strings.TrimSpace(p.EnvironmentClusters[environment]); clusterID != "" {
		return clusterID
	}
	for _, item := range p.Environments {
		if clusterID := strings.TrimSpace(p.EnvironmentClusters[item]); clusterID != "" {
			return clusterID
		}
	}
	for _, item := range p.Policies.AllowedClusterTargets {
		if clusterID := strings.TrimSpace(item); clusterID != "" {
			return clusterID
		}
	}
	return "default"
}

func (p ProjectContext) clusterIDs() []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(p.EnvironmentClusters))
	for _, environment := range p.Environments {
		clusterID := strings.TrimSpace(p.EnvironmentClusters[environment])
		if clusterID == "" {
			continue
		}
		if _, ok := seen[clusterID]; ok {
			continue
		}
		seen[clusterID] = struct{}{}
		items = append(items, clusterID)
	}
	if len(items) == 0 {
		items = append(items, p.clusterIDForEnvironment(""))
	}
	sort.Strings(items)
	return items
}

func (s LocalManifestStore) syncFluxWiring(record Record, project ProjectContext, previousEnvironment string) error {
	for _, clusterID := range project.clusterIDs() {
		if err := s.ensureFluxClusterScaffold(clusterID); err != nil {
			return err
		}
	}

	currentClusterID := project.clusterIDForEnvironment(record.DefaultEnvironment)
	if err := s.writeFluxChildKustomization(record, currentClusterID); err != nil {
		return err
	}
	if err := s.rewriteFluxClusterRoot(currentClusterID); err != nil {
		return err
	}

	previousClusterID := project.clusterIDForEnvironment(previousEnvironment)
	if previousClusterID != "" && previousClusterID != currentClusterID {
		if err := s.removeFluxChildKustomization(record, previousClusterID); err != nil {
			return err
		}
		if err := s.rewriteFluxClusterRoot(previousClusterID); err != nil {
			return err
		}
	}

	return nil
}

func (s LocalManifestStore) ensureFluxClusterScaffold(clusterID string) error {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		clusterID = "default"
	}

	clusterDir := s.fluxClusterDir(clusterID)
	if err := os.MkdirAll(filepath.Join(clusterDir, "applications"), 0o755); err != nil {
		return fmt.Errorf("create flux cluster directory: %w", err)
	}

	bootstrapDir := s.fluxBootstrapDir(clusterID)
	if err := os.MkdirAll(bootstrapDir, 0o755); err != nil {
		return fmt.Errorf("create flux bootstrap directory: %w", err)
	}

	if err := os.WriteFile(filepath.Join(bootstrapDir, "kustomization.yaml"), []byte(renderBootstrapKustomization()), 0o644); err != nil {
		return fmt.Errorf("write flux bootstrap kustomization: %w", err)
	}
	if err := os.WriteFile(
		filepath.Join(bootstrapDir, fluxBootstrapRootFileName),
		[]byte(s.renderFluxBootstrapRoot(clusterID)),
		0o644,
	); err != nil {
		return fmt.Errorf("write flux bootstrap root manifest: %w", err)
	}

	if err := s.rewriteFluxClusterRoot(clusterID); err != nil {
		return err
	}

	return nil
}

func (s LocalManifestStore) rewriteFluxClusterRoot(clusterID string) error {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		clusterID = "default"
	}

	applicationsDir := filepath.Join(s.fluxClusterDir(clusterID), "applications")
	entries, err := os.ReadDir(applicationsDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read flux applications directory: %w", err)
	}

	resources := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		resources = append(resources, filepath.ToSlash(filepath.Join("applications", entry.Name())))
	}
	sort.Strings(resources)

	if err := os.MkdirAll(s.fluxClusterDir(clusterID), 0o755); err != nil {
		return fmt.Errorf("create flux cluster root: %w", err)
	}
	if err := os.WriteFile(
		filepath.Join(s.fluxClusterDir(clusterID), "kustomization.yaml"),
		[]byte(renderFluxClusterRootKustomization(resources)),
		0o644,
	); err != nil {
		return fmt.Errorf("write flux cluster root kustomization: %w", err)
	}

	return nil
}

func (s LocalManifestStore) writeFluxChildKustomization(record Record, clusterID string) error {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		clusterID = "default"
	}

	path := filepath.Join(s.fluxClusterDir(clusterID), "applications", fluxChildFileName(record)+".yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create flux child directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(s.renderFluxChildKustomization(record)), 0o644); err != nil {
		return fmt.Errorf("write flux child kustomization: %w", err)
	}

	return nil
}

func (s LocalManifestStore) removeFluxChildKustomization(record Record, clusterID string) error {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return nil
	}
	path := filepath.Join(s.fluxClusterDir(clusterID), "applications", fluxChildFileName(record)+".yaml")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale flux child kustomization: %w", err)
	}
	return nil
}

func (s LocalManifestStore) fluxClusterDir(clusterID string) string {
	return filepath.Join(s.RepoRoot, "platform", "flux", "clusters", clusterID)
}

func (s LocalManifestStore) fluxBootstrapDir(clusterID string) string {
	return filepath.Join(s.RepoRoot, "platform", "flux", "bootstrap", clusterID)
}

func (s LocalManifestStore) renderFluxBootstrapRoot(clusterID string) string {
	return fmt.Sprintf(`apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: %s
  namespace: %s
spec:
  interval: 1m0s
  prune: true
  wait: false
  timeout: 3m0s
  path: %s
  sourceRef:
    kind: GitRepository
    name: %s
`, yamlScalar("aods-root-"+clusterID), yamlScalar(s.fluxKustomizationNamespace()), yamlScalar("./platform/flux/clusters/"+clusterID), yamlScalar(s.fluxSourceName()))
}

func (s LocalManifestStore) renderFluxChildKustomization(record Record) string {
	return fmt.Sprintf(`apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: %s
  namespace: %s
  annotations:
    aods.io/application-id: %s
    aods.io/project-id: %s
    aods.io/environment: %s
spec:
  interval: 1m0s
  prune: true
  wait: %t
  timeout: 3m0s
  path: %s
  targetNamespace: %s
  sourceRef:
    kind: GitRepository
    name: %s
`, yamlScalar(fluxChildName(record)), yamlScalar(s.fluxKustomizationNamespace()), yamlScalar(record.ID), yamlScalar(record.ProjectID), yamlScalar(record.DefaultEnvironment), fluxChildWait(record), yamlScalar("./"+fluxOverlayPath(record)), yamlScalar(record.Namespace), yamlScalar(s.fluxSourceName()))
}

func fluxChildWait(record Record) bool {
	return record.DeploymentStrategy != DeploymentStrategyCanary
}

func (s LocalManifestStore) fluxKustomizationNamespace() string {
	if namespace := strings.TrimSpace(s.FluxKustomizationNamespace); namespace != "" {
		return namespace
	}
	return defaultFluxKustomizationNamespace
}

func (s LocalManifestStore) fluxSourceName() string {
	if sourceName := strings.TrimSpace(s.FluxSourceName); sourceName != "" {
		return sourceName
	}
	return defaultFluxSourceName
}

func fluxChildName(record Record) string {
	return fmt.Sprintf("%s-%s", record.ProjectID, record.Name)
}

func fluxChildFileName(record Record) string {
	return fluxChildName(record)
}

func fluxOverlayPath(record Record) string {
	environment := strings.TrimSpace(record.DefaultEnvironment)
	if environment == "" {
		environment = "prod"
	}
	return filepath.ToSlash(path.Join("apps", record.ProjectID, record.Name, "overlays", environment))
}

func renderBootstrapKustomization() string {
	return fmt.Sprintf(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - %s
`, fluxBootstrapRootFileName)
}

func renderFluxClusterRootKustomization(resources []string) string {
	if len(resources) == 0 {
		return `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources: []
`
	}

	var builder strings.Builder
	builder.WriteString(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
`)
	for _, resource := range resources {
		builder.WriteString("  - ")
		builder.WriteString(resource)
		builder.WriteByte('\n')
	}

	return builder.String()
}
