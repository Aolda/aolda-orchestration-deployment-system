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

func TestCreateApplicationWithoutSecretsSkipsSecretArtifacts(t *testing.T) {
	env := newTestEnvironment(t)

	createPayload := map[string]any{
		"name":               "stateless-app",
		"description":        "No secrets required",
		"image":              "repo/stateless-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Standard",
	}

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", createPayload, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResponse.StatusCode)
	}

	appDir := filepath.Join(env.repoRoot, "apps", "project-a", "stateless-app", "base")
	if _, err := os.Stat(filepath.Join(appDir, "externalsecret.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected externalsecret.yaml to be skipped, got %v", err)
	}

	deploymentManifest, err := os.ReadFile(filepath.Join(appDir, "deployment.yaml"))
	if err != nil {
		t.Fatalf("read deployment manifest: %v", err)
	}
	if strings.Contains(string(deploymentManifest), "envFrom:") {
		t.Fatal("expected deployment manifest to omit envFrom when no secrets are provided")
	}

	kustomizationManifest, err := os.ReadFile(filepath.Join(appDir, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read kustomization manifest: %v", err)
	}
	if strings.Contains(string(kustomizationManifest), "externalsecret.yaml") {
		t.Fatal("expected kustomization to omit externalsecret.yaml")
	}

	stagingMatches, err := filepath.Glob(filepath.Join(env.vaultRoot, "aods", "staging", "*.json"))
	if err != nil {
		t.Fatalf("glob staging files: %v", err)
	}
	if len(stagingMatches) != 0 {
		t.Fatalf("expected no staged vault files, found %d", len(stagingMatches))
	}
}

func TestCanaryApplicationCreatesRolloutArtifacts(t *testing.T) {
	env := newTestEnvironment(t)

	createPayload := map[string]any{
		"name":               "canary-app",
		"description":        "Canary deployment",
		"image":              "repo/canary-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Canary",
		"environment":        "dev",
	}

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", createPayload, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResponse.StatusCode)
	}

	appDir := filepath.Join(env.repoRoot, "apps", "project-a", "canary-app", "base")
	for _, fileName := range []string{
		"kustomization.yaml",
		"rollout.yaml",
		"service.yaml",
		"canary-service.yaml",
		"virtualservice.yaml",
		"destinationrule.yaml",
	} {
		if _, err := os.Stat(filepath.Join(appDir, fileName)); err != nil {
			t.Fatalf("expected %s to exist: %v", fileName, err)
		}
	}

	if _, err := os.Stat(filepath.Join(env.repoRoot, "apps", "project-a", "canary-app", "overlays", "dev", "kustomization.yaml")); err != nil {
		t.Fatalf("expected dev overlay to exist: %v", err)
	}

	deploymentsResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/applications/project-a__canary-app/deployments", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if deploymentsResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from deployments, got %d", deploymentsResponse.StatusCode)
	}
}

func TestProjectPoliciesCanBeUpdatedByAdmin(t *testing.T) {
	env := newTestEnvironment(t)

	response := performJSONRequest(t, env, http.MethodPatch, "/api/v1/projects/project-a/policies", map[string]any{
		"minReplicas":                 3,
		"allowedEnvironments":         []string{"dev", "prod"},
		"allowedDeploymentStrategies": []string{"Standard", "Canary"},
		"allowedClusterTargets":       []string{"default"},
		"prodPRRequired":              true,
		"autoRollbackEnabled":         true,
		"requiredProbes":              true,
	}, map[string]string{
		"X-AODS-User-Id":  "admin-1",
		"X-AODS-Username": "admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}

	getResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/projects/project-a/policies", nil, map[string]string{
		"X-AODS-User-Id":  "admin-1",
		"X-AODS-Username": "admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if getResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from get policies, got %d", getResponse.StatusCode)
	}

	var body struct {
		MinReplicas         int  `json:"minReplicas"`
		ProdPRRequired      bool `json:"prodPRRequired"`
		AutoRollbackEnabled bool `json:"autoRollbackEnabled"`
	}
	decodeBody(t, getResponse, &body)
	if body.MinReplicas != 3 || !body.ProdPRRequired || !body.AutoRollbackEnabled {
		t.Fatalf("expected updated policy body, got %#v", body)
	}
}

