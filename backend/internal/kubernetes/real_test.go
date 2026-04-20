package kubernetes

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/core"
)

func TestFluxSyncStatusReaderTokenMode(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("expected bearer token auth, got %q", got)
		}
		if got := r.URL.Path; got != "/apis/kustomize.toolkit.fluxcd.io/v1/namespaces/flux-system/kustomizations" {
			t.Fatalf("unexpected request path %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{
					"metadata": {"name": "project-a-my-app", "namespace": "flux-system"},
					"spec": {"path": "./apps/project-a/my-app/overlays/shared", "targetNamespace": "project-a"},
					"status": {
						"conditions": [
							{
								"type": "Ready",
								"status": "True",
								"reason": "ReconciliationSucceeded",
								"message": "Applied revision: main@sha1:abc123",
								"lastTransitionTime": "2026-04-02T01:00:00Z"
							}
						]
					}
				}
			]
		}`))
	}))
	defer server.Close()

	reader, err := NewFluxSyncStatusReader(core.Config{
		KubernetesMode:             "token",
		KubernetesAPIURL:           server.URL,
		KubernetesBearerToken:      "test-token",
		KubernetesRequestTimeout:   2 * time.Second,
		FluxKustomizationNamespace: "flux-system",
	})
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}

	info, err := reader.Read(context.Background(), application.Record{
		ProjectID:          "project-a",
		Name:               "my-app",
		Namespace:          "project-a",
		DefaultEnvironment: "shared",
	})
	if err != nil {
		t.Fatalf("read sync status: %v", err)
	}

	if info.Status != application.SyncStatusSynced {
		t.Fatalf("expected Synced, got %s", info.Status)
	}
	if info.Message != "Applied revision: main@sha1:abc123" {
		t.Fatalf("unexpected message %q", info.Message)
	}
}

func TestFluxSyncStatusReaderKubeconfigExecMode(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer exec-token" {
			t.Fatalf("expected exec token auth, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	execScriptPath := filepath.Join(tempDir, "token.sh")
	script := "#!/bin/sh\nprintf '%s' '{\"status\":{\"token\":\"exec-token\"}}'\n"
	if err := os.WriteFile(execScriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write exec script: %v", err)
	}

	kubeconfigPath := filepath.Join(tempDir, "config.yaml")
	kubeconfig := "apiVersion: v1\n" +
		"kind: Config\n" +
		"current-context: test-context\n" +
		"contexts:\n" +
		"- name: test-context\n" +
		"  context:\n" +
		"    cluster: test-cluster\n" +
		"    user: test-user\n" +
		"clusters:\n" +
		"- name: test-cluster\n" +
		"  cluster:\n" +
		"    server: " + server.URL + "\n" +
		"users:\n" +
		"- name: test-user\n" +
		"  user:\n" +
		"    exec:\n" +
		"      command: " + execScriptPath + "\n"
	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0o644); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	reader, err := NewFluxSyncStatusReader(core.Config{
		KubernetesMode:             "kubeconfig",
		KubernetesKubeconfigPath:   kubeconfigPath,
		KubernetesRequestTimeout:   2 * time.Second,
		FluxKustomizationNamespace: "flux-system",
	})
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}

	info, err := reader.Read(context.Background(), application.Record{
		ProjectID: "project-a",
		Name:      "missing-app",
		Namespace: "project-a",
	})
	if err != nil {
		t.Fatalf("read sync status: %v", err)
	}

	if info.Status != application.SyncStatusUnknown {
		t.Fatalf("expected Unknown, got %s", info.Status)
	}
}

func TestSelectKustomizationMatchesDesiredPath(t *testing.T) {
	t.Parallel()

	item, ok := selectKustomization([]fluxKustomization{
		{
			Spec: struct {
				Path            string `json:"path"`
				TargetNamespace string `json:"targetNamespace"`
			}{
				Path:            "/apps/project-a/my-app/overlays/prod",
				TargetNamespace: "project-a",
			},
		},
	}, application.Record{
		ProjectID:          "project-a",
		Name:               "my-app",
		Namespace:          "project-a",
		DefaultEnvironment: "prod",
	})
	if !ok {
		t.Fatal("expected matching kustomization")
	}
	if got := normalizeFluxPath(item.Spec.Path); got != "apps/project-a/my-app/overlays/prod" {
		t.Fatalf("unexpected normalized path %q", got)
	}
}

func TestSelectKustomizationMatchesDefaultEnvironmentPath(t *testing.T) {
	t.Parallel()

	item, ok := selectKustomization([]fluxKustomization{
		{
			Spec: struct {
				Path            string `json:"path"`
				TargetNamespace string `json:"targetNamespace"`
			}{
				Path:            "./apps/project-a/my-app/overlays/dev",
				TargetNamespace: "project-a",
			},
		},
	}, application.Record{
		ProjectID:          "project-a",
		Name:               "my-app",
		Namespace:          "project-a",
		DefaultEnvironment: "dev",
	})
	if !ok {
		t.Fatal("expected matching kustomization")
	}
	if got := normalizeFluxPath(item.Spec.Path); got != "apps/project-a/my-app/overlays/dev" {
		t.Fatalf("unexpected normalized path %q", got)
	}
}

func TestPodMetricsReaderTokenModeProvidesCPUAndMemoryFallback(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer metrics-token" {
			t.Fatalf("expected bearer token auth, got %q", got)
		}
		if got := r.URL.Path; got != "/apis/metrics.k8s.io/v1beta1/namespaces/project-a/pods" {
			t.Fatalf("unexpected request path %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{
					"metadata": {"name": "my-app-7cc4d5f789-abcde"},
					"containers": [
						{"usage": {"cpu": "250m", "memory": "64Mi"}},
						{"usage": {"cpu": "125m", "memory": "16Mi"}}
					]
				},
				{
					"metadata": {"name": "other-app-7cc4d5f789-abcde"},
					"containers": [
						{"usage": {"cpu": "100m", "memory": "32Mi"}}
					]
				}
			]
		}`))
	}))
	defer server.Close()

	reader, err := NewPodMetricsReader(core.Config{
		KubernetesMode:           "token",
		KubernetesAPIURL:         server.URL,
		KubernetesBearerToken:    "metrics-token",
		KubernetesRequestTimeout: 2 * time.Second,
		PrometheusRange:          10 * time.Minute,
		PrometheusStep:           5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("new pod metrics reader: %v", err)
	}
	reader.Now = func() time.Time {
		return time.Date(2026, 4, 3, 1, 0, 0, 0, time.UTC)
	}

	metrics, err := reader.Read(context.Background(), application.Record{
		Name:      "my-app",
		Namespace: "project-a",
	}, 10*time.Minute, 5*time.Minute)
	if err != nil {
		t.Fatalf("read pod metrics: %v", err)
	}

	if len(metrics) != 5 {
		t.Fatalf("expected 5 metric series, got %d", len(metrics))
	}
	if metrics[0].Points[len(metrics[0].Points)-1].Value != nil {
		t.Fatal("expected request_rate to stay empty without prometheus data")
	}
	cpuValue := metrics[3].Points[len(metrics[3].Points)-1].Value
	if cpuValue == nil || *cpuValue != 0.375 {
		t.Fatalf("expected cpu_usage fallback to be 0.375, got %#v", cpuValue)
	}
	memoryValue := metrics[4].Points[len(metrics[4].Points)-1].Value
	if memoryValue == nil || *memoryValue != 80 {
		t.Fatalf("expected memory_usage fallback to be 80MiB, got %#v", memoryValue)
	}
}

