package change

import (
	"time"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/project"
)

type Status string

const (
	StatusDraft     Status = "Draft"
	StatusSubmitted Status = "Submitted"
	StatusApproved  Status = "Approved"
	StatusMerged    Status = "Merged"
)

type Operation string

const (
	OperationCreateApplication Operation = "CreateApplication"
	OperationUpdateApplication Operation = "UpdateApplication"
	OperationRedeploy          Operation = "Redeploy"
	OperationUpdatePolicies    Operation = "UpdatePolicies"
)

type Request struct {
	Operation          Operation                           `json:"operation"`
	ApplicationID      string                              `json:"applicationId,omitempty"`
	Name               string                              `json:"name,omitempty"`
	Description        string                              `json:"description,omitempty"`
	Image              string                              `json:"image,omitempty"`
	ServicePort        int                                 `json:"servicePort,omitempty"`
	DeploymentStrategy application.DeploymentStrategy      `json:"deploymentStrategy,omitempty"`
	Environment        string                              `json:"environment,omitempty"`
	ImageTag           string                              `json:"imageTag,omitempty"`
	Secrets            []application.SecretEntry           `json:"secrets,omitempty"`
	Policies           *project.PolicySummary              `json:"policies,omitempty"`
	Summary            string                              `json:"summary,omitempty"`
}

type Record struct {
	ID            string        `json:"id" yaml:"id"`
	ProjectID     string        `json:"projectId" yaml:"projectId"`
	ApplicationID string        `json:"applicationId,omitempty" yaml:"applicationId,omitempty"`
	Operation     Operation     `json:"operation" yaml:"operation"`
	Environment   string        `json:"environment" yaml:"environment"`
	WriteMode     project.WriteMode `json:"writeMode" yaml:"writeMode"`
	Status        Status        `json:"status" yaml:"status"`
	Summary       string        `json:"summary" yaml:"summary"`
	DiffPreview   []string      `json:"diffPreview" yaml:"diffPreview"`
	Request       Request       `json:"request" yaml:"request"`
	CreatedBy     string        `json:"createdBy" yaml:"createdBy"`
	ApprovedBy    string        `json:"approvedBy,omitempty" yaml:"approvedBy,omitempty"`
	MergedBy      string        `json:"mergedBy,omitempty" yaml:"mergedBy,omitempty"`
	CreatedAt     time.Time     `json:"createdAt" yaml:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt" yaml:"updatedAt"`
}
