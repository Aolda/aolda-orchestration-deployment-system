package admin

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/cluster"
	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/project"
)

var ErrForbidden = errors.New("admin forbidden")

const platformAdminGroup = "aods:platform:admin"

type ApplicationStore interface {
	ListApplications(ctx context.Context, projectID string) ([]application.Record, error)
}

type Service struct {
	Projects                 project.CatalogSource
	Clusters                 cluster.Source
	Applications             ApplicationStore
	ResourceOverviewReader   ResourceOverviewReader
	PlatformAdminAuthorities []string
}

type LocalResourceOverviewReader struct{}

type ErrorResourceOverviewReader struct {
	Err error
}

func (LocalResourceOverviewReader) Read(ctx context.Context, services []RuntimeServiceRef) (RuntimeSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeSnapshot{}, err
	}
	return RuntimeSnapshot{
		GeneratedAt:      time.Now().UTC(),
		RuntimeConnected: false,
		Message:          "Kubernetes API 연동이 설정되지 않아 클러스터 총량과 실사용 효율을 계산할 수 없습니다.",
		Services:         map[string]RuntimeServiceSnapshot{},
	}, nil
}

func (r ErrorResourceOverviewReader) Read(ctx context.Context, services []RuntimeServiceRef) (RuntimeSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeSnapshot{}, err
	}
	message := "클러스터 리소스 집계를 초기화하지 못했습니다."
	if r.Err != nil {
		message = "클러스터 리소스 집계를 초기화하지 못했습니다: " + r.Err.Error()
	}
	return RuntimeSnapshot{
		GeneratedAt:      time.Now().UTC(),
		RuntimeConnected: false,
		Message:          message,
		Services:         map[string]RuntimeServiceSnapshot{},
	}, nil
}

func (s Service) GetFleetResourceOverview(ctx context.Context, user core.User) (FleetResourceOverviewResponse, error) {
	if !isPlatformAdmin(user, s.platformAdminAuthorities()) {
		return FleetResourceOverviewResponse{}, ErrForbidden
	}
	if s.Projects == nil || s.Applications == nil {
		return FleetResourceOverviewResponse{}, errors.New("admin resource overview dependencies are not configured")
	}

	projects, err := s.Projects.ListProjects(ctx)
	if err != nil {
		return FleetResourceOverviewResponse{}, err
	}

	clusterNameByID := map[string]string{}
	if s.Clusters != nil {
		clusters, err := s.Clusters.ListClusters(ctx)
		if err == nil {
			for _, item := range clusters {
				clusterNameByID[item.ID] = item.Name
			}
		}
	}

	services := make([]ServiceResourceEfficiency, 0)
	runtimeRefs := make([]RuntimeServiceRef, 0)
	for _, item := range projects {
		records, err := s.Applications.ListApplications(ctx, item.ID)
		if err != nil {
			return FleetResourceOverviewResponse{}, err
		}
		environmentClusters := environmentClusterMap(item.Environments)
		for _, record := range records {
			clusterID := strings.TrimSpace(environmentClusters[strings.TrimSpace(record.DefaultEnvironment)])
			clusterName := strings.TrimSpace(clusterNameByID[clusterID])
			base := ServiceResourceEfficiency{
				ApplicationID: record.ID,
				ProjectID:     item.ID,
				ProjectName:   item.Name,
				ClusterID:     clusterID,
				ClusterName:   clusterName,
				Namespace:     record.Namespace,
				Name:          record.Name,
			}
			services = append(services, base)
			runtimeRefs = append(runtimeRefs, RuntimeServiceRef{
				ApplicationID: record.ID,
				ProjectID:     item.ID,
				ProjectName:   item.Name,
				ClusterID:     clusterID,
				ClusterName:   clusterName,
				Namespace:     record.Namespace,
				Name:          record.Name,
			})
		}
	}

	snapshot := RuntimeSnapshot{
		GeneratedAt:      time.Now().UTC(),
		RuntimeConnected: false,
		Message:          "클러스터 runtime 정보가 아직 연결되지 않았습니다.",
		Services:         map[string]RuntimeServiceSnapshot{},
	}
	if s.ResourceOverviewReader != nil {
		readSnapshot, err := s.ResourceOverviewReader.Read(ctx, runtimeRefs)
		if err != nil {
			snapshot.GeneratedAt = time.Now().UTC()
			snapshot.RuntimeConnected = false
			snapshot.Message = err.Error()
			snapshot.Services = map[string]RuntimeServiceSnapshot{}
		} else {
			snapshot = readSnapshot
		}
	}

	counts := EfficiencyCounts{}
	for index := range services {
		runtime := snapshot.Services[services[index].ApplicationID]
		services[index].PodCount = runtime.PodCount
		services[index].ReadyPodCount = runtime.ReadyPodCount
		services[index].CPURequestCores = runtime.CPURequestCores
		services[index].CPULimitCores = runtime.CPULimitCores
		services[index].CPUUsageCores = runtime.CPUUsageCores
		services[index].CPURequestUtilization = runtime.CPURequestUtilization
		services[index].CPULimitUtilization = runtime.CPULimitUtilization
		services[index].MemoryRequestMiB = runtime.MemoryRequestMiB
		services[index].MemoryLimitMiB = runtime.MemoryLimitMiB
		services[index].MemoryUsageMiB = runtime.MemoryUsageMiB
		services[index].MemoryRequestUtilization = runtime.MemoryRequestUtilization
		services[index].MemoryLimitUtilization = runtime.MemoryLimitUtilization
		services[index].Status, services[index].Summary = classifyServiceEfficiency(snapshot.RuntimeConnected, runtime)
		switch services[index].Status {
		case ServiceEfficiencyStatusBalanced:
			counts.Balanced++
		case ServiceEfficiencyStatusUnderutilized:
			counts.Underutilized++
		case ServiceEfficiencyStatusOverutilized:
			counts.Overutilized++
		case ServiceEfficiencyStatusNoMetrics:
			counts.NoMetrics++
		default:
			counts.Unknown++
		}
	}

	sort.SliceStable(services, func(i, j int) bool {
		left := services[i]
		right := services[j]
		if severity(left.Status) != severity(right.Status) {
			return severity(left.Status) < severity(right.Status)
		}
		if left.ProjectName != right.ProjectName {
			return left.ProjectName < right.ProjectName
		}
		return left.Name < right.Name
	})

	return FleetResourceOverviewResponse{
		GeneratedAt:      snapshot.GeneratedAt,
		RuntimeConnected: snapshot.RuntimeConnected,
		Message:          snapshot.Message,
		ProjectCount:     len(projects),
		ServiceCount:     len(services),
		Capacity:         snapshot.Capacity,
		Counts:           counts,
		Services:         services,
	}, nil
}

