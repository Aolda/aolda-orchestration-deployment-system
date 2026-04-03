package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/core"
	"gopkg.in/yaml.v3"
)

type LocalManifestStore struct {
	RepoRoot                   string
	FluxKustomizationNamespace string
	FluxSourceName             string
}

func (s LocalManifestStore) ListApplications(ctx context.Context, projectID string) ([]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(filepath.Join(s.RepoRoot, "apps", projectID))
	if errors.Is(err, os.ErrNotExist) {
		return []Record{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read application directory: %w", err)
	}

	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	sort.Strings(dirs)

	records := make([]Record, 0, len(dirs))
	for _, appName := range dirs {
		record, err := s.loadRecord(projectID, appName)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, nil
}

func (s LocalManifestStore) GetApplication(ctx context.Context, applicationID string) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}

	projectID, appName, err := splitApplicationID(applicationID)
	if err != nil {
		return Record{}, err
	}

	return s.loadRecord(projectID, appName)
}

func (s LocalManifestStore) CreateApplication(
	ctx context.Context,
	project ProjectContext,
	input CreateRequest,
	secretPath string,
) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}

	applicationDir := filepath.Join(s.RepoRoot, "apps", project.ID, input.Name)
	if _, err := os.Stat(applicationDir); err == nil {
		return Record{}, ErrConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return Record{}, fmt.Errorf("stat application directory: %w", err)
	}

	now := time.Now().UTC()
	defaultEnvironment := strings.TrimSpace(input.Environment)
	if defaultEnvironment == "" {
		if len(project.Environments) > 0 {
			defaultEnvironment = project.Environments[0]
		} else {
			defaultEnvironment = "prod"
		}
	}
	record := Record{
		ID:                  buildApplicationID(project.ID, input.Name),
		ProjectID:           project.ID,
		Namespace:           project.Namespace,
		Name:                input.Name,
		Description:         input.Description,
		Image:               input.Image,
		ServicePort:         input.ServicePort,
		Replicas:            desiredReplicas(input.Replicas, project.Policies.MinReplicas),
		RequiredProbes:      project.Policies.RequiredProbes,
		DeploymentStrategy:  input.DeploymentStrategy,
		DefaultEnvironment:  defaultEnvironment,
		CreatedAt:           now,
		UpdatedAt:           now,
		SecretPath:          secretPath,
		RepositoryID:        input.RepositoryID,
		RepositoryServiceID: input.RepositoryServiceID,
		ConfigPath:          input.ConfigPath,
	}
	if record.ConfigPath == "" {
		record.ConfigPath = "aolda-deploy.yaml"
	}

	environments := normalizedEnvironments(project.Environments, defaultEnvironment)
	if err := s.writeApplicationFiles(record, environments); err != nil {
		return Record{}, err
	}
	if err := s.syncFluxWiring(record, project, ""); err != nil {
		return Record{}, err
	}
	if err := writeMetadata(s.RepoRoot, record, environments); err != nil {
		return Record{}, err
	}
	initialDeployment := DeploymentRecord{
		DeploymentID:       initialDeploymentID(record),
		ApplicationID:      record.ID,
		ProjectID:          record.ProjectID,
		ApplicationName:    record.Name,
		Environment:        record.DefaultEnvironment,
		Image:              record.Image,
		ImageTag:           extractImageTag(record.Image),
		DeploymentStrategy: record.DeploymentStrategy,
		Status:             "Created",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := writeDeploymentRecord(s.RepoRoot, initialDeployment); err != nil {
		return Record{}, err
	}

	return record, nil
}

