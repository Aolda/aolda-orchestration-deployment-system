package application

import (
	"strings"
	"time"
)

type DeploymentStrategy string

const (
	DeploymentStrategyRollout   DeploymentStrategy = "Rollout"
	DeploymentStrategyStandard  DeploymentStrategy = "Standard"
	DeploymentStrategyCanary    DeploymentStrategy = "Canary"
	DefaultRepositoryConfigPath                    = "aolda_deploy.json"
)

var AllowedRepositoryPollIntervalsSeconds = []int{60, 300, 600}

func NormalizeDeploymentStrategy(value DeploymentStrategy) DeploymentStrategy {
	switch strings.TrimSpace(string(value)) {
	case string(DeploymentStrategyCanary):
		return DeploymentStrategyCanary
	case string(DeploymentStrategyRollout), string(DeploymentStrategyStandard):
		return DeploymentStrategyRollout
	default:
		return DeploymentStrategy(strings.TrimSpace(string(value)))
	}
}

func IsCanaryDeploymentStrategy(value DeploymentStrategy) bool {
	return NormalizeDeploymentStrategy(value) == DeploymentStrategyCanary
}

type SyncStatus string

const (
	SyncStatusUnknown  SyncStatus = "Unknown"
	SyncStatusSyncing  SyncStatus = "Syncing"
	SyncStatusSynced   SyncStatus = "Synced"
	SyncStatusDegraded SyncStatus = "Degraded"
)

type SecretEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ResourceQuantity struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

type ResourceRequirements struct {
	Requests ResourceQuantity `json:"requests,omitempty"`
	Limits   ResourceQuantity `json:"limits,omitempty"`
}

type CreateRequest struct {
	Name                          string             `json:"name"`
	Description                   string             `json:"description,omitempty"`
	Image                         string             `json:"image"`
	ServicePort                   int                `json:"servicePort"`
	Replicas                      int                `json:"replicas,omitempty"`
	DeploymentStrategy            DeploymentStrategy `json:"deploymentStrategy"`
	MeshEnabled                   bool               `json:"meshEnabled,omitempty"`
	LoadBalancerEnabled           bool               `json:"loadBalancerEnabled,omitempty"`
	Environment                   string             `json:"environment,omitempty"`
	Secrets                       []SecretEntry      `json:"secrets,omitempty"`
	RepositoryID                  string             `json:"repositoryId,omitempty"`
	RepositoryURL                 string             `json:"repositoryUrl,omitempty"`
	RepositoryBranch              string             `json:"repositoryBranch,omitempty"`
	RepositoryToken               string             `json:"repositoryToken,omitempty"`
	RepositoryServiceID           string             `json:"repositoryServiceId,omitempty"`
	ConfigPath                    string             `json:"configPath,omitempty"`
	RepositoryPollIntervalSeconds int                `json:"repositoryPollIntervalSeconds,omitempty"`
	RegistryServer                string             `json:"registryServer,omitempty"`
	RegistryUsername              string             `json:"registryUsername,omitempty"`
	RegistryToken                 string             `json:"registryToken,omitempty"`
	RepositoryTokenPath           string             `json:"-"`
	RegistrySecretPath            string             `json:"-"`
}

type PreviewRepositorySourceRequest struct {
	Name                string `json:"name,omitempty"`
	RepositoryURL       string `json:"repositoryUrl"`
	RepositoryBranch    string `json:"repositoryBranch,omitempty"`
	RepositoryToken     string `json:"repositoryToken,omitempty"`
	RepositoryServiceID string `json:"repositoryServiceId,omitempty"`
	ConfigPath          string `json:"configPath,omitempty"`
}

type PreviewRepositoryService struct {
	ServiceID string             `json:"serviceId"`
	Image     string             `json:"image"`
	Port      int                `json:"port"`
	Replicas  int                `json:"replicas"`
	Strategy  DeploymentStrategy `json:"strategy,omitempty"`
}

type PreviewRepositorySourceResponse struct {
	ConfigPath               string                     `json:"configPath"`
	Services                 []PreviewRepositoryService `json:"services"`
	SelectedServiceID        string                     `json:"selectedServiceId,omitempty"`
	RequiresServiceSelection bool                       `json:"requiresServiceSelection"`
}

