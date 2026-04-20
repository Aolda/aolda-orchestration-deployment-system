package application

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalManifestStorePatchApplicationSwitchesToLoadBalancerWithoutMesh(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := LocalManifestStore{RepoRoot: repoRoot}
	projectContext := ProjectContext{
		ID:           "shared",
		Namespace:    "shared",
		Environments: []string{"shared"},
		Policies: projectPolicy{
			MinReplicas:                 1,
			AllowedEnvironments:         []string{"shared"},
			AllowedDeploymentStrategies: []string{"Rollout", "Canary"},
			AllowedClusterTargets:       []string{"default"},
			RequiredProbes:              true,
		},
	}

	_, err := store.CreateApplication(context.Background(), projectContext, CreateRequest{
		Name:                "demo",
		Image:               "ghcr.io/aolda/demo:v1",
		ServicePort:         8080,
		DeploymentStrategy:  DeploymentStrategyRollout,
		Environment:         "shared",
		MeshEnabled:         true,
		LoadBalancerEnabled: false,
	}, "")
	if err != nil {
		t.Fatalf("create application: %v", err)
	}

	meshEnabled := false
	loadBalancerEnabled := true
	_, err = store.PatchApplication(context.Background(), projectContext, "shared__demo", UpdateApplicationRequest{
		MeshEnabled:         &meshEnabled,
		LoadBalancerEnabled: &loadBalancerEnabled,
	})
	if err != nil {
		t.Fatalf("patch application: %v", err)
	}

	baseDir := filepath.Join(repoRoot, "apps", "shared", "demo", "base")
	kustomizationData, err := os.ReadFile(filepath.Join(baseDir, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read kustomization: %v", err)
	}
	kustomization := string(kustomizationData)
	if strings.Contains(kustomization, "virtualservice.yaml") {
		t.Fatalf("expected mesh resources to be removed from kustomization: %s", kustomization)
	}
	if strings.Contains(kustomization, "destinationrule.yaml") {
		t.Fatalf("expected destinationrule to be removed from kustomization: %s", kustomization)
	}

	if _, err := os.Stat(filepath.Join(baseDir, "virtualservice.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected virtualservice.yaml to be removed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "destinationrule.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected destinationrule.yaml to be removed, got %v", err)
	}

	serviceData, err := os.ReadFile(filepath.Join(baseDir, "service.yaml"))
	if err != nil {
		t.Fatalf("read service manifest: %v", err)
	}
	serviceManifest := string(serviceData)
	if !strings.Contains(serviceManifest, "type: LoadBalancer") {
		t.Fatalf("expected service to request LoadBalancer exposure: %s", serviceManifest)
	}
	if strings.Contains(serviceManifest, "envoy-metrics") {
		t.Fatalf("expected service to omit envoy metrics port when mesh is disabled: %s", serviceManifest)
	}

	deploymentData, err := os.ReadFile(filepath.Join(baseDir, "deployment.yaml"))
	if err != nil {
		t.Fatalf("read deployment manifest: %v", err)
	}
	deployment := string(deploymentData)
	if !strings.Contains(deployment, `aods.io/mesh-enabled: "false"`) {
		t.Fatalf("expected deployment annotations to record mesh=false: %s", deployment)
	}
	if !strings.Contains(deployment, `aods.io/loadbalancer-enabled: "true"`) {
		t.Fatalf("expected deployment annotations to record loadBalancer=true: %s", deployment)
	}
	if strings.Contains(deployment, `sidecar.istio.io/inject: "true"`) {
		t.Fatalf("expected deployment to omit istio sidecar injection when mesh is disabled: %s", deployment)
	}
}
