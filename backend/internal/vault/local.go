package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	if err := ctx.Err(); err != nil {
		return application.StagedSecret{}, err
	}

	staged := application.StagedSecret{
		StagingPath: core.BuildVaultStagingPath(requestID),
		FinalPath:   core.BuildVaultFinalPath(projectID, appName),
	}

	document := map[string]any{
		"path": staged.StagingPath,
		"data": data,
		"metadata": map[string]any{
			"projectId": projectID,
			"appName":   appName,
			"createdBy": createdBy,
			"status":    "pending_commit",
		},
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