func (s LocalManifestStore) UpdateApplicationImage(ctx context.Context, project ProjectContext, applicationID string, imageTag string, deploymentID string) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}

	record, err := s.GetApplication(ctx, applicationID)
	if err != nil {
		return Record{}, err
	}

	record.Image = replaceImageTag(record.Image, imageTag)
	record.UpdatedAt = time.Now().UTC()
	environments := recordEnvironments(s.RepoRoot, record)
	if err := s.writeApplicationFiles(record, environments); err != nil {
		return Record{}, err
	}
	if err := s.syncFluxWiring(record, project, record.DefaultEnvironment); err != nil {
		return Record{}, err
	}
	if err := writeMetadata(s.RepoRoot, record, environments); err != nil {
		return Record{}, err
	}
	if err := writeDeploymentRecord(s.RepoRoot, DeploymentRecord{
		DeploymentID:       deploymentID,
		ApplicationID:      record.ID,
		ProjectID:          record.ProjectID,
		ApplicationName:    record.Name,
		Environment:        record.DefaultEnvironment,
		Image:              record.Image,
		ImageTag:           extractImageTag(record.Image),
		DeploymentStrategy: record.DeploymentStrategy,
		Status:             "Syncing",
		CreatedAt:          record.UpdatedAt,
		UpdatedAt:          record.UpdatedAt,
	}); err != nil {
		return Record{}, err
	}

	return record, nil
}

func (s LocalManifestStore) loadRecord(projectID string, appName string) (Record, error) {
	if metadata, err := readMetadata(s.RepoRoot, projectID, appName); err == nil {
		record := metadata.toRecord()
		if record.Replicas <= 0 {
			record.Replicas = 1
		}
		if record.DefaultEnvironment == "" {
			record.DefaultEnvironment = "prod"
		}
		return record, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return Record{}, err
	}

	deploymentPath := filepath.Join(s.RepoRoot, "apps", projectID, appName, "base", "deployment.yaml")
	rolloutPath := filepath.Join(s.RepoRoot, "apps", projectID, appName, "base", "rollout.yaml")
	servicePath := filepath.Join(s.RepoRoot, "apps", projectID, appName, "base", "service.yaml")
	externalSecretPath := filepath.Join(s.RepoRoot, "apps", projectID, appName, "base", "externalsecret.yaml")

	deploymentData, err := os.ReadFile(deploymentPath)
	useRollout := false
	if errors.Is(err, os.ErrNotExist) {
		deploymentData, err = os.ReadFile(rolloutPath)
		useRollout = true
	}
	if errors.Is(err, os.ErrNotExist) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("read deployment manifest: %w", err)
	}

	serviceData, err := os.ReadFile(servicePath)
	if err != nil {
		return Record{}, fmt.Errorf("read service manifest: %w", err)
	}

	var deployment deploymentManifest
	if err := yaml.Unmarshal(deploymentData, &deployment); err != nil {
		return Record{}, fmt.Errorf("decode workload manifest: %w", err)
	}

	var service serviceManifest
	if err := yaml.Unmarshal(serviceData, &service); err != nil {
		return Record{}, fmt.Errorf("decode service manifest: %w", err)
	}

	secretPath := ""
	if externalSecretData, err := os.ReadFile(externalSecretPath); err == nil {
		var externalSecret externalSecretManifest
		if err := yaml.Unmarshal(externalSecretData, &externalSecret); err != nil {
			return Record{}, fmt.Errorf("decode externalsecret manifest: %w", err)
		}
		secretPath = externalSecret.Metadata.Annotations["aods.io/vault-path"]
	} else if !errors.Is(err, os.ErrNotExist) {
		return Record{}, fmt.Errorf("read externalsecret manifest: %w", err)
	}

	annotations := deployment.Metadata.Annotations
	createdAt, _ := time.Parse(time.RFC3339, annotations["aods.io/created-at"])
	updatedAt, _ := time.Parse(time.RFC3339, annotations["aods.io/updated-at"])

	servicePort := 0
	if len(service.Spec.Ports) > 0 {
		servicePort = service.Spec.Ports[0].Port
	}

	image := ""
	requiredProbes := true
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		image = deployment.Spec.Template.Spec.Containers[0].Image
		requiredProbes = containerHasProbes(deployment.Spec.Template.Spec.Containers[0])
	}

	return Record{
		ID:                  buildApplicationID(projectID, appName),
		ProjectID:           projectID,
		Namespace:           deployment.Metadata.Namespace,
		Name:                deployment.Metadata.Name,
		Description:         annotations["aods.io/application-description"],
		Image:               image,
		ServicePort:         servicePort,
		Replicas:            maxInt(deployment.Spec.Replicas, 1),
		RequiredProbes:      requiredProbes,
		DeploymentStrategy:  inferStrategy(useRollout, annotations["aods.io/deployment-strategy"]),
		DefaultEnvironment:  "prod",
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
		SecretPath:          secretPath,
		RepositoryID:        annotations["aods.io/repository-id"],
		RepositoryServiceID: annotations["aods.io/repository-service-id"],
		ConfigPath:          annotations["aods.io/config-path"],
	}, nil
}

