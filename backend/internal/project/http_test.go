package project

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aolda/aods-backend/internal/core"
)

type conditionalCatalogSource struct {
	items []CatalogProject
}

func (s conditionalCatalogSource) ListProjects(context.Context) ([]CatalogProject, error) {
	return s.items, nil
}

func TestListProjectsReturnsNotModifiedForMatchingETag(t *testing.T) {
	t.Parallel()

	handler := Handler{
		Service: &Service{
			Source: conditionalCatalogSource{
				items: []CatalogProject{
					{
						ID:        "shared",
						Name:      "Shared",
						Namespace: "shared",
						Access: Access{
							ViewerGroups: []string{"aods:shared:view"},
						},
					},
				},
			},
		},
		Users: core.DevFallbackUserProvider{
			AllowDevFallback: true,
			DevUser: core.User{
				ID:     "user-1",
				Groups: []string{"aods:shared:view"},
			},
		},
	}

	first := httptest.NewRecorder()
	handler.ListProjects(first, httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil))

	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header")
	}

	secondRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	secondRequest.Header.Set("If-None-Match", etag)
	second := httptest.NewRecorder()
	handler.ListProjects(second, secondRequest)

	if got := second.Code; got != http.StatusNotModified {
		t.Fatalf("unexpected status %d", got)
	}
	if got := second.Body.String(); got != "" {
		t.Fatalf("expected empty 304 body, got %q", got)
	}
}
