export type CurrentUser = {
  id: string
  username: string
  displayName?: string
  groups: string[]
}

export type ProjectRole = 'viewer' | 'deployer' | 'admin'

export type ProjectSummary = {
  id: string
  name: string
  description?: string
  namespace: string
  role: ProjectRole
}

export type ProjectListResponse = {
  items: ProjectSummary[]
}

export type SecretEntry = {
  key: string
  value: string
}

export type CreateApplicationRequest = {
  name: string
  description?: string
  image: string
  servicePort: number
  replicas?: number
  deploymentStrategy: 'Standard' | 'Canary'
  environment?: string
  secrets?: SecretEntry[]
  repositoryId?: string
  repositoryServiceId?: string
  configPath?: string
}

export type SyncStatus = 'Unknown' | 'Syncing' | 'Synced' | 'Degraded'

export type Application = {
  id: string
  projectId: string
  name: string
  description?: string
  image: string
  servicePort: number
  replicas: number
  deploymentStrategy: 'Standard' | 'Canary'
  defaultEnvironment?: string
  syncStatus?: SyncStatus
  createdAt?: string
  updatedAt?: string
  repositoryId?: string
  repositoryServiceId?: string
  configPath?: string
}

export type ApplicationSummary = {
  id: string
  name: string
  image: string
  deploymentStrategy: 'Standard' | 'Canary'
  syncStatus: SyncStatus
}

export type ApplicationListResponse = {
  items: ApplicationSummary[]
}

export type CreateDeploymentRequest = {
  imageTag: string
  environment?: string
}

export type UpdateApplicationRequest = {
  description?: string
  image?: string
  servicePort?: number
  replicas?: number
  deploymentStrategy?: 'Standard' | 'Canary'
  environment?: string
  repositoryId?: string
  repositoryServiceId?: string
  configPath?: string
}

export type EnvironmentSummary = {
  id: string
  name: string
  clusterId: string
  writeMode: 'direct' | 'pull_request'
  default: boolean
}

export type EnvironmentListResponse = {
  items: EnvironmentSummary[]
}

export type RepositorySummary = {
  id: string
  name: string
  url: string
  description?: string
  configFile?: string
}

export type RepositoryListResponse = {
  items: RepositorySummary[]
}

export type ProjectPolicy = {
  minReplicas: number
  allowedEnvironments: string[]
  allowedDeploymentStrategies: Array<'Standard' | 'Canary'>
  allowedClusterTargets: string[]
  prodPRRequired: boolean
  autoRollbackEnabled: boolean
  requiredProbes: boolean
}

export type ClusterSummary = {
  id: string
  name: string
  description?: string
  default: boolean
}

export type ClusterListResponse = {
  items: ClusterSummary[]
}

export type DeploymentRecord = {
  deploymentId: string
  applicationId: string
  projectId: string
  applicationName: string
  environment: string
  image: string
  imageTag: string
  deploymentStrategy: 'Standard' | 'Canary'
  status: string
  syncStatus?: SyncStatus
  rolloutPhase?: string
  currentStep?: number
  canaryWeight?: number
  stableRevision?: string
  canaryRevision?: string
  message?: string
  createdAt: string
  updatedAt: string
}

export type DeploymentListResponse = {
  applicationId: string
  items: DeploymentRecord[]
}

export type RollbackPolicy = {
  enabled: boolean
  maxErrorRate?: number
  maxLatencyP95Ms?: number
  minRequestRate?: number
}

export type ApplicationEvent = {
  id: string
  type: string
  message: string
  createdAt: string
  metadata?: Record<string, unknown>
}

export type EventListResponse = {
  applicationId: string
  items: ApplicationEvent[]
}

export type ChangeOperation =
  | 'CreateApplication'
  | 'UpdateApplication'
  | 'Redeploy'
  | 'UpdatePolicies'

export type ChangeRecord = {
  id: string
  projectId: string
  applicationId?: string
  operation: ChangeOperation
  environment: string
  writeMode: 'direct' | 'pull_request'
  status: 'Draft' | 'Submitted' | 'Approved' | 'Merged'
  summary: string
  diffPreview: string[]
  createdBy: string
  approvedBy?: string
  mergedBy?: string
  request?: CreateChangeRequest
  createdAt: string
  updatedAt: string
}

export type CreateChangeRequest = {
  operation: ChangeOperation
  applicationId?: string
  name?: string
  description?: string
  image?: string
  servicePort?: number
  deploymentStrategy?: 'Standard' | 'Canary'
  environment?: string
  imageTag?: string
  secrets?: SecretEntry[]
  policies?: ProjectPolicy
  summary?: string
}

export type SyncStatusResponse = {
  applicationId: string
  status: SyncStatus
  message?: string
  observedAt?: string
}

export type MetricPoint = {
  timestamp: string
  value: number | null
}

export type MetricSeries = {
  key: string
  label: string
  unit: string
  points: MetricPoint[]
}

export type ApplicationMetricsResponse = {
  applicationId: string
  metrics: MetricSeries[]
}

export type ErrorResponse = {
  error: {
    code: string
    message: string
    details?: Record<string, unknown>
    requestId: string
    retryable?: boolean
  }
}
