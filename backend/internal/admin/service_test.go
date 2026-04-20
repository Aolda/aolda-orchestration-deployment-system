package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/cluster"
	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/project"
)

type stubProjectSource struct {
	items []project.CatalogProject
	err   error
}

func (s stubProjectSource) ListProjects(ctx context.Context) ([]project.CatalogProject, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

type stubClusterSource struct {
	items []cluster.Summary
	err   error
}

func (s stubClusterSource) ListClusters(ctx context.Context) ([]cluster.Summary, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

type stubApplicationStore struct {
	items map[string][]application.Record
	err   error
}

func (s stubApplicationStore) ListApplications(ctx context.Context, projectID string) ([]application.Record, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.items[projectID], nil
}

type stubResourceOverviewReader struct {
	snapshot RuntimeSnapshot
	err      error
}

func (s stubResourceOverviewReader) Read(ctx context.Context, services []RuntimeServiceRef) (RuntimeSnapshot, error) {
	if s.err != nil {
		return RuntimeSnapshot{}, s.err
	}
	return s.snapshot, nil
}

func TestGetFleetResourceOverviewRequiresPlatformAdmin(t *testing.T) {
	service := Service{
		Projects:     stubProjectSource{},
		Applications: stubApplicationStore{},
	}

	_, err := service.GetFleetResourceOverview(context.Background(), core.User{
		ID:       "user-1",
		Username: "viewer",
		Groups:   []string{"aods:project-a:view"},
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestGetFleetResourceOverviewBuildsServiceEfficiencySnapshot(t *testing.T) {
	generatedAt := time.Date(2026, 4, 18, 2, 0, 0, 0, time.UTC)

	service := Service{
		Projects: stubProjectSource{
			items: []project.CatalogProject{
				{
					ID:        "shared",
					Name:      "공용 프로젝트",
					Namespace: "shared",
					Environments: []project.Environment{
						{ID: "shared", ClusterID: "default", Default: true},
					},
				},
				{
					ID:        "project-b",
					Name:      "Project B",
					Namespace: "project-b",
					Environments: []project.Environment{
						{ID: "prod", ClusterID: "analytics", Default: true},
					},
				},
			},
		},
		Clusters: stubClusterSource{
			items: []cluster.Summary{
				{ID: "default", Name: "Shared Cluster"},
				{ID: "analytics", Name: "Analytics Cluster"},
			},
		},
		Applications: stubApplicationStore{
			items: map[string][]application.Record{
				"shared": {
					{
						ID:                 "shared__portal",
						ProjectID:          "shared",
						Namespace:          "shared",
						Name:               "portal",
						DefaultEnvironment: "shared",
					},
				},
			},
		},
		ResourceOverviewReader: stubResourceOverviewReader{
			snapshot: RuntimeSnapshot{
				GeneratedAt:      generatedAt,
				RuntimeConnected: true,
				Capacity: CapacitySummary{
					AllocatableCPUCores:  float64Ptr(16),
					RequestedCPUCores:    float64Ptr(6),
					AvailableCPUCores:    float64Ptr(10),
					AllocatableMemoryMiB: float64Ptr(32768),
					RequestedMemoryMiB:   float64Ptr(12288),
					AvailableMemoryMiB:   float64Ptr(20480),
				},
				Services: map[string]RuntimeServiceSnapshot{
					"shared__portal": {
						PodCount:                 2,
						ReadyPodCount:            2,
						CPURequestCores:          float64Ptr(0.5),
						CPULimitCores:            float64Ptr(1.0),
						CPUUsageCores:            float64Ptr(0.7),
						CPURequestUtilization:    float64Ptr(140),
						CPULimitUtilization:      float64Ptr(70),
						MemoryRequestMiB:         float64Ptr(512),
						MemoryLimitMiB:           float64Ptr(1024),
						MemoryUsageMiB:           float64Ptr(320),
						MemoryRequestUtilization: float64Ptr(62.5),
						MemoryLimitUtilization:   float64Ptr(31.25),
					},
				},
			},
		},
		PlatformAdminAuthorities: []string{"/Ajou_Univ/Aolda_Admin"},
	}

	response, err := service.GetFleetResourceOverview(context.Background(), core.User{
		ID:       "admin-1",
		Username: "platform-admin",
		Groups:   []string{"/Ajou_Univ/Aolda_Admin"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !response.GeneratedAt.Equal(generatedAt) {
		t.Fatalf("expected generatedAt %s, got %s", generatedAt, response.GeneratedAt)
	}
	if !response.RuntimeConnected {
		t.Fatal("expected runtimeConnected=true")
	}
	if response.ProjectCount != 2 {
		t.Fatalf("expected projectCount=2, got %d", response.ProjectCount)
	}
	if response.ServiceCount != 1 {
		t.Fatalf("expected serviceCount=1, got %d", response.ServiceCount)
	}
	if response.Counts.Overutilized != 1 {
		t.Fatalf("expected overutilized=1, got %#v", response.Counts)
	}
	if len(response.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(response.Services))
	}

	item := response.Services[0]
	if item.ClusterName != "Shared Cluster" {
		t.Fatalf("expected Shared Cluster, got %q", item.ClusterName)
	}
	if item.Status != ServiceEfficiencyStatusOverutilized {
		t.Fatalf("expected Overutilized, got %s", item.Status)
	}
	if item.PodCount != 2 || item.ReadyPodCount != 2 {
		t.Fatalf("expected pod counts 2/2, got %d/%d", item.ReadyPodCount, item.PodCount)
	}
	if item.Summary == "" {
		t.Fatal("expected summary to be populated")
	}
}

func float64Ptr(value float64) *float64 {
	return &value
}