type CreateDeploymentRequest struct {
	ImageTag    string `json:"imageTag"`
	Environment string `json:"environment,omitempty"`
}

type UpdateApplicationRequest struct {
	Description                   *string               `json:"description,omitempty"`
	Image                         *string               `json:"image,omitempty"`
	ServicePort                   *int                  `json:"servicePort,omitempty"`
	Replicas                      *int                  `json:"replicas,omitempty"`
	DeploymentStrategy            *DeploymentStrategy   `json:"deploymentStrategy,omitempty"`
	MeshEnabled                   *bool                 `json:"meshEnabled,omitempty"`
	LoadBalancerEnabled           *bool                 `json:"loadBalancerEnabled,omitempty"`
	Environment                   *string               `json:"environment,omitempty"`
	RepositoryID                  *string               `json:"repositoryId,omitempty"`
	RepositoryURL                 *string               `json:"repositoryUrl,omitempty"`
	RepositoryBranch              *string               `json:"repositoryBranch,omitempty"`
	RepositoryServiceID           *string               `json:"repositoryServiceId,omitempty"`
	ConfigPath                    *string               `json:"configPath,omitempty"`
	RepositoryPollIntervalSeconds *int                  `json:"repositoryPollIntervalSeconds,omitempty"`
	Resources                     *ResourceRequirements `json:"resources,omitempty"`
}

