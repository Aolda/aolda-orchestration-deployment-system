package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigReadsDeploymentOperationSettings(t *testing.T) {
	repoRoot := testRepoRoot(t)
	t.Setenv("AODS_REPO_ROOT", repoRoot)
	t.Setenv("AODS_AUTH_MODE", "oidc")
	t.Setenv("AODS_ALLOWED_ORIGIN", "http://localhost:5173")
	t.Setenv("AODS_ALLOW_DEV_FALLBACK", "false")
	t.Setenv("AODS_PLATFORM_ADMIN_AUTHORITIES", "aods:platform:admin, aods:ops:admin, aods:platform:admin")
	t.Setenv("AODS_MARIADB_DSN", "user:pass@tcp(db:3306)/aods")
	t.Setenv("AODS_APPLICATION_CATALOG_DSN", "postgres://aods:secret@postgres:5432/aolda?sslmode=disable")
	t.Setenv("AODS_DEPLOYMENT_OPERATION_INTERVAL", "3s")
	t.Setenv("AODS_DEPLOYMENT_OPERATION_LEASE", "7m")
	t.Setenv("AODS_DEPLOYMENT_OPERATION_MAX_ATTEMPTS", "9")
	t.Setenv("AODS_REPOSITORY_POLL_INTERVAL", "11m")
	t.Setenv("AODS_FLUX_STATUS_CACHE_TTL", "17s")
	t.Setenv("AODS_APPLICATION_CATALOG_CACHE_TTL", "13m")
	t.Setenv("AODS_APPLICATION_CATALOG_SYNC_INTERVAL", "19s")
	t.Setenv("AODS_DEV_GROUPS", "aods:shared:deploy,aods:platform:admin")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.RepoRoot != repoRoot {
		t.Fatalf("expected repo root %q, got %q", repoRoot, cfg.RepoRoot)
	}
	if !cfg.UseOIDCAuth() {
		t.Fatal("expected oidc auth mode")
	}
	if cfg.AllowDevFallback {
		t.Fatal("expected dev fallback to be disabled")
	}
	if !cfg.UseMariaDBOperations() {
		t.Fatal("expected MariaDB operations to be enabled")
	}
	if !cfg.UseApplicationCatalogCache() {
		t.Fatal("expected application catalog cache to be enabled")
	}
	if cfg.ResolvedApplicationCatalogDBDriver() != "postgres" {
		t.Fatalf("expected postgres catalog driver, got %q", cfg.ResolvedApplicationCatalogDBDriver())
	}
	if cfg.DeploymentOperationInterval != 3*time.Second {
		t.Fatalf("unexpected operation interval: %s", cfg.DeploymentOperationInterval)
	}
	if cfg.DeploymentOperationLease != 7*time.Minute {
		t.Fatalf("unexpected operation lease: %s", cfg.DeploymentOperationLease)
	}
	if cfg.DeploymentOperationMaxAttempts != 9 {
		t.Fatalf("unexpected max attempts: %d", cfg.DeploymentOperationMaxAttempts)
	}
	if cfg.RepositoryPollInterval != 11*time.Minute {
		t.Fatalf("unexpected repository poll interval: %s", cfg.RepositoryPollInterval)
	}
	if cfg.FluxStatusCacheTTL != 17*time.Second {
		t.Fatalf("unexpected Flux status cache TTL: %s", cfg.FluxStatusCacheTTL)
	}
	if cfg.ApplicationCatalogCacheTTL != 13*time.Minute {
		t.Fatalf("unexpected application catalog cache TTL: %s", cfg.ApplicationCatalogCacheTTL)
	}
	if cfg.ApplicationCatalogSyncInterval != 19*time.Second {
		t.Fatalf("unexpected application catalog sync interval: %s", cfg.ApplicationCatalogSyncInterval)
	}
	if got := strings.Join(cfg.PlatformAdminAuthorities, ","); got != "aods:platform:admin,aods:ops:admin" {
		t.Fatalf("expected deduped admin authorities, got %q", got)
	}
	if len(cfg.DevUser.Groups) != 2 {
		t.Fatalf("expected dev groups from env, got %#v", cfg.DevUser.Groups)
	}
}

func TestLoadConfigRejectsInvalidDeploymentOperationSettings(t *testing.T) {
	tests := []struct {
		name string
		key  string
		val  string
	}{
		{name: "interval", key: "AODS_DEPLOYMENT_OPERATION_INTERVAL", val: "not-a-duration"},
		{name: "lease", key: "AODS_DEPLOYMENT_OPERATION_LEASE", val: "not-a-duration"},
		{name: "attempts", key: "AODS_DEPLOYMENT_OPERATION_MAX_ATTEMPTS", val: "not-an-int"},
		{name: "flux status cache TTL", key: "AODS_FLUX_STATUS_CACHE_TTL", val: "not-a-duration"},
		{name: "application catalog cache TTL", key: "AODS_APPLICATION_CATALOG_CACHE_TTL", val: "not-a-duration"},
		{name: "application catalog sync interval", key: "AODS_APPLICATION_CATALOG_SYNC_INTERVAL", val: "not-a-duration"},
		{name: "dev fallback", key: "AODS_ALLOW_DEV_FALLBACK", val: "maybe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AODS_REPO_ROOT", testRepoRoot(t))
			t.Setenv(tt.key, tt.val)
			_, err := LoadConfig()
			if err == nil {
				t.Fatalf("expected invalid %s to fail", tt.key)
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("expected error to mention %s, got %v", tt.key, err)
			}
		})
	}
}

