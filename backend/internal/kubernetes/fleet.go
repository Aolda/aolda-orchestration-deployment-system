package kubernetes

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/admin"
	"github.com/aolda/aods-backend/internal/core"
)

type FleetResourceReader struct {
	Client *apiClient
}

type nodeListResponse struct {
	Items []nodeResponse `json:"items"`
}

type nodeResponse struct {
	Status struct {
		Allocatable map[string]string `json:"allocatable"`
	} `json:"status"`
}

func NewFleetResourceReader(cfg core.Config) (admin.ResourceOverviewReader, error) {
	client, err := newAPIClient(cfg)
	if err != nil {
		return nil, err
	}
	return FleetResourceReader{Client: client}, nil
}

func (r FleetResourceReader) Read(ctx context.Context, services []admin.RuntimeServiceRef) (admin.RuntimeSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return admin.RuntimeSnapshot{}, err
	}
	if r.Client == nil {
		return admin.RuntimeSnapshot{
			GeneratedAt:      time.Now().UTC(),
			RuntimeConnected: false,
			Message:          "Kubernetes API 클라이언트가 설정되지 않았습니다.",
			Services:         map[string]admin.RuntimeServiceSnapshot{},
		}, nil
	}

	nodes, err := r.listNodes(ctx)
	if err != nil {
		return admin.RuntimeSnapshot{}, err
	}
	pods, err := r.listPods(ctx)
	if err != nil {
		return admin.RuntimeSnapshot{}, err
	}
	podUsageByKey, metricsErr := r.listPodUsage(ctx)

	allocatableCPU, allocatableMemory, err := sumNodeAllocatable(nodes)
	if err != nil {
		return admin.RuntimeSnapshot{}, err
	}
	requestedCPU, requestedMemory, err := sumPodRequests(pods)
	if err != nil {
		return admin.RuntimeSnapshot{}, err
	}

	var usedCPU *float64
	var usedMemory *float64
	if metricsErr == nil {
		cpuValue, memoryValue := sumPodUsage(podUsageByKey)
		usedCPU = float64Pointer(cpuValue)
		usedMemory = float64Pointer(memoryValue)
	}

	snapshot := admin.RuntimeSnapshot{
		GeneratedAt:      time.Now().UTC(),
		RuntimeConnected: true,
		Message:          "",
		Capacity: admin.CapacitySummary{
			AllocatableCPUCores:      float64Pointer(allocatableCPU),
			AllocatableMemoryMiB:     float64Pointer(allocatableMemory),
			RequestedCPUCores:        float64Pointer(requestedCPU),
			RequestedMemoryMiB:       float64Pointer(requestedMemory),
			UsedCPUCores:             usedCPU,
			UsedMemoryMiB:            usedMemory,
			AvailableCPUCores:        float64Pointer(allocatableCPU - requestedCPU),
			AvailableMemoryMiB:       float64Pointer(allocatableMemory - requestedMemory),
			RequestCPUUtilization:    utilization(float64Pointer(requestedCPU), float64Pointer(allocatableCPU)),
			RequestMemoryUtilization: utilization(float64Pointer(requestedMemory), float64Pointer(allocatableMemory)),
			UsageCPUUtilization:      utilization(usedCPU, float64Pointer(allocatableCPU)),
			UsageMemoryUtilization:   utilization(usedMemory, float64Pointer(allocatableMemory)),
		},
		Services: map[string]admin.RuntimeServiceSnapshot{},
	}
	if metricsErr != nil {
		snapshot.Message = "metrics.k8s.io 실측값을 읽지 못해 usage 없이 request 기준만 표시합니다."
	}

	for _, service := range services {
		snapshot.Services[service.ApplicationID] = aggregateServiceRuntime(service, pods, podUsageByKey)
	}

	return snapshot, nil
}