func TestServiceNetworkExposureReaderReadyWhenIngressIsAssigned(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer exposure-token" {
			t.Fatalf("expected bearer token auth, got %q", got)
		}

		switch r.URL.Path {
		case "/api/v1/namespaces/shared/services/moltbot-front-poc-web":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"spec": {
					"type": "LoadBalancer",
					"ports": [
						{"name": "http", "protocol": "TCP", "port": 80, "targetPort": 3000, "nodePort": 32080}
					]
				},
				"status": {
					"loadBalancer": {
						"ingress": [{"ip": "172.31.0.238"}]
					}
				}
			}`))
		case "/api/v1/namespaces/shared/events":
			if got := r.URL.Query().Get("fieldSelector"); got != "involvedObject.kind=Service,involvedObject.name=moltbot-front-poc-web" {
				t.Fatalf("unexpected fieldSelector %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"items": [
					{
						"type": "Normal",
						"reason": "UpdatedLoadBalancer",
						"message": "Updated LoadBalancer with new IPs: [172.31.0.238]",
						"lastTimestamp": "2026-04-18T09:00:00Z",
						"metadata": {"creationTimestamp": "2026-04-18T08:59:00Z"}
					}
				]
			}`))
		default:
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	reader, err := NewServiceNetworkExposureReader(core.Config{
		KubernetesMode:           "token",
		KubernetesAPIURL:         server.URL,
		KubernetesBearerToken:    "exposure-token",
		KubernetesRequestTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new network exposure reader: %v", err)
	}

	info, err := reader.Read(context.Background(), application.Record{
		Name:                "moltbot-front-poc-web",
		Namespace:           "shared",
		LoadBalancerEnabled: true,
	})
	if err != nil {
		t.Fatalf("read network exposure: %v", err)
	}

	if info.Status != application.NetworkExposureStatusReady {
		t.Fatalf("expected Ready, got %s", info.Status)
	}
	if info.ServiceType != "LoadBalancer" {
		t.Fatalf("expected LoadBalancer service type, got %q", info.ServiceType)
	}
	if len(info.Addresses) != 1 || info.Addresses[0] != "172.31.0.238" {
		t.Fatalf("unexpected ingress addresses %#v", info.Addresses)
	}
	if len(info.Ports) != 1 {
		t.Fatalf("expected one service port, got %#v", info.Ports)
	}
	if info.Ports[0].Port != 80 || info.Ports[0].TargetPort != "3000" || info.Ports[0].NodePort != 32080 {
		t.Fatalf("unexpected service port mapping %#v", info.Ports[0])
	}
	if info.LastEvent == nil || info.LastEvent.Reason != "UpdatedLoadBalancer" {
		t.Fatalf("expected latest service event to be returned, got %#v", info.LastEvent)
	}
}