func (s LocalManifestStore) PatchApplication(ctx context.Context, project ProjectContext, applicationID string, input UpdateApplicationRequest) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}

	record, err := s.GetApplication(ctx, applicationID)
	if err != nil {
		return Record{}, err
	}
	previousEnvironment := record.DefaultEnvironment

	if input.Description != nil {
		record.Description = strings.TrimSpace(*input.Description)
	}
	if input.Image != nil {
		record.Image = strings.TrimSpace(*input.Image)
	}
	if input.ServicePort != nil {
		record.ServicePort = *input.ServicePort
	}
	if input.Replicas != nil {
		record.Replicas = *input.Replicas
	}
	if input.DeploymentStrategy != nil {
		record.DeploymentStrategy = *input.DeploymentStrategy
	}
	if input.Environment != nil && strings.TrimSpace(*input.Environment) != "" {
		record.DefaultEnvironment = strings.TrimSpace(*input.Environment)
	}
	if input.RepositoryID != nil {
		record.RepositoryID = *input.RepositoryID
	}
	if input.RepositoryServiceID != nil {
		record.RepositoryServiceID = *input.RepositoryServiceID
	}
	if input.ConfigPath != nil {
		record.ConfigPath = *input.ConfigPath
	}
	record.UpdatedAt = time.Now().UTC()

	environments := recordEnvironments(s.RepoRoot, record)
	if !containsEnvironment(environments, record.DefaultEnvironment) {
		environments = append(environments, record.DefaultEnvironment)
		sort.Strings(environments)
	}
	if err := s.writeApplicationFiles(record, environments); err != nil {
		return Record{}, err
	}
	if err := s.syncFluxWiring(record, project, previousEnvironment); err != nil {
		return Record{}, err
	}
	if err := writeMetadata(s.RepoRoot, record, environments); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s LocalManifestStore) ListDeployments(ctx context.Context, applicationID string) ([]DeploymentRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	projectID, appName, err := splitApplicationID(applicationID)
	if err != nil {
		return nil, err
	}
	return listDeploymentRecords(s.RepoRoot, projectID, appName)
}

func (s LocalManifestStore) GetDeployment(ctx context.Context, applicationID string, deploymentID string) (DeploymentRecord, error) {
	if err := ctx.Err(); err != nil {
		return DeploymentRecord{}, err
	}
	projectID, appName, err := splitApplicationID(applicationID)
	if err != nil {
		return DeploymentRecord{}, err
	}
	return readDeploymentRecord(s.RepoRoot, projectID, appName, deploymentID)
}

func (s LocalManifestStore) UpdateDeployment(ctx context.Context, applicationID string, deployment DeploymentRecord) (DeploymentRecord, error) {
	if err := ctx.Err(); err != nil {
		return DeploymentRecord{}, err
	}
	if deployment.ApplicationID == "" {
		deployment.ApplicationID = applicationID
	}
	if deployment.ProjectID == "" || deployment.ApplicationName == "" {
		projectID, appName, err := splitApplicationID(applicationID)
		if err != nil {
			return DeploymentRecord{}, err
		}
		deployment.ProjectID = projectID
		deployment.ApplicationName = appName
	}
	if err := writeDeploymentRecord(s.RepoRoot, deployment); err != nil {
		return DeploymentRecord{}, err
	}
	return deployment, nil
}

func (s LocalManifestStore) GetRollbackPolicy(ctx context.Context, applicationID string) (RollbackPolicy, error) {
	if err := ctx.Err(); err != nil {
		return RollbackPolicy{}, err
	}
	projectID, appName, err := splitApplicationID(applicationID)
	if err != nil {
		return RollbackPolicy{}, err
	}
	return readRollbackPolicy(s.RepoRoot, projectID, appName)
}

