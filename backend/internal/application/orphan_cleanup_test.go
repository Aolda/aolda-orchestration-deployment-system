package application

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocalManifestStoreCleanupOrphanFluxManifestsRemovesManagedOrphans(t *testing.T) {
	repoRoot := t.TempDir()
	store := LocalManifestStore{RepoRoot: repoRoot}

	if err := os.MkdirAll(filepath.Join(repoRoot, "platform"), 0o755); err != nil {
		t.Fatalf("create platform directory: %v", err)
	}
	projectsYAML := `projects:
  - id: project-a
    name: Project A
    description: Project A
    namespace: project-a
    access:
      viewerGroups: []
      deployerGroups: []
      adminGroups: []
    environments:
      - id: prod
        name: Production
        clusterId: default
        writeMode: direct
        default: true
      - id: dev
        name: Development
        clusterId: analytics
        writeMode: direct
        default: false
    policies:
      minReplicas: 1
      allowedEnvironments: [prod, dev]
      allowedDeploymentStrategies: [Standard, Canary]
      allowedClusterTargets: [default, analytics]
      prodPRRequired: false
      autoRollbackEnabled: false
      requiredProbes: true
`
	if err := os.WriteFile(filepath.Join(repoRoot, "platform", "projects.yaml"), []byte(projectsYAML), 0o644); err != nil {
		t.Fatalf("write projects.yaml: %v", err)
	}

	projectContext := ProjectContext{
		ID:           "project-a",
		Namespace:    "project-a",
		Environments: []string{"prod", "dev"},
		EnvironmentClusters: map[string]string{
			"prod": "default",
			"dev":  "analytics",
		},
		Policies: projectPolicy{
			MinReplicas:                 1,
			AllowedEnvironments:         []string{"prod", "dev"},
			AllowedDeploymentStrategies: []string{"Rollout", "Canary"},
			AllowedClusterTargets:       []string{"default", "analytics"},
			RequiredProbes:              true,
		},
	}

	record, err := store.CreateApplication(context.Background(), projectContext, CreateRequest{
		Name:                "live-app",
		Description:         "managed application",
		Image:               "repo/live-app:v1",
		ServicePort:         8080,
		DeploymentStrategy:  DeploymentStrategyStandard,
		Environment:         "dev",
		RepositoryID:        "repo-a",
		RepositoryServiceID: "svc-a",
		ConfigPath:          "aolda_deploy.json",
	}, "")
	if err != nil {
		t.Fatalf("create application: %v", err)
	}

	if err := store.ensureFluxClusterScaffold("default"); err != nil {
		t.Fatalf("ensure default scaffold: %v", err)
	}

	staleDuplicatePath := filepath.Join(repoRoot, "platform", "flux", "clusters", "default", "applications", "project-a-live-app.yaml")
	if err := os.WriteFile(staleDuplicatePath, []byte(store.renderFluxChildKustomization(record)), 0o644); err != nil {
		t.Fatalf("write stale duplicate manifest: %v", err)
	}

	deletedRecord := Record{
		ID:                 "project-a__deleted-app",
		ProjectID:          "project-a",
		Namespace:          "project-a",
		Name:               "deleted-app",
		Image:              "repo/deleted-app:v1",
		ServicePort:        8080,
		Replicas:           1,
		RequiredProbes:     true,
		DeploymentStrategy: DeploymentStrategyRollout,
		DefaultEnvironment: "prod",
	}
	deletedManifestPath := filepath.Join(repoRoot, "platform", "flux", "clusters", "default", "applications", "project-a-deleted-app.yaml")
	if err := os.WriteFile(deletedManifestPath, []byte(store.renderFluxChildKustomization(deletedRecord)), 0o644); err != nil {
		t.Fatalf("write orphan manifest: %v", err)
	}

	legacyRecord := Record{
		ID:                 "project-z__legacy-app",
		ProjectID:          "project-z",
		Namespace:          "legacy",
		Name:               "legacy-app",
		Image:              "repo/legacy-app:v1",
		ServicePort:        8080,
		Replicas:           1,
		RequiredProbes:     true,
		DeploymentStrategy: DeploymentStrategyRollout,
		DefaultEnvironment: "prod",
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}
	if err := writeMetadata(repoRoot, legacyRecord, []string{"prod"}); err != nil {
		t.Fatalf("write legacy metadata: %v", err)
	}
	legacyManifestPath := filepath.Join(repoRoot, "platform", "flux", "clusters", "default", "applications", "project-z-legacy-app.yaml")
	if err := os.WriteFile(legacyManifestPath, []byte(store.renderFluxChildKustomization(legacyRecord)), 0o644); err != nil {
		t.Fatalf("write legacy manifest: %v", err)
	}

	manualManifestPath := filepath.Join(repoRoot, "platform", "flux", "clusters", "default", "applications", "manual.yaml")
	if err := os.WriteFile(manualManifestPath, []byte(`apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: manual
  namespace: flux-system
spec:
  interval: 1m0s
`), 0o644); err != nil {
		t.Fatalf("write manual manifest: %v", err)
	}

	count, err := store.CleanupOrphanFluxManifests(context.Background())
	if err != nil {
		t.Fatalf("cleanup orphan manifests: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 orphan manifests to be removed, got %d", count)
	}

	if _, err := os.Stat(staleDuplicatePath); !os.IsNotExist(err) {
		t.Fatalf("expected stale duplicate manifest to be removed, got %v", err)
	}
	if _, err := os.Stat(deletedManifestPath); !os.IsNotExist(err) {
		t.Fatalf("expected deleted application manifest to be removed, got %v", err)
	}
	if _, err := os.Stat(legacyManifestPath); err != nil {
		t.Fatalf("expected legacy manifest to remain: %v", err)
	}
	if _, err := os.Stat(manualManifestPath); err != nil {
		t.Fatalf("expected manual manifest to remain: %v", err)
	}

	analyticsRoot, err := os.ReadFile(filepath.Join(repoRoot, "platform", "flux", "clusters", "analytics", "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read analytics root kustomization: %v", err)
	}
	if !strings.Contains(string(analyticsRoot), "applications/project-a-live-app.yaml") {
		t.Fatalf("expected analytics root to keep managed application: %s", analyticsRoot)
	}

	defaultRoot, err := os.ReadFile(filepath.Join(repoRoot, "platform", "flux", "clusters", "default", "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read default root kustomization: %v", err)
	}
	defaultRootContent := string(defaultRoot)
	if strings.Contains(defaultRootContent, "applications/project-a-live-app.yaml") {
		t.Fatalf("expected default root to drop stale duplicate child: %s", defaultRootContent)
	}
	if strings.Contains(defaultRootContent, "applications/project-a-deleted-app.yaml") {
		t.Fatalf("expected default root to drop deleted application child: %s", defaultRootContent)
	}
	if !strings.Contains(defaultRootContent, "applications/project-z-legacy-app.yaml") {
		t.Fatalf("expected default root to keep uncatalogued legacy child: %s", defaultRootContent)
	}
	if !strings.Contains(defaultRootContent, "applications/manual.yaml") {
		t.Fatalf("expected default root to keep unmanaged manifest: %s", defaultRootContent)
	}
}
