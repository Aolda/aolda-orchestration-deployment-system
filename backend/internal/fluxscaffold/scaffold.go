package fluxscaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultKustomizationNamespace = "flux-system"
	DefaultSourceName             = "aods-manifest"
	BootstrapRootFileName         = "root-kustomization.yaml"
)

type Config struct {
	RepoRoot               string
	ClusterID              string
	KustomizationNamespace string
	SourceName             string
}

func EnsureCluster(cfg Config) error {
	clusterID := normalizeClusterID(cfg.ClusterID)
	clusterDir := ClusterDir(cfg.RepoRoot, clusterID)
	if err := os.MkdirAll(filepath.Join(clusterDir, "applications"), 0o755); err != nil {
		return fmt.Errorf("create flux cluster directory: %w", err)
	}

	bootstrapDir := BootstrapDir(cfg.RepoRoot, clusterID)
	if err := os.MkdirAll(bootstrapDir, 0o755); err != nil {
		return fmt.Errorf("create flux bootstrap directory: %w", err)
	}

	if err := os.WriteFile(filepath.Join(bootstrapDir, "kustomization.yaml"), []byte(renderBootstrapKustomization()), 0o644); err != nil {
		return fmt.Errorf("write flux bootstrap kustomization: %w", err)
	}
	if err := os.WriteFile(
		filepath.Join(bootstrapDir, BootstrapRootFileName),
		[]byte(renderFluxBootstrapRoot(clusterID, cfg.kustomizationNamespace(), cfg.sourceName())),
		0o644,
	); err != nil {
		return fmt.Errorf("write flux bootstrap root manifest: %w", err)
	}

	return RewriteClusterRoot(cfg)
}

func RewriteClusterRoot(cfg Config) error {
	clusterID := normalizeClusterID(cfg.ClusterID)
	applicationsDir := filepath.Join(ClusterDir(cfg.RepoRoot, clusterID), "applications")
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

	clusterDir := ClusterDir(cfg.RepoRoot, clusterID)
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		return fmt.Errorf("create flux cluster root: %w", err)
	}
	if err := os.WriteFile(
		filepath.Join(clusterDir, "kustomization.yaml"),
		[]byte(renderFluxClusterRootKustomization(resources)),
		0o644,
	); err != nil {
		return fmt.Errorf("write flux cluster root kustomization: %w", err)
	}

	return nil
}

func ClusterDir(repoRoot string, clusterID string) string {
	return filepath.Join(repoRoot, "platform", "flux", "clusters", normalizeClusterID(clusterID))
}

func BootstrapDir(repoRoot string, clusterID string) string {
	return filepath.Join(repoRoot, "platform", "flux", "bootstrap", normalizeClusterID(clusterID))
}

func (c Config) kustomizationNamespace() string {
	if namespace := strings.TrimSpace(c.KustomizationNamespace); namespace != "" {
		return namespace
	}
	return DefaultKustomizationNamespace
}

func (c Config) sourceName() string {
	if sourceName := strings.TrimSpace(c.SourceName); sourceName != "" {
		return sourceName
	}
	return DefaultSourceName
}

func normalizeClusterID(clusterID string) string {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return "default"
	}
	return clusterID
}

func renderFluxBootstrapRoot(clusterID string, namespace string, sourceName string) string {
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
`, yamlScalar("aods-root-"+clusterID), yamlScalar(namespace), yamlScalar("./platform/flux/clusters/"+clusterID), yamlScalar(sourceName))
}

func renderBootstrapKustomization() string {
	return fmt.Sprintf(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - %s
`, BootstrapRootFileName)
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

func yamlScalar(value string) string {
	data, err := yaml.Marshal(value)
	if err != nil {
		return `""`
	}
	return strings.TrimSpace(string(data))
}
