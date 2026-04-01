package kubernetes

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/core"
	"gopkg.in/yaml.v3"
)

const fluxKustomizationResourcePath = "/apis/kustomize.toolkit.fluxcd.io/v1"

type ErrorSyncStatusReader struct {
	Err error
}

func (r ErrorSyncStatusReader) Read(ctx context.Context, record application.Record) (application.SyncInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.SyncInfo{}, err
	}
	if r.Err == nil {
		return application.SyncInfo{}, nil
	}
	return application.SyncInfo{}, r.Err
}

type FluxSyncStatusReader struct {
	Client                 *apiClient
	KustomizationNamespace string
}

func NewSyncStatusReader(cfg core.Config) application.StatusReader {
	if !cfg.UseKubernetesAPI() {
		return LocalSyncStatusReader{}
	}

	reader, err := NewFluxSyncStatusReader(cfg)
	if err != nil {
		return ErrorSyncStatusReader{Err: err}
	}

	return reader
}

func NewFluxSyncStatusReader(cfg core.Config) (FluxSyncStatusReader, error) {
	client, err := newAPIClient(cfg)
	if err != nil {
		return FluxSyncStatusReader{}, err
	}

	return FluxSyncStatusReader{
		Client:                 client,
		KustomizationNamespace: strings.TrimSpace(cfg.FluxKustomizationNamespace),
	}, nil
}

func (r FluxSyncStatusReader) Read(ctx context.Context, record application.Record) (application.SyncInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.SyncInfo{}, err
	}
	if r.Client == nil {
		return application.SyncInfo{}, fmt.Errorf("kubernetes api client is not configured")
	}

	response, err := r.listKustomizations(ctx)
	if err != nil {
		return application.SyncInfo{}, err
	}

	item, ok := selectKustomization(response.Items, record)
	if !ok {
		now := time.Now().UTC()
		return application.SyncInfo{
			Status:     application.SyncStatusUnknown,
			Message:    fmt.Sprintf("Flux Kustomization for %s was not found.", desiredFluxPath(record)),
			ObservedAt: now,
		}, nil
	}

	return mapSyncInfo(item), nil
}

func (r FluxSyncStatusReader) listKustomizations(ctx context.Context) (kustomizationListResponse, error) {
	resourcePath := fluxKustomizationResourcePath + "/kustomizations"
	namespace := strings.TrimSpace(r.KustomizationNamespace)
	if namespace != "" {
		resourcePath = fluxKustomizationResourcePath + "/namespaces/" + url.PathEscape(namespace) + "/kustomizations"
	}

	var response kustomizationListResponse
	if err := r.Client.GetJSON(ctx, resourcePath, &response); err != nil {
		return kustomizationListResponse{}, err
	}

	return response, nil
}

type apiClient struct {
	BaseURL             string
	BearerTokenProvider func(context.Context) (string, error)
	HTTPClient          *http.Client
}

func newAPIClient(cfg core.Config) (*apiClient, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.KubernetesMode)) {
	case "", "local":
		return nil, fmt.Errorf("kubernetes api mode is local")
	case "token":
		return newTokenAPIClient(cfg)
	case "kubeconfig":
		return newKubeconfigAPIClient(cfg)
	default:
		return nil, fmt.Errorf("unsupported kubernetes mode %q", cfg.KubernetesMode)
	}
}

func newTokenAPIClient(cfg core.Config) (*apiClient, error) {
	baseURL := strings.TrimSpace(cfg.KubernetesAPIURL)
	if baseURL == "" {
		return nil, fmt.Errorf("AODS_K8S_API_URL is required when AODS_K8S_MODE=token")
	}
	token := strings.TrimSpace(cfg.KubernetesBearerToken)
	if token == "" {
		return nil, fmt.Errorf("AODS_K8S_BEARER_TOKEN is required when AODS_K8S_MODE=token")
	}

	httpClient, err := newHTTPClient(cfg.KubernetesRequestTimeout, cfg.KubernetesCAFile, cfg.KubernetesCAData, false, nil)
	if err != nil {
		return nil, err
	}

	return &apiClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		BearerTokenProvider: func(context.Context) (string, error) {
			return token, nil
		},
		HTTPClient: httpClient,
	}, nil
}

