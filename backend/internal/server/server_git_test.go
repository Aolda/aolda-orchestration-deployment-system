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
		"virtualservice.yaml",
		"destinationrule.yaml",
		"externalsecret.yaml",
	} {
		if _, err := os.Stat(filepath.Join(appDir, fileName)); err != nil {
			t.Fatalf("expected %s in remote checkout: %v", fileName, err)
		}
	}

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

	handler := server.New(core.Config{
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
	})

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
