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
					"spec": {"path": "./apps/project-a/my-app/overlays/prod", "targetNamespace": "project-a"},
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
		ProjectID: "project-a",
		Name:      "my-app",
		Namespace: "project-a",
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
