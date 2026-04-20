package cluster

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aolda/aods-backend/internal/fluxscaffold"
)

func TestLocalSourceListClustersDefaultsAndNormalization(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	source := LocalSource{Path: filepath.Join(rootDir, "platform", "clusters.yaml"), RepoRoot: rootDir}

	items, err := source.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("list clusters without catalog: %v", err)
	}
	if len(items) != 1 || items[0].ID != "default" {
		t.Fatalf("expected default cluster fallback, got %#v", items)
	}

	if err := os.MkdirAll(filepath.Dir(source.Path), 0o755); err != nil {
		t.Fatalf("create catalog dir: %v", err)
	}
	if err := os.WriteFile(source.Path, []byte(`clusters:
  - id: edge
    name: "  Edge Cluster  "
  - id: analytics
    default: true
  - id: edge
    name: duplicate ignored because first wins?
  - id: ""
`), 0o644); err != nil {
		t.Fatalf("write cluster catalog: %v", err)
	}

	items, err = source.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("list normalized clusters: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 normalized clusters, got %d", len(items))
	}
	if items[0].Name != "Edge Cluster" {
		t.Fatalf("expected trimmed cluster name, got %#v", items[0])
	}
	if !items[1].Default {
		t.Fatalf("expected analytics to remain the single default, got %#v", items)
	}
}

func TestLocalSourceCreateClusterWritesCatalogAndFluxArtifacts(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	source := LocalSource{
		Path:                       filepath.Join(rootDir, "platform", "clusters.yaml"),
		RepoRoot:                   rootDir,
		FluxKustomizationNamespace: "custom-system",
		FluxSourceName:             "custom-source",
	}

	created, err := source.CreateCluster(context.Background(), CreateRequest{
		ID:          "edge",
		Name:        "Edge Cluster",
		Description: "Extra capacity",
		Default:     true,
	})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if created.ID != "edge" || !created.Default {
		t.Fatalf("unexpected created cluster: %#v", created)
	}

	catalogData, err := os.ReadFile(source.Path)
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	if string(catalogData) == "" {
		t.Fatal("expected cluster catalog to be written")
	}

	bootstrapRoot, err := os.ReadFile(filepath.Join(rootDir, "platform", "flux", "bootstrap", "edge", fluxscaffold.BootstrapRootFileName))
	if err != nil {
		t.Fatalf("read bootstrap root: %v", err)
	}
	if string(bootstrapRoot) == "" {
		t.Fatal("expected bootstrap root manifest to be written")
	}

	clusterRoot, err := os.ReadFile(filepath.Join(rootDir, "platform", "flux", "clusters", "edge", "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read cluster root: %v", err)
	}
	if string(clusterRoot) == "" {
		t.Fatal("expected cluster root manifest to be written")
	}

	if _, err := source.CreateCluster(context.Background(), CreateRequest{ID: "edge", Name: "Duplicate"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate cluster id, got %v", err)
	}
}

func TestLocalSourceCreateClusterRollsBackCatalogOnFluxFailure(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	invalidRepoRoot := filepath.Join(rootDir, "repo-root-file")
	if err := os.WriteFile(invalidRepoRoot, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write invalid repo root file: %v", err)
	}

	source := LocalSource{
		Path:     filepath.Join(rootDir, "platform", "clusters.yaml"),
		RepoRoot: invalidRepoRoot,
	}

	err := os.MkdirAll(filepath.Dir(source.Path), 0o755)
	if err != nil {
		t.Fatalf("create catalog dir: %v", err)
	}
	if err := os.WriteFile(source.Path, []byte("clusters:\n  - id: default\n    default: true\n"), 0o644); err != nil {
		t.Fatalf("write original catalog: %v", err)
	}

	_, err = source.CreateCluster(context.Background(), CreateRequest{
		ID:      "edge",
		Name:    "Edge Cluster",
		Default: true,
	})
	if err == nil {
		t.Fatal("expected flux scaffold failure")
	}

	catalogData, readErr := os.ReadFile(source.Path)
	if readErr != nil {
		t.Fatalf("read rolled back catalog: %v", readErr)
	}
	if string(catalogData) != "clusters:\n  - id: default\n    default: true\n" {
		t.Fatalf("expected original catalog to be restored, got:\n%s", string(catalogData))
	}
}