func (s LocalManifestStore) SaveRollbackPolicy(ctx context.Context, applicationID string, policy RollbackPolicy) (RollbackPolicy, error) {
	if err := ctx.Err(); err != nil {
		return RollbackPolicy{}, err
	}
	record, err := s.GetApplication(ctx, applicationID)
	if err != nil {
		return RollbackPolicy{}, err
	}
	if err := writeRollbackPolicy(s.RepoRoot, record, policy); err != nil {
		return RollbackPolicy{}, err
	}
	return policy, nil
}

func (s LocalManifestStore) ListEvents(ctx context.Context, applicationID string) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	projectID, appName, err := splitApplicationID(applicationID)
	if err != nil {
		return nil, err
	}
	return listEventRecords(s.RepoRoot, projectID, appName)
}

func (s LocalManifestStore) AppendEvent(ctx context.Context, applicationID string, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	projectID, appName, err := splitApplicationID(applicationID)
	if err != nil {
		return err
	}
	return writeEventRecord(s.RepoRoot, projectID, appName, event)
}

type deploymentManifest struct {
	Metadata struct {
		Name        string            `yaml:"name"`
		Namespace   string            `yaml:"namespace"`
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Spec struct {
		Replicas int `yaml:"replicas"`
		Template struct {
			Spec struct {
				Containers []struct {
					Image          string         `yaml:"image"`
					ReadinessProbe map[string]any `yaml:"readinessProbe"`
					LivenessProbe  map[string]any `yaml:"livenessProbe"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type serviceManifest struct {
	Spec struct {
		Ports []struct {
			Port int `yaml:"port"`
		} `yaml:"ports"`
	} `yaml:"spec"`
}

type externalSecretManifest struct {
	Metadata struct {
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
}

const (
	defaultMetricsPath      = "/metrics"
	defaultMetricsInterval  = "30s"
	defaultEnvoyMetricsPath = "/stats/prometheus"
	defaultEnvoyMetricsPort = 15090
)

func renderBaseKustomization(record Record) string {
	resources := []string{
		workloadFileName(record),
		"service.yaml",
		"servicemonitor.yaml",
		"virtualservice.yaml",
		"destinationrule.yaml",
	}
	if record.DeploymentStrategy == DeploymentStrategyCanary {
		resources = append(resources, "canary-service.yaml")
	}
	if record.SecretPath != "" {
		resources = append(resources, "externalsecret.yaml")
	}

	var builder strings.Builder
	builder.WriteString(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
`)
	for _, resource := range resources {
		builder.WriteString("  - ")
		builder.WriteString(resource)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func renderOverlayKustomization(environment string) string {
	return fmt.Sprintf(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
commonAnnotations:
  aods.io/environment: %s
`, yamlScalar(environment))
}

func workloadFileName(record Record) string {
	if record.DeploymentStrategy == DeploymentStrategyCanary {
		return "rollout.yaml"
	}
	return "deployment.yaml"
}

func renderWorkload(record Record) string {
	if record.DeploymentStrategy == DeploymentStrategyCanary {
		return renderRollout(record)
	}
	return renderDeployment(record)
}

func renderDeployment(record Record) string {
	envFromBlock := ""
	if record.SecretPath != "" {
		envFromBlock = fmt.Sprintf(`
          envFrom:
            - secretRef:
                name: %s-secrets`, record.Name)
	}
	probeBlock := renderProbeBlock(record)

	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    app.kubernetes.io/name: %s
    app.kubernetes.io/part-of: %s
  annotations:
    aods.io/project-id: %s
    aods.io/application-name: %s
    aods.io/application-description: %s
    aods.io/deployment-strategy: %s
    aods.io/repository-id: %s
    aods.io/repository-service-id: %s
    aods.io/config-path: %s
    aods.io/created-at: %s
    aods.io/updated-at: %s
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
      app.kubernetes.io/name: %s
  template:
    metadata:
      annotations:
        sidecar.istio.io/inject: "true"
      labels:
        sidecar.istio.io/inject: "true"
        app: %s
        app.kubernetes.io/name: %s
        app.kubernetes.io/part-of: %s
    spec:
      containers:
        - name: %s
          image: %s
          ports:
            - name: http
              containerPort: %d
%s
%s
`,
		record.Name,
		record.Namespace,
		record.Name,
		record.Name,
		record.ProjectID,
		record.ProjectID,
		record.Name,
		yamlScalar(record.Description),
		record.DeploymentStrategy,
		yamlScalar(record.RepositoryID),
		yamlScalar(record.RepositoryServiceID),
		yamlScalar(record.ConfigPath),
		record.CreatedAt.Format(time.RFC3339),
		record.UpdatedAt.Format(time.RFC3339),
		record.Replicas,
		record.Name,
		record.Name,
		record.Name,
		record.Name,
		record.ProjectID,
		record.Name,
		record.Image,
		record.ServicePort,
		probeBlock,
		envFromBlock,
	)
}

func renderRollout(record Record) string {
	envFromBlock := ""
	if record.SecretPath != "" {
		envFromBlock = fmt.Sprintf(`
          envFrom:
            - secretRef:
                name: %s-secrets`, record.Name)
	}
	probeBlock := renderProbeBlock(record)

	return fmt.Sprintf(`apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    app.kubernetes.io/name: %s
    app.kubernetes.io/part-of: %s
  annotations:
    aods.io/project-id: %s
    aods.io/application-name: %s
    aods.io/application-description: %s
    aods.io/deployment-strategy: %s
    aods.io/repository-id: %s
    aods.io/repository-service-id: %s
    aods.io/config-path: %s
    aods.io/created-at: %s
    aods.io/updated-at: %s
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
      app.kubernetes.io/name: %s
  template:
    metadata:
      annotations:
        sidecar.istio.io/inject: "true"
      labels:
        sidecar.istio.io/inject: "true"
        app: %s
        app.kubernetes.io/name: %s
        app.kubernetes.io/part-of: %s
    spec:
      containers:
        - name: %s
          image: %s
          ports:
            - name: http
              containerPort: %d
%s
%s
  strategy:
    canary:
      stableService: %s
      canaryService: %s-canary
      trafficRouting:
        istio:
          virtualService:
            name: %s
            routes:
              - primary
      steps:
        - setWeight: 5
        - pause: {}
        - setWeight: 25
        - pause: {}
        - setWeight: 50
        - pause: {}
        - setWeight: 100
`,
		record.Name,
		record.Namespace,
		record.Name,
		record.Name,
		record.ProjectID,
		record.ProjectID,
		record.Name,
		yamlScalar(record.Description),
		record.DeploymentStrategy,
		yamlScalar(record.RepositoryID),
		yamlScalar(record.RepositoryServiceID),
		yamlScalar(record.ConfigPath),
		record.CreatedAt.Format(time.RFC3339),
		record.UpdatedAt.Format(time.RFC3339),
		record.Replicas,
		record.Name,
		record.Name,
		record.Name,
		record.Name,
		record.ProjectID,
		record.Name,
		record.Image,
		record.ServicePort,
		probeBlock,
		envFromBlock,
		record.Name,
		record.Name,
		record.Name,
	)
}

func renderService(record Record) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    app.kubernetes.io/name: %s
    app.kubernetes.io/part-of: %s
    aods.io/metrics-scrape: "true"
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/path: %s
    prometheus.io/port: %s
spec:
  selector:
    app: %s
    app.kubernetes.io/name: %s
  ports:
    - name: http
      port: %d
      targetPort: %d
    - name: envoy-metrics
      port: %d
      targetPort: %d
`,
		record.Name,
		record.Namespace,
		record.Name,
		record.Name,
		record.ProjectID,
		yamlScalar(defaultMetricsPath),
		yamlScalar(fmt.Sprintf("%d", record.ServicePort)),
		record.Name,
		record.Name,
		record.ServicePort,
		record.ServicePort,
		defaultEnvoyMetricsPort,
		defaultEnvoyMetricsPort,
	)
}

func renderCanaryService(record Record) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s-canary
  namespace: %s
  labels:
    app: %s
    app.kubernetes.io/name: %s
    app.kubernetes.io/part-of: %s
spec:
  selector:
    app: %s
    app.kubernetes.io/name: %s
  ports:
    - name: http
      port: %d
      targetPort: %d
    - name: envoy-metrics
      port: %d
      targetPort: %d
`,
		record.Name,
		record.Namespace,
		record.Name,
		record.Name,
		record.ProjectID,
		record.Name,
		record.Name,
		record.ServicePort,
		record.ServicePort,
		defaultEnvoyMetricsPort,
		defaultEnvoyMetricsPort,
	)
}

func renderServiceMonitor(record Record) string {
	return fmt.Sprintf(`apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/name: %s
    app.kubernetes.io/part-of: %s
    prometheus: argo-cd-grafana
    release: kube-prometheus-stack
spec:
  namespaceSelector:
    matchNames:
      - %s
  selector:
    matchLabels:
      aods.io/metrics-scrape: "true"
      app.kubernetes.io/name: %s
  endpoints:
    - port: http
      path: %s
      interval: %s
    - port: envoy-metrics
      path: %s
      interval: %s
`,
		record.Name,
		record.Namespace,
		record.Name,
		record.ProjectID,
		record.Namespace,
		record.Name,
		yamlScalar(defaultMetricsPath),
		yamlScalar(defaultMetricsInterval),
		yamlScalar(defaultEnvoyMetricsPath),
		yamlScalar(defaultMetricsInterval),
	)
}

func renderVirtualService(record Record) string {
	host := fmt.Sprintf("%s.%s.svc.cluster.local", record.Name, record.Namespace)
	if record.DeploymentStrategy == DeploymentStrategyCanary {
		return fmt.Sprintf(`apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: %s
  namespace: %s
spec:
  hosts:
    - %s
  http:
    - name: primary
      route:
        - destination:
            host: %s
          weight: 100
        - destination:
            host: %s-canary.%s.svc.cluster.local
          weight: 0
`,
			record.Name,
			record.Namespace,
			host,
			host,
			record.Name,
			record.Namespace,
		)
	}
	return fmt.Sprintf(`apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: %s
  namespace: %s
spec:
  hosts:
    - %s
  http:
    - route:
        - destination:
            host: %s
            subset: stable
`,
		record.Name,
		record.Namespace,
		host,
		host,
	)
}

func renderDestinationRule(record Record) string {
	host := fmt.Sprintf("%s.%s.svc.cluster.local", record.Name, record.Namespace)
	if record.DeploymentStrategy == DeploymentStrategyCanary {
		return fmt.Sprintf(`apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: %s
  namespace: %s
spec:
  host: %s
  subsets:
    - name: stable
      labels:
        app.kubernetes.io/name: %s
    - name: canary
      labels:
        app.kubernetes.io/name: %s
`,
			record.Name,
			record.Namespace,
			host,
			record.Name,
			record.Name,
		)
	}
	return fmt.Sprintf(`apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: %s
  namespace: %s
spec:
  host: %s
  subsets:
    - name: stable
      labels:
        app.kubernetes.io/name: %s
`,
		record.Name,
		record.Namespace,
		host,
		record.Name,
	)
}

func renderExternalSecret(record Record) string {
	return fmt.Sprintf(`apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: %s
  namespace: %s
  annotations:
    aods.io/vault-path: %s
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aods-vault
    kind: ClusterSecretStore
  target:
    name: %s-secrets
    creationPolicy: Owner
  dataFrom:
    - extract:
        key: %s
`,
		record.Name,
		record.Namespace,
		record.SecretPath,
		record.Name,
		core.VaultExtractKey(record.SecretPath),
	)
}

func renderProbeBlock(record Record) string {
	if !record.RequiredProbes {
		return ""
	}
	return fmt.Sprintf(`          readinessProbe:
            httpGet:
              path: /
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /
              port: http
            initialDelaySeconds: 15
            periodSeconds: 20`)
}

func replaceImageTag(image string, tag string) string {
	withoutDigest := image
	if index := strings.Index(withoutDigest, "@"); index >= 0 {
		withoutDigest = withoutDigest[:index]
	}

	lastSlash := strings.LastIndex(withoutDigest, "/")
	lastColon := strings.LastIndex(withoutDigest, ":")
	if lastColon > lastSlash {
		return withoutDigest[:lastColon+1] + tag
	}

	return withoutDigest + ":" + tag
}

func yamlScalar(value string) string {
	data, err := yaml.Marshal(value)
	if err != nil {
		return `""`
	}
	return strings.TrimSpace(string(data))
}

func containerHasProbes(container struct {
	Image          string         `yaml:"image"`
	ReadinessProbe map[string]any `yaml:"readinessProbe"`
	LivenessProbe  map[string]any `yaml:"livenessProbe"`
}) bool {
	return len(container.ReadinessProbe) > 0 || len(container.LivenessProbe) > 0
}

func inferStrategy(useRollout bool, annotation string) DeploymentStrategy {
	if useRollout {
		return DeploymentStrategyCanary
	}
	if strings.TrimSpace(annotation) == "" {
		return DeploymentStrategyStandard
	}
	return DeploymentStrategy(annotation)
}

func maxInt(value int, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}

func desiredReplicas(value int, minimum int) int {
	if value > 0 {
		return value
	}
	return maxInt(minimum, 1)
}

func normalizedEnvironments(environments []string, defaultEnvironment string) []string {
	items := make([]string, 0, len(environments)+1)
	for _, environment := range environments {
		if strings.TrimSpace(environment) == "" || containsEnvironment(items, environment) {
			continue
		}
		items = append(items, environment)
	}
	if !containsEnvironment(items, defaultEnvironment) {
		items = append(items, defaultEnvironment)
	}
	if len(items) == 0 {
		items = append(items, "prod")
	}
	sort.Strings(items)
	return items
}

func containsEnvironment(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func (s LocalManifestStore) writeApplicationFiles(record Record, environments []string) error {
	applicationDir := filepath.Join(s.RepoRoot, "apps", record.ProjectID, record.Name)
	files := map[string]string{
		filepath.Join(applicationDir, "base", "kustomization.yaml"):     renderBaseKustomization(record),
		filepath.Join(applicationDir, "base", workloadFileName(record)): renderWorkload(record),
		filepath.Join(applicationDir, "base", "service.yaml"):           renderService(record),
		filepath.Join(applicationDir, "base", "servicemonitor.yaml"):    renderServiceMonitor(record),
		filepath.Join(applicationDir, "base", "virtualservice.yaml"):    renderVirtualService(record),
		filepath.Join(applicationDir, "base", "destinationrule.yaml"):   renderDestinationRule(record),
	}
	if record.DeploymentStrategy == DeploymentStrategyCanary {
		files[filepath.Join(applicationDir, "base", "canary-service.yaml")] = renderCanaryService(record)
	}
	if record.SecretPath != "" {
		files[filepath.Join(applicationDir, "base", "externalsecret.yaml")] = renderExternalSecret(record)
	}
	for _, environment := range normalizedEnvironments(environments, record.DefaultEnvironment) {
		files[filepath.Join(applicationDir, "overlays", environment, "kustomization.yaml")] = renderOverlayKustomization(environment)
	}

	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create manifest directory: %w", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write manifest %s: %w", path, err)
		}
	}

	alternateWorkload := filepath.Join(applicationDir, "base", "deployment.yaml")
	if record.DeploymentStrategy == DeploymentStrategyStandard {
		alternateWorkload = filepath.Join(applicationDir, "base", "rollout.yaml")
	}
	_ = os.Remove(alternateWorkload)
	if record.DeploymentStrategy != DeploymentStrategyCanary {
		_ = os.Remove(filepath.Join(applicationDir, "base", "canary-service.yaml"))
	}
	if record.SecretPath == "" {
		_ = os.Remove(filepath.Join(applicationDir, "base", "externalsecret.yaml"))
	}

	return nil
}

func recordEnvironments(repoRoot string, record Record) []string {
	metadata, err := readMetadata(repoRoot, record.ProjectID, record.Name)
	if err != nil || len(metadata.Environments) == 0 {
		return []string{record.DefaultEnvironment}
	}
	return normalizedEnvironments(metadata.Environments, record.DefaultEnvironment)
}

func extractImageTag(image string) string {
	withoutDigest := image
	if index := strings.Index(withoutDigest, "@"); index >= 0 {
		withoutDigest = withoutDigest[:index]
	}

	lastSlash := strings.LastIndex(withoutDigest, "/")
	lastColon := strings.LastIndex(withoutDigest, ":")
	if lastColon > lastSlash {
		return withoutDigest[lastColon+1:]
	}
	return withoutDigest
}

func initialDeploymentID(record Record) string {
	return fmt.Sprintf("dep_init_%d", record.CreatedAt.Unix())
}
