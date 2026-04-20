package cluster

import (
	"context"
	"testing"

	"github.com/aolda/aods-backend/internal/core"
)

type stubSource struct {
	items []Summary
	err   error
}

func (s stubSource) ListClusters(context.Context) ([]Summary, error) {
	return append([]Summary(nil), s.items...), s.err
}

type stubStore struct {
	items      []Summary
	createErr  error
	createCall []CreateRequest
}

func (s *stubStore) ListClusters(context.Context) ([]Summary, error) {
	return append([]Summary(nil), s.items...), nil
}

func (s *stubStore) CreateCluster(_ context.Context, input CreateRequest) (Summary, error) {
	s.createCall = append(s.createCall, input)
	if s.createErr != nil {
		return Summary{}, s.createErr
	}
	return Summary{
		ID:          input.ID,
		Name:        input.Name,
		Description: input.Description,
		Default:     input.Default,
	}, nil
}

func TestServiceListReturnsDefaultClusterWhenSourceIsMissing(t *testing.T) {
	t.Parallel()

	service := Service{}

	items, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("list clusters: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected one synthesized cluster, got %d", len(items))
	}
	if items[0].ID != "default" {
		t.Fatalf("expected default cluster id, got %s", items[0].ID)
	}
	if !items[0].Default {
		t.Fatal("expected synthesized cluster to be default")
	}
}

func TestServiceListReturnsDefaultClusterWhenCatalogIsEmpty(t *testing.T) {
	t.Parallel()

	service := Service{Source: stubSource{}}

	items, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("list clusters: %v", err)
	}

	if len(items) != 1 || items[0].ID != "default" {
		t.Fatalf("expected fallback default cluster, got %#v", items)
	}
}

func TestServiceCreateRejectsNonAdmin(t *testing.T) {
	t.Parallel()

	service := Service{Source: &stubStore{}}

	_, err := service.Create(context.Background(), core.User{Groups: []string{"aods:project-a:deploy"}}, CreateRequest{
		ID:   "edge",
		Name: "Edge Cluster",
	})
	if err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestServiceCreateValidatesInput(t *testing.T) {
	t.Parallel()

	service := Service{Source: &stubStore{}}

	_, err := service.Create(context.Background(), core.User{Groups: []string{platformAdminGroup}}, CreateRequest{
		ID:   "INVALID_SLUG",
		Name: "Edge Cluster",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Details["field"] != "id" {
		t.Fatalf("expected field=id, got %#v", validationErr.Details)
	}
}

func TestServiceCreatePersistsClusterForPlatformAdmin(t *testing.T) {
	t.Parallel()

	store := &stubStore{}
	service := Service{Source: store}

	cluster, err := service.Create(context.Background(), core.User{Groups: []string{platformAdminGroup}}, CreateRequest{
		ID:          "edge",
		Name:        "Edge Cluster",
		Description: "extra capacity",
		Default:     true,
	})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	if len(store.createCall) != 1 {
		t.Fatalf("expected one store create call, got %d", len(store.createCall))
	}
	if cluster.ID != "edge" || cluster.Name != "Edge Cluster" {
		t.Fatalf("unexpected cluster summary: %#v", cluster)
	}
	if !cluster.Default {
		t.Fatal("expected created cluster to keep default flag")
	}
}

func TestServiceCreateAllowsConfiguredPlatformAdminAuthority(t *testing.T) {
	t.Parallel()

	store := &stubStore{}
	service := Service{
		Source:                   store,
		PlatformAdminAuthorities: []string{"/Ajou_Univ/Aolda_Admin"},
	}

	cluster, err := service.Create(context.Background(), core.User{Groups: []string{"/Ajou_Univ/Aolda_Admin"}}, CreateRequest{
		ID:   "edge",
		Name: "Edge Cluster",
	})
	if err != nil {
		t.Fatalf("create cluster with configured platform admin: %v", err)
	}
	if cluster.ID != "edge" {
		t.Fatalf("expected configured admin to create cluster, got %#v", cluster)
	}
}