func newKubeconfigAPIClient(cfg core.Config) (*apiClient, error) {
	configPath := strings.TrimSpace(cfg.KubernetesKubeconfigPath)
	if configPath == "" {
		return nil, fmt.Errorf("AODS_K8S_KUBECONFIG is required when AODS_K8S_MODE=kubeconfig")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read kubeconfig: %w", err)
	}

	var kubeconfig kubeconfigDocument
	if err := yaml.Unmarshal(data, &kubeconfig); err != nil {
		return nil, fmt.Errorf("decode kubeconfig: %w", err)
	}

	selectedContextName := strings.TrimSpace(cfg.KubernetesContext)
	if selectedContextName == "" {
		selectedContextName = strings.TrimSpace(kubeconfig.CurrentContext)
	}
	if selectedContextName == "" {
		return nil, fmt.Errorf("kubeconfig current-context is empty")
	}

	selectedContext, err := kubeconfig.contextByName(selectedContextName)
	if err != nil {
		return nil, err
	}
	selectedCluster, err := kubeconfig.clusterByName(selectedContext.Context.Cluster)
	if err != nil {
		return nil, err
	}
	selectedUser, err := kubeconfig.userByName(selectedContext.Context.User)
	if err != nil {
		return nil, err
	}

	clientCertificate, err := selectedUser.User.resolveClientCertificate()
	if err != nil {
		return nil, err
	}
	bearerTokenProvider := selectedUser.User.resolveBearerTokenProvider()
	if bearerTokenProvider == nil && clientCertificate == nil {
		return nil, fmt.Errorf("kubeconfig user does not provide exec, token, token-file, or client certificate credentials")
	}

	httpClient, err := newHTTPClient(
		cfg.KubernetesRequestTimeout,
		selectedCluster.Cluster.CertificateAuthority,
		selectedCluster.Cluster.CertificateAuthorityData,
		selectedCluster.Cluster.InsecureSkipTLSVerify,
		clientCertificate,
	)
	if err != nil {
		return nil, err
	}

	return &apiClient{
		BaseURL:             strings.TrimRight(selectedCluster.Cluster.Server, "/"),
		BearerTokenProvider: bearerTokenProvider,
		HTTPClient:          httpClient,
	}, nil
}

func newHTTPClient(
	timeout time.Duration,
	caFile string,
	caData string,
	insecureSkipTLSVerify bool,
	clientCertificate *tls.Certificate,
) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: insecureSkipTLSVerify,
	}
	if clientCertificate != nil {
		transport.TLSClientConfig.Certificates = []tls.Certificate{*clientCertificate}
	}

	if !insecureSkipTLSVerify {
		rootCAs, err := x509.SystemCertPool()
		if err != nil || rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}

		if strings.TrimSpace(caFile) != "" {
			pem, err := os.ReadFile(caFile)
			if err != nil {
				return nil, fmt.Errorf("read kubernetes CA file: %w", err)
			}
			if !rootCAs.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("append kubernetes CA file: no certificates found")
			}
		}

		if strings.TrimSpace(caData) != "" {
			pem, err := base64.StdEncoding.DecodeString(caData)
			if err != nil {
				return nil, fmt.Errorf("decode kubernetes CA data: %w", err)
			}
			if !rootCAs.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("append kubernetes CA data: no certificates found")
			}
		}

		transport.TLSClientConfig.RootCAs = rootCAs
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}

func (c *apiClient) GetJSON(ctx context.Context, resourcePath string, target any) error {
	if c == nil {
		return fmt.Errorf("kubernetes api client is not configured")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+resourcePath, nil)
	if err != nil {
		return fmt.Errorf("create kubernetes api request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if c.BearerTokenProvider != nil {
		token, err := c.BearerTokenProvider(ctx)
		if err != nil {
			return fmt.Errorf("resolve kubernetes api bearer token: %w", err)
		}
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
	}

	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("perform kubernetes api request: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read kubernetes api response: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = response.Status
		}
		return fmt.Errorf("kubernetes api %s failed with status %d: %s", resourcePath, response.StatusCode, message)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode kubernetes api response: %w", err)
	}

	return nil
}

type kubeconfigDocument struct {
	CurrentContext string               `yaml:"current-context"`
	Clusters       []namedKubeCluster   `yaml:"clusters"`
	Contexts       []namedKubeContext   `yaml:"contexts"`
	Users          []namedKubeUserEntry `yaml:"users"`
}

type namedKubeCluster struct {
	Name    string      `yaml:"name"`
	Cluster kubeCluster `yaml:"cluster"`
}

type kubeCluster struct {
	Server                   string `yaml:"server"`
	CertificateAuthority     string `yaml:"certificate-authority"`
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
	InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify"`
}

type namedKubeContext struct {
	Name    string      `yaml:"name"`
	Context kubeContext `yaml:"context"`
}

type kubeContext struct {
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
}

type namedKubeUserEntry struct {
	Name string   `yaml:"name"`
	User kubeUser `yaml:"user"`
}

