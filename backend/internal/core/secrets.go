package core

import "strings"

func BuildVaultStagingPath(requestID string) string {
	return "secret/aods/staging/" + requestID
}

func BuildVaultFinalPath(projectID string, appName string) string {
	return "secret/aods/apps/" + projectID + "/" + appName + "/prod"
}

func VaultExtractKey(path string) string {
	return strings.TrimPrefix(path, "secret/")
}
