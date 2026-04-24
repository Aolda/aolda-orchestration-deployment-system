package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/server"
)

func TestGitModeCreateAndRedeployApplication(t *testing.T) {
	env := newGitTestEnvironment(t)

	createPayload := map[string]any{
		"name":               "git-app",
		"description":        "Git-backed deployment",
		"image":              "repo/git-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Standard",
		"secrets": []map[string]string{
			{"key": "DATABASE_URL", "value": "postgres://git-app"},
		},
	}

	createResponse := performJSONRequest(t, env, "POST", "/api/v1/projects/project-a/applications", createPayload, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if createResponse.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", createResponse.StatusCode)
	}

	verifyDir := cloneRemoteForVerification(t, env.repoRoot)
	appDir := filepath.Join(verifyDir, "apps", "project-a", "git-app", "base")
	for _, fileName := range []string{
		"kustomization.yaml",
		"deployment.yaml",
		"service.yaml",
		"servicemonitor.yaml",
		"prometheusrule.yaml",
		"externalsecret.yaml",
	} {
		if _, err := os.Stat(filepath.Join(appDir, fileName)); err != nil {
			t.Fatalf("expected %s in remote checkout: %v", fileName, err)
		}
	}
	for _, fileName := range []string{"virtualservice.yaml", "destinationrule.yaml"} {
		if _, err := os.Stat(filepath.Join(appDir, fileName)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be skipped for non-mesh rollout apps, got %v", fileName, err)
		}
	}
	serviceMonitorManifest, err := os.ReadFile(filepath.Join(appDir, "servicemonitor.yaml"))
	if err != nil {
		t.Fatalf("read remote servicemonitor manifest: %v", err)
	}
	if !strings.Contains(string(serviceMonitorManifest), "kind: ServiceMonitor") {
		t.Fatal("expected remote checkout to contain ServiceMonitor manifest")
	}
	prometheusRuleManifest, err := os.ReadFile(filepath.Join(appDir, "prometheusrule.yaml"))
	if err != nil {
		t.Fatalf("read remote prometheusrule manifest: %v", err)
	}
	if !strings.Contains(string(prometheusRuleManifest), "kind: PrometheusRule") {
		t.Fatal("expected remote checkout to contain PrometheusRule manifest")
	}
	assertFluxBootstrapFiles(t, verifyDir, "default")
	assertFluxChildManifestPath(t, verifyDir, "default", "project-a-git-app", "./apps/project-a/git-app/overlays/shared", true)

	redeployPayload := map[string]string{"imageTag": "v2"}
	redeployResponse := performJSONRequest(t, env, "POST", "/api/v1/applications/project-a__git-app/deployments", redeployPayload, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if redeployResponse.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", redeployResponse.StatusCode)
	}

	verifyDir = cloneRemoteForVerification(t, env.repoRoot)
	deploymentManifest, err := os.ReadFile(filepath.Join(verifyDir, "apps", "project-a", "git-app", "base", "deployment.yaml"))
	if err != nil {
		t.Fatalf("read remote deployment manifest: %v", err)
	}
	if !strings.Contains(string(deploymentManifest), "repo/git-app:v2") {
		t.Fatal("expected remote repo to contain redeployed image tag")
	}
	assertFluxChildManifestPath(t, verifyDir, "default", "project-a-git-app", "./apps/project-a/git-app/overlays/shared", true)

	if _, err := os.Stat(filepath.Join(env.vaultRoot, "aods", "apps", "project-a", "git-app", "prod.json")); err != nil {
		t.Fatalf("expected local vault final file: %v", err)
	}
}

func TestGitModeMissingProjectCatalogReturnsBootstrapError(t *testing.T) {
	env := newGitTestEnvironmentWithoutCatalog(t)

	response := performJSONRequest(t, env, "GET", "/api/v1/projects", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", response.StatusCode)
	}

	var body struct {
		Error struct {
			Code      string         `json:"code"`
			Message   string         `json:"message"`
			Details   map[string]any `json:"details"`
			Retryable bool           `json:"retryable"`
		} `json:"error"`
	}
	decodeBody(t, response, &body)

	if body.Error.Code != "PROJECT_CATALOG_NOT_BOOTSTRAPPED" {
		t.Fatalf("expected PROJECT_CATALOG_NOT_BOOTSTRAPPED, got %s", body.Error.Code)
	}
	if body.Error.Retryable {
		t.Fatal("expected bootstrap error to be non-retryable")
	}
	if got := body.Error.Details["path"]; got != filepath.Join("platform", "projects.yaml") {
		t.Fatalf("expected missing catalog path, got %#v", got)
	}
}

