package admin

import (
	"context"
	"time"
)

type ServiceEfficiencyStatus string

const (
	ServiceEfficiencyStatusBalanced      ServiceEfficiencyStatus = "Balanced"
	ServiceEfficiencyStatusUnderutilized ServiceEfficiencyStatus = "Underutilized"
	ServiceEfficiencyStatusOverutilized  ServiceEfficiencyStatus = "Overutilized"
	ServiceEfficiencyStatusNoMetrics     ServiceEfficiencyStatus = "NoMetrics"
	ServiceEfficiencyStatusUnknown       ServiceEfficiencyStatus = "Unknown"
)

type CapacitySummary struct {
	AllocatableCPUCores      *float64 `json:"allocatableCpuCores,omitempty"`
	AllocatableMemoryMiB     *float64 `json:"allocatableMemoryMiB,omitempty"`
	RequestedCPUCores        *float64 `json:"requestedCpuCores,omitempty"`
	RequestedMemoryMiB       *float64 `json:"requestedMemoryMiB,omitempty"`
	UsedCPUCores             *float64 `json:"usedCpuCores,omitempty"`
	UsedMemoryMiB            *float64 `json:"usedMemoryMiB,omitempty"`
	AvailableCPUCores        *float64 `json:"availableCpuCores,omitempty"`
	AvailableMemoryMiB       *float64 `json:"availableMemoryMiB,omitempty"`
	RequestCPUUtilization    *float64 `json:"requestCpuUtilization,omitempty"`
	RequestMemoryUtilization *float64 `json:"requestMemoryUtilization,omitempty"`
	UsageCPUUtilization      *float64 `json:"usageCpuUtilization,omitempty"`
	UsageMemoryUtilization   *float64 `json:"usageMemoryUtilization,omitempty"`
}

type RuntimeServiceRef struct {
	ApplicationID string
	ProjectID     string
	ProjectName   string
	ClusterID     string
	ClusterName   string
	Namespace     string
	Name          string
}

type RuntimeServiceSnapshot struct {
	PodCount                 int
	ReadyPodCount            int
	CPURequestCores          *float64
	CPULimitCores            *float64
	CPUUsageCores            *float64
	CPURequestUtilization    *float64
	CPULimitUtilization      *float64
	MemoryRequestMiB         *float64
	MemoryLimitMiB           *float64
	MemoryUsageMiB           *float64
	MemoryRequestUtilization *float64
	MemoryLimitUtilization   *float64
}

type RuntimeSnapshot struct {
	GeneratedAt      time.Time
	RuntimeConnected bool
	Message          string
	Capacity         CapacitySummary
	Services         map[string]RuntimeServiceSnapshot
}

type ResourceOverviewReader interface {
	Read(context.Context, []RuntimeServiceRef) (RuntimeSnapshot, error)
}

type ServiceResourceEfficiency struct {
	ApplicationID            string                  `json:"applicationId"`
	ProjectID                string                  `json:"projectId"`
	ProjectName              string                  `json:"projectName"`
	ClusterID                string                  `json:"clusterId,omitempty"`
	ClusterName              string                  `json:"clusterName,omitempty"`
	Namespace                string                  `json:"namespace"`
	Name                     string                  `json:"name"`
	PodCount                 int                     `json:"podCount"`
	ReadyPodCount            int                     `json:"readyPodCount"`
	Status                   ServiceEfficiencyStatus `json:"status"`
	Summary                  string                  `json:"summary"`
	CPURequestCores          *float64                `json:"cpuRequestCores,omitempty"`
	CPULimitCores            *float64                `json:"cpuLimitCores,omitempty"`
	CPUUsageCores            *float64                `json:"cpuUsageCores,omitempty"`
	CPURequestUtilization    *float64                `json:"cpuRequestUtilization,omitempty"`
	CPULimitUtilization      *float64                `json:"cpuLimitUtilization,omitempty"`
	MemoryRequestMiB         *float64                `json:"memoryRequestMiB,omitempty"`
	MemoryLimitMiB           *float64                `json:"memoryLimitMiB,omitempty"`
	MemoryUsageMiB           *float64                `json:"memoryUsageMiB,omitempty"`
	MemoryRequestUtilization *float64                `json:"memoryRequestUtilization,omitempty"`
	MemoryLimitUtilization   *float64                `json:"memoryLimitUtilization,omitempty"`
}

type EfficiencyCounts struct {
	Balanced      int `json:"balanced"`
	Underutilized int `json:"underutilized"`
	Overutilized  int `json:"overutilized"`
	NoMetrics     int `json:"noMetrics"`
	Unknown       int `json:"unknown"`
}

type FleetResourceOverviewResponse struct {
	GeneratedAt      time.Time                   `json:"generatedAt"`
	RuntimeConnected bool                        `json:"runtimeConnected"`
	Message          string                      `json:"message,omitempty"`
	ProjectCount     int                         `json:"projectCount"`
	ServiceCount     int                         `json:"serviceCount"`
	Capacity         CapacitySummary             `json:"capacity"`
	Counts           EfficiencyCounts            `json:"counts"`
	Services         []ServiceResourceEfficiency `json:"services"`
}
