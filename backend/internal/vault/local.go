package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
		"path": staged.StagingPath,
		"data": data,
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

	document := map[string]any{
		"path": staged.FinalPath,
		"data": data,
	}

	if err := s.writeDocument(pathToFile(s.RootDir, staged.FinalPath), document); err != nil {
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

	var document struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode local secret: %w", err)
	}

	return document.Data, nil
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
