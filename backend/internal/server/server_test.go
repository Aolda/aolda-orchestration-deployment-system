package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func TestPreviewRepositorySourceRouteIsRegistered(t *testing.T) {
	env := newTestEnvironment(t)

	response := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications/source-preview", map[string]any{}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "alice",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 from preview route, got %d", response.StatusCode)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeBody(t, response, &body)

	if body.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %s", body.Error.Code)
	}
	if body.Error.Message != "repositoryUrl must point to a GitHub repository" {
		t.Fatalf("expected repositoryUrl validation, got %q", body.Error.Message)
	}
}

func TestVerifyImageAccessRouteUsesProjectPermissions(t *testing.T) {
	env := newTestEnvironment(t)

	response := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications/image-access", map[string]any{
		"image": "ghcr.io/aolda/demo:v1",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "alice",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from image access route, got %d", response.StatusCode)
	}

	var body struct {
		Image      string `json:"image"`
		Registry   string `json:"registry"`
		Accessible bool   `json:"accessible"`
		Message    string `json:"message"`
	}
	decodeBody(t, response, &body)

	if body.Image != "ghcr.io/aolda/demo:v1" {
		t.Fatalf("expected image echo, got %q", body.Image)
	}
	if body.Registry != "ghcr.io" {
		t.Fatalf("expected ghcr.io registry, got %q", body.Registry)
	}
	if !body.Accessible {
		t.Fatal("expected accessible=true")
	}

	viewerResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications/image-access", map[string]any{
		"image": "ghcr.io/aolda/demo:v1",
	}, map[string]string{
		"X-AODS-User-Id":  "user-2",
		"X-AODS-Username": "viewer",
		"X-AODS-Groups":   "aods:project-a:view",
	})

	if viewerResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected viewer to get 403, got %d", viewerResponse.StatusCode)
	}
}

