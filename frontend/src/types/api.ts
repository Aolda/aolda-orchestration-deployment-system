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

export type HealthStatus = 'Healthy' | 'Warning' | 'Critical' | 'Unknown'
export type HealthSignalStatus = 'OK' | 'Warning' | 'Critical' | 'Unknown' | 'Unavailable'

export type HealthSignal = {
  key: string
  status: HealthSignalStatus
  message: string
  observedAt?: string
  details?: Record<string, unknown>
}

export type ApplicationHealthSnapshot = {
  applicationId: string
  name: string
  namespace: string
  status: HealthStatus
  syncStatus: SyncStatus
  deploymentStrategy: string
  metrics: MetricSeries[]
  latestDeployment?: DeploymentRecord | null
  signals: HealthSignal[]
}

export type ProjectHealthResponse = {
  projectId: string
  observedAt: string
  items: ApplicationHealthSnapshot[]
}

export type ProjectAccess = {
  viewerGroups?: string[]
  deployerGroups?: string[]
  adminGroups?: string[]
}

export type ProjectEnvironmentInput = {
  id: string
  name?: string
  clusterId?: string
  writeMode?: WriteMode
  default?: boolean
}

export type ProjectRepositoryInput = {
  id: string
  name: string
  url: string
  description?: string
  branch?: string
  authSecretPath?: string
  configFile?: string
}

export type CreateProjectRequest = {
  id: string
  name: string
  description?: string
  namespace?: string
  access?: ProjectAccess
  environments?: ProjectEnvironmentInput[]
  repositories?: ProjectRepositoryInput[]
  policies?: ProjectPolicy
}

export type ProjectLifecycleResponse = {
  projectId: string
  name: string
  namespace: string
  status: 'deleted'
  deletedAt?: string
}

export type SecretEntry = {
  key: string
  value: string
}

export type ApplicationSecretSummary = {
  key: string
}

export type ApplicationSecretVersionSummary = {
  version: number
  createdAt?: string
  updatedBy?: string
  current: boolean
  deleted?: boolean
  destroyed?: boolean
  keyCount?: number
}

export type ApplicationSecretsResponse = {
  applicationId: string
  secretPath: string
  configured: boolean
  versioningEnabled: boolean
  currentVersion?: number
  updatedAt?: string
  items: ApplicationSecretSummary[]
}

export type ApplicationSecretVersionsResponse = {
  applicationId: string
  secretPath: string
  currentVersion?: number
  items: ApplicationSecretVersionSummary[]
}

export type UpdateApplicationSecretsRequest = {
  set?: SecretEntry[]
  delete?: string[]
}

export type CreateApplicationRequest = {
  name?: string
  description?: string
  image?: string
  servicePort?: number
  replicas?: number
  deploymentStrategy?: 'Rollout' | 'Canary'
  meshEnabled?: boolean
  loadBalancerEnabled?: boolean
  environment?: string
  secrets?: SecretEntry[]
  repositoryId?: string
  repositoryUrl?: string
  repositoryBranch?: string
  repositoryToken?: string
  repositoryServiceId?: string
  configPath?: string
  repositoryPollIntervalSeconds?: number
  registryServer?: string
  registryUsername?: string
  registryToken?: string
}

export type PreviewApplicationSourceRequest = {
  name?: string
  repositoryUrl: string
  repositoryBranch?: string
  repositoryToken?: string
  repositoryServiceId?: string
  configPath?: string
}

export type PreviewApplicationSourceService = {
  serviceId: string
  image: string
  port: number
  replicas: number
  strategy?: 'Rollout' | 'Canary'
}

export type PreviewApplicationSourceResponse = {
  configPath: string
  services: PreviewApplicationSourceService[]
  selectedServiceId?: string
  requiresServiceSelection: boolean
}

export type VerifyImageAccessRequest = {
  image: string
  registryServer?: string
  registryUsername?: string
  registryToken?: string
}

export type VerifyImageAccessResponse = {
  image: string
  registry: string
  accessible: boolean
  message: string
}

export type SyncStatus = 'Unknown' | 'Syncing' | 'Synced' | 'Degraded'
export type RepositoryPollResult = 'Pending' | 'Success' | 'Error'