func TestServiceNetworkExposureReaderReturnsProvisioningWithoutIngress(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/namespaces/shared/services/moltbot-front-poc-web":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"spec": {"type": "LoadBalancer"},
				"status": {
					"loadBalancer": {
						"ingress": []
					}
				}
			}`))
		case "/api/v1/namespaces/shared/events":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"items": [
					{
						"type": "Normal",
						"reason": "EnsuringLoadBalancer",
						"message": "Ensuring load balancer",
						"lastTimestamp": "2026-04-18T09:05:00Z"
					}
				]
			}`))
		default:
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	reader, err := NewServiceNetworkExposureReader(core.Config{
		KubernetesMode:           "token",
		KubernetesAPIURL:         server.URL,
		KubernetesBearerToken:    "exposure-token",
		KubernetesRequestTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new network exposure reader: %v", err)
	}

	info, err := reader.Read(context.Background(), application.Record{
		Name:                "moltbot-front-poc-web",
		Namespace:           "shared",
		LoadBalancerEnabled: true,
	})
	if err != nil {
		t.Fatalf("read network exposure: %v", err)
	}

	if info.Status != application.NetworkExposureStatusProvisioning {
		t.Fatalf("expected Provisioning, got %s", info.Status)
	}
	if info.LastEvent == nil || info.LastEvent.Reason != "EnsuringLoadBalancer" {
		t.Fatalf("expected provisioning event, got %#v", info.LastEvent)
	}
}