func TestProjectRepositoriesCanBeListed(t *testing.T) {
	env := newTestEnvironment(t)

	response := performJSONRequest(t, env, http.MethodGet, "/api/v1/projects/project-a/repositories", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "alice",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}

	var body struct {
		Items []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"items"`
	}
	decodeBody(t, response, &body)

	if len(body.Items) != 0 {
		t.Fatalf("expected no seeded repositories, got %#v", body.Items)
	}
}

func TestPlatformAdminCanBootstrapCluster(t *testing.T) {
	env := newTestEnvironment(t)

	payload := map[string]any{
		"id":          "edge",
		"name":        "Edge Cluster",
		"description": "Dedicated edge workloads",
		"default":     true,
	}

	response := performJSONRequest(t, env, http.MethodPost, "/api/v1/clusters", payload, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", response.StatusCode)
	}

	var body struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Default     bool   `json:"default"`
	}
	decodeBody(t, response, &body)

	if body.ID != "edge" {
		t.Fatalf("expected edge cluster, got %s", body.ID)
	}
	if !body.Default {
		t.Fatal("expected created cluster to become default")
	}

	clustersYAML, err := os.ReadFile(filepath.Join(env.repoRoot, "platform", "clusters.yaml"))
	if err != nil {
		t.Fatalf("read clusters.yaml: %v", err)
	}
	content := string(clustersYAML)
	if !strings.Contains(content, "id: edge") {
		t.Fatalf("expected clusters.yaml to contain edge cluster: %s", content)
	}
	if strings.Count(content, "default: true") != 1 {
		t.Fatalf("expected exactly one default cluster after bootstrap: %s", content)
	}

	assertFluxBootstrapFiles(t, env.repoRoot, "edge")
	rootContent, err := os.ReadFile(filepath.Join(env.repoRoot, "platform", "flux", "clusters", "edge", "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read cluster root kustomization: %v", err)
	}
	if !strings.Contains(string(rootContent), "resources: []") {
		t.Fatalf("expected new cluster root to start empty: %s", rootContent)
	}
}

func TestConfiguredPlatformAdminAuthorityWorksAcrossHandlers(t *testing.T) {
	env := newTestEnvironmentWithConfig(t, func(cfg *core.Config) {
		cfg.PlatformAdminAuthorities = []string{"aods:platform:admin"}
	})

	projectResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/projects", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if projectResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", projectResponse.StatusCode)
	}

	var projectBody struct {
		Items []struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		} `json:"items"`
	}
	decodeBody(t, projectResponse, &projectBody)
	if len(projectBody.Items) != 1 || projectBody.Items[0].Role != "admin" {
		t.Fatalf("expected configured platform admin to see project as admin, got %#v", projectBody.Items)
	}

	clusterResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/clusters", map[string]any{
		"id":   "edge",
		"name": "Edge Cluster",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if clusterResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", clusterResponse.StatusCode)
	}

	adminResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/admin/resource-overview", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if adminResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", adminResponse.StatusCode)
	}

	var adminBody struct {
		RuntimeConnected bool `json:"runtimeConnected"`
		ProjectCount     int  `json:"projectCount"`
		ServiceCount     int  `json:"serviceCount"`
	}
	decodeBody(t, adminResponse, &adminBody)
	if adminBody.RuntimeConnected {
		t.Fatal("expected runtimeConnected=false in local test environment")
	}
	if adminBody.ProjectCount != 1 {
		t.Fatalf("expected projectCount=1, got %d", adminBody.ProjectCount)
	}
	if adminBody.ServiceCount != 0 {
		t.Fatalf("expected serviceCount=0, got %d", adminBody.ServiceCount)
	}
}

func TestNonPlatformAdminCannotBootstrapCluster(t *testing.T) {
	env := newTestEnvironment(t)

	payload := map[string]any{
		"id":   "edge",
		"name": "Edge Cluster",
	}

	response := performJSONRequest(t, env, http.MethodPost, "/api/v1/clusters", payload, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})

	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.StatusCode)
	}
}

func TestNonPlatformAdminCannotGetAdminResourceOverview(t *testing.T) {
	env := newTestEnvironment(t)

	response := performJSONRequest(t, env, http.MethodGet, "/api/v1/admin/resource-overview", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})

	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.StatusCode)
	}
}

func TestBootstrapClusterConflictReturns409(t *testing.T) {
	env := newTestEnvironment(t)

	payload := map[string]any{
		"id":   "default",
		"name": "Duplicate Default",
	}

	response := performJSONRequest(t, env, http.MethodPost, "/api/v1/clusters", payload, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})

	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", response.StatusCode)
	}
}

func TestPlatformAdminCanBootstrapProject(t *testing.T) {
	env := newTestEnvironment(t)

	payload := map[string]any{
		"id":          "project-z",
		"name":        "project-z",
		"description": "Bootstrap target",
		"environments": []map[string]any{
			{
				"id":        "staging",
				"name":      "Staging",
				"clusterId": "analytics",
				"writeMode": "direct",
				"default":   true,
			},
		},
		"repositories": []map[string]any{
			{
				"id":   "project-z-api",
				"name": "Project Z API",
				"url":  "https://github.com/aolda-demo/project-z-api",
			},
		},
	}

	response := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects", payload, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", response.StatusCode)
	}

	var body struct {
		ID        string `json:"id"`
		Namespace string `json:"namespace"`
		Role      string `json:"role"`
	}
	decodeBody(t, response, &body)

	if body.ID != "project-z" {
		t.Fatalf("expected project-z, got %s", body.ID)
	}
	if body.Namespace != "project-z" {
		t.Fatalf("expected namespace project-z, got %s", body.Namespace)
	}
	if body.Role != "admin" {
		t.Fatalf("expected role admin, got %s", body.Role)
	}

	projectsYAML, err := os.ReadFile(filepath.Join(env.repoRoot, "platform", "projects.yaml"))
	if err != nil {
		t.Fatalf("read projects.yaml: %v", err)
	}
	content := string(projectsYAML)
	for _, needle := range []string{
		"id: project-z",
		"namespace: project-z",
		"- aods:project-z:view",
		"- aods:project-z:deploy",
		"- aods:project-z:admin",
		"- aods:platform:admin",
		"clusterId: analytics",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected projects.yaml to contain %q, got:\n%s", needle, content)
		}
	}

	assertFluxBootstrapFiles(t, env.repoRoot, "analytics")
}

func TestNonPlatformAdminCannotBootstrapProject(t *testing.T) {
	env := newTestEnvironment(t)

	response := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects", map[string]any{
		"id":   "project-z",
		"name": "project-z",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})

	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.StatusCode)
	}
}

func TestBootstrapProjectRejectsDisplayStyleName(t *testing.T) {
	env := newTestEnvironment(t)

	response := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects", map[string]any{
		"id":   "project-z",
		"name": "Project Z",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.StatusCode)
	}

	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	decodeBody(t, response, &body)

	if body.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %#v", body.Error)
	}
	if body.Error.Message != "project name must be a lowercase slug" {
		t.Fatalf("unexpected message: %#v", body.Error)
	}
}

func TestBootstrapProjectConflictReturns409(t *testing.T) {
	env := newTestEnvironment(t)

	response := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects", map[string]any{
		"id":   "project-a",
		"name": "project-a",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})

	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", response.StatusCode)
	}
}

func TestPlatformAdminCanDeleteProject(t *testing.T) {
	env := newTestEnvironment(t)

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", map[string]any{
		"name":               "delete-project-app",
		"image":              "repo/delete-project-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Rollout",
		"secrets": []map[string]string{
			{"key": "DATABASE_URL", "value": "postgres://project-delete"},
		},
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResponse.StatusCode)
	}

	deleteResponse := performJSONRequest(t, env, http.MethodDelete, "/api/v1/projects/project-a", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if deleteResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", deleteResponse.StatusCode)
	}

	var body struct {
		ProjectID string `json:"projectId"`
		Status    string `json:"status"`
	}
	decodeBody(t, deleteResponse, &body)
	if body.ProjectID != "project-a" || body.Status != "deleted" {
		t.Fatalf("unexpected delete response: %#v", body)
	}

	projectsYAML, err := os.ReadFile(filepath.Join(env.repoRoot, "platform", "projects.yaml"))
	if err != nil {
		t.Fatalf("read projects.yaml: %v", err)
	}
	if strings.Contains(string(projectsYAML), "id: project-a") {
		t.Fatalf("expected project-a to be removed from catalog, got:\n%s", projectsYAML)
	}

	if _, err := os.Stat(filepath.Join(env.repoRoot, "apps", "project-a")); !os.IsNotExist(err) {
		t.Fatalf("expected project application directory to be removed, got %v", err)
	}
	assertNoFluxChildManifest(t, env.repoRoot, "default", "project-a-delete-project-app")

	if _, err := os.Stat(filepath.Join(env.vaultRoot, "aods", "apps", "project-a", "delete-project-app", "prod.json")); !os.IsNotExist(err) {
		t.Fatalf("expected project vault secrets to be removed, got %v", err)
	}
}

func TestSharedNamespaceProjectCannotBeDeleted(t *testing.T) {
	env := newTestEnvironment(t)

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects", map[string]any{
		"id":        "shared",
		"name":      "shared",
		"namespace": "shared",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResponse.StatusCode)
	}

	deleteResponse := performJSONRequest(t, env, http.MethodDelete, "/api/v1/projects/shared", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if deleteResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", deleteResponse.StatusCode)
	}

	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	decodeBody(t, deleteResponse, &body)

	if body.Error.Code != "PROJECT_DELETE_PROTECTED" {
		t.Fatalf("expected PROJECT_DELETE_PROTECTED, got %s", body.Error.Code)
	}
	if body.Error.Details["namespace"] != "shared" {
		t.Fatalf("expected shared namespace detail, got %#v", body.Error.Details)
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
		"servicemonitor.yaml",
		"prometheusrule.yaml",
		"externalsecret.yaml",
	}

	for _, fileName := range requiredFiles {
		if _, err := os.Stat(filepath.Join(appDir, fileName)); err != nil {
			t.Fatalf("expected %s to exist: %v", fileName, err)
		}
	}
	assertFluxBootstrapFiles(t, env.repoRoot, "default")
	assertFluxChildManifestPath(t, env.repoRoot, "default", "project-a-my-app", "./apps/project-a/my-app/overlays/prod", true)

	repoFiles := []string{
		filepath.Join(appDir, "deployment.yaml"),
		filepath.Join(appDir, "service.yaml"),
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

	serviceMonitorManifest, err := os.ReadFile(filepath.Join(appDir, "servicemonitor.yaml"))
	if err != nil {
		t.Fatalf("read servicemonitor manifest: %v", err)
	}
	if !strings.Contains(string(serviceMonitorManifest), "kind: ServiceMonitor") {
		t.Fatal("expected ServiceMonitor manifest to be generated")
	}
	if !strings.Contains(string(serviceMonitorManifest), "prometheus: argo-cd-grafana") {
		t.Fatal("expected ServiceMonitor manifest to include Prometheus selector labels")
	}
	if !strings.Contains(string(serviceMonitorManifest), `aods.io/metrics-scrape: "true"`) {
		t.Fatal("expected ServiceMonitor manifest to target the stable service")
	}
	prometheusRuleManifest, err := os.ReadFile(filepath.Join(appDir, "prometheusrule.yaml"))
	if err != nil {
		t.Fatalf("read prometheusrule manifest: %v", err)
	}
	if !strings.Contains(string(prometheusRuleManifest), "kind: PrometheusRule") {
		t.Fatal("expected PrometheusRule manifest to be generated")
	}
	if !strings.Contains(string(prometheusRuleManifest), "AODSApplicationHighErrorRate") {
		t.Fatal("expected PrometheusRule manifest to include high error rate alert")
	}
	if !strings.Contains(string(prometheusRuleManifest), "AODSApplicationFluxDegraded") {
		t.Fatal("expected PrometheusRule manifest to include Flux degraded alert")
	}
	for _, fileName := range []string{"virtualservice.yaml", "destinationrule.yaml"} {
		if _, err := os.Stat(filepath.Join(appDir, fileName)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be skipped for non-mesh rollout apps, got %v", fileName, err)
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
	if syncBody.Status != "Unknown" {
		t.Fatalf("expected Unknown status, got %s", syncBody.Status)
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
	if len(metricsBody.Metrics) != 0 {
		t.Fatalf("expected no metric series without metrics integration, got %d", len(metricsBody.Metrics))
	}

	diagnosticsResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/applications/project-a__my-app/metrics/diagnostics", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if diagnosticsResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from metrics diagnostics, got %d", diagnosticsResponse.StatusCode)
	}
	var diagnosticsBody struct {
		ApplicationID string `json:"applicationId"`
		Status        string `json:"status"`
		ScrapeTargets []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"scrapeTargets"`
	}
	decodeBody(t, diagnosticsResponse, &diagnosticsBody)
	if diagnosticsBody.ApplicationID != "project-a__my-app" {
		t.Fatalf("expected diagnostics for project-a__my-app, got %s", diagnosticsBody.ApplicationID)
	}
	if diagnosticsBody.Status != "Unavailable" {
		t.Fatalf("expected Unavailable diagnostics without metrics integration, got %s", diagnosticsBody.Status)
	}
	if len(diagnosticsBody.ScrapeTargets) != 1 || diagnosticsBody.ScrapeTargets[0].Path != "/metrics" {
		t.Fatalf("expected application scrape target, got %#v", diagnosticsBody.ScrapeTargets)
	}

	healthResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/projects/project-a/health", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if healthResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from project health, got %d", healthResponse.StatusCode)
	}
	var healthBody struct {
		ProjectID string `json:"projectId"`
		Items     []struct {
			ApplicationID string `json:"applicationId"`
			Status        string `json:"status"`
			Metrics       []struct {
				Key string `json:"key"`
			} `json:"metrics"`
			LatestDeployment *struct {
				ImageTag string `json:"imageTag"`
				Status   string `json:"status"`
			} `json:"latestDeployment"`
			Signals []struct {
				Key    string `json:"key"`
				Status string `json:"status"`
			} `json:"signals"`
		} `json:"items"`
	}
	decodeBody(t, healthResponse, &healthBody)
	if healthBody.ProjectID != "project-a" {
		t.Fatalf("expected project-a health, got %s", healthBody.ProjectID)
	}
	if len(healthBody.Items) != 1 {
		t.Fatalf("expected one health item, got %d", len(healthBody.Items))
	}
	if healthBody.Items[0].ApplicationID != "project-a__my-app" || healthBody.Items[0].Status != "Warning" {
		t.Fatalf("unexpected health item: %#v", healthBody.Items[0])
	}
	if len(healthBody.Items[0].Metrics) != 0 {
		t.Fatalf("expected no project health metrics without metrics integration, got %d", len(healthBody.Items[0].Metrics))
	}
	if healthBody.Items[0].LatestDeployment == nil || healthBody.Items[0].LatestDeployment.ImageTag != "v2" {
		t.Fatalf("expected latest deployment summary from project health, got %#v", healthBody.Items[0].LatestDeployment)
	}
}

