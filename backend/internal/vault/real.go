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
	"time"

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
	return s.StageAt(ctx, requestID, core.BuildVaultFinalPath(projectID, appName), map[string]string{
		"projectId": projectID,
		"appName":   appName,
		"createdBy": createdBy,
	}, data)
}

func (s RealStore) StageAt(
	ctx context.Context,
	requestID string,
	finalPath string,
	metadata map[string]string,
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
		FinalPath:   finalPath,
	}

	if err := s.writeKVv2(ctx, staged.StagingPath, data); err != nil {
		return application.StagedSecret{}, err
	}
	metadataPayload := map[string]string{
		"status":    pendingCommitStatus,
		"createdAt": time.Now().UTC().Format(time.RFC3339Nano),
	}
	for key, value := range metadata {
		metadataPayload[key] = value
	}
	if err := s.writeMetadata(ctx, staged.StagingPath, metadataPayload); err != nil {
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

func (s RealStore) Delete(ctx context.Context, logicalPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(logicalPath) == "" {
		return fmt.Errorf("vault logical path is required")
	}
	return s.deleteMetadata(ctx, logicalPath)
}

func (s RealStore) CleanupStale(ctx context.Context, cutoff time.Time) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if strings.TrimSpace(s.Token) == "" {
		return 0, fmt.Errorf("vault token is required")
	}

	return s.cleanupStaleUnderPath(ctx, core.BuildVaultStagingRootPath(), cutoff)
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

type kvMetadata struct {
	CreatedTime    string            `json:"created_time"`
	UpdatedTime    string            `json:"updated_time"`
	CustomMetadata map[string]string `json:"custom_metadata"`
}

func (s RealStore) cleanupStaleUnderPath(ctx context.Context, logicalPath string, cutoff time.Time) (int, error) {
	keys, err := s.listMetadataKeys(ctx, logicalPath)
	if err != nil {
		return 0, err
	}

	cleaned := 0
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return cleaned, err
		}

		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		childPath := logicalPath + "/" + strings.TrimSuffix(key, "/")
		if strings.HasSuffix(key, "/") {
			count, err := s.cleanupStaleUnderPath(ctx, childPath, cutoff)
			cleaned += count
			if err != nil {
				return cleaned, err
			}
			continue
		}

		metadata, found, err := s.readMetadata(ctx, childPath)
		if err != nil {
			return cleaned, err
		}
		if !found || !metadata.isStalePendingCommit(cutoff) {
			continue
		}

		if err := s.deleteMetadata(ctx, childPath); err != nil {
			return cleaned, err
		}
		cleaned++
	}

	return cleaned, nil
}

func (s RealStore) listMetadataKeys(ctx context.Context, logicalPath string) ([]string, error) {
	endpoint, err := s.kvEndpoint(logicalPath, "metadata")
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse vault endpoint: %w", err)
	}
	query := u.Query()
	query.Set("exclude_deleted", "true")
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, "LIST", u.String(), nil)
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
		return nil, nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("vault API returned %s: %s", resp.Status, string(body))
	}

	var payload struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode vault metadata list response: %w", err)
	}

	return payload.Data.Keys, nil
}

func (s RealStore) readMetadata(ctx context.Context, logicalPath string) (kvMetadata, bool, error) {
	endpoint, err := s.kvEndpoint(logicalPath, "metadata")
	if err != nil {
		return kvMetadata{}, false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return kvMetadata{}, false, fmt.Errorf("build vault request: %w", err)
	}

	req.Header.Set("X-Vault-Token", s.Token)
	if strings.TrimSpace(s.Namespace) != "" {
		req.Header.Set("X-Vault-Namespace", s.Namespace)
	}

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return kvMetadata{}, false, fmt.Errorf("send vault request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return kvMetadata{}, false, nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return kvMetadata{}, false, fmt.Errorf("vault API returned %s: %s", resp.Status, string(body))
	}

	var payload struct {
		Data kvMetadata `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return kvMetadata{}, false, fmt.Errorf("decode vault metadata response: %w", err)
	}

	return payload.Data, true, nil
}

func (m kvMetadata) isStalePendingCommit(cutoff time.Time) bool {
	status := strings.TrimSpace(m.CustomMetadata["status"])
	if status != "" && status != pendingCommitStatus {
		return false
	}

	createdAt, ok := m.createdAt()
	if !ok {
		return false
	}
	return !createdAt.After(cutoff)
}

func (m kvMetadata) createdAt() (time.Time, bool) {
	candidates := []string{
		strings.TrimSpace(m.CustomMetadata["createdAt"]),
		strings.TrimSpace(m.CreatedTime),
		strings.TrimSpace(m.UpdatedTime),
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339Nano, candidate)
		if err == nil {
			return parsed.UTC(), true
		}
	}

	return time.Time{}, false
}