func TestGitModeBootstrapCluster(t *testing.T) {
	env := newGitTestEnvironment(t)

	payload := map[string]any{
		"id":          "edge",
		"name":        "Edge Cluster",
		"description": "Dedicated edge workloads",
	}

	response := performJSONRequest(t, env, "POST", "/api/v1/clusters", payload, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", response.StatusCode)
	}

	verifyDir := cloneRemoteForVerification(t, env.repoRoot)
	clustersYAML, err := os.ReadFile(filepath.Join(verifyDir, "platform", "clusters.yaml"))
	if err != nil {
		t.Fatalf("read remote clusters.yaml: %v", err)
	}
	if !strings.Contains(string(clustersYAML), "id: edge") {
		t.Fatalf("expected remote cluster catalog to contain edge cluster: %s", clustersYAML)
	}

	assertFluxBootstrapFiles(t, verifyDir, "edge")
	clusterRoot, err := os.ReadFile(filepath.Join(verifyDir, "platform", "flux", "clusters", "edge", "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read remote cluster root kustomization: %v", err)
	}
	if !strings.Contains(string(clusterRoot), "resources: []") {
		t.Fatalf("expected remote cluster root to start empty: %s", clusterRoot)
	}
}

func TestGitModeBootstrapProject(t *testing.T) {
	env := newGitTestEnvironment(t)

	payload := map[string]any{
		"id":   "project-z",
		"name": "project-z",
		"environments": []map[string]any{
			{
				"id":        "staging",
				"clusterId": "default",
				"writeMode": "direct",
				"default":   true,
			},
		},
	}

	response := performJSONRequest(t, env, "POST", "/api/v1/projects", payload, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", response.StatusCode)
	}

	verifyDir := cloneRemoteForVerification(t, env.repoRoot)
	projectsYAML, err := os.ReadFile(filepath.Join(verifyDir, "platform", "projects.yaml"))
	if err != nil {
		t.Fatalf("read remote projects.yaml: %v", err)
	}
	content := string(projectsYAML)
	for _, needle := range []string{
		"id: project-z",
		"namespace: project-z",
		"- aods:project-z:view",
		"- aods:project-z:deploy",
		"- aods:project-z:admin",
		"clusterId: default",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected remote projects.yaml to contain %q, got:\n%s", needle, content)
		}
	}

	assertFluxBootstrapFiles(t, verifyDir, "default")
}