type kubeUser struct {
	Token                 string           `yaml:"token"`
	TokenFile             string           `yaml:"token-file"`
	ClientCertificate     string           `yaml:"client-certificate"`
	ClientCertificateData string           `yaml:"client-certificate-data"`
	ClientKey             string           `yaml:"client-key"`
	ClientKeyData         string           `yaml:"client-key-data"`
	Exec                  *kubeExecCommand `yaml:"exec"`
}

type kubeExecCommand struct {
	Command string        `yaml:"command"`
	Args    []string      `yaml:"args"`
	Env     []kubeExecEnv `yaml:"env"`
}

type kubeExecEnv struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type execCredentialResponse struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}

func (c kubeconfigDocument) contextByName(name string) (namedKubeContext, error) {
	for _, item := range c.Contexts {
		if item.Name == name {
			return item, nil
		}
	}
	return namedKubeContext{}, fmt.Errorf("kubeconfig context %q was not found", name)
}

func (c kubeconfigDocument) clusterByName(name string) (namedKubeCluster, error) {
	for _, item := range c.Clusters {
		if item.Name == name {
			return item, nil
		}
	}
	return namedKubeCluster{}, fmt.Errorf("kubeconfig cluster %q was not found", name)
}

func (c kubeconfigDocument) userByName(name string) (namedKubeUserEntry, error) {
	for _, item := range c.Users {
		if item.Name == name {
			return item, nil
		}
	}
	return namedKubeUserEntry{}, fmt.Errorf("kubeconfig user %q was not found", name)
}

func (u kubeUser) resolveBearerTokenProvider() func(context.Context) (string, error) {
	if strings.TrimSpace(u.Token) != "" {
		token := strings.TrimSpace(u.Token)
		return func(context.Context) (string, error) {
			return token, nil
		}
	}

	if strings.TrimSpace(u.TokenFile) != "" {
		tokenFile := strings.TrimSpace(u.TokenFile)
		return func(context.Context) (string, error) {
			data, err := os.ReadFile(tokenFile)
			if err != nil {
				return "", fmt.Errorf("read kubeconfig token file: %w", err)
			}
			return strings.TrimSpace(string(data)), nil
		}
	}

	if u.Exec != nil {
		return func(ctx context.Context) (string, error) {
			return u.Exec.resolveToken(ctx)
		}
	}

	return nil
}

func (u kubeUser) resolveClientCertificate() (*tls.Certificate, error) {
	certPEM, err := u.readClientCertificatePEM()
	if err != nil {
		return nil, err
	}
	keyPEM, err := u.readClientKeyPEM()
	if err != nil {
		return nil, err
	}
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return nil, nil
	}

	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig client certificate: %w", err)
	}

	return &certificate, nil
}

func (u kubeUser) readClientCertificatePEM() ([]byte, error) {
	if strings.TrimSpace(u.ClientCertificateData) != "" {
		pem, err := base64.StdEncoding.DecodeString(strings.TrimSpace(u.ClientCertificateData))
		if err != nil {
			return nil, fmt.Errorf("decode kubeconfig client-certificate-data: %w", err)
		}
		return pem, nil
	}

	if strings.TrimSpace(u.ClientCertificate) != "" {
		pem, err := os.ReadFile(strings.TrimSpace(u.ClientCertificate))
		if err != nil {
			return nil, fmt.Errorf("read kubeconfig client-certificate: %w", err)
		}
		return pem, nil
	}

	return nil, nil
}

func (u kubeUser) readClientKeyPEM() ([]byte, error) {
	if strings.TrimSpace(u.ClientKeyData) != "" {
		pem, err := base64.StdEncoding.DecodeString(strings.TrimSpace(u.ClientKeyData))
		if err != nil {
			return nil, fmt.Errorf("decode kubeconfig client-key-data: %w", err)
		}
		return pem, nil
	}

	if strings.TrimSpace(u.ClientKey) != "" {
		pem, err := os.ReadFile(strings.TrimSpace(u.ClientKey))
		if err != nil {
			return nil, fmt.Errorf("read kubeconfig client-key: %w", err)
		}
		return pem, nil
	}

	return nil, nil
}

func (e kubeExecCommand) resolveToken(ctx context.Context) (string, error) {
	if strings.TrimSpace(e.Command) == "" {
		return "", fmt.Errorf("kubeconfig exec command is empty")
	}

	cmd := exec.CommandContext(ctx, e.Command, e.Args...)
	cmd.Env = append(os.Environ(), renderExecEnv(e.Env)...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("execute kubeconfig auth command: %w: %s", err, strings.TrimSpace(string(output)))
	}

	var credential execCredentialResponse
	if err := json.Unmarshal(output, &credential); err != nil {
		return "", fmt.Errorf("decode kubeconfig exec credential: %w", err)
	}
	if strings.TrimSpace(credential.Status.Token) == "" {
		return "", fmt.Errorf("kubeconfig exec credential did not include a token")
	}

	return strings.TrimSpace(credential.Status.Token), nil
}

