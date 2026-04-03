package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/core"
)

type RealStore struct {
	Address   string
	Token     string
	Namespace string
	Client    *http.Client
}

func (s RealStore) Stage(
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

	if strings.TrimSpace(s.Address) == "" {
		return application.StagedSecret{}, fmt.Errorf("vault address is required")
	}
	if strings.TrimSpace(s.Token) == "" {
		return application.StagedSecret{}, fmt.Errorf("vault token is required")
	}

	staged := application.StagedSecret{
		StagingPath: core.BuildVaultStagingPath(requestID),
		FinalPath:   core.BuildVaultFinalPath(projectID, appName),
	}

	if err := s.writeKVv2(ctx, staged.StagingPath, data); err != nil {
		return application.StagedSecret{}, err
	}
	if err := s.writeMetadata(ctx, staged.StagingPath, map[string]string{
		"projectId": projectID,
		"appName":   appName,
		"createdBy": createdBy,
		"status":    "pending_commit",
	}); err != nil {
		return application.StagedSecret{}, err
	}

	return staged, nil
}

func (s RealStore) Finalize(ctx context.Context, staged application.StagedSecret, data map[string]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := s.writeKVv2(ctx, staged.FinalPath, data); err != nil {
		return err
	}

	if err := s.deleteMetadata(ctx, staged.StagingPath); err != nil {
		return err
	}

	return nil
}

func (s RealStore) writeKVv2(ctx context.Context, logicalPath string, data map[string]string) error {
	endpoint, err := s.kvEndpoint(logicalPath, "data")
	if err != nil {
		return err
	}

	body, err := json.Marshal(map[string]any{
		"data": data,
	})
	if err != nil {
		return fmt.Errorf("encode vault payload: %w", err)
	}

	if err := s.doJSON(ctx, http.MethodPost, endpoint, body); err != nil {
		return fmt.Errorf("write vault secret %s: %w", logicalPath, err)
	}

	return nil
}

func (s RealStore) writeMetadata(ctx context.Context, logicalPath string, metadata map[string]string) error {
	endpoint, err := s.kvEndpoint(logicalPath, "metadata")
	if err != nil {
		return err
	}

	body, err := json.Marshal(map[string]any{
		"custom_metadata": metadata,
	})
	if err != nil {
		return fmt.Errorf("encode vault metadata payload: %w", err)
	}

	if err := s.doJSON(ctx, http.MethodPost, endpoint, body); err != nil {
		return fmt.Errorf("write vault metadata %s: %w", logicalPath, err)
	}

	return nil
}

func (s RealStore) deleteMetadata(ctx context.Context, logicalPath string) error {
	endpoint, err := s.kvEndpoint(logicalPath, "metadata")
	if err != nil {
		return err
	}

	if err := s.doJSON(ctx, http.MethodDelete, endpoint, nil); err != nil {
		return fmt.Errorf("delete vault metadata %s: %w", logicalPath, err)
	}

	return nil
}

func (s RealStore) doJSON(ctx context.Context, method string, endpoint string, body []byte) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("build vault request: %w", err)
	}

	req.Header.Set("X-Vault-Token", s.Token)
	if strings.TrimSpace(s.Namespace) != "" {
		req.Header.Set("X-Vault-Namespace", s.Namespace)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("send vault request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	message := strings.TrimSpace(string(responseBody))
	if message == "" {
		message = resp.Status
	}
	return fmt.Errorf("vault API returned %s", message)
}

func (s RealStore) Get(ctx context.Context, logicalPath string) (map[string]string, error) {
	endpoint, err := s.kvEndpoint(logicalPath, "data")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build vault request: %w", err)
	}

	req.Header.Set("X-Vault-Token", s.Token)
	if strings.TrimSpace(s.Namespace) != "" {
		req.Header.Set("X-Vault-Namespace", s.Namespace)
	}

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("send vault request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Not found is not an error here, but can be handled by caller
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("vault API returned %s: %s", resp.Status, string(body))
	}

	var payload struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode vault response: %w", err)
	}

	return payload.Data.Data, nil
}

func (s RealStore) httpClient() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return http.DefaultClient
}

func (s RealStore) kvEndpoint(logicalPath string, section string) (string, error) {
	base := strings.TrimSpace(s.Address)
	if base == "" {
		return "", fmt.Errorf("vault address is required")
	}
	if strings.TrimSpace(logicalPath) == "" {
		return "", fmt.Errorf("vault logical path is required")
	}

	parts := strings.Split(strings.Trim(logicalPath, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("vault logical path %q must include mount and key", logicalPath)
	}

	mount := parts[0]
	secretPath := strings.Join(parts[1:], "/")

	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return "", fmt.Errorf("parse vault address: %w", err)
	}

	u.Path = strings.TrimRight(u.Path, "/") + "/v1/" + mount + "/" + section + "/" + secretPath
	return u.String(), nil
}