func TestNetworkExposureRouteReturnsPendingWhenLoadBalancerEnabledWithoutKubernetesAPI(t *testing.T) {
	env := newTestEnvironment(t)

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", map[string]any{
		"name":                "lb-app",
		"description":         "LB requested app",
		"image":               "repo/lb-app:v1",
		"servicePort":         8080,
		"deploymentStrategy":  "Standard",
		"loadBalancerEnabled": true,
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResponse.StatusCode)
	}

	exposureResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/applications/project-a__lb-app/network-exposure", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if exposureResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from network-exposure, got %d", exposureResponse.StatusCode)
	}

	var body struct {
		Enabled     bool     `json:"enabled"`
		Status      string   `json:"status"`
		ServiceType string   `json:"serviceType"`
		Addresses   []string `json:"addresses"`
		Message     string   `json:"message"`
	}
	decodeBody(t, exposureResponse, &body)

	if !body.Enabled {
		t.Fatal("expected load balancer exposure to be enabled")
	}
	if body.Status != "Pending" {
		t.Fatalf("expected Pending status, got %s", body.Status)
	}
	if body.ServiceType != "LoadBalancer" {
		t.Fatalf("expected LoadBalancer service type, got %q", body.ServiceType)
	}
	if len(body.Addresses) != 0 {
		t.Fatalf("expected no ingress addresses in local mode, got %#v", body.Addresses)
	}
	if !strings.Contains(body.Message, "Kubernetes API 연동이 설정되지 않아") {
		t.Fatalf("expected local-mode explanation, got %q", body.Message)
	}
}