func TestGitModeDeleteApplication(t *testing.T) {
	env := newGitTestEnvironment(t)

	createResponse := performJSONRequest(t, env, "POST", "/api/v1/projects/project-a/applications", map[string]any{
		"name":               "git-delete-app",
		"image":              "repo/git-delete-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Rollout",
		"secrets": []map[string]string{
			{"key": "DATABASE_URL", "value": "postgres://git-delete"},
		},
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResponse.StatusCode)
	}

	deleteResponse := performJSONRequest(t, env, "DELETE", "/api/v1/applications/project-a__git-delete-app", nil, map[string]string{
		"X-AODS-User-Id":  "user-2",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if deleteResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", deleteResponse.StatusCode)
	}

	verifyDir := cloneRemoteForVerification(t, env.repoRoot)
	if _, err := os.Stat(filepath.Join(verifyDir, "apps", "project-a", "git-delete-app")); !os.IsNotExist(err) {
		t.Fatalf("expected remote application directory to be removed, got %v", err)
	}
	assertNoFluxChildManifest(t, verifyDir, "default", "project-a-git-delete-app")

	if _, err := os.Stat(filepath.Join(env.vaultRoot, "aods", "apps", "project-a", "git-delete-app", "prod.json")); !os.IsNotExist(err) {
		t.Fatalf("expected final vault secret to be removed, got %v", err)
	}
}

func TestGitModeDeleteProject(t *testing.T) {
	env := newGitTestEnvironment(t)

	createResponse := performJSONRequest(t, env, "POST", "/api/v1/projects/project-a/applications", map[string]any{
		"name":               "git-delete-project-app",
		"image":              "repo/git-delete-project-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Rollout",
		"secrets": []map[string]string{
			{"key": "DATABASE_URL", "value": "postgres://git-project-delete"},
		},
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResponse.StatusCode)
	}

	deleteResponse := performJSONRequest(t, env, "DELETE", "/api/v1/projects/project-a", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if deleteResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", deleteResponse.StatusCode)
	}

	verifyDir := cloneRemoteForVerification(t, env.repoRoot)
	projectsYAML, err := os.ReadFile(filepath.Join(verifyDir, "platform", "projects.yaml"))
	if err != nil {
		t.Fatalf("read remote projects.yaml: %v", err)
	}
	if strings.Contains(string(projectsYAML), "id: project-a") {
		t.Fatalf("expected remote catalog to remove project-a, got:\n%s", projectsYAML)
	}
	if _, err := os.Stat(filepath.Join(verifyDir, "apps", "project-a")); !os.IsNotExist(err) {
		t.Fatalf("expected remote project directory to be removed, got %v", err)
	}
	assertNoFluxChildManifest(t, verifyDir, "default", "project-a-git-delete-project-app")

	if _, err := os.Stat(filepath.Join(env.vaultRoot, "aods", "apps", "project-a", "git-delete-project-app", "prod.json")); !os.IsNotExist(err) {
		t.Fatalf("expected local project vault secret to be removed, got %v", err)
	}
}

func newGitTestEnvironment(t *testing.T) testEnvironment {
	return newGitTestEnvironmentWithCatalog(t, true)
}

func newGitTestEnvironmentWithoutCatalog(t *testing.T) testEnvironment {
	return newGitTestEnvironmentWithCatalog(t, false)
}

func newGitTestEnvironmentWithCatalog(t *testing.T, includeCatalog bool) testEnvironment {
	t.Helper()

	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	managedRepoDir := filepath.Join(t.TempDir(), "managed-checkout")
	vaultRoot := t.TempDir()

	runGit(t, "", "init", "--bare", "--initial-branch=main", remoteDir)

	seedDir := filepath.Join(t.TempDir(), "seed")
	runGit(t, "", "clone", remoteDir, seedDir)
	runGit(t, seedDir, "config", "user.name", "Test User")
	runGit(t, seedDir, "config", "user.email", "test@example.com")

	readmePath := filepath.Join(seedDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# test manifest repo\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	runGit(t, seedDir, "add", "README.md")

	if includeCatalog {
		if err := os.MkdirAll(filepath.Join(seedDir, "platform"), 0o755); err != nil {
			t.Fatalf("create platform directory: %v", err)
		}

		projectsYAML := `projects:
  - id: project-a
    name: Project A
    description: Test project
    namespace: project-a
    access:
      viewerGroups:
        - aods:project-a:view
      deployerGroups:
        - aods:project-a:deploy
      adminGroups:
        - aods:platform:admin
`
		if err := os.WriteFile(filepath.Join(seedDir, "platform", "projects.yaml"), []byte(projectsYAML), 0o644); err != nil {
			t.Fatalf("write projects.yaml: %v", err)
		}

		runGit(t, seedDir, "add", "platform/projects.yaml")
		runGit(t, seedDir, "commit", "-m", "seed project catalog")
	} else {
		runGit(t, seedDir, "commit", "-m", "seed repo without project catalog")
	}
	runGit(t, seedDir, "push", "origin", "HEAD:main")

	cfg := core.Config{
		RepoRoot:         remoteDir,
		GitMode:          "git",
		GitRepoDir:       managedRepoDir,
		GitRemote:        remoteDir,
		GitBranch:        "main",
		GitAuthorName:    "AODS Bot",
		GitAuthorEmail:   "aods-bot@example.com",
		AllowedOrigin:    "*",
		AllowDevFallback: false,
		LocalVaultDir:    vaultRoot,
	}

	handler, _, _ := server.New(cfg)
	httpServer := httptestNewServer(t, handler)

	return testEnvironment{
		server:    httpServer,
		repoRoot:  remoteDir,
		vaultRoot: vaultRoot,
	}
}

func cloneRemoteForVerification(t *testing.T, remoteDir string) string {
	t.Helper()

	verifyDir := filepath.Join(t.TempDir(), "verify")
	runGit(t, "", "clone", "--branch=main", "--single-branch", remoteDir, verifyDir)
	return verifyDir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func httptestNewServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)
	return httpServer
}
