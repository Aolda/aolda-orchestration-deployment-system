package project

import "testing"

func TestApplyEnvironmentDefaultsReturnsSharedWhenCatalogIsEmpty(t *testing.T) {
	t.Parallel()

	items := applyEnvironmentDefaults(nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 environment, got %d", len(items))
	}
	if items[0].ID != "shared" {
		t.Fatalf("expected shared environment, got %s", items[0].ID)
	}
	if items[0].Name != "Shared" {
		t.Fatalf("expected Shared name, got %s", items[0].Name)
	}
	if items[0].ClusterID != "default" {
		t.Fatalf("expected default cluster, got %s", items[0].ClusterID)
	}
	if items[0].WriteMode != WriteModeDirect {
		t.Fatalf("expected direct write mode, got %s", items[0].WriteMode)
	}
	if !items[0].Default {
		t.Fatal("expected synthesized shared environment to be default")
	}
}

func TestApplyEnvironmentDefaultsKeepsSingleExplicitDefault(t *testing.T) {
	t.Parallel()

	items := applyEnvironmentDefaults([]Environment{
		{
			ID:        "dev",
			Name:      "Development",
			ClusterID: "default",
			WriteMode: WriteModeDirect,
		},
		{
			ID:        "prod",
			Name:      "Production",
			ClusterID: "default",
			WriteMode: WriteModePullRequest,
			Default:   true,
		},
	})

	if len(items) != 2 {
		t.Fatalf("expected 2 environments, got %d", len(items))
	}
	if items[0].Default {
		t.Fatal("expected first environment not to become default when an explicit default exists")
	}
	if !items[1].Default {
		t.Fatal("expected explicit default environment to remain default")
	}
}

func TestApplyPolicyDefaultsDerivesEnvironmentAndClusterTargets(t *testing.T) {
	t.Parallel()

	items := applyPolicyDefaults(PolicySet{}, []Environment{
		{
			ID:        "dev",
			Name:      "Development",
			ClusterID: "default",
			WriteMode: WriteModeDirect,
		},
		{
			ID:        "prod",
			Name:      "Production",
			ClusterID: "analytics",
			WriteMode: WriteModePullRequest,
			Default:   true,
		},
	})

	if items.MinReplicas != 1 {
		t.Fatalf("expected default min replicas 1, got %d", items.MinReplicas)
	}
	if len(items.AllowedEnvironments) != 2 || items.AllowedEnvironments[0] != "dev" || items.AllowedEnvironments[1] != "prod" {
		t.Fatalf("expected derived environments [dev prod], got %#v", items.AllowedEnvironments)
	}
	if len(items.AllowedDeploymentStrategies) != 2 || items.AllowedDeploymentStrategies[0] != "Rollout" || items.AllowedDeploymentStrategies[1] != "Canary" {
		t.Fatalf("expected default deployment strategies, got %#v", items.AllowedDeploymentStrategies)
	}
	if len(items.AllowedClusterTargets) != 2 || items.AllowedClusterTargets[0] != "default" || items.AllowedClusterTargets[1] != "analytics" {
		t.Fatalf("expected unique cluster targets [default analytics], got %#v", items.AllowedClusterTargets)
	}
	if !items.RequiredProbes {
		t.Fatal("expected required probes to default to true")
	}
}

func TestApplyPolicyDefaultsPreservesExplicitDisabledProbes(t *testing.T) {
	t.Parallel()

	items := applyPolicyDefaults(PolicySet{
		MinReplicas:                 2,
		AllowedEnvironments:         []string{"prod"},
		AllowedDeploymentStrategies: []string{"Standard"},
		AllowedClusterTargets:       []string{"default"},
		RequiredProbes:              false,
	}, []Environment{
		{
			ID:        "prod",
			Name:      "Production",
			ClusterID: "default",
			WriteMode: WriteModeDirect,
			Default:   true,
		},
	})

	if items.RequiredProbes {
		t.Fatal("expected explicit requiredProbes=false to be preserved")
	}
	if len(items.AllowedDeploymentStrategies) != 1 || items.AllowedDeploymentStrategies[0] != "Rollout" {
		t.Fatalf("expected Standard to normalize to Rollout, got %#v", items.AllowedDeploymentStrategies)
	}
}
