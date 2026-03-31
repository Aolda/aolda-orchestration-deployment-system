package application

import "time"

type DeploymentStrategy string

const DeploymentStrategyStandard DeploymentStrategy = "Standard"

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
	Name               string             `json:"name"`
	Description        string             `json:"description,omitempty"`
	Image              string             `json:"image"`
	ServicePort        int                `json:"servicePort"`
	DeploymentStrategy DeploymentStrategy `json:"deploymentStrategy"`
	Secrets            []SecretEntry      `json:"secrets,omitempty"`
}

type CreateDeploymentRequest struct {
	ImageTag string `json:"imageTag"`
}

type Record struct {
	ID                 string
	ProjectID          string
	Namespace          string
	Name               string
	Description        string
	Image              string
	ServicePort        int
	DeploymentStrategy DeploymentStrategy
	CreatedAt          time.Time
	UpdatedAt          time.Time
	SecretPath         string
}

type ProjectContext struct {
	ID        string
	Namespace string
}

type Summary struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Image              string     `json:"image"`
	DeploymentStrategy string     `json:"deploymentStrategy"`
	SyncStatus         SyncStatus `json:"syncStatus"`
}

type Application struct {
	ID                 string     `json:"id"`
	ProjectID          string     `json:"projectId"`
	Name               string     `json:"name"`
	Description        string     `json:"description,omitempty"`
	Image              string     `json:"image"`
	ServicePort        int        `json:"servicePort"`
	DeploymentStrategy string     `json:"deploymentStrategy"`
	SyncStatus         SyncStatus `json:"syncStatus,omitempty"`
	CreatedAt          time.Time  `json:"createdAt,omitempty"`
	UpdatedAt          time.Time  `json:"updatedAt,omitempty"`
}

type DeploymentResponse struct {
	DeploymentID  string `json:"deploymentId"`
	ApplicationID string `json:"applicationId"`
	ImageTag      string `json:"imageTag"`
	Status        string `json:"status"`
}

type SyncInfo struct {
	Status     SyncStatus
	Message    string
	ObservedAt time.Time
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
