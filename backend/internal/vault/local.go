package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/core"
)

type LocalStore struct {
	RootDir string
}

func (s LocalStore) Stage(
	ctx context.Context,
	requestID string,
	projectID string,
	appName string,
	createdBy string,
	data map[string]string,
) (application.StagedSecret, error) {
	return s.StageAt(ctx, requestID, core.BuildVaultFinalPath(projectID, appName), map[string]string{
		"projectId": projectID,
		"appName":   appName,
		"createdBy": createdBy,
	}, data)
}

func (s LocalStore) StageAt(
	ctx context.Context,
	requestID string,
	finalPath string,
	metadata map[string]string,
	data map[string]string,
) (application.StagedSecret, error) {
	if err := ctx.Err(); err != nil {
		return application.StagedSecret{}, err
	}

	staged := application.StagedSecret{
		StagingPath: core.BuildVaultStagingPath(requestID),
		FinalPath:   finalPath,
	}

	documentMetadata := map[string]any{
		"status":    pendingCommitStatus,
		"createdAt": time.Now().UTC().Format(time.RFC3339Nano),
	}
	for key, value := range metadata {
		documentMetadata[key] = value
	}

	document := map[string]any{
		"path":     staged.StagingPath,
		"data":     data,
		"metadata": documentMetadata,
	}

	if err := s.writeDocument(pathToFile(s.RootDir, staged.StagingPath), document); err != nil {
		return application.StagedSecret{}, err
	}

	return staged, nil
}

func (s LocalStore) Finalize(ctx context.Context, staged application.StagedSecret, data map[string]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	stagedDocument, _ := readLocalDocument(pathToFile(s.RootDir, staged.StagingPath))
	document, err := readLocalDocument(pathToFile(s.RootDir, staged.FinalPath))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	previousData := cloneStringMap(document.Data)
	document.Path = staged.FinalPath
	if document.Metadata == nil {
		document.Metadata = map[string]any{}
	}
	if document.Versions == nil {
		document.Versions = map[string]localVaultVersion{}
	}
	if len(document.Versions) == 0 && len(previousData) > 0 {
		createdAt := metadataString(document.Metadata, "updatedAt")
		if createdAt == "" {
			createdAt = time.Now().UTC().Format(time.RFC3339Nano)
		}
		document.Versions["1"] = localVaultVersion{
			Data:      previousData,
			CreatedAt: createdAt,
			Metadata:  cloneAnyMap(document.Metadata),
		}
		document.Metadata["currentVersion"] = float64(1)
	}

	version := localCurrentVersion(document) + 1
	now := time.Now().UTC().Format(time.RFC3339Nano)
	versionMetadata := cloneAnyMap(stagedDocument.Metadata)
	versionMetadata["status"] = "finalized"
	versionMetadata["updatedAt"] = now
	document.Data = cloneStringMap(data)
	document.Metadata = cloneAnyMap(versionMetadata)
	document.Metadata["currentVersion"] = float64(version)
	document.Metadata["updatedAt"] = now
	document.Versions[strconv.Itoa(version)] = localVaultVersion{
		Data:      cloneStringMap(data),
		CreatedAt: now,
		Metadata:  versionMetadata,
	}

	if err := writeLocalDocument(pathToFile(s.RootDir, staged.FinalPath), document); err != nil {
		return err
	}

	if err := os.Remove(pathToFile(s.RootDir, staged.StagingPath)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete staged secret: %w", err)
	}

	return nil
}

func (s LocalStore) Get(ctx context.Context, logicalPath string) (map[string]string, error) {
	if strings.TrimSpace(s.RootDir) == "" {
		return nil, fmt.Errorf("local vault root directory is required")
	}

	path := pathToFile(s.RootDir, logicalPath)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read local secret: %w", err)
	}

	var document localVaultDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode local secret: %w", err)
	}

	return cloneStringMap(document.Data), nil
}