type ApplicationLifecycleResponse struct {
	ApplicationID string     `json:"applicationId"`
	ProjectID     string     `json:"projectId"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	ArchivedAt    *time.Time `json:"archivedAt,omitempty"`
	DeletedAt     *time.Time `json:"deletedAt,omitempty"`
	secretPaths   []string
}

type Record struct {
	ID                            string
	ProjectID                     string
	Namespace                     string
	Name                          string
	Description                   string
	Image                         string
	ServicePort                   int
	Replicas                      int
	RequiredProbes                bool
	DeploymentStrategy            DeploymentStrategy
	DefaultEnvironment            string
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
	SecretPath                    string
	RepositoryID                  string
	RepositoryURL                 string
	RepositoryBranch              string
	RepositoryServiceID           string
	ConfigPath                    string
	RepositoryTokenPath           string
	RegistrySecretPath            string
	RepositoryPollIntervalSeconds int
	Resources                     ResourceRequirements
	MeshEnabled                   bool
	LoadBalancerEnabled           bool
}

type ProjectContext struct {
	ID                  string
	Namespace           string
	Environments        []string
	EnvironmentClusters map[string]string
	Policies            projectPolicy
}

type projectPolicy struct {
	MinReplicas                 int
	AllowedEnvironments         []string
	AllowedDeploymentStrategies []string
	AllowedClusterTargets       []string
	ProdPRRequired              bool
	AutoRollbackEnabled         bool
	RequiredProbes              bool
}

type Summary struct {
	ID                  string               `json:"id"`
	Name                string               `json:"name"`
	Image               string               `json:"image"`
	DeploymentStrategy  string               `json:"deploymentStrategy"`
	SyncStatus          SyncStatus           `json:"syncStatus"`
	Resources           ResourceRequirements `json:"resources,omitempty"`
	MeshEnabled         bool                 `json:"meshEnabled"`
	LoadBalancerEnabled bool                 `json:"loadBalancerEnabled"`
}

type Application struct {
	ID                            string               `json:"id"`
	ProjectID                     string               `json:"projectId"`
	Name                          string               `json:"name"`
	Description                   string               `json:"description,omitempty"`
	Image                         string               `json:"image"`
	ServicePort                   int                  `json:"servicePort"`
	Replicas                      int                  `json:"replicas"`
	DeploymentStrategy            string               `json:"deploymentStrategy"`
	DefaultEnvironment            string               `json:"defaultEnvironment,omitempty"`
	SyncStatus                    SyncStatus           `json:"syncStatus,omitempty"`
	CreatedAt                     time.Time            `json:"createdAt,omitempty"`
	UpdatedAt                     time.Time            `json:"updatedAt,omitempty"`
	RepositoryID                  string               `json:"repositoryId,omitempty"`
	RepositoryURL                 string               `json:"repositoryUrl,omitempty"`
	RepositoryBranch              string               `json:"repositoryBranch,omitempty"`
	RepositoryServiceID           string               `json:"repositoryServiceId,omitempty"`
	ConfigPath                    string               `json:"configPath,omitempty"`
	RepositoryPollIntervalSeconds int                  `json:"repositoryPollIntervalSeconds,omitempty"`
	Resources                     ResourceRequirements `json:"resources,omitempty"`
	MeshEnabled                   bool                 `json:"meshEnabled"`
	LoadBalancerEnabled           bool                 `json:"loadBalancerEnabled"`
}

type DeploymentResponse struct {
	DeploymentID  string `json:"deploymentId"`
	ApplicationID string `json:"applicationId"`
	ImageTag      string `json:"imageTag"`
	Environment   string `json:"environment,omitempty"`
	Status        string `json:"status"`
	CommitSHA     string `json:"commitSha,omitempty"`
}

type DeploymentRecord struct {
	DeploymentID       string             `json:"deploymentId"`
	ApplicationID      string             `json:"applicationId"`
	ProjectID          string             `json:"projectId"`
	ApplicationName    string             `json:"applicationName"`
	Environment        string             `json:"environment"`
	Image              string             `json:"image"`
	ImageTag           string             `json:"imageTag"`
	DeploymentStrategy DeploymentStrategy `json:"deploymentStrategy"`
	Status             string             `json:"status"`
	SyncStatus         SyncStatus         `json:"syncStatus,omitempty"`
	RolloutPhase       string             `json:"rolloutPhase,omitempty"`
	CurrentStep        *int               `json:"currentStep,omitempty"`
	CanaryWeight       *int               `json:"canaryWeight,omitempty"`
	StableRevision     string             `json:"stableRevision,omitempty"`
	CanaryRevision     string             `json:"canaryRevision,omitempty"`
	Message            string             `json:"message,omitempty"`
	CommitSHA          string             `json:"commitSha,omitempty"`
	CreatedAt          time.Time          `json:"createdAt"`
	UpdatedAt          time.Time          `json:"updatedAt"`
}

type DeploymentListResponse struct {
	ApplicationID string             `json:"applicationId"`
	Items         []DeploymentRecord `json:"items"`
}

type RollbackPolicy struct {
	Enabled         bool     `json:"enabled"`
	MaxErrorRate    *float64 `json:"maxErrorRate,omitempty"`
	MaxLatencyP95Ms *int     `json:"maxLatencyP95Ms,omitempty"`
	MinRequestRate  *float64 `json:"minRequestRate,omitempty"`
}

type Event struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Message   string         `json:"message"`
	CreatedAt time.Time      `json:"createdAt"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type EventListResponse struct {
	ApplicationID string  `json:"applicationId"`
	Items         []Event `json:"items"`
}

type SyncInfo struct {
	Status     SyncStatus
	Message    string
	ObservedAt time.Time
}

type RolloutInfo struct {
	Phase          string
	CurrentStep    *int
	CanaryWeight   *int
	StableRevision string
	CanaryRevision string
	Message        string
}

type SyncStatusResponse struct {
	ApplicationID  string                `json:"applicationId"`
	Status         SyncStatus            `json:"status"`
	Message        string                `json:"message,omitempty"`
	ObservedAt     time.Time             `json:"observedAt,omitempty"`
	RepositoryPoll *RepositoryPollStatus `json:"repositoryPoll,omitempty"`
}

type RepositorySyncResponse struct {
	ApplicationID       string                `json:"applicationId"`
	CheckedAt           time.Time             `json:"checkedAt"`
	Message             string                `json:"message"`
	Source              string                `json:"source,omitempty"`
	SettingsApplied     bool                  `json:"settingsApplied,omitempty"`
	DeploymentTriggered bool                  `json:"deploymentTriggered,omitempty"`
	RepositoryPoll      *RepositoryPollStatus `json:"repositoryPoll,omitempty"`
}

type NetworkExposureStatus string

const (
	NetworkExposureStatusInternal     NetworkExposureStatus = "Internal"
	NetworkExposureStatusPending      NetworkExposureStatus = "Pending"
	NetworkExposureStatusProvisioning NetworkExposureStatus = "Provisioning"
	NetworkExposureStatusReady        NetworkExposureStatus = "Ready"
	NetworkExposureStatusError        NetworkExposureStatus = "Error"
)

type NetworkExposureEvent struct {
	Type       string    `json:"type"`
	Reason     string    `json:"reason,omitempty"`
	Message    string    `json:"message"`
	ObservedAt time.Time `json:"observedAt,omitempty"`
}

type NetworkExposurePort struct {
	Name       string `json:"name,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	Port       int    `json:"port"`
	TargetPort string `json:"targetPort,omitempty"`
	NodePort   int    `json:"nodePort,omitempty"`
}

type NetworkExposureInfo struct {
	Status      NetworkExposureStatus
	Message     string
	ServiceType string
	Addresses   []string
	Ports       []NetworkExposurePort
	LastEvent   *NetworkExposureEvent
	ObservedAt  time.Time
}

type NetworkExposureResponse struct {
	ApplicationID string                `json:"applicationId"`
	Enabled       bool                  `json:"enabled"`
	Status        NetworkExposureStatus `json:"status"`
	Message       string                `json:"message,omitempty"`
	ServiceType   string                `json:"serviceType,omitempty"`
	Addresses     []string              `json:"addresses,omitempty"`
	Ports         []NetworkExposurePort `json:"ports,omitempty"`
	LastEvent     *NetworkExposureEvent `json:"lastEvent,omitempty"`
	ObservedAt    time.Time             `json:"observedAt,omitempty"`
}

type MetricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     *float64  `json:"value"`
}

type MetricSeries struct {
	Key    string        `json:"key"`
	Label  string        `json:"label"`
	Unit   string        `json:"unit"`
	Points []MetricPoint `json:"points"`
}

type MetricsResponse struct {
	ApplicationID string         `json:"applicationId"`
	Metrics       []MetricSeries `json:"metrics"`
}

type ContainerLogStream struct {
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`
	Phase         string `json:"phase,omitempty"`
	Ready         bool   `json:"ready"`
	RestartCount  int    `json:"restartCount"`
	Content       string `json:"content"`
}

type ContainerLogsResponse struct {
	ApplicationID string               `json:"applicationId"`
	CollectedAt   time.Time            `json:"collectedAt"`
	TailLines     int                  `json:"tailLines"`
	Items         []ContainerLogStream `json:"items"`
}

type ContainerLogTargetContainer struct {
	Name           string                   `json:"name"`
	Ready          bool                     `json:"ready"`
	RestartCount   int                      `json:"restartCount"`
	Default        bool                     `json:"default"`
	ResourceStatus *ContainerResourceStatus `json:"resourceStatus,omitempty"`
}

type ContainerResourceStatus struct {
	CPUUsageCores            *float64 `json:"cpuUsageCores,omitempty"`
	CPURequestCores          *float64 `json:"cpuRequestCores,omitempty"`
	CPULimitCores            *float64 `json:"cpuLimitCores,omitempty"`
	CPURequestUtilization    *float64 `json:"cpuRequestUtilization,omitempty"`
	CPULimitUtilization      *float64 `json:"cpuLimitUtilization,omitempty"`
	MemoryUsageMiB           *float64 `json:"memoryUsageMiB,omitempty"`
	MemoryRequestMiB         *float64 `json:"memoryRequestMiB,omitempty"`
	MemoryLimitMiB           *float64 `json:"memoryLimitMiB,omitempty"`
	MemoryRequestUtilization *float64 `json:"memoryRequestUtilization,omitempty"`
	MemoryLimitUtilization   *float64 `json:"memoryLimitUtilization,omitempty"`
}

type ContainerLogTarget struct {
	PodName    string                        `json:"podName"`
	Phase      string                        `json:"phase,omitempty"`
	Containers []ContainerLogTargetContainer `json:"containers"`
}

type ContainerLogTargetsResponse struct {
	ApplicationID string               `json:"applicationId"`
	CollectedAt   time.Time            `json:"collectedAt"`
	Items         []ContainerLogTarget `json:"items"`
}

type ContainerLogEvent struct {
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`
	Timestamp     string `json:"timestamp,omitempty"`
	Message       string `json:"message"`
	RawLine       string `json:"rawLine"`
}

type StagedSecret struct {
	StagingPath string
	FinalPath   string
}