func TestServiceNetworkExposureReaderReturnsErrorOnWarningEvent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/namespaces/shared/services/moltbot-front-poc-web":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"spec": {"type": "LoadBalancer"},
				"status": {
					"loadBalancer": {
						"ingress": []
					}
				}
			}`))
		case "/api/v1/namespaces/shared/events":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"items": [
					{
						"type": "Warning",
						"reason": "SyncLoadBalancerFailed",
						"message": "failed to ensure external LB",
						"lastTimestamp": "2026-04-18T09:10:00Z"
					}
				]
			}`))
		default:
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	reader, err := NewServiceNetworkExposureReader(core.Config{
		KubernetesMode:           "token",
		KubernetesAPIURL:         server.URL,
		KubernetesBearerToken:    "exposure-token",
		KubernetesRequestTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new network exposure reader: %v", err)
	}

	info, err := reader.Read(context.Background(), application.Record{
		Name:                "moltbot-front-poc-web",
		Namespace:           "shared",
		LoadBalancerEnabled: true,
	})
	if err != nil {
		t.Fatalf("read network exposure: %v", err)
	}

	if info.Status != application.NetworkExposureStatusError {
		t.Fatalf("expected Error, got %s", info.Status)
	}
	if !strings.Contains(info.Message, "failed to ensure external LB") {
		t.Fatalf("expected warning message to surface, got %q", info.Message)
	}
}

