package cluster

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type conditionalClusterSource struct {
	items []Summary
}

func (s conditionalClusterSource) ListClusters(context.Context) ([]Summary, error) {
	return s.items, nil
}

func TestListClustersReturnsNotModifiedForMatchingETag(t *testing.T) {
	t.Parallel()

	handler := Handler{
		Service: &Service{
			Source: conditionalClusterSource{
				items: []Summary{
					{
						ID:      "default",
						Name:    "Default Cluster",
						Default: true,
					},
				},
			},
		},
	}

	first := httptest.NewRecorder()
	handler.ListClusters(first, httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil))

	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header")
	}

	secondRequest := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	secondRequest.Header.Set("If-None-Match", etag)
	second := httptest.NewRecorder()
	handler.ListClusters(second, secondRequest)

	if got := second.Code; got != http.StatusNotModified {
		t.Fatalf("unexpected status %d", got)
	}
	if got := second.Body.String(); got != "" {
		t.Fatalf("expected empty 304 body, got %q", got)
	}
}
