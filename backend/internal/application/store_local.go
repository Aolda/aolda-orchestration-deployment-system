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
	RepoRoot string
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
	record := Record{
		ID:                 buildApplicationID(project.ID, input.Name),
		ProjectID:          project.ID,
		Namespace:          project.Namespace,
		Name:               input.Name,
		Description:        input.Description,
		Image:              input.Image,
		ServicePort:        input.ServicePort,
		DeploymentStrategy: input.DeploymentStrategy,
		CreatedAt:          now,
		UpdatedAt:          now,
		SecretPath:         secretPath,
	}

	files := map[string]string{
		filepath.Join(applicationDir, "base", "kustomization.yaml"):             renderBaseKustomization(record.SecretPath != ""),
		filepath.Join(applicationDir, "base", "deployment.yaml"):                renderDeployment(record),
		filepath.Join(applicationDir, "base", "service.yaml"):                   renderService(record),
		filepath.Join(applicationDir, "base", "virtualservice.yaml"):            renderVirtualService(record),
		filepath.Join(applicationDir, "base", "destinationrule.yaml"):           renderDestinationRule(record),
		filepath.Join(applicationDir, "overlays", "prod", "kustomization.yaml"): renderOverlayKustomization(),
	}
	if record.SecretPath != "" {
		files[filepath.Join(applicationDir, "base", "externalsecret.yaml")] = renderExternalSecret(record)
	}

	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return Record{}, fmt.Errorf("create manifest directory: %w", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return Record{}, fmt.Errorf("write manifest %s: %w", path, err)
		}
	}

	return record, nil
}

func (s LocalManifestStore) UpdateApplicationImage(ctx context.Context, applicationID string, imageTag string) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}

	record, err := s.GetApplication(ctx, applicationID)
	if err != nil {
		return Record{}, err
	}

	record.Image = replaceImageTag(record.Image, imageTag)
	record.UpdatedAt = time.Now().UTC()

	deploymentPath := filepath.Join(s.RepoRoot, "apps", record.ProjectID, record.Name, "base", "deployment.yaml")
	if err := os.WriteFile(deploymentPath, []byte(renderDeployment(record)), 0o644); err != nil {
		return Record{}, fmt.Errorf("write deployment manifest: %w", err)
	}

	return record, nil
}

func (s LocalManifestStore) loadRecord(projectID string, appName string) (Record, error) {
	deploymentPath := filepath.Join(s.RepoRoot, "apps", projectID, appName, "base", "deployment.yaml")
	servicePath := filepath.Join(s.RepoRoot, "apps", projectID, appName, "base", "service.yaml")
	externalSecretPath := filepath.Join(s.RepoRoot, "apps", projectID, appName, "base", "externalsecret.yaml")

	deploymentData, err := os.ReadFile(deploymentPath)
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
		return Record{}, fmt.Errorf("decode deployment manifest: %w", err)
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
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		image = deployment.Spec.Template.Spec.Containers[0].Image
	}

	return Record{
		ID:                 buildApplicationID(projectID, appName),
		ProjectID:          projectID,
		Namespace:          deployment.Metadata.Namespace,
		Name:               deployment.Metadata.Name,
		Description:        annotations["aods.io/application-description"],
		Image:              image,
		ServicePort:        servicePort,
		DeploymentStrategy: DeploymentStrategy(annotations["aods.io/deployment-strategy"]),
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
		SecretPath:         secretPath,
	}, nil
}

type deploymentManifest struct {
	Metadata struct {
		Name        string            `yaml:"name"`
		Namespace   string            `yaml:"namespace"`
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Image string `yaml:"image"`
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

func renderBaseKustomization(includeExternalSecret bool) string {
	resources := []string{
		"deployment.yaml",
		"service.yaml",
		"virtualservice.yaml",
		"destinationrule.yaml",
	}
	if includeExternalSecret {
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

func renderOverlayKustomization() string {
	return `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
`
}

func renderDeployment(record Record) string {
	envFromBlock := ""
	if record.SecretPath != "" {
		envFromBlock = fmt.Sprintf(`
          envFrom:
            - secretRef:
                name: %s-secrets`, record.Name)
	}

	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/name: %s
    app.kubernetes.io/part-of: %s
  annotations:
    aods.io/project-id: %s
    aods.io/application-name: %s
    aods.io/application-description: %s
    aods.io/deployment-strategy: %s
    aods.io/created-at: %s
    aods.io/updated-at: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: %s
  template:
    metadata:
      labels:
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
`,
		record.Name,
		record.Namespace,
		record.Name,
		record.ProjectID,
		record.ProjectID,
		record.Name,
		yamlScalar(record.Description),
		record.DeploymentStrategy,
		record.CreatedAt.Format(time.RFC3339),
		record.UpdatedAt.Format(time.RFC3339),
		record.Name,
		record.Name,
		record.ProjectID,
		record.Name,
		record.Image,
		record.ServicePort,
		envFromBlock,
	)
}

func renderService(record Record) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
spec:
  selector:
    app.kubernetes.io/name: %s
  ports:
    - name: http
      port: %d
      targetPort: %d
`,
		record.Name,
		record.Namespace,
		record.Name,
		record.ServicePort,
		record.ServicePort,
	)
}

func renderVirtualService(record Record) string {
	host := fmt.Sprintf("%s.%s.svc.cluster.local", record.Name, record.Namespace)
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
