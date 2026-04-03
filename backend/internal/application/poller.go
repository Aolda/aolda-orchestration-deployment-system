package application

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/project"
	"gopkg.in/yaml.v3"
)

type AutoUpdatePoller struct {
	Service  *Service
	Projects *project.Service
	Interval time.Duration
	Client   *http.Client
}

type updateConfig struct {
	Version  string `yaml:"version"`
	ImageTag string `yaml:"imageTag"`
}

type repositoryDescriptor struct {
	Services []repositoryDescriptorService `json:"services"`
}

type repositoryDescriptorService struct {
	ServiceID string             `json:"serviceId"`
	Image     string             `json:"image"`
	Port      int                `json:"port"`
	Replicas  int                `json:"replicas"`
	Strategy  DeploymentStrategy `json:"strategy,omitempty"`
}

type desiredRepositoryState struct {
	ServiceID string
	Image     string
	Port      int
	Replicas  int
}

func (p *AutoUpdatePoller) Start(ctx context.Context) {
	if p.Interval <= 0 {
		p.Interval = 5 * time.Minute
	}
	if p.Client == nil {
		p.Client = &http.Client{Timeout: 10 * time.Second}
	}

	slog.Info("starting auto-update poller", "interval", p.Interval)
	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollAll(ctx)
		}
	}
}

func (p *AutoUpdatePoller) pollAll(ctx context.Context) {
	systemUser := core.User{
		ID:          "system-poller",
		Username:    "system.poller",
		DisplayName: "AODS 자동 업데이트 봇",
		Groups:      []string{"aods:platform:admin"},
	}

	projects, err := p.Projects.Source.ListProjects(ctx)
	if err != nil {
		slog.Error("poller failed to list projects", "error", err)
		return
	}

	for _, proj := range projects {
		apps, err := p.Service.Store.ListApplications(ctx, proj.ID)
		if err != nil {
			slog.Error("poller failed to list applications", "project", proj.ID, "error", err)
			continue
		}

		repoMap := make(map[string]project.Repository)
		for _, repo := range proj.Repositories {
			repoMap[repo.ID] = repo
		}

		for _, app := range apps {
			if app.RepositoryID == "" {
				continue
			}

			repo, ok := repoMap[app.RepositoryID]
			if !ok {
				slog.Warn("application linked to non-existent repository", "app", app.ID, "repoId", app.RepositoryID)
				continue
			}

			p.pollApp(ctx, systemUser, proj, app, repo)
		}
	}
}

func (p *AutoUpdatePoller) pollApp(ctx context.Context, user core.User, proj project.CatalogProject, app Record, repo project.Repository) {
	token := ""
	if repo.AuthSecretPath != "" && p.Service.Secrets != nil {
		secrets, err := p.Service.Secrets.Get(ctx, repo.AuthSecretPath)
		if err != nil {
			slog.Error("poller failed to read repository secret", "app", app.ID, "path", repo.AuthSecretPath, "error", err)
			return
		}
		if secrets != nil {
			token = secrets["token"]
		}
	}

	desired, source, err := p.resolveDesiredState(ctx, repo, app, token)
	if err != nil {
		slog.Error("poller failed to resolve repository desired state", "app", app.ID, "repoId", repo.ID, "error", err)
		return
	}
	if desired == nil {
		return
	}

	ctx = WithChangeGuardBypass(ctx)
	updatedApp, appChanged, err := p.reconcileRepositoryState(ctx, user, app, *desired)
	if err != nil {
		slog.Error("poller failed to reconcile repository desired state", "app", app.ID, "error", err)
		return
	}

	currentTag := extractImageTag(updatedApp.Image)
	desiredTag := extractImageTag(desired.Image)
	if desiredTag == "" || desiredTag == currentTag {
		if appChanged {
			_ = p.Service.appendEvent(ctx, updatedApp.ID, "RepositoryDescriptorApplied", fmt.Sprintf("저장소 %s의 서비스 설정을 반영했습니다.", repo.Name), map[string]any{
				"repositoryId":        repo.ID,
				"repositoryServiceId": desired.ServiceID,
				"source":              source,
			})
		}
		return
	}

	slog.Info("auto-update triggered", "app", updatedApp.ID, "oldTag", currentTag, "newTag", desiredTag, "source", source)
	requestID := fmt.Sprintf("req_auto_%d", time.Now().Unix())
	if _, err := p.Service.CreateDeployment(ctx, user, updatedApp.ID, desiredTag, updatedApp.DefaultEnvironment, requestID); err != nil {
		slog.Error("poller failed to trigger deployment", "app", updatedApp.ID, "error", err)
		return
	}

	_ = p.Service.appendEvent(ctx, updatedApp.ID, "AutoUpdateTriggered", fmt.Sprintf("저장소 %s의 변경을 감지하여 자동 배포를 수행합니다. (태그: %s)", repo.Name, desiredTag), map[string]any{
		"repositoryId":        repo.ID,
		"repositoryServiceId": desired.ServiceID,
		"newTag":              desiredTag,
		"source":              source,
	})
}

