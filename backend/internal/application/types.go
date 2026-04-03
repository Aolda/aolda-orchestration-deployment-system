package application

import "time"

type DeploymentStrategy string

const (
	DeploymentStrategyStandard DeploymentStrategy = "Standard"
	DeploymentStrategyCanary   DeploymentStrategy = "Canary"
)

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

type CreateRequest struct {
	Name                string             `json:"name"`
	Description         string             `json:"description,omitempty"`
	Image               string             `json:"image"`
	ServicePort         int                `json:"servicePort"`
	Replicas            int                `json:"replicas,omitempty"`
	DeploymentStrategy  DeploymentStrategy `json:"deploymentStrategy"`
	Environment         string             `json:"environment,omitempty"`
	Secrets             []SecretEntry      `json:"secrets,omitempty"`
	RepositoryID        string             `json:"repositoryId,omitempty"`
	RepositoryServiceID string             `json:"repositoryServiceId,omitempty"`
	ConfigPath          string             `json:"configPath,omitempty"`
}

type CreateDeploymentRequest struct {
	ImageTag    string `json:"imageTag"`
	Environment string `json:"environment,omitempty"`
}

type UpdateApplicationRequest struct {
	Description         *string             `json:"description,omitempty"`
	Image               *string             `json:"image,omitempty"`
	ServicePort         *int                `json:"servicePort,omitempty"`
	Replicas            *int                `json:"replicas,omitempty"`
	DeploymentStrategy  *DeploymentStrategy `json:"deploymentStrategy,omitempty"`
	Environment         *string             `json:"environment,omitempty"`
	RepositoryID        *string             `json:"repositoryId,omitempty"`
	RepositoryServiceID *string             `json:"repositoryServiceId,omitempty"`
	ConfigPath          *string             `json:"configPath,omitempty"`
}

type Record struct {
	ID                  string
	ProjectID           string
	Namespace           string
	Name                string
	Description         string
	Image               string
	ServicePort         int
	Replicas            int
	RequiredProbes      bool
	DeploymentStrategy  DeploymentStrategy
	DefaultEnvironment  string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	SecretPath          string
	RepositoryID        string
	RepositoryServiceID string
	ConfigPath          string
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
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Image              string     `json:"image"`
	DeploymentStrategy string     `json:"deploymentStrategy"`
	SyncStatus         SyncStatus `json:"syncStatus"`
}

type Application struct {
	ID                  string     `json:"id"`
	ProjectID           string     `json:"projectId"`
	Name                string     `json:"name"`
	Description         string     `json:"description,omitempty"`
	Image               string     `json:"image"`
	ServicePort         int        `json:"servicePort"`
	Replicas            int        `json:"replicas"`
	DeploymentStrategy  string     `json:"deploymentStrategy"`
	DefaultEnvironment  string     `json:"defaultEnvironment,omitempty"`
	SyncStatus          SyncStatus `json:"syncStatus,omitempty"`
	CreatedAt           time.Time  `json:"createdAt,omitempty"`
	UpdatedAt           time.Time  `json:"updatedAt,omitempty"`
	RepositoryID        string     `json:"repositoryId,omitempty"`
	RepositoryServiceID string     `json:"repositoryServiceId,omitempty"`
	ConfigPath          string     `json:"configPath,omitempty"`
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
	ApplicationID string     `json:"applicationId"`
	Status        SyncStatus `json:"status"`
	Message       string     `json:"message,omitempty"`
	ObservedAt    time.Time  `json:"observedAt,omitempty"`
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

type StagedSecret struct {
	StagingPath string
	FinalPath   string
}
