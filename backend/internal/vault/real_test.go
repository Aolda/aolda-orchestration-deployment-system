package vault

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

func TestRealStoreStageAndFinalizeUsesKVv2Endpoints(t *testing.T) {
	t.Helper()

	var requests []struct {
		Method string
		Path   string
		Token  string
		Body   map[string]any
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}

		requests = append(requests, struct {
			Method string
			Path   string
			Token  string
			Body   map[string]any
		}{
			Method: r.Method,
			Path:   r.URL.Path,
			Token:  r.Header.Get("X-Vault-Token"),
			Body:   body,
		})

		if r.Method == http.MethodGet && r.URL.Path == "/v1/secret/metadata/aods/staging/req_123" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"current_version": 1,
					"custom_metadata": map[string]string{
						"status":    "pending_commit",
						"projectId": "project-a",
						"appName":   "my-app",
						"updatedBy": "deployer",
					},
					"versions": map[string]any{
						"1": map[string]any{
							"version":      1,
							"created_time": time.Now().UTC().Format(time.RFC3339Nano),
						},
					},
				},
			})
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	store := RealStore{
		Address: server.URL,
		Token:   "test-token",
		Client:  server.Client(),
	}

	staged, err := store.Stage(
		context.Background(),
		"req_123",
		"project-a",
		"my-app",
		"deployer",
		map[string]string{"DATABASE_URL": "postgres://db"},
	)
	if err != nil {
		t.Fatalf("stage secret: %v", err)
	}

	if staged.StagingPath != "secret/aods/staging/req_123" {
		t.Fatalf("unexpected staging path: %s", staged.StagingPath)
	}
	if staged.FinalPath != "secret/aods/apps/project-a/my-app/prod" {
		t.Fatalf("unexpected final path: %s", staged.FinalPath)
	}

	if err := store.Finalize(context.Background(), staged, map[string]string{"DATABASE_URL": "postgres://db"}); err != nil {
		t.Fatalf("finalize secret: %v", err)
	}

	if len(requests) != 6 {
		t.Fatalf("expected 6 vault requests, got %d", len(requests))
	}

	if requests[0].Method != http.MethodPost || requests[0].Path != "/v1/secret/data/aods/staging/req_123" {
		t.Fatalf("unexpected stage write request: %+v", requests[0])
	}
	if requests[0].Token != "test-token" {
		t.Fatalf("expected vault token header")
	}

	if requests[1].Method != http.MethodPost || requests[1].Path != "/v1/secret/metadata/aods/staging/req_123" {
		t.Fatalf("unexpected stage metadata request: %+v", requests[1])
	}
	customMetadata, ok := requests[1].Body["custom_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected custom_metadata payload, got %#v", requests[1].Body)
	}
	if customMetadata["status"] != "pending_commit" {
		t.Fatalf("expected pending_commit metadata, got %#v", customMetadata["status"])
	}

	if requests[2].Method != http.MethodGet || requests[2].Path != "/v1/secret/metadata/aods/staging/req_123" {
		t.Fatalf("unexpected staging metadata read request: %+v", requests[2])
	}
	if requests[3].Method != http.MethodPost || requests[3].Path != "/v1/secret/data/aods/apps/project-a/my-app/prod" {
		t.Fatalf("unexpected final write request: %+v", requests[3])
	}
	if requests[4].Method != http.MethodPost || requests[4].Path != "/v1/secret/metadata/aods/apps/project-a/my-app/prod" {
		t.Fatalf("unexpected final metadata request: %+v", requests[4])
	}
	finalMetadata, ok := requests[4].Body["custom_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected final custom_metadata payload, got %#v", requests[4].Body)
	}
	if finalMetadata["updatedBy"] != "deployer" || finalMetadata["status"] != "finalized" {
		t.Fatalf("expected final metadata to preserve staging metadata, got %#v", finalMetadata)
	}
	if requests[5].Method != http.MethodDelete || requests[5].Path != "/v1/secret/metadata/aods/staging/req_123" {
		t.Fatalf("unexpected cleanup request: %+v", requests[5])
	}
}

func TestRealStoreAddsNamespaceHeader(t *testing.T) {
	t.Helper()

	var namespace string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		namespace = r.Header.Get("X-Vault-Namespace")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	store := RealStore{
		Address:   server.URL,
		Token:     "test-token",
		Namespace: "team-a",
		Client:    server.Client(),
	}

	if _, err := store.Stage(context.Background(), "req_123", "project-a", "my-app", "deployer", map[string]string{"X": "Y"}); err != nil {
		t.Fatalf("stage secret: %v", err)
	}

	if namespace != "team-a" {
		t.Fatalf("expected vault namespace header, got %q", namespace)
	}
}

func TestRealStoreCleanupStaleDeletesExpiredPendingCommitSecrets(t *testing.T) {
	t.Helper()

	var deletePaths []string
	var methods []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method+" "+r.URL.Path)

		switch {
		case r.Method == "LIST" && r.URL.Path == "/v1/secret/metadata/aods/staging":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"keys": []string{"req_old", "req_fresh", "req_done"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/secret/metadata/aods/staging/req_old":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"created_time": time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339Nano),
					"updated_time": time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339Nano),
					"custom_metadata": map[string]string{
						"status":    pendingCommitStatus,
						"createdAt": time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339Nano),
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/secret/metadata/aods/staging/req_fresh":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"created_time": time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339Nano),
					"updated_time": time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339Nano),
					"custom_metadata": map[string]string{
						"status":    pendingCommitStatus,
						"createdAt": time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339Nano),
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/secret/metadata/aods/staging/req_done":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"created_time": time.Now().UTC().Add(-72 * time.Hour).Format(time.RFC3339Nano),
					"updated_time": time.Now().UTC().Add(-72 * time.Hour).Format(time.RFC3339Nano),
					"custom_metadata": map[string]string{
						"status":    "finalized",
						"createdAt": time.Now().UTC().Add(-72 * time.Hour).Format(time.RFC3339Nano),
					},
				},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/secret/metadata/aods/staging/req_old":
			deletePaths = append(deletePaths, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected vault request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	store := RealStore{
		Address: server.URL,
		Token:   "test-token",
		Client:  server.Client(),
	}

	count, err := store.CleanupStale(context.Background(), time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("cleanup stale secrets: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 cleaned staged secret, got %d", count)
	}
	if !slices.Equal(deletePaths, []string{"/v1/secret/metadata/aods/staging/req_old"}) {
		t.Fatalf("unexpected delete paths: %#v", deletePaths)
	}
}