func TestChangeReviewFlowAppliesPRManagedMutation(t *testing.T) {
	env := newTestEnvironment(t)

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
    environments:
      - id: prod
        name: Production
        clusterId: default
        writeMode: pull_request
        default: true
    policies:
      minReplicas: 1
      allowedEnvironments:
        - prod
      allowedDeploymentStrategies:
        - Standard
        - Canary
      allowedClusterTargets:
        - default
      prodPRRequired: true
      autoRollbackEnabled: false
      requiredProbes: true
`
	if err := os.WriteFile(filepath.Join(env.repoRoot, "platform", "projects.yaml"), []byte(projectsYAML), 0o644); err != nil {
		t.Fatalf("rewrite projects.yaml: %v", err)
	}

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", map[string]any{
		"name":               "reviewed-app",
		"description":        "Needs review",
		"image":              "repo/reviewed-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Standard",
		"environment":        "prod",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if createResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for direct create, got %d", createResponse.StatusCode)
	}

	changeResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/changes", map[string]any{
		"operation":          "CreateApplication",
		"name":               "reviewed-app",
		"description":        "Needs review",
		"image":              "repo/reviewed-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Standard",
		"environment":        "prod",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if changeResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 from create change, got %d", changeResponse.StatusCode)
	}

	var changeBody struct {
		ID string `json:"id"`
	}
	decodeBody(t, changeResponse, &changeBody)

	submitResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/changes/"+changeBody.ID+"/submit", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if submitResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from submit, got %d", submitResponse.StatusCode)
	}

	approveResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/changes/"+changeBody.ID+"/approve", nil, map[string]string{
		"X-AODS-User-Id":  "admin-1",
		"X-AODS-Username": "admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if approveResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from approve, got %d", approveResponse.StatusCode)
	}

	mergeResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/changes/"+changeBody.ID+"/merge", nil, map[string]string{
		"X-AODS-User-Id":  "admin-1",
		"X-AODS-Username": "admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if mergeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from merge, got %d", mergeResponse.StatusCode)
	}

	listResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/projects/project-a/applications", nil, map[string]string{
		"X-AODS-User-Id":  "admin-1",
		"X-AODS-Username": "admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from list, got %d", listResponse.StatusCode)
	}

	var listBody struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	decodeBody(t, listResponse, &listBody)
	if len(listBody.Items) != 1 || listBody.Items[0].ID != "project-a__reviewed-app" {
		t.Fatalf("expected reviewed app to be created, got %#v", listBody.Items)
	}
}

func TestViewerCannotSubmitOrMergeChange(t *testing.T) {
	env := newTestEnvironment(t)

	changeResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/changes", map[string]any{
		"operation":          "CreateApplication",
		"name":               "guarded-app",
		"description":        "Needs deploy role",
		"image":              "repo/guarded-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Standard",
		"environment":        "dev",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if changeResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 from create change, got %d", changeResponse.StatusCode)
	}

	var changeBody struct {
		ID string `json:"id"`
	}
	decodeBody(t, changeResponse, &changeBody)

	submitResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/changes/"+changeBody.ID+"/submit", nil, map[string]string{
		"X-AODS-User-Id":  "viewer-1",
		"X-AODS-Username": "viewer",
		"X-AODS-Groups":   "aods:project-a:view",
	})
	if submitResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 from submit, got %d", submitResponse.StatusCode)
	}

	mergeResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/changes/"+changeBody.ID+"/merge", nil, map[string]string{
		"X-AODS-User-Id":  "viewer-1",
		"X-AODS-Username": "viewer",
		"X-AODS-Groups":   "aods:project-a:view",
	})
	if mergeResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 from merge, got %d", mergeResponse.StatusCode)
	}
}

func TestRedeployCanSwitchEnvironment(t *testing.T) {
	env := newTestEnvironment(t)

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", map[string]any{
		"name":               "env-switch-app",
		"description":        "Environment switch",
		"image":              "repo/env-switch-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Standard",
		"environment":        "prod",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d", createResponse.StatusCode)
	}

	redeployResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/applications/project-a__env-switch-app/deployments", map[string]any{
		"imageTag":    "v2",
		"environment": "dev",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if redeployResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 from redeploy, got %d", redeployResponse.StatusCode)
	}

	var body struct {
		Environment string `json:"environment"`
	}
	decodeBody(t, redeployResponse, &body)
	if body.Environment != "dev" {
		t.Fatalf("expected redeploy environment dev, got %s", body.Environment)
	}

	metadataPath := filepath.Join(env.repoRoot, "apps", "project-a", "env-switch-app", ".aods", "metadata.yaml")
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if !strings.Contains(string(metadata), "defaultEnvironment: dev") {
		t.Fatal("expected default environment to switch to dev")
	}
}

func TestRollbackPolicyCanBeSavedAndRetrieved(t *testing.T) {
	env := newTestEnvironment(t)

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", map[string]any{
		"name":               "policy-app",
		"description":        "Rollback policy target",
		"image":              "repo/policy-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Canary",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d", createResponse.StatusCode)
	}

	saveResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/applications/project-a__policy-app/rollback-policies", map[string]any{
		"enabled":         true,
		"maxErrorRate":    1.5,
		"maxLatencyP95Ms": 1200,
		"minRequestRate":  10,
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if saveResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from save rollback policy, got %d", saveResponse.StatusCode)
	}

	getResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/applications/project-a__policy-app/rollback-policies", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if getResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from get rollback policy, got %d", getResponse.StatusCode)
	}

	var body struct {
		Enabled         bool     `json:"enabled"`
		MaxErrorRate    *float64 `json:"maxErrorRate"`
		MaxLatencyP95Ms *int     `json:"maxLatencyP95Ms"`
		MinRequestRate  *float64 `json:"minRequestRate"`
	}
	decodeBody(t, getResponse, &body)
	if !body.Enabled || body.MaxErrorRate == nil || *body.MaxErrorRate != 1.5 {
		t.Fatalf("expected saved maxErrorRate, got %#v", body)
	}
	if body.MaxLatencyP95Ms == nil || *body.MaxLatencyP95Ms != 1200 {
		t.Fatalf("expected saved maxLatencyP95Ms, got %#v", body)
	}
	if body.MinRequestRate == nil || *body.MinRequestRate != 10 {
		t.Fatalf("expected saved minRequestRate, got %#v", body)
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
    environments:
      - id: dev
        name: Development
        clusterId: default
        writeMode: direct
      - id: prod
        name: Production
        clusterId: default
        writeMode: direct
        default: true
    policies:
      minReplicas: 1
      allowedEnvironments:
        - dev
        - prod
      allowedDeploymentStrategies:
        - Standard
        - Canary
      allowedClusterTargets:
        - default
      prodPRRequired: false
      autoRollbackEnabled: false
      requiredProbes: true
`
	if err := os.WriteFile(filepath.Join(repoRoot, "platform", "projects.yaml"), []byte(projectsYAML), 0o644); err != nil {
		t.Fatalf("write projects.yaml: %v", err)
	}
	clustersYAML := `clusters:
  - id: default
    name: Default Cluster
    description: Test cluster
    default: true
`
	if err := os.WriteFile(filepath.Join(repoRoot, "platform", "clusters.yaml"), []byte(clustersYAML), 0o644); err != nil {
		t.Fatalf("write clusters.yaml: %v", err)
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