func TestArchiveApplicationRemovesDesiredStateButKeepsHistory(t *testing.T) {
	env := newTestEnvironment(t)

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", map[string]any{
		"name":               "archive-app",
		"image":              "repo/archive-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Rollout",
		"secrets": []map[string]string{
			{"key": "DATABASE_URL", "value": "postgres://archive"},
		},
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResponse.StatusCode)
	}

	archiveResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/applications/project-a__archive-app/archive", nil, map[string]string{
		"X-AODS-User-Id":  "user-2",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if archiveResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", archiveResponse.StatusCode)
	}

	var body struct {
		ApplicationID string `json:"applicationId"`
		Status        string `json:"status"`
	}
	decodeBody(t, archiveResponse, &body)
	if body.ApplicationID != "project-a__archive-app" || body.Status != "archived" {
		t.Fatalf("unexpected archive response: %#v", body)
	}

	appDir := filepath.Join(env.repoRoot, "apps", "project-a", "archive-app")
	if _, err := os.Stat(filepath.Join(appDir, "base")); !os.IsNotExist(err) {
		t.Fatalf("expected base directory to be removed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "overlays")); !os.IsNotExist(err) {
		t.Fatalf("expected overlays directory to be removed, got %v", err)
	}

	metadataContent, err := os.ReadFile(filepath.Join(appDir, ".aods", "metadata.yaml"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if !strings.Contains(string(metadataContent), "archivedAt:") {
		t.Fatalf("expected archivedAt in metadata, got:\n%s", metadataContent)
	}
	if !strings.Contains(string(metadataContent), "archivedBy: platform-admin") {
		t.Fatalf("expected archivedBy marker, got:\n%s", metadataContent)
	}

	assertNoFluxChildManifest(t, env.repoRoot, "default", "project-a-archive-app")

	if _, err := os.Stat(filepath.Join(env.vaultRoot, "aods", "apps", "project-a", "archive-app", "prod.json")); err != nil {
		t.Fatalf("expected final vault secret to remain after archive: %v", err)
	}

	listResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/projects/project-a/applications", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", listResponse.StatusCode)
	}

	var listBody struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	decodeBody(t, listResponse, &listBody)
	for _, item := range listBody.Items {
		if item.ID == "project-a__archive-app" {
			t.Fatalf("expected archived app to be hidden from list response")
		}
	}
}

