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
		ProjectID: "project-a",
		Name:      "my-app",
		Namespace: "project-a",
	})
	if !ok {
		t.Fatal("expected matching kustomization")
	}
	if got := normalizeFluxPath(item.Spec.Path); got != "apps/project-a/my-app/overlays/prod" {
		t.Fatalf("unexpected normalized path %q", got)
	}
}

func TestKubeUserResolveClientCertificateFromData(t *testing.T) {
	t.Parallel()

	certPEM, keyPEM := generateClientCertificatePEM(t)
	user := kubeUser{
		ClientCertificateData: base64.StdEncoding.EncodeToString(certPEM),
		ClientKeyData:         base64.StdEncoding.EncodeToString(keyPEM),
	}

	certificate, err := user.resolveClientCertificate()
	if err != nil {
		t.Fatalf("resolve client certificate: %v", err)
	}
	if certificate == nil {
		t.Fatal("expected client certificate")
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