func TestConfigModeHelpersAndEnvParsing(t *testing.T) {
	cfg := Config{
		GitMode:                    "git",
		AuthMode:                   " oidc ",
		KubernetesMode:             "kubeconfig",
		ImageVerificationMode:      "anonymous",
		PrometheusMode:             "prometheus",
		VaultMode:                  "token",
		MariaDBDSN:                 "dsn",
		ApplicationCatalogCacheTTL: time.Minute,
	}
	if !cfg.UseGitRepo() || !cfg.UseOIDCAuth() || !cfg.UseKubernetesAPI() || !cfg.UseImageVerification() || !cfg.UsePrometheusAPI() || !cfg.UseVaultAPI() || !cfg.UseMariaDBOperations() || !cfg.UseApplicationCatalogCache() {
		t.Fatalf("expected all configured mode helpers to be true: %#v", cfg)
	}

	cfg = Config{
		KubernetesMode:        "local",
		ImageVerificationMode: "local",
		PrometheusMode:        "local",
		VaultMode:             "local",
	}
	if cfg.UseGitRepo() || cfg.UseOIDCAuth() || cfg.UseKubernetesAPI() || cfg.UseImageVerification() || cfg.UsePrometheusAPI() || cfg.UseVaultAPI() || cfg.UseMariaDBOperations() || cfg.UseApplicationCatalogCache() {
		t.Fatalf("expected local/default mode helpers to be false: %#v", cfg)
	}

	cfg = Config{
		ApplicationCatalogDSN:      "postgres://aods:secret@localhost:5432/aolda?sslmode=disable",
		ApplicationCatalogCacheTTL: time.Minute,
	}
	if !cfg.UseApplicationCatalogCache() {
		t.Fatal("expected postgres application catalog cache to be enabled")
	}
	if cfg.UseMariaDBOperations() {
		t.Fatal("expected postgres application catalog cache not to enable MariaDB operations")
	}
	if cfg.ResolvedApplicationCatalogDBDriver() != "postgres" {
		t.Fatalf("expected postgres driver, got %q", cfg.ResolvedApplicationCatalogDBDriver())
	}

	cfg.ApplicationCatalogDBDriver = "pg"
	if cfg.ResolvedApplicationCatalogDBDriver() != "postgres" {
		t.Fatalf("expected pg alias to resolve to postgres, got %q", cfg.ResolvedApplicationCatalogDBDriver())
	}

	t.Setenv("AODS_TEST_STRING", "value")
	if got := envOrDefault("AODS_TEST_STRING", "fallback"); got != "value" {
		t.Fatalf("expected env value, got %q", got)
	}
	t.Setenv("AODS_TEST_STRING", "")
	if got := envOrDefault("AODS_TEST_STRING", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback value, got %q", got)
	}

	if got, err := envBool("AODS_MISSING_BOOL", true); err != nil || !got {
		t.Fatalf("expected bool fallback true, got %v err=%v", got, err)
	}
	if got, err := envInt("AODS_MISSING_INT", 12); err != nil || got != 12 {
		t.Fatalf("expected int fallback 12, got %d err=%v", got, err)
	}
	if got, err := envDuration("AODS_MISSING_DURATION", 4*time.Second); err != nil || got != 4*time.Second {
		t.Fatalf("expected duration fallback 4s, got %s err=%v", got, err)
	}
}

func TestResolveRepoRootAndFileHelpers(t *testing.T) {
	t.Parallel()

	root := testRepoRoot(t)
	if got, err := resolveRepoRoot(root); err != nil || got != root {
		t.Fatalf("expected explicit repo root, got %q err=%v", got, err)
	}
	if !fileExists(filepath.Join(root, "AGENTS.md")) {
		t.Fatal("expected AGENTS.md to exist")
	}
	if fileExists(filepath.Join(root, "docs")) {
		t.Fatal("expected directory not to count as a file")
	}
	if defaultKubeconfigPath() == "" && os.Getenv("HOME") != "" {
		t.Fatal("expected default kubeconfig path when HOME is available")
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("instructions"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	docsDir := filepath.Join(root, "docs", "internal-platform")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("create docs directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "openapi.yaml"), []byte("openapi: 3.0.3"), 0o644); err != nil {
		t.Fatalf("write openapi.yaml: %v", err)
	}
	return root
}