func TestDeleteApplicationRemovesDirectoryAndSecret(t *testing.T) {
	env := newTestEnvironment(t)

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", map[string]any{
		"name":               "delete-app",
		"image":              "repo/delete-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Rollout",
		"secrets": []map[string]string{
			{"key": "DATABASE_URL", "value": "postgres://delete"},
		},
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResponse.StatusCode)
	}

	deleteResponse := performJSONRequest(t, env, http.MethodDelete, "/api/v1/applications/project-a__delete-app", nil, map[string]string{
		"X-AODS-User-Id":  "user-2",
		"X-AODS-Username": "platform-admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if deleteResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", deleteResponse.StatusCode)
	}

	var body struct {
		ApplicationID string `json:"applicationId"`
		Status        string `json:"status"`
	}
	decodeBody(t, deleteResponse, &body)
	if body.ApplicationID != "project-a__delete-app" || body.Status != "deleted" {
		t.Fatalf("unexpected delete response: %#v", body)
	}

	if _, err := os.Stat(filepath.Join(env.repoRoot, "apps", "project-a", "delete-app")); !os.IsNotExist(err) {
		t.Fatalf("expected application directory to be removed, got %v", err)
	}
	assertNoFluxChildManifest(t, env.repoRoot, "default", "project-a-delete-app")

	if _, err := os.Stat(filepath.Join(env.vaultRoot, "aods", "apps", "project-a", "delete-app", "prod.json")); !os.IsNotExist(err) {
		t.Fatalf("expected final vault secret to be removed, got %v", err)
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
	if strings.Contains(string(deploymentManifest), `sidecar.istio.io/inject: "true"`) {
		t.Fatal("expected deployment manifest to omit Istio sidecar injection for default rollout apps")
	}
	if !strings.Contains(string(deploymentManifest), "app: stateless-app") {
		t.Fatal("expected deployment manifest to include Istio telemetry app label")
	}
	serviceManifest, err := os.ReadFile(filepath.Join(appDir, "service.yaml"))
	if err != nil {
		t.Fatalf("read service manifest: %v", err)
	}
	if strings.Contains(string(serviceManifest), "name: envoy-metrics") {
		t.Fatal("expected service manifest to omit envoy metrics port when mesh is disabled")
	}
	serviceMonitorManifest, err := os.ReadFile(filepath.Join(appDir, "servicemonitor.yaml"))
	if err != nil {
		t.Fatalf("read servicemonitor manifest: %v", err)
	}
	if strings.Contains(string(serviceMonitorManifest), "port: envoy-metrics") || strings.Contains(string(serviceMonitorManifest), "path: /stats/prometheus") {
		t.Fatal("expected ServiceMonitor to omit envoy metrics endpoint when mesh is disabled")
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

func TestCreateApplicationWithRegistryCredentialWritesRegistryArtifacts(t *testing.T) {
	env := newTestEnvironment(t)

	createPayload := map[string]any{
		"name":               "private-ghcr-app",
		"description":        "Private registry deployment",
		"image":              "ghcr.io/aolda/private-ghcr-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Standard",
		"registryUsername":   "octocat",
		"registryToken":      "ghcr_pat_456",
	}

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", createPayload, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResponse.StatusCode)
	}

	appDir := filepath.Join(env.repoRoot, "apps", "project-a", "private-ghcr-app", "base")
	if _, err := os.Stat(filepath.Join(appDir, "registry-externalsecret.yaml")); err != nil {
		t.Fatalf("expected registry-externalsecret.yaml to exist: %v", err)
	}

	deploymentManifest, err := os.ReadFile(filepath.Join(appDir, "deployment.yaml"))
	if err != nil {
		t.Fatalf("read deployment manifest: %v", err)
	}
	if !strings.Contains(string(deploymentManifest), "imagePullSecrets:") {
		t.Fatal("expected deployment manifest to include imagePullSecrets")
	}
	if !strings.Contains(string(deploymentManifest), "name: private-ghcr-app-registry") {
		t.Fatal("expected deployment manifest to reference the registry secret")
	}

	registryExternalSecret, err := os.ReadFile(filepath.Join(appDir, "registry-externalsecret.yaml"))
	if err != nil {
		t.Fatalf("read registry externalsecret manifest: %v", err)
	}
	if !strings.Contains(string(registryExternalSecret), "type: kubernetes.io/dockerconfigjson") {
		t.Fatal("expected registry ExternalSecret to materialize a dockerconfigjson secret")
	}
	if strings.Contains(string(registryExternalSecret), "ghcr_pat_456") {
		t.Fatal("expected registry token not to leak into manifests")
	}

	kustomizationManifest, err := os.ReadFile(filepath.Join(appDir, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read kustomization manifest: %v", err)
	}
	if !strings.Contains(string(kustomizationManifest), "registry-externalsecret.yaml") {
		t.Fatal("expected kustomization to include registry-externalsecret.yaml")
	}

	registrySecretFile := filepath.Join(env.vaultRoot, "aods", "apps", "project-a", "private-ghcr-app", "registry.json")
	registrySecretData, err := os.ReadFile(registrySecretFile)
	if err != nil {
		t.Fatalf("read registry vault file: %v", err)
	}
	if !strings.Contains(string(registrySecretData), "\"dockerconfigjson\"") {
		t.Fatal("expected registry vault file to store dockerconfigjson payload")
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
		"servicemonitor.yaml",
		"prometheusrule.yaml",
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
	assertFluxBootstrapFiles(t, env.repoRoot, "default")
	assertFluxChildManifestPath(t, env.repoRoot, "default", "project-a-canary-app", "./apps/project-a/canary-app/overlays/dev", false)
	rolloutManifest, err := os.ReadFile(filepath.Join(appDir, "rollout.yaml"))
	if err != nil {
		t.Fatalf("read rollout manifest: %v", err)
	}
	if !strings.Contains(string(rolloutManifest), `sidecar.istio.io/inject: "true"`) {
		t.Fatal("expected rollout manifest to opt workloads into Istio sidecar injection")
	}
	if !strings.Contains(string(rolloutManifest), "labels:\n        sidecar.istio.io/inject: \"true\"") {
		t.Fatal("expected rollout manifest to label pods for Istio sidecar injection")
	}
	if !strings.Contains(string(rolloutManifest), "app: canary-app") {
		t.Fatal("expected rollout manifest to include Istio telemetry app label")
	}
	serviceManifest, err := os.ReadFile(filepath.Join(appDir, "service.yaml"))
	if err != nil {
		t.Fatalf("read service manifest: %v", err)
	}
	if !strings.Contains(string(serviceManifest), "name: envoy-metrics") {
		t.Fatal("expected canary service manifest to expose envoy metrics port")
	}
	serviceMonitorManifest, err := os.ReadFile(filepath.Join(appDir, "servicemonitor.yaml"))
	if err != nil {
		t.Fatalf("read servicemonitor manifest: %v", err)
	}
	if !strings.Contains(string(serviceMonitorManifest), "port: envoy-metrics") || !strings.Contains(string(serviceMonitorManifest), "path: /stats/prometheus") {
		t.Fatal("expected canary ServiceMonitor to scrape Istio sidecar envoy metrics")
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

func TestInvalidChangeOperationReturnsBadRequest(t *testing.T) {
	env := newTestEnvironment(t)

	changeResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/changes", map[string]any{
		"operation":     "DestroyEverything",
		"applicationId": "project-a__guarded-app",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if changeResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 from invalid change operation, got %d", changeResponse.StatusCode)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeBody(t, changeResponse, &body)
	if body.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %s", body.Error.Code)
	}
}

func TestDeployerCannotApproveChange(t *testing.T) {
	env := newTestEnvironment(t)

	changeResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/changes", map[string]any{
		"operation":          "CreateApplication",
		"name":               "approval-guard-app",
		"description":        "Needs admin approval",
		"image":              "repo/approval-guard-app:v1",
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

	approveResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/changes/"+changeBody.ID+"/approve", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if approveResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 from approve, got %d", approveResponse.StatusCode)
	}
}

func TestApproveRequiresSubmittedChange(t *testing.T) {
	env := newTestEnvironment(t)

	changeResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/changes", map[string]any{
		"operation":          "CreateApplication",
		"name":               "approval-state-app",
		"description":        "Must be submitted before approval",
		"image":              "repo/approval-state-app:v1",
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

	approveResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/changes/"+changeBody.ID+"/approve", nil, map[string]string{
		"X-AODS-User-Id":  "admin-1",
		"X-AODS-Username": "admin",
		"X-AODS-Groups":   "aods:platform:admin",
	})
	if approveResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 from approve before submit, got %d", approveResponse.StatusCode)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeBody(t, approveResponse, &body)
	if body.Error.Code != "CHANGE_SUBMISSION_REQUIRED" {
		t.Fatalf("expected CHANGE_SUBMISSION_REQUIRED, got %s", body.Error.Code)
	}
}