func environmentClusterMap(items []project.Environment) map[string]string {
	values := make(map[string]string, len(items))
	for _, item := range items {
		values[strings.TrimSpace(item.ID)] = strings.TrimSpace(item.ClusterID)
	}
	return values
}

func classifyServiceEfficiency(runtimeConnected bool, runtime RuntimeServiceSnapshot) (ServiceEfficiencyStatus, string) {
	if !runtimeConnected {
		return ServiceEfficiencyStatusUnknown, "현재 연결된 Kubernetes runtime 정보가 없습니다."
	}
	if runtime.PodCount == 0 {
		return ServiceEfficiencyStatusUnknown, "실행 중인 pod를 찾지 못했습니다."
	}
	if runtime.CPUUsageCores == nil && runtime.MemoryUsageMiB == nil {
		return ServiceEfficiencyStatusNoMetrics, "metrics.k8s.io 실측값이 없어 요청치 대비 효율을 계산하지 못했습니다."
	}

	utilizations := collectUtilizations(runtime.CPURequestUtilization, runtime.MemoryRequestUtilization)
	if len(utilizations) == 0 {
		return ServiceEfficiencyStatusUnknown, "요청치가 없어 효율을 계산하지 못했습니다."
	}

	maxUtilization := utilizations[0]
	for _, item := range utilizations[1:] {
		if item > maxUtilization {
			maxUtilization = item
		}
	}

	if maxUtilization > 100 {
		return ServiceEfficiencyStatusOverutilized, "요청치를 초과해 사용 중입니다. request 상향 또는 workload 점검이 필요합니다."
	}
	if maxUtilization < 30 {
		return ServiceEfficiencyStatusUnderutilized, "요청 대비 사용량이 낮습니다. request를 줄여도 될 가능성이 큽니다."
	}
	return ServiceEfficiencyStatusBalanced, "요청 대비 사용량이 현재 수준에서는 안정적입니다."
}

func collectUtilizations(values ...*float64) []float64 {
	items := make([]float64, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		items = append(items, *value)
	}
	return items
}

func severity(status ServiceEfficiencyStatus) int {
	switch status {
	case ServiceEfficiencyStatusOverutilized:
		return 0
	case ServiceEfficiencyStatusUnknown:
		return 1
	case ServiceEfficiencyStatusNoMetrics:
		return 2
	case ServiceEfficiencyStatusUnderutilized:
		return 3
	default:
		return 4
	}
}

func isPlatformAdmin(user core.User, authorities []string) bool {
	if len(authorities) == 0 {
		authorities = []string{platformAdminGroup}
	}
	groupSet := make(map[string]struct{}, len(user.Groups))
	for _, item := range user.Groups {
		groupSet[item] = struct{}{}
	}
	for _, authority := range authorities {
		if _, ok := groupSet[strings.TrimSpace(authority)]; ok {
			return true
		}
	}
	return false
}

func (s Service) platformAdminAuthorities() []string {
	items := make([]string, 0, len(s.PlatformAdminAuthorities))
	seen := make(map[string]struct{}, len(s.PlatformAdminAuthorities))
	for _, authority := range s.PlatformAdminAuthorities {
		trimmed := strings.TrimSpace(authority)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}
