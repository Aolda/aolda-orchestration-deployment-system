package application

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aolda/aods-backend/internal/core"
)

func TestListApplicationsReturnsNotModifiedForMatchingETag(t *testing.T) {
	t.Parallel()

	handler := Handler{
		Service: &Service{
			Projects: authorizedProjectService(),
			Store: stubStore{
				records: []Record{
					{
						ID:                 "project-a__api",
						ProjectID:          "project-a",
						Name:               "api",
						Image:              "ghcr.io/aolda/api:v1",
						DeploymentStrategy: DeploymentStrategyRollout,
					},
				},
			},
			StatusReader: &batchStatusReaderStub{},
		},
		Users: core.DevFallbackUserProvider{
			AllowDevFallback: true,
			DevUser: core.User{
				ID:     "user-1",
				Groups: []string{"aods:project-a:deploy"},
			},
		},
	}

	firstRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/project-a/applications", nil)
	firstRequest.SetPathValue("projectId", "project-a")
	first := httptest.NewRecorder()
	handler.ListApplications(first, firstRequest)

	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header")
	}

	secondRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/project-a/applications", nil)
	secondRequest.SetPathValue("projectId", "project-a")
	secondRequest.Header.Set("If-None-Match", etag)
	second := httptest.NewRecorder()
	handler.ListApplications(second, secondRequest)

	if got := second.Code; got != http.StatusNotModified {
		t.Fatalf("unexpected status %d", got)
	}
	if got := second.Body.String(); got != "" {
		t.Fatalf("expected empty 304 body, got %q", got)
	}
}