func TestMergeRequiresApprovalForPRManagedChange(t *testing.T) {
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

	changeResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/changes", map[string]any{
		"operation":          "CreateApplication",
		"name":               "approval-required-app",
		"description":        "Must be approved before merge",
		"image":              "repo/approval-required-app:v1",
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

	mergeResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/changes/"+changeBody.ID+"/merge", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if mergeResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 from merge without approval, got %d", mergeResponse.StatusCode)
	}

	var mergeBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeBody(t, mergeResponse, &mergeBody)
	if mergeBody.Error.Code != "CHANGE_APPROVAL_REQUIRED" {
		t.Fatalf("expected CHANGE_APPROVAL_REQUIRED, got %s", mergeBody.Error.Code)
	}
}

func TestDirectChangeMergeRequiresSubmit(t *testing.T) {
	env := newTestEnvironment(t)

	changeResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/changes", map[string]any{
		"operation":          "CreateApplication",
		"name":               "direct-merge-state-app",
		"description":        "Must be submitted before merge",
		"image":              "repo/direct-merge-state-app:v1",
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

	mergeResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/changes/"+changeBody.ID+"/merge", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if mergeResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 from merge before submit, got %d", mergeResponse.StatusCode)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeBody(t, mergeResponse, &body)
	if body.Error.Code != "CHANGE_SUBMISSION_REQUIRED" {
		t.Fatalf("expected CHANGE_SUBMISSION_REQUIRED, got %s", body.Error.Code)
	}
}

