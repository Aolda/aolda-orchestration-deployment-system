package kubernetes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aolda/aods-backend/internal/admin"
)

func TestFleetResourceReaderWithoutClientReturnsDisconnectedSnapshot(t *testing.T) {
	t.Parallel()

	snapshot, err := (FleetResourceReader{}).Read(context.Background(), []admin.RuntimeServiceRef{
		{ApplicationID: "app-1", Namespace: "shared", Name: "demo"},
	})
	if err != nil {
		t.Fatalf("read fleet snapshot: %v", err)
	}
	if snapshot.RuntimeConnected {
		t.Fatal("expected disconnected snapshot when client is nil")
	}
	if len(snapshot.Services) != 0 {
		t.Fatalf("expected no service snapshots, got %#v", snapshot.Services)
	}
}

func TestFleetResourceReaderAggregatesKubernetesAPIResponses(t *testing.T) {
	t.Parallel()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/nodes":
			w.Write([]byte(`{
				"items": [
					{"status":{"allocatable":{"cpu":"4","memory":"8Gi"}}}
				]
			}`))
		case "/api/v1/pods":
			w.Write([]byte(`{
				"items": [
					{
						"metadata":{"namespace":"shared","name":"demo-abc123","labels":{"app.kubernetes.io/name":"demo"}},
						"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"250m","memory":"256Mi"},"limits":{"cpu":"500m","memory":"512Mi"}}}]},
						"status":{"phase":"Running","containerStatuses":[{"name":"app","ready":true}]}
					}
				]
			}`))
		case kubernetesPodMetricsResourcePath + "/pods":
			w.Write([]byte(`{
				"items": [
					{
						"metadata":{"namespace":"shared","name":"demo-abc123"},
						"containers":[{"name":"app","usage":{"cpu":"100m","memory":"64Mi"}}]
					}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	reader := FleetResourceReader{Client: &apiClient{
		BaseURL:    api.URL,
		HTTPClient: api.Client(),
	}}
	snapshot, err := reader.Read(context.Background(), []admin.RuntimeServiceRef{
		{ApplicationID: "shared__demo", ProjectID: "shared", Namespace: "shared", Name: "demo"},
	})
	if err != nil {
		t.Fatalf("read fleet snapshot: %v", err)
	}
	if !snapshot.RuntimeConnected {
		t.Fatalf("expected connected snapshot, got %#v", snapshot)
	}
	assertFloatPointer(t, "allocatable cpu", snapshot.Capacity.AllocatableCPUCores, 4)
	assertFloatPointer(t, "requested cpu", snapshot.Capacity.RequestedCPUCores, 0.25)
	assertFloatPointer(t, "used cpu", snapshot.Capacity.UsedCPUCores, 0.1)
	if snapshot.Services["shared__demo"].PodCount != 1 || snapshot.Services["shared__demo"].ReadyPodCount != 1 {
		t.Fatalf("unexpected service runtime snapshot: %#v", snapshot.Services["shared__demo"])
	}
}

func TestFleetResourceReaderContinuesWithoutMetricsAPI(t *testing.T) {
	t.Parallel()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/nodes":
			w.Write([]byte(`{"items":[{"status":{"allocatable":{"cpu":"1","memory":"1Gi"}}}]}`))
		case "/api/v1/pods":
			w.Write([]byte(`{"items":[]}`))
		case kubernetesPodMetricsResourcePath + "/pods":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	reader := FleetResourceReader{Client: &apiClient{
		BaseURL:    api.URL,
		HTTPClient: api.Client(),
	}}
	snapshot, err := reader.Read(context.Background(), nil)
	if err != nil {
		t.Fatalf("read fleet snapshot without metrics: %v", err)
	}
	if !snapshot.RuntimeConnected || snapshot.Message == "" {
		t.Fatalf("expected connected snapshot with metrics warning, got %#v", snapshot)
	}
	if snapshot.Capacity.UsedCPUCores != nil || snapshot.Capacity.UsedMemoryMiB != nil {
		t.Fatalf("expected usage values to be absent when metrics API fails, got %#v", snapshot.Capacity)
	}
}

func TestFleetCapacityAggregationSkipsTerminalPods(t *testing.T) {
	t.Parallel()

	nodes := []nodeResponse{
		fleetNode("4", "8Gi"),
		fleetNode("2500m", "1024Mi"),
	}
	pods := []podResponse{
		fleetPod("shared", "demo-abc123", "Running", true, fleetResources{
			requestCPU: "250m", requestMemory: "256Mi",
		}),
		fleetPod("shared", "done", "Succeeded", true, fleetResources{
			requestCPU: "8", requestMemory: "32Gi",
		}),
	}

	cpu, memory, err := sumNodeAllocatable(nodes)
	if err != nil {
		t.Fatalf("sum node allocatable: %v", err)
	}
	if cpu != 6.5 || memory != 9216 {
		t.Fatalf("unexpected allocatable capacity cpu=%f memory=%f", cpu, memory)
	}

	requestCPU, requestMemory, err := sumPodRequests(pods)
	if err != nil {
		t.Fatalf("sum pod requests: %v", err)
	}
	if requestCPU != 0.25 || requestMemory != 256 {
		t.Fatalf("unexpected requested resources cpu=%f memory=%f", requestCPU, requestMemory)
	}

	usageCPU, usageMemory := sumPodUsage(map[string]podContainerUsage{
		"shared/demo-abc123": {CPUCores: float64Pointer(0.1), MemoryMiB: float64Pointer(64)},
		"shared/other":       {CPUCores: nil, MemoryMiB: float64Pointer(32)},
	})
	if usageCPU != 0.1 || usageMemory != 96 {
		t.Fatalf("unexpected pod usage cpu=%f memory=%f", usageCPU, usageMemory)
	}
}

func TestAggregateServiceRuntimeMatchesLabelsAndNames(t *testing.T) {
	t.Parallel()

	service := admin.RuntimeServiceRef{
		ApplicationID: "shared__demo",
		Namespace:     "shared",
		Name:          "demo",
	}
	pods := []podResponse{
		fleetPod("shared", "demo-abc123", "Running", true, fleetResources{
			requestCPU: "250m", limitCPU: "500m", requestMemory: "256Mi", limitMemory: "512Mi",
		}),
		fleetPodWithLabels("shared", "unrelated-name", "Running", true, map[string]string{"app.kubernetes.io/name": "demo"}, fleetResources{
			requestCPU: "250m", limitCPU: "1", requestMemory: "128Mi", limitMemory: "256Mi",
		}),
		fleetPod("shared", "demo-failed", "Failed", true, fleetResources{
			requestCPU: "8", requestMemory: "8Gi",
		}),
		fleetPod("other", "demo-abc123", "Running", true, fleetResources{
			requestCPU: "8", requestMemory: "8Gi",
		}),
	}
	usageByKey := map[string]podContainerUsage{
		"shared/demo-abc123":    {CPUCores: float64Pointer(0.2), MemoryMiB: float64Pointer(100)},
		"shared/unrelated-name": {CPUCores: float64Pointer(0.3), MemoryMiB: float64Pointer(50)},
	}

	snapshot := aggregateServiceRuntime(service, pods, usageByKey)
	if snapshot.PodCount != 2 || snapshot.ReadyPodCount != 2 {
		t.Fatalf("expected two ready matching pods, got %#v", snapshot)
	}
	assertFloatPointer(t, "cpu request", snapshot.CPURequestCores, 0.5)
	assertFloatPointer(t, "cpu limit", snapshot.CPULimitCores, 1.5)
	assertFloatPointer(t, "memory request", snapshot.MemoryRequestMiB, 384)
	assertFloatPointer(t, "memory limit", snapshot.MemoryLimitMiB, 768)
	assertFloatPointer(t, "cpu usage", snapshot.CPUUsageCores, 0.5)
	assertFloatPointer(t, "memory usage", snapshot.MemoryUsageMiB, 150)
	if snapshot.CPURequestUtilization == nil || *snapshot.CPURequestUtilization != 100 {
		t.Fatalf("expected cpu request utilization 100, got %v", snapshot.CPURequestUtilization)
	}
}

func TestFleetPodMatchingAndStatusHelpers(t *testing.T) {
	t.Parallel()

	service := admin.RuntimeServiceRef{Namespace: "shared", Name: "demo"}
	if !podMatchesService(fleetPod(" shared ", "demo-abc123", "Running", true, fleetResources{}), service) {
		t.Fatal("expected deployment-style pod name to match service")
	}
	if !podMatchesService(fleetPodWithLabels("shared", "custom-pod", "Running", true, map[string]string{"app.kubernetes.io/name": "demo"}, fleetResources{}), service) {
		t.Fatal("expected app label to match service")
	}
	if podMatchesService(fleetPod("other", "demo-abc123", "Running", true, fleetResources{}), service) {
		t.Fatal("expected namespace mismatch not to match")
	}
	if !isTerminalPod("Succeeded") || !isTerminalPod("Failed") || isTerminalPod("Running") {
		t.Fatal("unexpected terminal pod phase mapping")
	}
	if isPodReady(fleetPod("shared", "demo-pending", "Pending", true, fleetResources{})) {
		t.Fatal("expected non-running pod not to be ready")
	}
	if isPodReady(fleetPod("shared", "demo-not-ready", "Running", false, fleetResources{})) {
		t.Fatal("expected not-ready container to make pod not ready")
	}
	if podKey(" shared ", " demo ") != "shared/demo" {
		t.Fatalf("unexpected pod key")
	}
}

func TestErrorsAsStatus(t *testing.T) {
	t.Parallel()

	var target apiRequestError
	if errorsAsStatus(nil, &target, http.StatusNotFound) {
		t.Fatal("nil error should not match")
	}
	if errorsAsStatus(errors.New("plain"), &target, http.StatusNotFound) {
		t.Fatal("plain error should not match api request status")
	}
	if !errorsAsStatus(apiRequestError{StatusCode: http.StatusNotFound}, &target, http.StatusNotFound) {
		t.Fatal("expected api status to match")
	}
	if errorsAsStatus(apiRequestError{StatusCode: http.StatusInternalServerError}, &target, http.StatusNotFound) {
		t.Fatal("expected different api status not to match")
	}
}

type fleetResources struct {
	requestCPU    string
	limitCPU      string
	requestMemory string
	limitMemory   string
}

func fleetNode(cpu string, memory string) nodeResponse {
	var node nodeResponse
	node.Status.Allocatable = map[string]string{"cpu": cpu, "memory": memory}
	return node
}

func fleetPod(namespace string, name string, phase string, ready bool, resources fleetResources) podResponse {
	return fleetPodWithLabels(namespace, name, phase, ready, nil, resources)
}

func fleetPodWithLabels(namespace string, name string, phase string, ready bool, labels map[string]string, resources fleetResources) podResponse {
	var pod podResponse
	pod.Metadata.Namespace = namespace
	pod.Metadata.Name = name
	pod.Metadata.Labels = labels
	pod.Status.Phase = phase
	pod.Status.ContainerStatuses = []podContainerStatus{{Name: "app", Ready: ready}}
	pod.Spec.Containers = []podSpecContainer{
		{
			Name: "app",
			Resources: podContainerResources{
				Requests: map[string]string{
					"cpu":    resources.requestCPU,
					"memory": resources.requestMemory,
				},
				Limits: map[string]string{
					"cpu":    resources.limitCPU,
					"memory": resources.limitMemory,
				},
			},
		},
	}
	return pod
}

func assertFloatPointer(t *testing.T, label string, got *float64, want float64) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s: expected %f, got nil", label, want)
	}
	if *got != want {
		t.Fatalf("%s: got %f want %f", label, *got, want)
	}
}
