package project

import "testing"

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