func TestSubmittedDirectChangeCanMerge(t *testing.T) {
	env := newTestEnvironment(t)

	changeResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/changes", map[string]any{
		"operation":          "CreateApplication",
		"name":               "direct-merge-success-app",
		"description":        "Direct change should merge after submit",
		"image":              "repo/direct-merge-success-app:v1",
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
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if submitResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from submit, got %d", submitResponse.StatusCode)
	}

	mergeResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/changes/"+changeBody.ID+"/merge", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if mergeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from merge after submit, got %d", mergeResponse.StatusCode)
	}

	listResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/projects/project-a/applications", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from list applications, got %d", listResponse.StatusCode)
	}

	var listBody struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	decodeBody(t, listResponse, &listBody)
	if len(listBody.Items) != 1 || listBody.Items[0].ID != "project-a__direct-merge-success-app" {
		t.Fatalf("expected direct change merge to create application, got %#v", listBody.Items)
	}
}

func TestRedeployCanSwitchEnvironment(t *testing.T) {
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
      - id: dev
        name: Development
        clusterId: analytics
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
        - analytics
      prodPRRequired: false
      autoRollbackEnabled: false
      requiredProbes: true
`
	if err := os.WriteFile(filepath.Join(env.repoRoot, "platform", "projects.yaml"), []byte(projectsYAML), 0o644); err != nil {
		t.Fatalf("rewrite projects.yaml: %v", err)
	}

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
	assertFluxChildManifestPath(t, env.repoRoot, "default", "project-a-env-switch-app", "./apps/project-a/env-switch-app/overlays/prod", true)

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
	assertNoFluxChildManifest(t, env.repoRoot, "default", "project-a-env-switch-app")
	assertFluxBootstrapFiles(t, env.repoRoot, "analytics")
	assertFluxChildManifestPath(t, env.repoRoot, "analytics", "project-a-env-switch-app", "./apps/project-a/env-switch-app/overlays/dev", true)
}

func TestCreateApplicationRejectsDisallowedEnvironment(t *testing.T) {
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
        - prod
      allowedDeploymentStrategies:
        - Standard
      allowedClusterTargets:
        - default
      prodPRRequired: false
      autoRollbackEnabled: false
      requiredProbes: true
`
	if err := os.WriteFile(filepath.Join(env.repoRoot, "platform", "projects.yaml"), []byte(projectsYAML), 0o644); err != nil {
		t.Fatalf("rewrite projects.yaml: %v", err)
	}

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", map[string]any{
		"name":               "forbidden-env-app",
		"description":        "Environment guardrail",
		"image":              "repo/forbidden-env-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Standard",
		"environment":        "dev",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if createResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 from disallowed environment, got %d", createResponse.StatusCode)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeBody(t, createResponse, &body)
	if body.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %s", body.Error.Code)
	}
}