func (s LocalStore) ListVersions(ctx context.Context, logicalPath string) (application.ApplicationSecretVersionsResponse, error) {
	if err := ctx.Err(); err != nil {
		return application.ApplicationSecretVersionsResponse{}, err
	}
	if strings.TrimSpace(s.RootDir) == "" {
		return application.ApplicationSecretVersionsResponse{}, fmt.Errorf("local vault root directory is required")
	}

	document, err := readLocalDocument(pathToFile(s.RootDir, logicalPath))
	if os.IsNotExist(err) {
		return application.ApplicationSecretVersionsResponse{SecretPath: logicalPath, Items: []application.SecretVersionSummary{}}, nil
	}
	if err != nil {
		return application.ApplicationSecretVersionsResponse{}, err
	}
	return localVersionResponse(logicalPath, document), nil
}

func (s LocalStore) GetVersion(ctx context.Context, logicalPath string, version int) (map[string]string, application.SecretVersionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, application.SecretVersionSummary{}, err
	}
	if version <= 0 {
		return nil, application.SecretVersionSummary{}, fmt.Errorf("local vault version must be positive")
	}

	document, err := readLocalDocument(pathToFile(s.RootDir, logicalPath))
	if err != nil {
		return nil, application.SecretVersionSummary{}, err
	}
	if len(document.Versions) == 0 && len(document.Data) > 0 {
		document.Versions = map[string]localVaultVersion{
			"1": {
				Data:      cloneStringMap(document.Data),
				CreatedAt: metadataString(document.Metadata, "updatedAt"),
				Metadata:  cloneAnyMap(document.Metadata),
			},
		}
	}
	item, ok := document.Versions[strconv.Itoa(version)]
	if !ok {
		return nil, application.SecretVersionSummary{}, fmt.Errorf("local vault version %d was not found", version)
	}
	summary := localVersionSummary(version, item, localCurrentVersion(document))
	summary.KeyCount = len(item.Data)
	return cloneStringMap(item.Data), summary, nil
}

func (s LocalStore) Delete(ctx context.Context, logicalPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(s.RootDir) == "" {
		return fmt.Errorf("local vault root directory is required")
	}

	path := pathToFile(s.RootDir, logicalPath)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete local secret: %w", err)
	}
	return nil
}

func (s LocalStore) CleanupStale(ctx context.Context, cutoff time.Time) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if strings.TrimSpace(s.RootDir) == "" {
		return 0, fmt.Errorf("local vault root directory is required")
	}

	stagingDir := filepath.Join(s.RootDir, strings.TrimPrefix(core.BuildVaultStagingRootPath(), "secret/"))
	entries, err := os.ReadDir(stagingDir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read local staging directory: %w", err)
	}

	cleaned := 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return cleaned, err
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(stagingDir, entry.Name())
		stale, err := isLocalStagingFileStale(path, cutoff)
		if err != nil {
			return cleaned, err
		}
		if !stale {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return cleaned, fmt.Errorf("delete local staged secret %s: %w", entry.Name(), err)
		}
		cleaned++
	}

	return cleaned, nil
}

func (s LocalStore) writeDocument(path string, document map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create local vault directory: %w", err)
	}

	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode local vault document: %w", err)
	}

	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write local vault document: %w", err)
	}

	return nil
}

func pathToFile(root string, logicalPath string) string {
	return filepath.Join(root, strings.TrimPrefix(logicalPath, "secret/")) + ".json"
}

type localVaultDocument struct {
	Path     string                       `json:"path"`
	Data     map[string]string            `json:"data"`
	Metadata map[string]any               `json:"metadata,omitempty"`
	Versions map[string]localVaultVersion `json:"versions,omitempty"`
}

type localVaultVersion struct {
	Data      map[string]string `json:"data"`
	CreatedAt string            `json:"createdAt,omitempty"`
	Deleted   bool              `json:"deleted,omitempty"`
	Destroyed bool              `json:"destroyed,omitempty"`
	Metadata  map[string]any    `json:"metadata,omitempty"`
}