export type RepositoryPollStatus = {
  enabled: boolean
  intervalSeconds: number
  lastCheckedAt?: string
  lastSucceededAt?: string
  nextScheduledAt?: string
  lastResult?: RepositoryPollResult
  lastError?: string
  source?: string
}

export type WriteMode = 'direct' | 'pull_request'

export type Application = {
  id: string
  projectId: string
  name: string
  description?: string
  image: string
  servicePort: number
  replicas: number
  deploymentStrategy: 'Rollout' | 'Canary'
  defaultEnvironment?: string
  syncStatus?: SyncStatus
  createdAt?: string
  updatedAt?: string
  repositoryId?: string
  repositoryUrl?: string
  repositoryBranch?: string
  repositoryServiceId?: string
  configPath?: string
  repositoryPollIntervalSeconds?: number
  resources?: ApplicationResources
  meshEnabled: boolean
  loadBalancerEnabled: boolean
}

export type ApplicationSummary = {
  id: string
  name: string
  image: string
  deploymentStrategy: 'Rollout' | 'Canary'
  syncStatus: SyncStatus
  resources?: ApplicationResources
  meshEnabled: boolean
  loadBalancerEnabled: boolean
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
  deploymentStrategy?: 'Rollout' | 'Canary'
  meshEnabled?: boolean
  loadBalancerEnabled?: boolean
  environment?: string
  repositoryId?: string
  repositoryUrl?: string
  repositoryBranch?: string
  repositoryServiceId?: string
  configPath?: string
  repositoryPollIntervalSeconds?: number
  resources?: ApplicationResources
}

export type ResourceQuantity = {
  cpu?: string
  memory?: string
}

export type ApplicationResources = {
  requests?: ResourceQuantity
  limits?: ResourceQuantity
}

export type ApplicationLifecycleResponse = {
  applicationId: string
  projectId: string
  name: string
  status: 'archived' | 'deleted'
  archivedAt?: string
  deletedAt?: string
}

export type RepositorySyncResponse = {
  applicationId: string
  checkedAt: string
  message: string
  source?: string
  settingsApplied?: boolean
  deploymentTriggered?: boolean
  repositoryPoll?: RepositoryPollStatus
}

export type EnvironmentSummary = {
  id: string
  name: string
  clusterId: string
  writeMode: WriteMode
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
  access: 'public' | 'private'
  branch?: string
  configFile?: string
}

export type RepositoryListResponse = {
  items: RepositorySummary[]
}

