package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address                     string
	RepoRoot                    string
	AuthMode                    string
	PlatformAdminAuthorities    []string
	OIDCIssuerURL               string
	OIDCJWKSURL                 string
	OIDCAudience                string
	OIDCUserIDClaim             string
	OIDCUsernameClaim           string
	OIDCDisplayNameClaim        string
	OIDCGroupsClaim             string
	OIDCRoleMappings            map[string][]string
	OIDCRequestTimeout          time.Duration
	GitMode                     string
	GitRepoDir                  string
	GitRemote                   string
	GitBranch                   string
	GitAuthorName               string
	GitAuthorEmail              string
	GitCommandTimeout           time.Duration
	GitSyncTTL                  time.Duration
	RepositoryPollInterval      time.Duration
	KubernetesMode              string
	KubernetesAPIURL            string
	KubernetesBearerToken       string
	KubernetesCAFile            string
	KubernetesCAData            string
	KubernetesKubeconfigPath    string
	KubernetesContext           string
	KubernetesRequestTimeout    time.Duration
	ImageVerificationMode       string
	ImageVerificationTimeout    time.Duration
	FluxKustomizationNamespace  string
	FluxSourceName              string
	PrometheusMode              string
	PrometheusURL               string
	PrometheusRequestTimeout    time.Duration
	PrometheusRange             time.Duration
	PrometheusStep              time.Duration
	VaultMode                   string
	VaultAddress                string
	VaultToken                  string
	VaultNamespace              string
	VaultRequestTimeout         time.Duration
	VaultStagingCleanupInterval time.Duration
	VaultStagingMaxAge          time.Duration
	OrphanFluxCleanupInterval   time.Duration
	AllowedOrigin               string
	AllowDevFallback            bool
	DevUser                     User
	LocalVaultDir               string
}