func (p *AutoUpdatePoller) reconcileRepositoryState(ctx context.Context, user core.User, app Record, desired desiredRepositoryState) (Record, bool, error) {
	var patch UpdateApplicationRequest
	changed := false

	if desired.Port > 0 && desired.Port != app.ServicePort {
		patch.ServicePort = intPointer(desired.Port)
		changed = true
	}
	if desired.Replicas > 0 && desired.Replicas != app.Replicas {
		patch.Replicas = intPointer(desired.Replicas)
		changed = true
	}
	if strings.TrimSpace(desired.ServiceID) != "" && desired.ServiceID != app.RepositoryServiceID {
		serviceID := desired.ServiceID
		patch.RepositoryServiceID = &serviceID
		changed = true
	}
	if desired.Image != "" && imageRepositoryRef(desired.Image) != imageRepositoryRef(app.Image) {
		image := desired.Image
		patch.Image = &image
		changed = true
	}
	if !changed {
		return app, false, nil
	}

	if _, err := p.Service.PatchApplication(ctx, user, app.ID, patch); err != nil {
		return Record{}, false, err
	}

	updated, err := p.Service.Store.GetApplication(ctx, app.ID)
	if err != nil {
		return Record{}, false, err
	}
	return updated, true, nil
}

func (p *AutoUpdatePoller) resolveDesiredState(ctx context.Context, repo project.Repository, app Record, token string) (*desiredRepositoryState, string, error) {
	if strings.TrimSpace(repo.ConfigFile) != "" {
		rawURL := p.convertToRawURL(repo.URL, repo.ConfigFile, "")
		if rawURL == "" {
			return nil, "", fmt.Errorf("could not determine descriptor raw URL for repository %s", repo.URL)
		}

		data, err := p.fetchRemoteFile(ctx, rawURL, token)
		if err == nil {
			descriptor, parseErr := parseRepositoryDescriptor(data)
			if parseErr != nil {
				return nil, "", fmt.Errorf("parse repository descriptor: %w", parseErr)
			}
			service, ok := descriptor.resolveService(app)
			if !ok {
				return nil, "", fmt.Errorf("repository descriptor does not define service for app %s", app.ID)
			}
			return &desiredRepositoryState{
				ServiceID: service.ServiceID,
				Image:     service.Image,
				Port:      service.Port,
				Replicas:  service.Replicas,
			}, repo.ConfigFile, nil
		}

		if strings.TrimSpace(app.ConfigPath) == "" {
			return nil, "", err
		}
	}

	rawURL := p.convertToRawURL(repo.URL, app.ConfigPath, "aolda-deploy.yaml")
	if rawURL == "" {
		return nil, "", fmt.Errorf("could not determine raw URL for repository %s", repo.URL)
	}

	config, err := p.fetchUpdateConfig(ctx, rawURL, token)
	if err != nil {
		return nil, "", err
	}
	if config.ImageTag == "" {
		return nil, app.ConfigPath, nil
	}

	return &desiredRepositoryState{
		Image: replaceImageTag(app.Image, config.ImageTag),
	}, app.ConfigPath, nil
}

func parseRepositoryDescriptor(data []byte) (repositoryDescriptor, error) {
	var descriptor repositoryDescriptor
	if err := json.Unmarshal(data, &descriptor); err != nil {
		return repositoryDescriptor{}, err
	}
	if len(descriptor.Services) == 0 {
		return repositoryDescriptor{}, fmt.Errorf("services is required")
	}

	for _, service := range descriptor.Services {
		if !slugPattern.MatchString(strings.TrimSpace(service.ServiceID)) {
			return repositoryDescriptor{}, fmt.Errorf("serviceId must be a DNS-1123 style slug")
		}
		if strings.TrimSpace(service.Image) == "" {
			return repositoryDescriptor{}, fmt.Errorf("image is required for service %s", service.ServiceID)
		}
		if service.Port < 1 || service.Port > 65535 {
			return repositoryDescriptor{}, fmt.Errorf("port must be between 1 and 65535 for service %s", service.ServiceID)
		}
		if service.Replicas < 1 {
			return repositoryDescriptor{}, fmt.Errorf("replicas must be at least 1 for service %s", service.ServiceID)
		}
	}

	return descriptor, nil
}

func (d repositoryDescriptor) resolveService(app Record) (repositoryDescriptorService, bool) {
	targetServiceID := strings.TrimSpace(app.RepositoryServiceID)
	if targetServiceID == "" {
		targetServiceID = strings.TrimSpace(app.Name)
	}
	if targetServiceID != "" {
		for _, service := range d.Services {
			if service.ServiceID == targetServiceID {
				return service, true
			}
		}
	}
	if len(d.Services) == 1 {
		return d.Services[0], true
	}
	return repositoryDescriptorService{}, false
}

func imageRepositoryRef(image string) string {
	withoutDigest := image
	if index := strings.Index(withoutDigest, "@"); index >= 0 {
		withoutDigest = withoutDigest[:index]
	}

	lastSlash := strings.LastIndex(withoutDigest, "/")
	lastColon := strings.LastIndex(withoutDigest, ":")
	if lastColon > lastSlash {
		return withoutDigest[:lastColon]
	}
	return withoutDigest
}

func intPointer(value int) *int {
	return &value
}

func (p *AutoUpdatePoller) convertToRawURL(repoURL string, relativePath string, defaultPath string) string {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(repoURL), "https://"), ".git")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 || parts[0] != "github.com" {
		return ""
	}

	path := strings.TrimSpace(relativePath)
	if path == "" {
		path = defaultPath
	}
	if path == "" {
		return ""
	}

	org := parts[1]
	repo := parts[2]
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s", org, repo, strings.TrimPrefix(path, "/"))
}

func (p *AutoUpdatePoller) fetchRemoteFile(ctx context.Context, url string, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (p *AutoUpdatePoller) fetchUpdateConfig(ctx context.Context, url string, token string) (updateConfig, error) {
	data, err := p.fetchRemoteFile(ctx, url, token)
	if err != nil {
		return updateConfig{}, err
	}

	var config updateConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return updateConfig{}, fmt.Errorf("parse YAML: %w", err)
	}
	return config, nil
}
