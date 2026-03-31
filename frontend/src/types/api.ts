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
  deploymentStrategy: 'Standard'
  secrets?: SecretEntry[]
}

export type SyncStatus = 'Unknown' | 'Syncing' | 'Synced' | 'Degraded'

export type Application = {
  id: string
  projectId: string
  name: string
  description?: string
  image: string
  servicePort: number
  deploymentStrategy: 'Standard'
  syncStatus?: SyncStatus
  createdAt?: string
  updatedAt?: string
}

export type ApplicationSummary = {
  id: string
  name: string
  image: string
  deploymentStrategy: 'Standard'
  syncStatus: SyncStatus
}

export type ApplicationListResponse = {
  items: ApplicationSummary[]
}

export type CreateDeploymentRequest = {
  imageTag: string
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