func TestPodLogReaderTokenModeReturnsLatestPrimaryContainerLogs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer logs-token" {
			t.Fatalf("expected bearer token auth, got %q", got)
		}
		if got := r.Header.Get("Accept"); got == "text/plain" {
			t.Fatalf("unexpected Accept header %q", got)
		}

		switch {
		case r.URL.Path == "/api/v1/namespaces/shared/pods":
			if got := r.URL.Query().Get("labelSelector"); got != "app.kubernetes.io/name=moltbot-front-poc-web" {
				t.Fatalf("unexpected label selector %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"items": [
					{
						"metadata": {"name": "moltbot-front-poc-web-7cc4d5f789-newer", "creationTimestamp": "2026-04-17T10:00:00Z"},
						"spec": {"containers": [{"name": "moltbot-front-poc-web"}, {"name": "istio-proxy"}]},
						"status": {
							"phase": "Running",
							"containerStatuses": [
								{"name": "moltbot-front-poc-web", "ready": true, "restartCount": 1},
								{"name": "istio-proxy", "ready": true, "restartCount": 0}
							]
						}
					},
					{
						"metadata": {"name": "moltbot-front-poc-web-7cc4d5f789-older", "creationTimestamp": "2026-04-17T09:00:00Z"},
						"spec": {"containers": [{"name": "moltbot-front-poc-web"}]},
						"status": {
							"phase": "Running",
							"containerStatuses": [
								{"name": "moltbot-front-poc-web", "ready": false, "restartCount": 3}
							]
						}
					}
				]
			}`))
		case r.URL.Path == "/api/v1/namespaces/shared/pods/moltbot-front-poc-web-7cc4d5f789-newer/log":
			if got := r.URL.Query().Get("container"); got != "moltbot-front-poc-web" {
				t.Fatalf("unexpected container query %q", got)
			}
			if got := r.URL.Query().Get("tailLines"); got != "120" {
				t.Fatalf("unexpected tailLines %q", got)
			}
			_, _ = w.Write([]byte("2026-04-17T10:12:00Z server started\n2026-04-17T10:12:03Z ready"))
		case r.URL.Path == "/api/v1/namespaces/shared/pods/moltbot-front-poc-web-7cc4d5f789-older/log":
			_, _ = w.Write([]byte("2026-04-17T09:10:00Z booting"))
		default:
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	reader, err := NewPodLogReader(core.Config{
		KubernetesMode:           "token",
		KubernetesAPIURL:         server.URL,
		KubernetesBearerToken:    "logs-token",
		KubernetesRequestTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new pod log reader: %v", err)
	}

	logs, err := reader.Read(context.Background(), application.Record{
		Name:      "moltbot-front-poc-web",
		Namespace: "shared",
	}, 120)
	if err != nil {
		t.Fatalf("read pod logs: %v", err)
	}

	if len(logs) != 2 {
		t.Fatalf("expected 2 log streams, got %d", len(logs))
	}
	if logs[0].PodName != "moltbot-front-poc-web-7cc4d5f789-newer" {
		t.Fatalf("expected newest pod first, got %q", logs[0].PodName)
	}
	if logs[0].ContainerName != "moltbot-front-poc-web" {
		t.Fatalf("expected primary container logs, got %q", logs[0].ContainerName)
	}
	if !logs[0].Ready || logs[0].RestartCount != 1 {
		t.Fatalf("unexpected primary container status: %#v", logs[0])
	}
	if !strings.Contains(logs[0].Content, "server started") {
		t.Fatalf("expected log content, got %q", logs[0].Content)
	}
}

func TestPodLogReaderListTargetsIncludesResourceStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/namespaces/shared/pods":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"items": [
					{
						"metadata": {"name": "moltbot-front-poc-web-7cc4d5f789-newer", "creationTimestamp": "2026-04-17T10:00:00Z"},
						"spec": {
							"containers": [
								{
									"name": "moltbot-front-poc-web",
									"resources": {
										"requests": {"cpu": "500m", "memory": "256Mi"},
										"limits": {"cpu": "1000m", "memory": "512Mi"}
									}
								},
								{
									"name": "istio-proxy",
									"resources": {
										"requests": {"cpu": "100m", "memory": "64Mi"},
										"limits": {"cpu": "200m", "memory": "128Mi"}
									}
								}
							]
						},
						"status": {
							"phase": "Running",
							"containerStatuses": [
								{"name": "moltbot-front-poc-web", "ready": true, "restartCount": 1},
								{"name": "istio-proxy", "ready": true, "restartCount": 0}
							]
						}
					}
				]
			}`))
		case "/apis/metrics.k8s.io/v1beta1/namespaces/shared/pods":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"items": [
					{
						"metadata": {"name": "moltbot-front-poc-web-7cc4d5f789-newer"},
						"containers": [
							{"name": "moltbot-front-poc-web", "usage": {"cpu": "125m", "memory": "128Mi"}},
							{"name": "istio-proxy", "usage": {"cpu": "20m", "memory": "48Mi"}}
						]
					}
				]
			}`))
		default:
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	reader, err := NewPodLogReader(core.Config{
		KubernetesMode:           "token",
		KubernetesAPIURL:         server.URL,
		KubernetesBearerToken:    "logs-token",
		KubernetesRequestTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new pod log reader: %v", err)
	}

	targets, err := reader.ListTargets(context.Background(), application.Record{
		Name:      "moltbot-front-poc-web",
		Namespace: "shared",
	})
	if err != nil {
		t.Fatalf("list targets: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 pod target, got %d", len(targets))
	}
	if len(targets[0].Containers) != 2 {
		t.Fatalf("expected 2 container targets, got %d", len(targets[0].Containers))
	}
	if !targets[0].Containers[0].Default {
		t.Fatal("expected first app container to be default")
	}
	resourceStatus := targets[0].Containers[0].ResourceStatus
	if resourceStatus == nil {
		t.Fatal("expected resource status for primary container")
	}
	if resourceStatus.CPUUsageCores == nil || *resourceStatus.CPUUsageCores != 0.125 {
		t.Fatalf("unexpected cpu usage %#v", resourceStatus.CPUUsageCores)
	}
	if resourceStatus.CPURequestUtilization == nil || int(*resourceStatus.CPURequestUtilization) != 25 {
		t.Fatalf("unexpected cpu request utilization %#v", resourceStatus.CPURequestUtilization)
	}
	if resourceStatus.MemoryLimitUtilization == nil || int(*resourceStatus.MemoryLimitUtilization) != 25 {
		t.Fatalf("unexpected memory limit utilization %#v", resourceStatus.MemoryLimitUtilization)
	}
}

func TestPodLogReaderStreamsSelectedContainerLogs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/shared/pods/moltbot-front-poc-web-7cc4d5f789-newer/log" {
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("follow"); got != "true" {
			t.Fatalf("expected follow=true, got %q", got)
		}
		w.Header().Set("Content-Type", "text/plain")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("2026-04-17T10:12:00Z server started\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("2026-04-17T10:12:03Z ready\n"))
		flusher.Flush()
	}))
	defer server.Close()

	reader, err := NewPodLogReader(core.Config{
		KubernetesMode:           "token",
		KubernetesAPIURL:         server.URL,
		KubernetesBearerToken:    "logs-token",
		KubernetesRequestTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new pod log reader: %v", err)
	}

	var events []application.ContainerLogEvent
	err = reader.Stream(
		context.Background(),
		application.Record{Name: "moltbot-front-poc-web", Namespace: "shared"},
		"moltbot-front-poc-web-7cc4d5f789-newer",
		"moltbot-front-poc-web",
		120,
		func(event application.ContainerLogEvent) error {
			events = append(events, event)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("stream logs: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 streamed events, got %d", len(events))
	}
	if events[0].Timestamp != "2026-04-17T10:12:00Z" {
		t.Fatalf("unexpected first timestamp %q", events[0].Timestamp)
	}
	if events[1].Message != "ready" {
		t.Fatalf("unexpected second message %q", events[1].Message)
	}
}

func TestPodLogReaderStreamIgnoresHTTPClientTimeoutForLongLivedLogs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/shared/pods/moltbot-front-poc-web-7cc4d5f789-newer/log" {
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("2026-04-17T10:12:00Z first line\n"))
		flusher.Flush()
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte("2026-04-17T10:12:03Z second line\n"))
		flusher.Flush()
	}))
	defer server.Close()

	reader, err := NewPodLogReader(core.Config{
		KubernetesMode:           "token",
		KubernetesAPIURL:         server.URL,
		KubernetesBearerToken:    "logs-token",
		KubernetesRequestTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new pod log reader: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var events []application.ContainerLogEvent
	err = reader.Stream(
		ctx,
		application.Record{Name: "moltbot-front-poc-web", Namespace: "shared"},
		"moltbot-front-poc-web-7cc4d5f789-newer",
		"moltbot-front-poc-web",
		120,
		func(event application.ContainerLogEvent) error {
			events = append(events, event)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("stream logs: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 streamed events, got %d", len(events))
	}
	if events[1].Message != "second line" {
		t.Fatalf("unexpected second message %q", events[1].Message)
	}
}

func TestKubeUserResolveClientCertificateFromData(t *testing.T) {
	t.Parallel()

	certPEM, keyPEM := generateClientCertificatePEM(t)
	user := kubeUser{
		ClientCertificateData: base64.StdEncoding.EncodeToString(certPEM),
		ClientKeyData:         base64.StdEncoding.EncodeToString(keyPEM),
	}

	certificate, err := user.resolveClientCertificate("")
	if err != nil {
		t.Fatalf("resolve client certificate: %v", err)
	}
	if certificate == nil {
		t.Fatal("expected client certificate")
	}
}

func TestFluxSyncStatusReaderKubeconfigResolvesRelativeTokenFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer relative-token" {
			t.Fatalf("expected bearer token from relative token-file, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	otherDir := t.TempDir()
	if err := os.Chdir(otherDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "token.txt"), []byte("relative-token\n"), 0o644); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	kubeconfigPath := filepath.Join(configDir, "config.yaml")
	kubeconfig := strings.Join([]string{
		"apiVersion: v1",
		"kind: Config",
		"current-context: test-context",
		"contexts:",
		"- name: test-context",
		"  context:",
		"    cluster: test-cluster",
		"    user: test-user",
		"clusters:",
		"- name: test-cluster",
		"  cluster:",
		"    server: " + server.URL,
		"users:",
		"- name: test-user",
		"  user:",
		"    token-file: token.txt",
	}, "\n") + "\n"
	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0o644); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	reader, err := NewFluxSyncStatusReader(core.Config{
		KubernetesMode:             "kubeconfig",
		KubernetesKubeconfigPath:   kubeconfigPath,
		KubernetesRequestTimeout:   2 * time.Second,
		FluxKustomizationNamespace: "flux-system",
	})
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}

	if _, err := reader.Read(context.Background(), application.Record{
		ID:        "project-a__relative-app",
		ProjectID: "project-a",
		Name:      "relative-app",
		Namespace: "project-a",
	}); err != nil {
		t.Fatalf("read sync status: %v", err)
	}
}

func generateClientCertificatePEM(t *testing.T) ([]byte, []byte) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal EC private key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return certPEM, keyPEM
}