func renderExecEnv(values []kubeExecEnv) []string {
	rendered := make([]string, 0, len(values))
	for _, item := range values {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		rendered = append(rendered, item.Name+"="+item.Value)
	}
	return rendered
}

type kustomizationListResponse struct {
	Items []fluxKustomization `json:"items"`
}

type fluxKustomization struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Path            string `json:"path"`
		TargetNamespace string `json:"targetNamespace"`
	} `json:"spec"`
	Status struct {
		Conditions []fluxCondition `json:"conditions"`
	} `json:"status"`
}

type fluxCondition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Reason             string `json:"reason"`
	Message            string `json:"message"`
	LastTransitionTime string `json:"lastTransitionTime"`
}

func selectKustomization(items []fluxKustomization, record application.Record) (fluxKustomization, bool) {
	desiredPathValue := desiredFluxPath(record)

	var matches []fluxKustomization
	for _, item := range items {
		if normalizeFluxPath(item.Spec.Path) == desiredPathValue {
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		return fluxKustomization{}, false
	}
	if len(matches) == 1 {
		return matches[0], true
	}

	for _, item := range matches {
		if strings.TrimSpace(item.Spec.TargetNamespace) == record.Namespace {
			return item, true
		}
	}

	return matches[0], true
}

func desiredFluxPath(record application.Record) string {
	return normalizeFluxPath(path.Join("apps", record.ProjectID, record.Name, "overlays", "prod"))
}

func normalizeFluxPath(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "./")
	trimmed = strings.TrimPrefix(trimmed, "/")
	trimmed = path.Clean(trimmed)
	if trimmed == "." {
		return ""
	}
	return trimmed
}

func mapSyncInfo(item fluxKustomization) application.SyncInfo {
	ready := findCondition(item.Status.Conditions, "Ready")
	reconciling := findCondition(item.Status.Conditions, "Reconciling")
	stalled := findCondition(item.Status.Conditions, "Stalled")

	switch {
	case conditionIsTrue(ready) && ready.Reason == "ReconciliationSucceeded":
		return buildSyncInfo(application.SyncStatusSynced, *ready)
	case conditionIsFalse(ready):
		return buildSyncInfo(application.SyncStatusDegraded, *ready)
	case conditionIsTrue(stalled):
		return buildSyncInfo(application.SyncStatusDegraded, *stalled)
	case conditionIsTrue(reconciling):
		return buildSyncInfo(application.SyncStatusSyncing, *reconciling)
	case conditionStatusEquals(ready, "Unknown"):
		return buildSyncInfo(application.SyncStatusSyncing, *ready)
	default:
		condition := firstNonEmptyCondition(ready, reconciling, stalled)
		if condition == nil {
			return application.SyncInfo{
				Status:     application.SyncStatusUnknown,
				Message:    "Flux Kustomization does not expose usable status conditions yet.",
				ObservedAt: time.Now().UTC(),
			}
		}
		return buildSyncInfo(application.SyncStatusUnknown, *condition)
	}
}

func buildSyncInfo(status application.SyncStatus, condition fluxCondition) application.SyncInfo {
	observedAt, err := time.Parse(time.RFC3339, condition.LastTransitionTime)
	if err != nil || observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	message := strings.TrimSpace(condition.Message)
	if message == "" {
		message = strings.TrimSpace(condition.Reason)
	}
	if message == "" {
		message = "Flux Kustomization status condition was read successfully."
	}

	return application.SyncInfo{
		Status:     status,
		Message:    message,
		ObservedAt: observedAt,
	}
}

func findCondition(conditions []fluxCondition, kind string) *fluxCondition {
	for _, item := range conditions {
		if item.Type == kind {
			condition := item
			return &condition
		}
	}
	return nil
}

func firstNonEmptyCondition(conditions ...*fluxCondition) *fluxCondition {
	for _, item := range conditions {
		if item != nil {
			return item
		}
	}
	return nil
}

func conditionIsTrue(condition *fluxCondition) bool {
	return conditionStatusEquals(condition, "True")
}

func conditionIsFalse(condition *fluxCondition) bool {
	return conditionStatusEquals(condition, "False")
}

func conditionStatusEquals(condition *fluxCondition, expected string) bool {
	return condition != nil && strings.EqualFold(strings.TrimSpace(condition.Status), expected)
}
