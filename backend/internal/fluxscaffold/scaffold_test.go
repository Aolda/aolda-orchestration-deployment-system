package fluxscaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureClusterCreatesBootstrapAndClusterScaffold(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	cfg := Config{
		RepoRoot:               repoRoot,
		ClusterID:              "  ",
		KustomizationNamespace: "custom-system",
		SourceName:             "custom-source",
	}

	if err := EnsureCluster(cfg); err != nil {
		t.Fatalf("ensure cluster scaffold: %v", err)
	}

	clusterDir := ClusterDir(repoRoot, "default")
	bootstrapDir := BootstrapDir(repoRoot, "default")
	for _, path := range []string{
		filepath.Join(clusterDir, "applications"),
		filepath.Join(clusterDir, "kustomization.yaml"),
		filepath.Join(bootstrapDir, "kustomization.yaml"),
		filepath.Join(bootstrapDir, BootstrapRootFileName),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected scaffold path %s to exist: %v", path, err)
		}
	}

	bootstrapRoot, err := os.ReadFile(filepath.Join(bootstrapDir, BootstrapRootFileName))
	if err != nil {
		t.Fatalf("read bootstrap root manifest: %v", err)
	}
	manifest := string(bootstrapRoot)
	if !strings.Contains(manifest, "namespace: custom-system") {
		t.Fatalf("expected custom namespace in manifest, got:\n%s", manifest)
	}
	if !strings.Contains(manifest, "name: custom-source") {
		t.Fatalf("expected custom source name in manifest, got:\n%s", manifest)
	}
	if !strings.Contains(manifest, "path: ./platform/flux/clusters/default") {
		t.Fatalf("expected default cluster path in manifest, got:\n%s", manifest)
	}
}

func TestRewriteClusterRootSortsResourcesAndIgnoresNonManifestFiles(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	applicationsDir := filepath.Join(ClusterDir(repoRoot, "edge"), "applications")
	if err := os.MkdirAll(filepath.Join(applicationsDir, "nested"), 0o755); err != nil {
		t.Fatalf("create applications dir: %v", err)
	}

	files := map[string]string{
		filepath.Join(applicationsDir, "b.yaml"):          "b",
		filepath.Join(applicationsDir, "a.yaml"):          "a",
		filepath.Join(applicationsDir, "notes.txt"):       "ignore",
		filepath.Join(applicationsDir, "nested", "c.yaml"): "ignore nested",
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
	}

	if err := RewriteClusterRoot(Config{RepoRoot: repoRoot, ClusterID: "edge"}); err != nil {
		t.Fatalf("rewrite cluster root: %v", err)
	}

	kustomization, err := os.ReadFile(filepath.Join(ClusterDir(repoRoot, "edge"), "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read cluster root kustomization: %v", err)
	}

	want := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - applications/a.yaml
  - applications/b.yaml
`
	if string(kustomization) != want {
		t.Fatalf("unexpected cluster root kustomization:\n%s", string(kustomization))
	}
}

func TestConfigDefaultValues(t *testing.T) {
	t.Parallel()

	cfg := Config{}

	if got := cfg.kustomizationNamespace(); got != DefaultKustomizationNamespace {
		t.Fatalf("expected default namespace %q, got %q", DefaultKustomizationNamespace, got)
	}
	if got := cfg.sourceName(); got != DefaultSourceName {
		t.Fatalf("expected default source name %q, got %q", DefaultSourceName, got)
	}
	if got := normalizeClusterID(""); got != "default" {
		t.Fatalf("expected normalized cluster id default, got %q", got)
	}
}
