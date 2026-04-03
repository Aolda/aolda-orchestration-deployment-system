package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aolda/aods-backend/internal/core"
)

func TestCreateApplicationRejectsMissingImageFromRegistryCheck(t *testing.T) {
	registry := newImageRegistryServer(t)
	env := newTestEnvironmentWithConfig(t, func(cfg *core.Config) {
		cfg.ImageVerificationMode = "anonymous"
		cfg.ImageVerificationTimeout = 2 * time.Second
	})

	response := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", map[string]any{
		"name":               "missing-image-app",
		"image":              imageRef(registry, "missing", "v1"),
		"servicePort":        8080,
		"deploymentStrategy": "Standard",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.StatusCode)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeBody(t, response, &body)
	if body.Error.Code != "IMAGE_NOT_FOUND" {
		t.Fatalf("expected IMAGE_NOT_FOUND, got %s", body.Error.Code)
	}
}

func TestCreateChangeRejectsImageWhenRegistryAuthIsRequired(t *testing.T) {
	registry := newImageRegistryServer(t)
	env := newTestEnvironmentWithConfig(t, func(cfg *core.Config) {
		cfg.ImageVerificationMode = "anonymous"
		cfg.ImageVerificationTimeout = 2 * time.Second
	})

	response := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/changes", map[string]any{
		"operation":          "CreateApplication",
		"name":               "private-image-app",
		"image":              imageRef(registry, "private-app", "v1"),
		"servicePort":        8080,
		"deploymentStrategy": "Standard",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.StatusCode)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeBody(t, response, &body)
	if body.Error.Code != "IMAGE_AUTH_REQUIRED" {
		t.Fatalf("expected IMAGE_AUTH_REQUIRED, got %s", body.Error.Code)
	}
}

func TestRedeployRecordsPreflightFailureEventWhenRegistryAuthIsRequired(t *testing.T) {
	registry := newImageRegistryServer(t)
	env := newTestEnvironmentWithConfig(t, func(cfg *core.Config) {
		cfg.ImageVerificationMode = "anonymous"
		cfg.ImageVerificationTimeout = 2 * time.Second
	})

	createResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/projects/project-a/applications", map[string]any{
		"name":               "preflight-app",
		"image":              imageRef(registry, "token-ok", "v1"),
		"servicePort":        8080,
		"deploymentStrategy": "Standard",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResponse.StatusCode)
	}

	redeployResponse := performJSONRequest(t, env, http.MethodPost, "/api/v1/applications/project-a__preflight-app/deployments", map[string]any{
		"imageTag": "private",
	}, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if redeployResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", redeployResponse.StatusCode)
	}

	var redeployBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeBody(t, redeployResponse, &redeployBody)
	if redeployBody.Error.Code != "IMAGE_AUTH_REQUIRED" {
		t.Fatalf("expected IMAGE_AUTH_REQUIRED, got %s", redeployBody.Error.Code)
	}

	eventsResponse := performJSONRequest(t, env, http.MethodGet, "/api/v1/applications/project-a__preflight-app/events", nil, map[string]string{
		"X-AODS-User-Id":  "user-1",
		"X-AODS-Username": "deployer",
		"X-AODS-Groups":   "aods:project-a:deploy",
	})
	if eventsResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", eventsResponse.StatusCode)
	}

	var eventsBody struct {
		Items []struct {
			Type    string         `json:"type"`
			Message string         `json:"message"`
			Metadata map[string]any `json:"metadata"`
		} `json:"items"`
	}
	decodeBody(t, eventsResponse, &eventsBody)

	found := false
	for _, item := range eventsBody.Items {
		if item.Type == "DeploymentPreflightFailed" {
			found = true
			if code, ok := item.Metadata["code"].(string); !ok || code != "IMAGE_AUTH_REQUIRED" {
				t.Fatalf("expected metadata code IMAGE_AUTH_REQUIRED, got %#v", item.Metadata["code"])
			}
		}
	}
	if !found {
		t.Fatal("expected DeploymentPreflightFailed event")
	}
}

func newImageRegistryServer(t *testing.T) *httptest.Server {
	t.Helper()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "anon-token"})
			return
		case strings.HasPrefix(r.URL.Path, "/v2/"):
			repo, identifier, ok := parseManifestPath(r.URL.Path)
			if !ok {
				http.NotFound(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			switch {
			case repo == "missing":
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errors": []map[string]string{{"code": "NAME_UNKNOWN"}},
				})
				return
			case repo == "private-app":
				w.Header().Set("WWW-Authenticate", registryBearerChallenge(server.URL, repo))
				w.WriteHeader(http.StatusUnauthorized)
				return
			case repo == "token-ok":
				if authHeader == "Bearer anon-token" && identifier == "v1" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"schemaVersion":2}`))
					return
				}
				w.Header().Set("WWW-Authenticate", registryBearerChallenge(server.URL, repo))
				w.WriteHeader(http.StatusUnauthorized)
				return
			case repo == "preflight-app":
				if identifier == "private" {
					w.Header().Set("WWW-Authenticate", registryBearerChallenge(server.URL, repo))
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if authHeader == "Bearer anon-token" || identifier == "v1" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"schemaVersion":2}`))
					return
				}
				w.Header().Set("WWW-Authenticate", registryBearerChallenge(server.URL, repo))
				w.WriteHeader(http.StatusUnauthorized)
				return
			default:
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"schemaVersion":2}`))
				return
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

func imageRef(server *httptest.Server, repository string, tag string) string {
	return strings.TrimPrefix(server.URL, "http://") + "/" + repository + ":" + tag
}

func parseManifestPath(path string) (string, string, bool) {
	trimmed := strings.TrimPrefix(path, "/v2/")
	repository, identifier, ok := strings.Cut(trimmed, "/manifests/")
	return repository, identifier, ok
}

func registryBearerChallenge(baseURL string, repository string) string {
	return fmt.Sprintf(`Bearer realm="%s/token",service="aods-test-registry",scope="repository:%s:pull"`, baseURL, repository)
}
