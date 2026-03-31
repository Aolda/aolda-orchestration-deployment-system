package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/server"
)

func TestProjectsAreFilteredByRole(t *testing.T) {
	env := newTestEnvironment(t)

	response := performJSONRequest(t, env, http.MethodGet, "/api/v1/projects", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "alice",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}

	var body struct {
		Items []struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		} `json:"items"`
	}
	decodeBody(t, response, &body)

	if len(body.Items) != 1 {
		t.Fatalf("expected 1 project, got %d", len(body.Items))
	}
	if body.Items[0].ID != "project-a" {
		t.Fatalf("expected project-a, got %s", body.Items[0].ID)
	}
	if body.Items[0].Role != "deployer" {
		t.Fatalf("expected deployer role, got %s", body.Items[0].Role)
	}
}

func TestViewerCannotCreateApplication(t *testing.T) {
	env := newTestEnvironment(t)

	payload := map[string]any{
		"name":               "forbidden-app",
		"image":              "repo/forbidden-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Standard",
	}

	response := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", payload, map[string]string{
		"X-AODS-User-Id":  "user-2",
		"X-AODS-Username": "viewer",
		"X-AODS-Groups":   "aods:project-a:view",
	})

	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.StatusCode)
	}
}

func TestCreateRedeployAndObserveApplication(t *testing.T) {
	env := newTestEnvironment(t)

	secretValue := "postgres://user:password@db.internal:5432/app"
	createPayload := map[string]any{
		"name":               "my-app",
		"description":        "Internal standard deployment",
		"image":              "repo/my-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Standard",
		"secrets": []map[string]string{
			{"key": "DATABASE_URL", "value": secretValue},
		},
	}

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", createPayload, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})

	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResponse.StatusCode)
	}

	appDir := filepath.Join(env.repoRoot, "apps", "project-a", "my-app", "base")
	requiredFiles := []string{
		"kustomization.yaml",
		"deployment.yaml",
		"service.yaml",
		"virtualservice.yaml",
		"destinationrule.yaml",
		"externalsecret.yaml",
	}

	for _, fileName := range requiredFiles {
		if _, err := os.Stat(filepath.Join(appDir, fileName)); err != nil {
			t.Fatalf("expected %s to exist: %v", fileName, err)
		}
	}

	repoFiles := []string{
		filepath.Join(appDir, "deployment.yaml"),
		filepath.Join(appDir, "service.yaml"),
		filepath.Join(appDir, "virtualservice.yaml"),
		filepath.Join(appDir, "destinationrule.yaml"),
		filepath.Join(appDir, "externalsecret.yaml"),
	}
	for _, path := range repoFiles {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(content), secretValue) {
			t.Fatalf("secret value leaked into manifest %s", path)
		}
	}

	finalVaultFile := filepath.Join(env.vaultRoot, "aods", "apps", "project-a", "my-app", "prod.json")
	vaultContent, err := os.ReadFile(finalVaultFile)
	if err != nil {
		t.Fatalf("expected local vault final file: %v", err)
	}
	if !strings.Contains(string(vaultContent), secretValue) {
		t.Fatal("expected local vault file to contain the secret value")
	}

	stagingMatches, err := filepath.Glob(filepath.Join(env.vaultRoot, "aods", "staging", "*.json"))
	if err != nil {
		t.Fatalf("glob staging files: %v", err)
	}
	if len(stagingMatches) != 0 {
		t.Fatalf("expected staging files to be cleaned up, found %d", len(stagingMatches))
	}

	listResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/projects/project-a/applications", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from list applications, got %d", listResponse.StatusCode)
	}

	redeployPayload := map[string]string{"imageTag": "v2"}
	redeployResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/applications/project-a__my-app/deployments", redeployPayload, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if redeployResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 from redeploy, got %d", redeployResponse.StatusCode)
	}

	deploymentManifest, err := os.ReadFile(filepath.Join(appDir, "deployment.yaml"))
	if err != nil {
		t.Fatalf("read deployment manifest: %v", err)
	}
	if !strings.Contains(string(deploymentManifest), "repo/my-app:v2") {
		t.Fatal("expected redeploy to update deployment image tag")
	}

	syncResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/applications/project-a__my-app/sync-status", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if syncResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from sync-status, got %d", syncResponse.StatusCode)
	}

	var syncBody struct {
		Status string `json:"status"`
	}
	decodeBody(t, syncResponse, &syncBody)
	if syncBody.Status != "Synced" {
		t.Fatalf("expected Synced status, got %s", syncBody.Status)
	}

	metricsResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/applications/project-a__my-app/metrics", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if metricsResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from metrics, got %d", metricsResponse.StatusCode)
	}

	var metricsBody struct {
		Metrics []struct {
			Key string `json:"key"`
		} `json:"metrics"`
	}
	decodeBody(t, metricsResponse, &metricsBody)
	if len(metricsBody.Metrics) != 5 {
		t.Fatalf("expected 5 metric series, got %d", len(metricsBody.Metrics))
	}
}

type testEnvironment struct {
	server    *httptest.Server
	repoRoot  string
	vaultRoot string
}

func newTestEnvironment(t *testing.T) testEnvironment {
	t.Helper()

	repoRoot := t.TempDir()
	vaultRoot := t.TempDir()

	if err := os.MkdirAll(filepath.Join(repoRoot, "platform"), 0o755); err != nil {
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
	if err := os.WriteFile(filepath.Join(repoRoot, "platform", "projects.yaml"), []byte(projectsYAML), 0o644); err != nil {
		t.Fatalf("write projects.yaml: %v", err)
	}

	handler := server.New(core.Config{
		RepoRoot:         repoRoot,
		AllowedOrigin:    "*",
		AllowDevFallback: false,
		LocalVaultDir:    vaultRoot,
	})

	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	return testEnvironment{
		server:    httpServer,
		repoRoot:  repoRoot,
		vaultRoot: vaultRoot,
	}
}

func performJSONRequest(
	t *testing.T,
	env testEnvironment,
	method string,
	path string,
	body any,
	headers map[string]string,
) *http.Response {
	t.Helper()

	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	}

	request, err := http.NewRequest(method, env.server.URL+path, requestBody)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := env.server.Client().Do(request)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	t.Cleanup(func() {
		_ = response.Body.Close()
	})

	return response
}

func decodeBody(t *testing.T, response *http.Response, target any) {
	t.Helper()

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}