func TestCreateApplicationRejectsDisallowedDeploymentStrategy(t *testing.T) {
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
        writeMode: direct
        default: true
    policies:
      minReplicas: 1
      allowedEnvironments:
        - prod
      allowedDeploymentStrategies:
        - Standard
      allowedClusterTargets:
        - default
      prodPRRequired: false
      autoRollbackEnabled: false
      requiredProbes: true
`
	if err := os.WriteFile(filepath.Join(env.repoRoot, "platform", "projects.yaml"), []byte(projectsYAML), 0o644); err != nil {
		t.Fatalf("rewrite projects.yaml: %v", err)
	}

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", map[string]any{
		"name":               "forbidden-strategy-app",
		"description":        "Strategy guardrail",
		"image":              "repo/forbidden-strategy-app:v1",
		"servicePort":        8080,
		"deploymentStrategy": "Canary",
		"environment":        "prod",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if createResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 from disallowed strategy, got %d", createResponse.StatusCode)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeBody(t, createResponse, &body)
	if body.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %s", body.Error.Code)
	}
}

func TestProjectPoliciesCanDisableRequiredProbes(t *testing.T) {
	env := newTestEnvironment(t)

	response := performJSONRequest(t, env, http.MethodPatch, "/api/v1/projects/project-a/policies", map[string]any{
		"minReplicas":                 2,
		"allowedEnvironments":         []string{"dev", "prod"},
		"allowedDeploymentStrategies": []string{"Standard", "Canary"},
		"allowedClusterTargets":       []string{"default"},
		"prodPRRequired":              false,
		"autoRollbackEnabled":         false,
		"requiredProbes":              false,
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
		RequiredProbes bool `json:"requiredProbes"`
	}
	decodeBody(t, getResponse, &body)
	if body.RequiredProbes {
		t.Fatal("expected requiredProbes=false to be preserved")
	}

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", map[string]any{
		"name":               "probe-optional-app",
		"description":        "Disabled probes",
		"image":              "repo/probe-optional-app:v1",
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

	deploymentManifest, err := os.ReadFile(filepath.Join(env.repoRoot, "apps", "project-a", "probe-optional-app", "base", "deployment.yaml"))
	if err != nil {
		t.Fatalf("read deployment manifest: %v", err)
	}
	if strings.Contains(string(deploymentManifest), "readinessProbe:") || strings.Contains(string(deploymentManifest), "livenessProbe:") {
		t.Fatal("expected deployment manifest to omit probes when requiredProbes=false")
	}

	metadata, err := os.ReadFile(filepath.Join(env.repoRoot, "apps", "project-a", "probe-optional-app", ".aods", "metadata.yaml"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if !strings.Contains(string(metadata), "requiredProbes: false") {
		t.Fatal("expected metadata to preserve requiredProbes=false")
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
	return newTestEnvironmentWithConfig(t, nil)
}

func newTestEnvironmentWithConfig(t *testing.T, mutate func(*core.Config)) testEnvironment {
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
  - id: analytics
    name: Analytics Cluster
    description: Analytics test cluster
    default: false
`
	if err := os.WriteFile(filepath.Join(repoRoot, "platform", "clusters.yaml"), []byte(clustersYAML), 0o644); err != nil {
		t.Fatalf("write clusters.yaml: %v", err)
	}

	cfg := core.Config{
		RepoRoot:         repoRoot,
		AllowedOrigin:    "*",
		AllowDevFallback: false,
		LocalVaultDir:    vaultRoot,
	}
	if mutate != nil {
		mutate(&cfg)
	}

	handler, _, _ := server.New(cfg)

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

func assertFluxBootstrapFiles(t *testing.T, repoRoot string, clusterID string) {
	t.Helper()

	rootPath := filepath.Join(repoRoot, "platform", "flux", "bootstrap", clusterID, "root-kustomization.yaml")
	rootContent, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatalf("read flux bootstrap root manifest: %v", err)
	}
	if !strings.Contains(string(rootContent), "kind: Kustomization") {
		t.Fatalf("expected bootstrap root manifest to define Flux Kustomization: %s", rootPath)
	}
	if !strings.Contains(string(rootContent), "name: aods-root-"+clusterID) {
		t.Fatalf("expected bootstrap root manifest to target cluster %s", clusterID)
	}
	if !strings.Contains(string(rootContent), "path: ./platform/flux/clusters/"+clusterID) {
		t.Fatalf("expected bootstrap root manifest to reference cluster path for %s", clusterID)
	}
	if !strings.Contains(string(rootContent), "wait: false") {
		t.Fatalf("expected bootstrap root manifest to disable wait for cluster %s", clusterID)
	}

	kustomizationPath := filepath.Join(repoRoot, "platform", "flux", "bootstrap", clusterID, "kustomization.yaml")
	kustomizationContent, err := os.ReadFile(kustomizationPath)
	if err != nil {
		t.Fatalf("read flux bootstrap kustomization: %v", err)
	}
	if !strings.Contains(string(kustomizationContent), "root-kustomization.yaml") {
		t.Fatalf("expected bootstrap kustomization to include root manifest for %s", clusterID)
	}
}

func assertFluxChildManifestPath(t *testing.T, repoRoot string, clusterID string, fileBase string, overlayPath string, wait bool) {
	t.Helper()

	childPath := filepath.Join(repoRoot, "platform", "flux", "clusters", clusterID, "applications", fileBase+".yaml")
	childContent, err := os.ReadFile(childPath)
	if err != nil {
		t.Fatalf("read flux child manifest: %v", err)
	}
	if !strings.Contains(string(childContent), "kind: Kustomization") {
		t.Fatalf("expected flux child manifest to define Flux Kustomization: %s", childPath)
	}
	if !strings.Contains(string(childContent), "path: "+overlayPath) {
		t.Fatalf("expected flux child manifest to reference %s, got:\n%s", overlayPath, childContent)
	}
	if !strings.Contains(string(childContent), "name: aods-manifest") {
		t.Fatalf("expected flux child manifest to reference default GitRepository source: %s", childPath)
	}
	if !strings.Contains(string(childContent), fmt.Sprintf("wait: %t", wait)) {
		t.Fatalf("expected flux child manifest to set wait=%t, got:\n%s", wait, childContent)
	}

	rootPath := filepath.Join(repoRoot, "platform", "flux", "clusters", clusterID, "kustomization.yaml")
	rootContent, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatalf("read flux root kustomization: %v", err)
	}
	resource := "applications/" + fileBase + ".yaml"
	if !strings.Contains(string(rootContent), resource) {
		t.Fatalf("expected flux root kustomization to include %s", resource)
	}
}

func assertNoFluxChildManifest(t *testing.T, repoRoot string, clusterID string, fileBase string) {
	t.Helper()

	childPath := filepath.Join(repoRoot, "platform", "flux", "clusters", clusterID, "applications", fileBase+".yaml")
	if _, err := os.Stat(childPath); !os.IsNotExist(err) {
		t.Fatalf("expected no flux child manifest at %s, got %v", childPath, err)
	}
}
