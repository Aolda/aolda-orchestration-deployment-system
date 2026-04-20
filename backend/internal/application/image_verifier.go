package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type ImageVerifier interface {
	Verify(ctx context.Context, image string, credential *RegistryCredential) error
}

type NoopImageVerifier struct{}

func (NoopImageVerifier) Verify(context.Context, string, *RegistryCredential) error {
	return nil
}

type RegistryImageVerifier struct {
	Client *http.Client
}

type RegistryCredential struct {
	Server   string
	Username string
	Password string
}

type ImageValidationError struct {
	Code     string
	Message  string
	Image    string
	Registry string
}

func (e ImageValidationError) Error() string {
	return e.Message
}

type imageReference struct {
	Original   string
	Registry   string
	Repository string
	Identifier string
	Scheme     string
}

func (v RegistryImageVerifier) Verify(ctx context.Context, image string, credential *RegistryCredential) error {
	ref, err := parseImageReference(image)
	if err != nil {
		return ImageValidationError{
			Code:    "INVALID_IMAGE_REFERENCE",
			Message: "컨테이너 이미지 형식이 올바르지 않습니다.",
			Image:   image,
		}
	}

	status, body, tokenErr := v.fetchManifest(ctx, ref, "")
	if tokenErr != nil {
		return imageCheckFailure(ref, tokenErr)
	}

	switch status {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		challenge, ok := parseBearerChallenge(body.authenticate)
		if !ok {
			return authRequiredError(ref)
		}

		if credential != nil && credential.matches(ref.Registry) {
			token, err := v.fetchRegistryToken(ctx, challenge, *credential)
			if err == nil {
				status, body, err = v.fetchManifest(ctx, ref, token)
				if err != nil {
					return imageCheckFailure(ref, err)
				}
				switch status {
				case http.StatusOK:
					return nil
				case http.StatusNotFound:
					return imageNotFoundError(ref)
				case http.StatusUnauthorized, http.StatusForbidden:
					// Fall back to anonymous token exchange below before surfacing auth failure.
				default:
					return statusToImageError(ref, status, body.payload)
				}
			}
		}

		token, err := v.fetchAnonymousToken(ctx, challenge)
		if err != nil {
			return authRequiredError(ref)
		}
		status, body, err = v.fetchManifest(ctx, ref, token)
		if err != nil {
			return imageCheckFailure(ref, err)
		}
		switch status {
		case http.StatusOK:
			return nil
		case http.StatusNotFound:
			return imageNotFoundError(ref)
		case http.StatusUnauthorized, http.StatusForbidden:
			return authRequiredError(ref)
		default:
			return statusToImageError(ref, status, body.payload)
		}
	case http.StatusNotFound:
		return imageNotFoundError(ref)
	default:
		return statusToImageError(ref, status, body.payload)
	}
}

type manifestResponse struct {
	authenticate string
	payload      []byte
}

func (v RegistryImageVerifier) fetchManifest(
	ctx context.Context,
	ref imageReference,
	token string,
) (int, manifestResponse, error) {
	client := v.Client
	if client == nil {
		client = http.DefaultClient
	}

	manifestURL := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", ref.Scheme, ref.Registry, ref.Repository, url.PathEscape(ref.Identifier))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return 0, manifestResponse{}, err
	}
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.index.v1+json",
		"application/json",
	}, ", "))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, manifestResponse{}, err
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	return resp.StatusCode, manifestResponse{
		authenticate: resp.Header.Get("WWW-Authenticate"),
		payload:      payload,
	}, nil
}

type bearerChallenge struct {
	realm   string
	service string
	scope   string
}

func parseBearerChallenge(header string) (bearerChallenge, bool) {
	header = strings.TrimSpace(header)
	if header == "" || !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return bearerChallenge{}, false
	}

	values := strings.Split(header[len("Bearer "):], ",")
	challenge := bearerChallenge{}
	for _, item := range values {
		key, value, ok := strings.Cut(strings.TrimSpace(item), "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "realm":
			challenge.realm = value
		case "service":
			challenge.service = value
		case "scope":
			challenge.scope = value
		}
	}

	return challenge, challenge.realm != ""
}

func (v RegistryImageVerifier) fetchAnonymousToken(ctx context.Context, challenge bearerChallenge) (string, error) {
	return v.fetchToken(ctx, challenge, nil)
}

func (v RegistryImageVerifier) fetchRegistryToken(ctx context.Context, challenge bearerChallenge, credential RegistryCredential) (string, error) {
	username := strings.TrimSpace(credential.Username)
	password := credential.Password
	if username == "" || password == "" {
		return "", errors.New("registry credentials are incomplete")
	}
	return v.fetchToken(ctx, challenge, &credential)
}

func (v RegistryImageVerifier) fetchToken(ctx context.Context, challenge bearerChallenge, credential *RegistryCredential) (string, error) {
	client := v.Client
	if client == nil {
		client = http.DefaultClient
	}

	tokenURL, err := url.Parse(challenge.realm)
	if err != nil {
		return "", err
	}

	query := tokenURL.Query()
	if challenge.service != "" {
		query.Set("service", challenge.service)
	}
	if challenge.scope != "" {
		query.Set("scope", challenge.scope)
	}
	query.Set("client_id", "aods")
	tokenURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL.String(), nil)
	if err != nil {
		return "", err
	}
	if credential != nil {
		req.SetBasicAuth(strings.TrimSpace(credential.Username), credential.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anonymous token request returned %d", resp.StatusCode)
	}

	var payload struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<10)).Decode(&payload); err != nil {
		return "", err
	}

	if payload.Token != "" {
		return payload.Token, nil
	}
	if payload.AccessToken != "" {
		return payload.AccessToken, nil
	}
	return "", errors.New("token response did not include a token")
}

