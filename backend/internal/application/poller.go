package application

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/project"
	"gopkg.in/yaml.v3"
)

type AutoUpdatePoller struct {
	Service                 *Service
	Projects                *project.Service
	Interval                time.Duration
	Client                  *http.Client
	RollbackEvaluationDelay time.Duration
	RollbackMetricsRange    time.Duration
	RollbackMetricsStep     time.Duration
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

type remoteFileTarget struct {
	URL               string
	UseGitHubContents bool
}

type autoRollbackDecision struct {
	Message  string
	Metadata map[string]any
}

func (p *AutoUpdatePoller) Start(ctx context.Context) {
	if p.Interval <= 0 {
		p.Interval = 5 * time.Minute
	}
	if p.Client == nil {
		if p.Service != nil {
			p.Client = p.Service.httpClient()
		} else {
			p.Client = &http.Client{Timeout: 10 * time.Second}
		}
	}

	tickInterval := p.tickInterval()
	slog.Info("starting auto-update poller", "interval", p.Interval, "tickInterval", tickInterval)
	ticker := time.NewTicker(tickInterval)
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
	if p.Service != nil && p.Service.PollTracker != nil {
		p.Service.PollTracker.BeginCycle(timeNowUTC())
	}

	systemGroups := []string{"aods:platform:admin"}
	if p.Projects != nil {
		systemGroups = p.Projects.PlatformAdminAuthoritiesOrDefault()
	}
	systemUser := core.User{
		ID:          "system-poller",
		Username:    "system.poller",
		DisplayName: "AODS 자동 업데이트 봇",
		Groups:      systemGroups,
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

		now := timeNowUTC()
		for _, app := range apps {
			currentApp := app
			repo, ok := p.repositoryForApp(app, repoMap)
			if !ok {
				p.pollAutoRollback(ctx, systemUser, proj, currentApp)
				continue
			}
			if p.Service != nil && p.Service.PollTracker != nil && !p.Service.PollTracker.Due(app, now) {
				p.pollAutoRollback(ctx, systemUser, proj, currentApp)
				continue
			}

			p.pollApp(ctx, systemUser, proj, app, repo)
			updatedApp, err := p.Service.Store.GetApplication(ctx, app.ID)
			if err != nil {
				slog.Warn("poller failed to refresh application after repository sync", "app", app.ID, "error", err)
			} else {
				currentApp = updatedApp
			}

			p.pollAutoRollback(ctx, systemUser, proj, currentApp)
		}
	}
}

func (p *AutoUpdatePoller) pollApp(ctx context.Context, user core.User, proj project.CatalogProject, app Record, repo project.Repository) {
	if _, err := p.SyncRepositoryNow(ctx, user, proj, app, repo); err != nil {
		slog.Error("poller failed to sync repository state", "app", app.ID, "repoId", repo.ID, "error", err)
	}
}

func (p *AutoUpdatePoller) SyncRepositoryNow(
	ctx context.Context,
	user core.User,
	proj project.CatalogProject,
	app Record,
	repo project.Repository,
) (RepositorySyncResponse, error) {
	checkedAt := timeNowUTC()
	result := RepositorySyncResponse{
		ApplicationID: app.ID,
		CheckedAt:     checkedAt,
		Source:        repositoryPollSource(repo, app),
	}
	if p.Service != nil && p.Service.PollTracker != nil {
		p.Service.PollTracker.MarkAttempt(app, repo, checkedAt)
	}

	token := ""
	tokenPath := strings.TrimSpace(app.RepositoryTokenPath)
	if tokenPath == "" {
		tokenPath = strings.TrimSpace(repo.AuthSecretPath)
	}
	if tokenPath != "" && p.Service.Secrets != nil {
		secrets, err := p.Service.Secrets.Get(ctx, tokenPath)
		if err != nil {
			if p.Service != nil && p.Service.PollTracker != nil {
				p.Service.PollTracker.MarkFailure(app, repo, checkedAt, err)
			}
			return result, fmt.Errorf("read repository secret: %w", err)
		}
		if secrets != nil {
			token = resolveRepositoryToken(secrets)
		}
		if token == "" {
			err := fmt.Errorf("repository token was empty")
			if p.Service != nil && p.Service.PollTracker != nil {
				p.Service.PollTracker.MarkFailure(app, repo, checkedAt, err)
			}
			return result, err
		}
	}

	desired, source, err := p.resolveDesiredState(ctx, repo, app, token)
	if err != nil {
		if p.Service != nil && p.Service.PollTracker != nil {
			p.Service.PollTracker.MarkFailure(app, repo, checkedAt, err)
		}
		return result, fmt.Errorf("resolve repository desired state: %w", err)
	}
	if strings.TrimSpace(source) != "" {
		result.Source = source
	}
	if desired == nil {
		if p.Service != nil && p.Service.PollTracker != nil {
			p.Service.PollTracker.MarkSuccess(app, repo, checkedAt)
		}
		result.Message = fmt.Sprintf("%s 기준으로 저장소를 확인했지만 새 반영 내용은 없습니다.", result.Source)
		result.RepositoryPoll = p.repositoryPollSnapshot(app)
		return result, nil
	}

	ctx = WithChangeGuardBypass(ctx)
	updatedApp, appChanged, err := p.reconcileRepositoryState(ctx, user, app, *desired)
	if err != nil {
		if p.Service != nil && p.Service.PollTracker != nil {
			p.Service.PollTracker.MarkFailure(app, repo, checkedAt, err)
		}
		return result, fmt.Errorf("reconcile repository desired state: %w", err)
	}
	result.SettingsApplied = appChanged

	currentTag := extractImageTag(updatedApp.Image)
	desiredTag := extractImageTag(desired.Image)
	if desiredTag == "" || desiredTag == currentTag {
		if p.Service != nil && p.Service.PollTracker != nil {
			p.Service.PollTracker.MarkSuccess(updatedApp, repo, checkedAt)
		}
		if appChanged {
			_ = p.Service.appendEvent(ctx, updatedApp.ID, "RepositoryDescriptorApplied", fmt.Sprintf("저장소 %s의 서비스 설정을 반영했습니다.", repo.Name), map[string]any{
				"repositoryId":        repo.ID,
				"repositoryServiceId": desired.ServiceID,
				"source":              source,
			})
			result.Message = fmt.Sprintf("%s 기준으로 서비스 설정을 반영했습니다.", result.Source)
		} else {
			result.Message = fmt.Sprintf("%s 기준으로 저장소를 확인했지만 새 image tag 변경은 없습니다.", result.Source)
		}
		result.RepositoryPoll = p.repositoryPollSnapshot(updatedApp)
		return result, nil
	}

	slog.Info("auto-update triggered", "app", updatedApp.ID, "oldTag", currentTag, "newTag", desiredTag, "source", source)
	requestID := fmt.Sprintf("req_auto_%d", time.Now().Unix())
	if _, err := p.Service.CreateDeployment(ctx, user, updatedApp.ID, desiredTag, updatedApp.DefaultEnvironment, requestID); err != nil {
		if p.Service != nil && p.Service.PollTracker != nil {
			p.Service.PollTracker.MarkFailure(updatedApp, repo, checkedAt, err)
		}
		return result, fmt.Errorf("trigger deployment: %w", err)
	}

	if p.Service != nil && p.Service.PollTracker != nil {
		p.Service.PollTracker.MarkSuccess(updatedApp, repo, checkedAt)
	}

	_ = p.Service.appendEvent(ctx, updatedApp.ID, "AutoUpdateTriggered", fmt.Sprintf("저장소 %s의 변경을 감지하여 자동 배포를 수행합니다. (태그: %s)", repo.Name, desiredTag), map[string]any{
		"repositoryId":        repo.ID,
		"repositoryServiceId": desired.ServiceID,
		"newTag":              desiredTag,
		"source":              source,
	})
	result.DeploymentTriggered = true
	result.Message = fmt.Sprintf("%s 기준 변경을 감지해 %s 배포를 시작했습니다.", result.Source, desiredTag)
	result.RepositoryPoll = p.repositoryPollSnapshot(updatedApp)
	return result, nil
}

func (p *AutoUpdatePoller) tickInterval() time.Duration {
	if p.Interval > 0 && p.Interval < time.Minute {
		return p.Interval
	}
	return time.Minute
}

func (p *AutoUpdatePoller) repositoryPollSnapshot(app Record) *RepositoryPollStatus {
	if p == nil || p.Service == nil || p.Service.PollTracker == nil {
		return nil
	}
	return p.Service.PollTracker.Snapshot(app)
}

func (p *AutoUpdatePoller) pollAutoRollback(ctx context.Context, user core.User, proj project.CatalogProject, app Record) {
	if p.Service == nil || p.Service.Store == nil || p.Service.MetricsReader == nil {
		return
	}
	if !proj.Policies.AutoRollbackEnabled {
		return
	}

	policy, err := p.Service.Store.GetRollbackPolicy(ctx, app.ID)
	if err != nil {
		slog.Error("poller failed to read rollback policy", "app", app.ID, "error", err)
		return
	}
	if !policy.Enabled {
		return
	}

	deployments, err := p.Service.Store.ListDeployments(ctx, app.ID)
	if err != nil {
		slog.Error("poller failed to read deployment history", "app", app.ID, "error", err)
		return
	}

	latest, rollbackTarget, ok := selectAutoRollbackTarget(deployments, app.DefaultEnvironment)
	if !ok {
		return
	}
	if strings.EqualFold(strings.TrimSpace(latest.Status), "Aborted") || strings.EqualFold(strings.TrimSpace(latest.Status), "AutoRollbackTriggered") {
		return
	}

	delay := p.rollbackEvaluationDelay()
	if !latest.CreatedAt.IsZero() && delay > 0 && timeNowUTC().Sub(latest.CreatedAt) < delay {
		return
	}

	events, err := p.Service.Store.ListEvents(ctx, app.ID)
	if err != nil {
		slog.Error("poller failed to read application events", "app", app.ID, "error", err)
		return
	}
	if autoRollbackAlreadyTriggered(events, latest.DeploymentID) {
		return
	}

	metrics, err := p.Service.MetricsReader.Read(ctx, app, p.rollbackMetricRange(), p.rollbackMetricStep())
	if err != nil {
		slog.Error("poller failed to read rollback metrics", "app", app.ID, "error", err)
		return
	}

	decision, shouldRollback := evaluateAutoRollback(policy, metrics)
	if !shouldRollback {
		return
	}

	applyCtx := WithChangeGuardBypass(ctx)
	requestID := fmt.Sprintf("req_rollback_%d", timeNowUTC().UnixNano())
	if _, err := p.Service.CreateDeployment(applyCtx, user, app.ID, rollbackTarget.ImageTag, latest.Environment, requestID); err != nil {
		slog.Error("poller failed to trigger auto rollback", "app", app.ID, "error", err)
		_ = p.Service.appendEvent(ctx, app.ID, "AutoRollbackFailed", "자동 롤백 배포를 시작하지 못했습니다.", map[string]any{
			"sourceDeploymentId":     latest.DeploymentID,
			"rollbackToDeploymentId": rollbackTarget.DeploymentID,
			"rollbackToImageTag":     rollbackTarget.ImageTag,
			"reason":                 decision.Message,
		})
		return
	}

	latest.Status = "AutoRollbackTriggered"
	latest.Message = decision.Message
	latest.UpdatedAt = timeNowUTC()
	if _, err := p.Service.Store.UpdateDeployment(ctx, app.ID, latest); err != nil {
		slog.Error("poller failed to update deployment history after auto rollback", "app", app.ID, "deploymentId", latest.DeploymentID, "error", err)
	}

	metadata := copyMetadata(decision.Metadata)
	metadata["environment"] = latest.Environment
	metadata["sourceDeploymentId"] = latest.DeploymentID
	metadata["sourceImageTag"] = latest.ImageTag
	metadata["rollbackToDeploymentId"] = rollbackTarget.DeploymentID
	metadata["rollbackToImageTag"] = rollbackTarget.ImageTag

	_ = p.Service.appendEvent(ctx, app.ID, "AutoRollbackTriggered", fmt.Sprintf("자동 롤백을 수행했습니다. %s", decision.Message), metadata)
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
		target, err := p.resolveRepositoryFileTarget(repo, repo.ConfigFile, "", token)
		if err != nil {
			return nil, "", err
		}

		data, err := p.fetchRemoteFile(ctx, target, token)
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

	target, err := p.resolveRepositoryFileTarget(repo, app.ConfigPath, DefaultRepositoryConfigPath, token)
	if err != nil {
		return nil, "", err
	}

	config, err := p.fetchUpdateConfig(ctx, target, token)
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

func (p *AutoUpdatePoller) repositoryForApp(app Record, repoMap map[string]project.Repository) (project.Repository, bool) {
	if strings.TrimSpace(app.RepositoryURL) != "" {
		return project.Repository{
			ID:             app.RepositoryID,
			Name:           app.Name,
			URL:            app.RepositoryURL,
			Branch:         app.RepositoryBranch,
			ConfigFile:     app.ConfigPath,
			AuthSecretPath: app.RepositoryTokenPath,
		}, true
	}

	if strings.TrimSpace(app.RepositoryID) == "" {
		return project.Repository{}, false
	}

	repo, ok := repoMap[app.RepositoryID]
	if !ok {
		slog.Warn("application linked to non-existent repository", "app", app.ID, "repoId", app.RepositoryID)
		return project.Repository{}, false
	}
	return repo, true
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

func evaluateAutoRollback(policy RollbackPolicy, metrics []MetricSeries) (autoRollbackDecision, bool) {
	if !policy.Enabled {
		return autoRollbackDecision{}, false
	}

	requestRate := latestMetricValue(metrics, "request_rate")
	errorRate := latestMetricValue(metrics, "error_rate")
	latencyP95 := latestMetricValue(metrics, "latency_p95")

	if policy.MinRequestRate != nil {
		if requestRate == nil || *requestRate < *policy.MinRequestRate {
			return autoRollbackDecision{}, false
		}
	}

	breaches := make([]string, 0, 2)
	metadata := map[string]any{}
	if requestRate != nil {
		metadata["requestRate"] = *requestRate
	}
	if errorRate != nil {
		metadata["errorRate"] = *errorRate
	}
	if latencyP95 != nil {
		metadata["latencyP95Ms"] = *latencyP95
	}

	if policy.MaxErrorRate != nil && errorRate != nil && *errorRate > *policy.MaxErrorRate {
		breaches = append(breaches, fmt.Sprintf("에러율 %.2f%%가 임계치 %.2f%%를 초과했습니다.", *errorRate, *policy.MaxErrorRate))
		metadata["maxErrorRate"] = *policy.MaxErrorRate
	}
	if policy.MaxLatencyP95Ms != nil && latencyP95 != nil && *latencyP95 > float64(*policy.MaxLatencyP95Ms) {
		breaches = append(breaches, fmt.Sprintf("지연시간 P95 %.0fms가 임계치 %dms를 초과했습니다.", *latencyP95, *policy.MaxLatencyP95Ms))
		metadata["maxLatencyP95Ms"] = *policy.MaxLatencyP95Ms
	}

	if len(breaches) == 0 {
		return autoRollbackDecision{}, false
	}

	return autoRollbackDecision{
		Message:  strings.Join(breaches, " "),
		Metadata: metadata,
	}, true
}

func latestMetricValue(metrics []MetricSeries, key string) *float64 {
	for _, series := range metrics {
		if series.Key != key {
			continue
		}
		for idx := len(series.Points) - 1; idx >= 0; idx-- {
			if series.Points[idx].Value == nil {
				continue
			}
			value := *series.Points[idx].Value
			return &value
		}
	}
	return nil
}

func selectAutoRollbackTarget(deployments []DeploymentRecord, fallbackEnvironment string) (DeploymentRecord, DeploymentRecord, bool) {
	if len(deployments) < 2 {
		return DeploymentRecord{}, DeploymentRecord{}, false
	}

	latest := deployments[0]
	targetEnvironment := strings.TrimSpace(latest.Environment)
	if targetEnvironment == "" {
		targetEnvironment = strings.TrimSpace(fallbackEnvironment)
	}

	for _, candidate := range deployments[1:] {
		candidateEnvironment := strings.TrimSpace(candidate.Environment)
		if targetEnvironment != "" && candidateEnvironment != "" && candidateEnvironment != targetEnvironment {
			continue
		}
		if candidate.ImageTag == "" || candidate.ImageTag == latest.ImageTag {
			continue
		}
		return latest, candidate, true
	}

	return DeploymentRecord{}, DeploymentRecord{}, false
}

func autoRollbackAlreadyTriggered(events []Event, deploymentID string) bool {
	for _, event := range events {
		if event.Type != "AutoRollbackTriggered" || event.Metadata == nil {
			continue
		}
		sourceDeploymentID, _ := event.Metadata["sourceDeploymentId"].(string)
		if sourceDeploymentID == deploymentID {
			return true
		}
	}
	return false
}

func copyMetadata(values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}

	copied := make(map[string]any, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func (p *AutoUpdatePoller) rollbackEvaluationDelay() time.Duration {
	if p.RollbackEvaluationDelay > 0 {
		return p.RollbackEvaluationDelay
	}
	if p.Interval > 0 {
		return p.Interval
	}
	return 5 * time.Minute
}

func (p *AutoUpdatePoller) rollbackMetricRange() time.Duration {
	if p.RollbackMetricsRange > 0 {
		return p.RollbackMetricsRange
	}
	return 15 * time.Minute
}

func (p *AutoUpdatePoller) rollbackMetricStep() time.Duration {
	if p.RollbackMetricsStep > 0 {
		return p.RollbackMetricsStep
	}
	return time.Minute
}

func intPointer(value int) *int {
	return &value
}

func resolveRepositoryToken(values map[string]string) string {
	for _, key := range []string{"token", "pat", "github_pat", "githubPat", "github_token", "githubToken", "access_token", "accessToken"} {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	return ""
}

func (p *AutoUpdatePoller) resolveRepositoryFileTarget(repo project.Repository, relativePath string, defaultPath string, token string) (remoteFileTarget, error) {
	owner, name, ok := parseGitHubRepository(repo.URL)
	if !ok {
		return remoteFileTarget{}, fmt.Errorf("could not determine repository location for %s", repo.URL)
	}

	path := strings.TrimPrefix(strings.TrimSpace(relativePath), "/")
	if path == "" {
		path = strings.TrimPrefix(strings.TrimSpace(defaultPath), "/")
	}
	if path == "" {
		return remoteFileTarget{}, fmt.Errorf("repository path is required for %s", repo.URL)
	}

	branch := strings.TrimSpace(repo.Branch)
	if branch == "" {
		branch = "main"
	}

	if strings.TrimSpace(token) != "" {
		return remoteFileTarget{
			URL:               fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, name, path, url.QueryEscape(branch)),
			UseGitHubContents: true,
		}, nil
	}

	return remoteFileTarget{
		URL: fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, name, branch, path),
	}, nil
}

func parseGitHubRepository(repoURL string) (string, string, bool) {
	trimmed := strings.TrimSpace(repoURL)
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimPrefix(trimmed, "git@")
	trimmed = strings.TrimSuffix(trimmed, ".git")
	trimmed = strings.ReplaceAll(trimmed, ":", "/")

	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 || parts[0] != "github.com" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func (p *AutoUpdatePoller) fetchRemoteFile(ctx context.Context, target remoteFileTarget, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	if err != nil {
		return nil, err
	}
	if target.UseGitHubContents {
		req.Header.Set("Accept", "application/vnd.github.raw")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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

func (p *AutoUpdatePoller) fetchUpdateConfig(ctx context.Context, target remoteFileTarget, token string) (updateConfig, error) {
	data, err := p.fetchRemoteFile(ctx, target, token)
	if err != nil {
		return updateConfig{}, err
	}

	var config updateConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return updateConfig{}, fmt.Errorf("parse YAML: %w", err)
	}
	return config, nil
}
