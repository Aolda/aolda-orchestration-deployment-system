package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Address          string
	RepoRoot         string
	AllowedOrigin    string
	AllowDevFallback bool
	DevUser          User
	LocalVaultDir    string
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

	return Config{
		Address:          envOrDefault("AODS_ADDR", ":8080"),
		RepoRoot:         repoRoot,
		AllowedOrigin:    envOrDefault("AODS_ALLOWED_ORIGIN", "*"),
		AllowDevFallback: allowDevFallback,
		DevUser: User{
			ID:          envOrDefault("AODS_DEV_USER_ID", "local-user"),
			Username:    envOrDefault("AODS_DEV_USERNAME", "local.developer"),
			DisplayName: envOrDefault("AODS_DEV_DISPLAY_NAME", "Local Developer"),
			Groups: splitCommaSeparated(
				envOrDefault("AODS_DEV_GROUPS", "aods:project-a:deploy,aods:project-b:view"),
			),
		},
		LocalVaultDir: envOrDefault("AODS_LOCAL_VAULT_DIR", filepath.Join(os.TempDir(), "aods-local-vault")),
	}, nil
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