func (c RegistryCredential) matches(registry string) bool {
	if strings.TrimSpace(c.Server) == "" {
		return true
	}
	return normalizeRegistryServer(c.Server) == normalizeRegistryServer(registry)
}

func parseImageReference(image string) (imageReference, error) {
	trimmed := strings.TrimSpace(image)
	if trimmed == "" {
		return imageReference{}, errors.New("empty image reference")
	}

	namePart := trimmed
	identifier := "latest"

	if at := strings.LastIndex(trimmed, "@"); at >= 0 {
		namePart = trimmed[:at]
		identifier = trimmed[at+1:]
		if identifier == "" {
			return imageReference{}, errors.New("empty digest")
		}
	} else {
		lastSlash := strings.LastIndex(trimmed, "/")
		lastColon := strings.LastIndex(trimmed, ":")
		if lastColon > lastSlash {
			namePart = trimmed[:lastColon]
			identifier = trimmed[lastColon+1:]
			if identifier == "" {
				return imageReference{}, errors.New("empty tag")
			}
		}
	}

	parts := strings.Split(namePart, "/")
	if len(parts) == 0 {
		return imageReference{}, errors.New("missing repository")
	}

	registry := "registry-1.docker.io"
	repositoryParts := parts
	if isRegistryComponent(parts[0]) {
		registry = parts[0]
		repositoryParts = parts[1:]
	}
	if len(repositoryParts) == 0 {
		return imageReference{}, errors.New("missing repository path")
	}

	repository := strings.Join(repositoryParts, "/")
	if registry == "registry-1.docker.io" && !strings.Contains(repository, "/") {
		repository = "library/" + repository
	}

	return imageReference{
		Original:   trimmed,
		Registry:   registry,
		Repository: repository,
		Identifier: identifier,
		Scheme:     registryScheme(registry),
	}, nil
}

func isRegistryComponent(value string) bool {
	return strings.Contains(value, ".") || strings.Contains(value, ":") || value == "localhost"
}

func registryScheme(host string) string {
	if host == "localhost" {
		return "http"
	}
	serverHost, _, err := net.SplitHostPort(host)
	if err == nil {
		host = serverHost
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return "http"
	}
	if strings.HasPrefix(host, "127.") {
		return "http"
	}
	return "https"
}

func normalizeRegistryServer(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	return strings.TrimSuffix(trimmed, "/")
}

func buildDockerConfigJSON(server string, username string, password string) (string, error) {
	server = normalizeRegistryServer(server)
	if server == "" {
		return "", errors.New("registry server is required")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return "", errors.New("registry username is required")
	}
	if password == "" {
		return "", errors.New("registry password is required")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	payload := map[string]any{
		"auths": map[string]map[string]string{
			server: {
				"username": username,
				"password": password,
				"auth":     auth,
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func registryCredentialFromSecret(values map[string]string) (*RegistryCredential, error) {
	if len(values) == 0 {
		return nil, nil
	}

	server := normalizeRegistryServer(values["server"])
	username := strings.TrimSpace(values["username"])
	password := values["password"]
	if server == "" && username == "" && password == "" {
		return nil, nil
	}
	if server == "" || username == "" || password == "" {
		return nil, errors.New("stored registry credential is incomplete")
	}

	return &RegistryCredential{
		Server:   server,
		Username: username,
		Password: password,
	}, nil
}

func imageNotFoundError(ref imageReference) error {
	return ImageValidationError{
		Code:     "IMAGE_NOT_FOUND",
		Message:  "지정한 컨테이너 이미지를 찾을 수 없습니다. 이미지 이름과 태그를 확인해 주세요.",
		Image:    ref.Original,
		Registry: ref.Registry,
	}
}

func authRequiredError(ref imageReference) error {
	return ImageValidationError{
		Code:     "IMAGE_AUTH_REQUIRED",
		Message:  "이미지 레지스트리에 인증이 필요합니다. 현재 설정만으로는 이 이미지를 가져올 수 없습니다.",
		Image:    ref.Original,
		Registry: ref.Registry,
	}
}

func imageCheckFailure(ref imageReference, err error) error {
	return ImageValidationError{
		Code:     "IMAGE_CHECK_FAILED",
		Message:  fmt.Sprintf("이미지 접근 상태를 확인하지 못했습니다: %v", err),
		Image:    ref.Original,
		Registry: ref.Registry,
	}
}

func statusToImageError(ref imageReference, status int, payload []byte) error {
	message := fmt.Sprintf("이미지 레지스트리 확인에 실패했습니다. status=%d", status)
	switch status {
	case http.StatusNotFound:
		return imageNotFoundError(ref)
	case http.StatusUnauthorized, http.StatusForbidden:
		return authRequiredError(ref)
	}

	if code := extractRegistryErrorCode(payload); code != "" {
		message = fmt.Sprintf("%s (%s)", message, code)
	}
	return ImageValidationError{
		Code:     "IMAGE_CHECK_FAILED",
		Message:  message,
		Image:    ref.Original,
		Registry: ref.Registry,
	}
}

func extractRegistryErrorCode(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}

	var body struct {
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return ""
	}
	if len(body.Errors) == 0 {
		return ""
	}
	return body.Errors[0].Code
}