func (r FleetResourceReader) listNodes(ctx context.Context) ([]nodeResponse, error) {
	var response nodeListResponse
	if err := r.Client.GetJSON(ctx, "/api/v1/nodes", &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (r FleetResourceReader) listPods(ctx context.Context) ([]podResponse, error) {
	var response podListResponse
	if err := r.Client.GetJSON(ctx, "/api/v1/pods", &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (r FleetResourceReader) listPodUsage(ctx context.Context) (map[string]podContainerUsage, error) {
	var response podMetricsListResponse
	if err := r.Client.GetJSON(ctx, kubernetesPodMetricsResourcePath+"/pods", &response); err != nil {
		var apiErr apiRequestError
		if errorsAsStatus(err, &apiErr, http.StatusNotFound) {
			return map[string]podContainerUsage{}, err
		}
		return nil, err
	}

	usageByKey := make(map[string]podContainerUsage, len(response.Items))
	for _, item := range response.Items {
		totalCPU := 0.0
		totalMemory := 0.0
		for _, container := range item.Containers {
			cpuValue, err := parseCPUQuantityToCores(container.Usage.CPU)
			if err != nil {
				return nil, err
			}
			memoryValue, err := parseMemoryQuantityToMiB(container.Usage.Memory)
			if err != nil {
				return nil, err
			}
			totalCPU += cpuValue
			totalMemory += memoryValue
		}
		cpuCopy := totalCPU
		memoryCopy := totalMemory
		usageByKey[podKey(item.Metadata.Namespace, item.Metadata.Name)] = podContainerUsage{
			CPUCores:  &cpuCopy,
			MemoryMiB: &memoryCopy,
		}
	}
	return usageByKey, nil
}

func sumNodeAllocatable(items []nodeResponse) (float64, float64, error) {
	totalCPU := 0.0
	totalMemory := 0.0
	for _, item := range items {
		if value := strings.TrimSpace(item.Status.Allocatable["cpu"]); value != "" {
			parsed, err := parseCPUQuantityToCores(value)
			if err != nil {
				return 0, 0, err
			}
			totalCPU += parsed
		}
		if value := strings.TrimSpace(item.Status.Allocatable["memory"]); value != "" {
			parsed, err := parseMemoryQuantityToMiB(value)
			if err != nil {
				return 0, 0, err
			}
			totalMemory += parsed
		}
	}
	return totalCPU, totalMemory, nil
}

func sumPodRequests(items []podResponse) (float64, float64, error) {
	totalCPU := 0.0
	totalMemory := 0.0
	for _, pod := range items {
		if isTerminalPod(pod.Status.Phase) {
			continue
		}
		for _, container := range pod.Spec.Containers {
			if value := strings.TrimSpace(container.Resources.Requests["cpu"]); value != "" {
				parsed, err := parseCPUQuantityToCores(value)
				if err != nil {
					return 0, 0, err
				}
				totalCPU += parsed
			}
			if value := strings.TrimSpace(container.Resources.Requests["memory"]); value != "" {
				parsed, err := parseMemoryQuantityToMiB(value)
				if err != nil {
					return 0, 0, err
				}
				totalMemory += parsed
			}
		}
	}
	return totalCPU, totalMemory, nil
}

func sumPodUsage(items map[string]podContainerUsage) (float64, float64) {
	totalCPU := 0.0
	totalMemory := 0.0
	for _, usage := range items {
		if usage.CPUCores != nil {
			totalCPU += *usage.CPUCores
		}
		if usage.MemoryMiB != nil {
			totalMemory += *usage.MemoryMiB
		}
	}
	return totalCPU, totalMemory
}

func aggregateServiceRuntime(service admin.RuntimeServiceRef, pods []podResponse, usageByKey map[string]podContainerUsage) admin.RuntimeServiceSnapshot {
	snapshot := admin.RuntimeServiceSnapshot{}
	totalCPURequest := 0.0
	totalCPULimit := 0.0
	totalMemoryRequest := 0.0
	totalMemoryLimit := 0.0
	totalCPUUsage := 0.0
	totalMemoryUsage := 0.0
	hasCPURequest := false
	hasCPULimit := false
	hasMemoryRequest := false
	hasMemoryLimit := false
	hasCPUUsage := false
	hasMemoryUsage := false

	for _, pod := range pods {
		if !podMatchesService(pod, service) || isTerminalPod(pod.Status.Phase) {
			continue
		}
		snapshot.PodCount++
		if isPodReady(pod) {
			snapshot.ReadyPodCount++
		}

		for _, container := range pod.Spec.Containers {
			if value := strings.TrimSpace(container.Resources.Requests["cpu"]); value != "" {
				parsed, err := parseCPUQuantityToCores(value)
				if err == nil {
					totalCPURequest += parsed
					hasCPURequest = true
				}
			}
			if value := strings.TrimSpace(container.Resources.Limits["cpu"]); value != "" {
				parsed, err := parseCPUQuantityToCores(value)
				if err == nil {
					totalCPULimit += parsed
					hasCPULimit = true
				}
			}
			if value := strings.TrimSpace(container.Resources.Requests["memory"]); value != "" {
				parsed, err := parseMemoryQuantityToMiB(value)
				if err == nil {
					totalMemoryRequest += parsed
					hasMemoryRequest = true
				}
			}
			if value := strings.TrimSpace(container.Resources.Limits["memory"]); value != "" {
				parsed, err := parseMemoryQuantityToMiB(value)
				if err == nil {
					totalMemoryLimit += parsed
					hasMemoryLimit = true
				}
			}
		}

		if usage, ok := usageByKey[podKey(pod.Metadata.Namespace, pod.Metadata.Name)]; ok {
			if usage.CPUCores != nil {
				totalCPUUsage += *usage.CPUCores
				hasCPUUsage = true
			}
			if usage.MemoryMiB != nil {
				totalMemoryUsage += *usage.MemoryMiB
				hasMemoryUsage = true
			}
		}
	}

	if hasCPURequest {
		snapshot.CPURequestCores = float64Pointer(totalCPURequest)
	}
	if hasCPULimit {
		snapshot.CPULimitCores = float64Pointer(totalCPULimit)
	}
	if hasMemoryRequest {
		snapshot.MemoryRequestMiB = float64Pointer(totalMemoryRequest)
	}
	if hasMemoryLimit {
		snapshot.MemoryLimitMiB = float64Pointer(totalMemoryLimit)
	}
	if hasCPUUsage {
		snapshot.CPUUsageCores = float64Pointer(totalCPUUsage)
	}
	if hasMemoryUsage {
		snapshot.MemoryUsageMiB = float64Pointer(totalMemoryUsage)
	}
	snapshot.CPURequestUtilization = utilization(snapshot.CPUUsageCores, snapshot.CPURequestCores)
	snapshot.CPULimitUtilization = utilization(snapshot.CPUUsageCores, snapshot.CPULimitCores)
	snapshot.MemoryRequestUtilization = utilization(snapshot.MemoryUsageMiB, snapshot.MemoryRequestMiB)
	snapshot.MemoryLimitUtilization = utilization(snapshot.MemoryUsageMiB, snapshot.MemoryLimitMiB)

	return snapshot
}

func podMatchesService(pod podResponse, service admin.RuntimeServiceRef) bool {
	if strings.TrimSpace(pod.Metadata.Namespace) != strings.TrimSpace(service.Namespace) {
		return false
	}
	if value := strings.TrimSpace(pod.Metadata.Labels["app.kubernetes.io/name"]); value != "" {
		return value == strings.TrimSpace(service.Name)
	}
	pattern := regexp.MustCompile("^" + regexp.QuoteMeta(strings.TrimSpace(service.Name)) + `-[a-z0-9]+(?:-[a-z0-9]+)?$`)
	return pattern.MatchString(strings.TrimSpace(pod.Metadata.Name))
}

func isTerminalPod(phase string) bool {
	switch strings.TrimSpace(phase) {
	case "Succeeded", "Failed":
		return true
	default:
		return false
	}
}

func isPodReady(pod podResponse) bool {
	if strings.TrimSpace(pod.Status.Phase) != "Running" {
		return false
	}
	if len(pod.Status.ContainerStatuses) == 0 {
		return false
	}
	for _, item := range pod.Status.ContainerStatuses {
		if !item.Ready {
			return false
		}
	}
	return true
}

func podKey(namespace string, name string) string {
	return strings.TrimSpace(namespace) + "/" + strings.TrimSpace(name)
}

func errorsAsStatus(err error, target *apiRequestError, statusCode int) bool {
	if err == nil {
		return false
	}
	if !errors.As(err, target) {
		return false
	}
	return target.StatusCode == statusCode
}