func LoadConfig() (Config, error) {
	repoRoot, err := resolveRepoRoot(os.Getenv("AODS_REPO_ROOT"))
	if err != nil {
		return Config{}, err
	}

	allowDevFallback, err := envBool("AODS_ALLOW_DEV_FALLBACK", true)
	if err != nil {
		return Config{}, err
	}

	kubernetesRequestTimeout, err := envDuration("AODS_K8S_REQUEST_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	imageVerificationTimeout, err := envDuration("AODS_IMAGE_CHECK_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	oidcRequestTimeout, err := envDuration("AODS_OIDC_REQUEST_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	oidcRoleMappings, err := parseAuthorityMappings(os.Getenv("AODS_OIDC_ROLE_MAPPINGS"))
	if err != nil {
		return Config{}, err
	}

	platformAdminAuthorities := dedupeAuthorityStrings(
		splitCommaSeparated(envOrDefault("AODS_PLATFORM_ADMIN_AUTHORITIES", "aods:platform:admin")),
	)

	gitCommandTimeout, err := envDuration("AODS_GIT_COMMAND_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}

	gitSyncTTL, err := envDuration("AODS_GIT_SYNC_TTL", 3*time.Second)
	if err != nil {
		return Config{}, err
	}

	repositoryPollInterval, err := envDuration("AODS_REPOSITORY_POLL_INTERVAL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}

	prometheusRequestTimeout, err := envDuration("AODS_PROMETHEUS_REQUEST_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	prometheusRange, err := envDuration("AODS_PROMETHEUS_RANGE", time.Hour)
	if err != nil {
		return Config{}, err
	}

	prometheusStep, err := envDuration("AODS_PROMETHEUS_STEP", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}

	vaultRequestTimeout, err := envDuration("AODS_VAULT_REQUEST_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	vaultStagingCleanupInterval, err := envDuration("AODS_VAULT_STAGING_CLEANUP_INTERVAL", time.Hour)
	if err != nil {
		return Config{}, err
	}

	vaultStagingMaxAge, err := envDuration("AODS_VAULT_STAGING_MAX_AGE", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	orphanFluxCleanupInterval, err := envDuration("AODS_ORPHAN_FLUX_CLEANUP_INTERVAL", time.Hour)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Address:                     envOrDefault("AODS_ADDR", ":8080"),
		RepoRoot:                    repoRoot,
		AuthMode:                    envOrDefault("AODS_AUTH_MODE", "header"),
		PlatformAdminAuthorities:    platformAdminAuthorities,
		OIDCIssuerURL:               envOrDefault("AODS_OIDC_ISSUER_URL", ""),
		OIDCJWKSURL:                 envOrDefault("AODS_OIDC_JWKS_URL", ""),
		OIDCAudience:                envOrDefault("AODS_OIDC_AUDIENCE", ""),
		OIDCUserIDClaim:             envOrDefault("AODS_OIDC_USER_ID_CLAIM", "sub"),
		OIDCUsernameClaim:           envOrDefault("AODS_OIDC_USERNAME_CLAIM", "preferred_username"),
		OIDCDisplayNameClaim:        envOrDefault("AODS_OIDC_DISPLAY_NAME_CLAIM", "name"),
		OIDCGroupsClaim:             envOrDefault("AODS_OIDC_GROUPS_CLAIM", "groups"),
		OIDCRoleMappings:            oidcRoleMappings,
		OIDCRequestTimeout:          oidcRequestTimeout,
		GitMode:                     envOrDefault("AODS_GIT_MODE", "local"),
		GitRepoDir:                  envOrDefault("AODS_GIT_REPO_DIR", filepath.Join(os.TempDir(), "aods-managed-gitops")),
		GitRemote:                   envOrDefault("AODS_GIT_REMOTE", ""),
		GitBranch:                   envOrDefault("AODS_GIT_BRANCH", "main"),
		GitAuthorName:               envOrDefault("AODS_GIT_AUTHOR_NAME", "AODS Bot"),
		GitAuthorEmail:              envOrDefault("AODS_GIT_AUTHOR_EMAIL", "aods-bot@local"),
		GitCommandTimeout:           gitCommandTimeout,
		GitSyncTTL:                  gitSyncTTL,
		RepositoryPollInterval:      repositoryPollInterval,
		KubernetesMode:              envOrDefault("AODS_K8S_MODE", "local"),
		KubernetesAPIURL:            envOrDefault("AODS_K8S_API_URL", ""),
		KubernetesBearerToken:       envOrDefault("AODS_K8S_BEARER_TOKEN", ""),
		KubernetesCAFile:            envOrDefault("AODS_K8S_CA_FILE", ""),
		KubernetesCAData:            envOrDefault("AODS_K8S_CA_DATA", ""),
		KubernetesKubeconfigPath:    envOrDefault("AODS_K8S_KUBECONFIG", defaultKubeconfigPath()),
		KubernetesContext:           envOrDefault("AODS_K8S_CONTEXT", ""),
		KubernetesRequestTimeout:    kubernetesRequestTimeout,
		ImageVerificationMode:       envOrDefault("AODS_IMAGE_CHECK_MODE", "anonymous"),
		ImageVerificationTimeout:    imageVerificationTimeout,
		FluxKustomizationNamespace:  envOrDefault("AODS_FLUX_KUSTOMIZATION_NAMESPACE", "flux-system"),
		FluxSourceName:              envOrDefault("AODS_FLUX_SOURCE_NAME", "aods-manifest"),
		PrometheusMode:              envOrDefault("AODS_PROMETHEUS_MODE", "local"),
		PrometheusURL:               envOrDefault("AODS_PROMETHEUS_URL", ""),
		PrometheusRequestTimeout:    prometheusRequestTimeout,
		PrometheusRange:             prometheusRange,
		PrometheusStep:              prometheusStep,
		VaultMode:                   envOrDefault("AODS_VAULT_MODE", "local"),
		VaultAddress:                envOrDefault("AODS_VAULT_ADDR", ""),
		VaultToken:                  envOrDefault("AODS_VAULT_TOKEN", ""),
		VaultNamespace:              envOrDefault("AODS_VAULT_NAMESPACE", ""),
		VaultRequestTimeout:         vaultRequestTimeout,
		VaultStagingCleanupInterval: vaultStagingCleanupInterval,
		VaultStagingMaxAge:          vaultStagingMaxAge,
		OrphanFluxCleanupInterval:   orphanFluxCleanupInterval,
		AllowedOrigin:               envOrDefault("AODS_ALLOWED_ORIGIN", "*"),
		AllowDevFallback:            allowDevFallback,
		DevUser: User{
			ID:          envOrDefault("AODS_DEV_USER_ID", "local-user"),
			Username:    envOrDefault("AODS_DEV_USERNAME", "local.developer"),
			DisplayName: envOrDefault("AODS_DEV_DISPLAY_NAME", "로컬 운영자"),
			Groups: splitCommaSeparated(
				envOrDefault("AODS_DEV_GROUPS", "aods:shared:deploy"),
			),
		},
		LocalVaultDir: envOrDefault("AODS_LOCAL_VAULT_DIR", filepath.Join(os.TempDir(), "aods-local-vault")),
	}, nil
}

func (c Config) UseGitRepo() bool {
	return strings.EqualFold(c.GitMode, "git")
}

func (c Config) UseOIDCAuth() bool {
	return strings.EqualFold(strings.TrimSpace(c.AuthMode), "oidc")
}

func (c Config) UseKubernetesAPI() bool {
	mode := strings.TrimSpace(c.KubernetesMode)
	return mode != "" && !strings.EqualFold(mode, "local")
}

func (c Config) UseImageVerification() bool {
	mode := strings.TrimSpace(c.ImageVerificationMode)
	return mode != "" && !strings.EqualFold(mode, "local")
}

func (c Config) UsePrometheusAPI() bool {
	mode := strings.TrimSpace(c.PrometheusMode)
	return mode != "" && !strings.EqualFold(mode, "local")
}

func (c Config) UseVaultAPI() bool {
	mode := strings.TrimSpace(c.VaultMode)
	return mode != "" && !strings.EqualFold(mode, "local")
}

func envOrDefault(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) (bool, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", key, err)
	}

	return parsed, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}

	return parsed, nil
}

func resolveRepoRoot(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	dir := currentDir
	for {
		if fileExists(filepath.Join(dir, "AGENTS.md")) && fileExists(filepath.Join(dir, "docs", "internal-platform", "openapi.yaml")) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not resolve repository root from %s", currentDir)
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func defaultKubeconfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".kube", "config")
}
