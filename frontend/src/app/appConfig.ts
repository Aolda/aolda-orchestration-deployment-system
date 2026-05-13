export const supportedDeploymentStrategies = ['Rollout'] as const

export const repositoryPollIntervalOptions = [
  { value: '60', label: '1분' },
  { value: '300', label: '5분' },
  { value: '600', label: '10분' },
]

export const projectRefreshIntervalMs = 15000
export const applicationDetailsRefreshIntervalMs = 15000
export const externalInternetConnectionURL = 'https://itda.aoldacloud.com/login'

export const showProjectComposer = false
export const showRollbackPolicyControls = false
export const showServiceMeshControls = false
export const showEmergencyActionControls = false
export const showApplicationLifecycleControls = true

export const cpuLimitPresetOptions = [
  { value: '500m', label: '기본 상한 500m' },
  { value: '1000m', label: '확장 상한 1000m' },
]

export const memoryLimitPresetOptions = [
  { value: '512Mi', label: '기본 상한 512Mi' },
  { value: '1Gi', label: '확장 상한 1Gi' },
]
