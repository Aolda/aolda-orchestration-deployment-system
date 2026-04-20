package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRunRequiresOutputDir(t *testing.T) {
	err := run(nil)
	if !errors.Is(err, errUsage) {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestRunGeneratesFixtureRepository(t *testing.T) {
	outputDir := t.TempDir()

	if err := run([]string{outputDir}); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	expectedFiles := []string{
		filepath.Join(outputDir, "platform", "projects.yaml"),
		filepath.Join(outputDir, "apps", "project-a", "rollout-app", "base", "deployment.yaml"),
		filepath.Join(outputDir, "apps", "project-a", "canary-app", "base", "rollout.yaml"),
		filepath.Join(outputDir, "platform", "flux", "clusters", "default", "applications", "project-a-rollout-app.yaml"),
		filepath.Join(outputDir, "platform", "flux", "clusters", "edge", "applications", "project-a-canary-app.yaml"),
	}

	for _, path := range expectedFiles {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated file %s: %v", path, err)
		}
	}
}