export type ProjectPolicy = {
  minReplicas: number
  allowedEnvironments: string[]
  allowedDeploymentStrategies: Array<'Rollout' | 'Canary'>
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

export type CreateClusterRequest = {
  id: string
  name: string
  description?: string
  default?: boolean
}

export type ClusterListResponse = {
  items: ClusterSummary[]
}

export type CapacitySummary = {
  allocatableCpuCores?: number
  allocatableMemoryMiB?: number
  requestedCpuCores?: number
  requestedMemoryMiB?: number
  usedCpuCores?: number
  usedMemoryMiB?: number
  availableCpuCores?: number
  availableMemoryMiB?: number
  requestCpuUtilization?: number
  requestMemoryUtilization?: number
  usageCpuUtilization?: number
  usageMemoryUtilization?: number
}

export type ServiceEfficiencyStatus =
  | 'Balanced'
  | 'Underutilized'
  | 'Overutilized'
  | 'NoMetrics'
  | 'Unknown'

export type EfficiencyCounts = {
  balanced: number
  underutilized: number
  overutilized: number
  noMetrics: number
  unknown: number
}

export type ServiceResourceEfficiency = {
  applicationId: string
  projectId: string
  projectName: string
  clusterId?: string
  clusterName?: string
  namespace: string
  name: string
  podCount: number
  readyPodCount: number
  status: ServiceEfficiencyStatus
  summary: string
  cpuRequestCores?: number
  cpuLimitCores?: number
  cpuUsageCores?: number
  cpuRequestUtilization?: number
  cpuLimitUtilization?: number
  memoryRequestMiB?: number
  memoryLimitMiB?: number
  memoryUsageMiB?: number
  memoryRequestUtilization?: number
  memoryLimitUtilization?: number
}

export type FleetResourceOverviewResponse = {
  generatedAt: string
  runtimeConnected: boolean
  message?: string
  projectCount: number
  serviceCount: number
  capacity: CapacitySummary
  counts: EfficiencyCounts
  services: ServiceResourceEfficiency[]
}

export type DeploymentRecord = {
  deploymentId: string
  applicationId: string
  projectId: string
  applicationName: string
  environment: string
  image: string
  imageTag: string
  deploymentStrategy: 'Rollout' | 'Canary'
  status: string
  syncStatus?: SyncStatus
  rolloutPhase?: string
  currentStep?: number
  canaryWeight?: number
  stableRevision?: string
  canaryRevision?: string
  message?: string
  commitSha?: string
  createdAt: string
  updatedAt: string
}

export type DeploymentListResponse = {
  applicationId: string
  items: DeploymentRecord[]
}

export type ContainerLogStream = {
  podName: string
  containerName: string
  phase?: string
  ready: boolean
  restartCount: number
  content: string
}

export type ContainerLogsResponse = {
  applicationId: string
  collectedAt: string
  tailLines: number
  items: ContainerLogStream[]
}

export type ContainerResourceStatus = {
  cpuUsageCores?: number
  cpuRequestCores?: number
  cpuLimitCores?: number
  cpuRequestUtilization?: number
  cpuLimitUtilization?: number
  memoryUsageMiB?: number
  memoryRequestMiB?: number
  memoryLimitMiB?: number
  memoryRequestUtilization?: number
  memoryLimitUtilization?: number
}

export type ContainerLogTargetContainer = {
  name: string
  ready: boolean
  restartCount: number
  default: boolean
  resourceStatus?: ContainerResourceStatus
}

export type ContainerLogTarget = {
  podName: string
  phase?: string
  containers: ContainerLogTargetContainer[]
}

export type ContainerLogTargetsResponse = {
  applicationId: string
  collectedAt: string
  items: ContainerLogTarget[]
}

export type ContainerLogEvent = {
  podName: string
  containerName: string
  timestamp?: string
  message: string
  rawLine: string
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

export type ChangeStatus = 'Draft' | 'Submitted' | 'Approved' | 'Merged'

export type ChangeRecord = {
  id: string
  projectId: string
  applicationId?: string
  operation: ChangeOperation
  environment: string
  writeMode: WriteMode
  status: ChangeStatus
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
  deploymentStrategy?: 'Rollout' | 'Canary'
  environment?: string
  imageTag?: string
  secrets?: SecretEntry[]
  repositoryUrl?: string
  repositoryBranch?: string
  repositoryToken?: string
  repositoryServiceId?: string
  configPath?: string
  registryServer?: string
  registryUsername?: string
  registryToken?: string
  policies?: ProjectPolicy
  summary?: string
}

export type SyncStatusResponse = {
  applicationId: string
  status: SyncStatus
  message?: string
  observedAt?: string
  repositoryPoll?: RepositoryPollStatus
}

export type NetworkExposureStatus = 'Internal' | 'Pending' | 'Provisioning' | 'Ready' | 'Error'

export type NetworkExposureEvent = {
  type: string
  reason?: string
  message: string
  observedAt?: string
}

export type NetworkExposurePort = {
  name?: string
  protocol?: string
  port: number
  targetPort?: string
  nodePort?: number
}

export type NetworkExposureResponse = {
  applicationId: string
  enabled: boolean
  status: NetworkExposureStatus
  message?: string
  serviceType?: string
  addresses?: string[]
  ports?: NetworkExposurePort[]
  lastEvent?: NetworkExposureEvent | null
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

export type MetricSeriesDiagnostic = {
  key: string
  label: string
  unit: string
  status: HealthSignalStatus
  message: string
  pointCount: number
  valueCount: number
  latestValue?: number | null
}

export type MetricsScrapeTarget = {
  name: string
  port: string
  path: string
  required: boolean
}

export type MetricsDiagnosticsResponse = {
  applicationId: string
  checkedAt: string
  status: HealthSignalStatus
  message: string
  meshEnabled: boolean
  scrapeTargets: MetricsScrapeTarget[]
  series: MetricSeriesDiagnostic[]
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
