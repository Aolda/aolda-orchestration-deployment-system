package core

import "strings"

const vaultStagingRootPath = "secret/aods/staging"

func BuildVaultStagingRootPath() string {
	return vaultStagingRootPath
}

func BuildVaultStagingPath(requestID string) string {
	return vaultStagingRootPath + "/" + requestID
}

func BuildVaultFinalPath(projectID string, appName string) string {
	return "secret/aods/apps/" + projectID + "/" + appName + "/prod"
}

func BuildVaultRepositoryTokenPath(projectID string, appName string) string {
	return "secret/aods/apps/" + projectID + "/" + appName + "/repository"
}

func BuildVaultRegistryCredentialPath(projectID string, appName string) string {
	return "secret/aods/apps/" + projectID + "/" + appName + "/registry"
}

func VaultExtractKey(path string) string {
	return strings.TrimPrefix(path, "secret/")
}