func readLocalDocument(path string) (localVaultDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return localVaultDocument{}, err
	}
	var document localVaultDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return localVaultDocument{}, fmt.Errorf("decode local vault document: %w", err)
	}
	if document.Data == nil {
		document.Data = map[string]string{}
	}
	if document.Metadata == nil {
		document.Metadata = map[string]any{}
	}
	return document, nil
}

func writeLocalDocument(path string, document localVaultDocument) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create local vault directory: %w", err)
	}
	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode local vault document: %w", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write local vault document: %w", err)
	}
	return nil
}

func localVersionResponse(logicalPath string, document localVaultDocument) application.ApplicationSecretVersionsResponse {
	currentVersion := localCurrentVersion(document)
	keys := make([]int, 0, len(document.Versions))
	for rawVersion := range document.Versions {
		version, err := strconv.Atoi(rawVersion)
		if err == nil {
			keys = append(keys, version)
		}
	}
	if len(keys) == 0 && len(document.Data) > 0 {
		keys = append(keys, 1)
		document.Versions = map[string]localVaultVersion{
			"1": {
				Data:      cloneStringMap(document.Data),
				CreatedAt: metadataString(document.Metadata, "updatedAt"),
				Metadata:  cloneAnyMap(document.Metadata),
			},
		}
		currentVersion = 1
	}
	sort.Sort(sort.Reverse(sort.IntSlice(keys)))

	items := make([]application.SecretVersionSummary, 0, len(keys))
	for _, version := range keys {
		item := document.Versions[strconv.Itoa(version)]
		items = append(items, localVersionSummary(version, item, currentVersion))
	}
	return application.ApplicationSecretVersionsResponse{
		SecretPath:     logicalPath,
		CurrentVersion: currentVersion,
		Items:          items,
	}
}

func localVersionSummary(version int, item localVaultVersion, currentVersion int) application.SecretVersionSummary {
	var createdAt *time.Time
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(item.CreatedAt)); err == nil {
		createdAt = &parsed
	}
	updatedBy := metadataString(item.Metadata, "updatedBy")
	if updatedBy == "" {
		updatedBy = metadataString(item.Metadata, "createdBy")
	}
	return application.SecretVersionSummary{
		Version:   version,
		CreatedAt: createdAt,
		UpdatedBy: updatedBy,
		Current:   version == currentVersion,
		Deleted:   item.Deleted,
		Destroyed: item.Destroyed,
		KeyCount:  len(item.Data),
	}
}

func localCurrentVersion(document localVaultDocument) int {
	if version := metadataInt(document.Metadata, "currentVersion"); version > 0 {
		return version
	}
	current := 0
	for rawVersion := range document.Versions {
		version, err := strconv.Atoi(rawVersion)
		if err == nil && version > current {
			current = version
		}
	}
	return current
}

func cloneStringMap(values map[string]string) map[string]string {
	cloned := map[string]string{}
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneAnyMap(values map[string]any) map[string]any {
	cloned := map[string]any{}
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func isLocalStagingFileStale(path string, cutoff time.Time) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read local staged secret %s: %w", filepath.Base(path), err)
	}

	var document struct {
		Metadata map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return false, fmt.Errorf("decode local staged secret %s: %w", filepath.Base(path), err)
	}

	status := metadataString(document.Metadata, "status")
	if status != "" && status != pendingCommitStatus {
		return false, nil
	}

	createdAt, ok := metadataTime(document.Metadata, "createdAt")
	if !ok {
		info, err := os.Stat(path)
		if err != nil {
			return false, fmt.Errorf("stat local staged secret %s: %w", filepath.Base(path), err)
		}
		createdAt = info.ModTime().UTC()
	}

	return !createdAt.After(cutoff), nil
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func metadataInt(metadata map[string]any, key string) int {
	if metadata == nil {
		return 0
	}
	value, ok := metadata[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return 0
}

func metadataTime(metadata map[string]any, key string) (time.Time, bool) {
	value := metadataString(metadata, key)
	if value == "" {
		return time.Time{}, false
	}

	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}
