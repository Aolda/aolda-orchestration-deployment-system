import type {
  Application,
  ApplicationListResponse,
  ApplicationLifecycleResponse,
  ApplicationMetricsResponse,
  ApplicationSecretsResponse,
  ApplicationSecretVersionsResponse,
  ChangeRecord,
  ClusterListResponse,
  ClusterSummary,
  ContainerLogEvent,
  ContainerLogsResponse,
  ContainerLogTargetsResponse,
  CreateApplicationRequest,
  PreviewApplicationSourceRequest,
  PreviewApplicationSourceResponse,
  CreateClusterRequest,
  CreateChangeRequest,
  CreateDeploymentRequest,
  CreateProjectRequest,
  CurrentUser,
  DeploymentRecord,
  DeploymentListResponse,
  EnvironmentListResponse,
  ErrorResponse,
  EventListResponse,
  FleetResourceOverviewResponse,
  MetricsDiagnosticsResponse,
  NetworkExposureResponse,
  ProjectListResponse,
  ProjectHealthResponse,
  ProjectLifecycleResponse,
  ProjectSummary,
  ProjectPolicy,
  RepositorySyncResponse,
  RepositoryListResponse,
  RollbackPolicy,
  SyncStatusResponse,
  UpdateApplicationRequest,
  UpdateApplicationSecretsRequest,
} from '../types/api'
import {
  clearEmergencyAuthSession,
  clearOIDCSession,
  ensureOIDCAccessToken,
  hasEmergencyAuthSession,
  isOIDCAuthEnabled,
} from '../auth/oidc'

const apiBaseUrl = (import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080').trim()
const apiRequestTimeoutMs = Number(import.meta.env.VITE_AODS_API_REQUEST_TIMEOUT_MS ?? '15000')

function buildApiUrl(path: string) {
  const normalizedBase = apiBaseUrl.replace(/\/+$/, '')
  const normalizedPath = path.startsWith('/') ? path : `/${path}`

  if (normalizedBase.endsWith('/api/v1') && normalizedPath.startsWith('/api/v1/')) {
    return `${normalizedBase}${normalizedPath.slice('/api/v1'.length)}`
  }

  if (normalizedBase.endsWith('/api') && normalizedPath.startsWith('/api/')) {
    return `${normalizedBase}${normalizedPath.slice('/api'.length)}`
  }

  return `${normalizedBase}${normalizedPath}`
}

export class ApiError extends Error {
  code: string
  details?: Record<string, unknown>

  constructor(message: string, code: string, details?: Record<string, unknown>) {
    super(message)
    this.code = code
    this.details = details
  }
}

async function buildRequestHeaders(initHeaders?: HeadersInit) {
  const headers = new Headers(initHeaders ?? {})
  if (!headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  if (isOIDCAuthEnabled() && !hasEmergencyAuthSession() && !headers.has('Authorization')) {
    const accessToken = await ensureOIDCAccessToken()
    if (accessToken) {
      headers.set('Authorization', `Bearer ${accessToken}`)
    }
  }

  return headers
}

function handleUnauthorizedResponse(response: Response) {
  if (response.status === 401 && isOIDCAuthEnabled()) {
    clearOIDCSession()
    clearEmergencyAuthSession()
  }
}

function parseApiErrorPayload(text: string) {
  if (!text) {
    return null
  }
  try {
    return JSON.parse(text) as ErrorResponse
  } catch {
    return null
  }
}

async function request<T>(path: string, init?: RequestInit) {
  const headers = await buildRequestHeaders(init?.headers)

  const response = await fetchWithTimeout(buildApiUrl(path), {
    ...init,
    headers,
  })

  handleUnauthorizedResponse(response)

  const text = await response.text()
  const payload = text ? (JSON.parse(text) as T | ErrorResponse) : null

  if (!response.ok) {
    const errorPayload = payload as ErrorResponse | null
    throw new ApiError(
      errorPayload?.error.message ?? response.statusText,
      errorPayload?.error.code ?? 'UNKNOWN_ERROR',
      errorPayload?.error.details,
    )
  }

  return payload as T
}

async function fetchWithTimeout(input: RequestInfo | URL, init?: RequestInit) {
  const controller = new AbortController()
  const timeout = window.setTimeout(() => controller.abort(), apiRequestTimeoutMs)

  try {
    return await fetch(input, {
      ...init,
      signal: init?.signal ?? controller.signal,
    })
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      throw new ApiError(
        `AODS API 응답이 지연되어 요청을 중단했습니다. ${Math.round(apiRequestTimeoutMs / 1000)}초 뒤 다시 시도하세요.`,
        'REQUEST_TIMEOUT',
      )
    }
    if (error instanceof TypeError) {
      throw new ApiError(
        'AODS API에 연결하지 못했습니다. 백엔드가 실행 중인지 확인하세요.',
        'NETWORK_ERROR',
      )
    }
    throw error
  } finally {
    window.clearTimeout(timeout)
  }
}

type StreamApplicationLogsOptions = {
  podName: string
  containerName: string
  tailLines?: number
  signal?: AbortSignal
  onEvent: (event: ContainerLogEvent) => void
}

function parseSSEMessage(chunk: string) {
  let event = 'message'
  const dataLines: string[] = []

  chunk.split('\n').forEach((line) => {
    if (line.startsWith('event:')) {
      event = line.slice('event:'.length).trim()
      return
    }
    if (line.startsWith('data:')) {
      dataLines.push(line.slice('data:'.length).trimStart())
    }
  })

  return {
    event,
    data: dataLines.join('\n'),
  }
}

async function consumeSSEStream(
  response: Response,
  { onEvent, signal }: Pick<StreamApplicationLogsOptions, 'onEvent' | 'signal'>,
) {
  if (!response.body) {
    return
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    buffer += decoder.decode(value ?? new Uint8Array(), { stream: !done }).replace(/\r\n/g, '\n')

    let boundary = buffer.indexOf('\n\n')
    while (boundary >= 0) {
      const rawMessage = buffer.slice(0, boundary)
      buffer = buffer.slice(boundary + 2)
      const message = parseSSEMessage(rawMessage)

      if (message.event === 'log' && message.data) {
        onEvent(JSON.parse(message.data) as ContainerLogEvent)
      } else if (message.event === 'error' && message.data) {
        const payload = JSON.parse(message.data) as { message?: string }
        throw new ApiError(payload.message ?? '로그 스트림 처리 중 오류가 발생했습니다.', 'STREAM_ERROR')
      }

      boundary = buffer.indexOf('\n\n')
    }

    if (done) {
      break
    }
    if (signal?.aborted) {
      return
    }
  }
}

export const api = {
  getCurrentUser() {
    return request<CurrentUser>('/api/v1/me')
  },
  getProjects() {
    return request<ProjectListResponse>('/api/v1/projects')
  },
  createProject(body: CreateProjectRequest) {
    return request<ProjectSummary>('/api/v1/projects', {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  deleteProject(projectId: string) {
    return request<ProjectLifecycleResponse>(`/api/v1/projects/${projectId}`, {
      method: 'DELETE',
    })
  },
  getApplications(projectId: string) {
    return request<ApplicationListResponse>(`/api/v1/projects/${projectId}/applications`)
  },
  getProjectHealth(projectId: string) {
    return request<ProjectHealthResponse>(`/api/v1/projects/${projectId}/health`)
  },
  previewApplicationSource(projectId: string, body: PreviewApplicationSourceRequest) {
    return request<PreviewApplicationSourceResponse>(`/api/v1/projects/${projectId}/applications/source-preview`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  createApplication(projectId: string, body: CreateApplicationRequest) {
    return request<Application>(`/api/v1/projects/${projectId}/applications`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  getProjectEnvironments(projectId: string) {
    return request<EnvironmentListResponse>(`/api/v1/projects/${projectId}/environments`)
  },
  getProjectRepositories(projectId: string) {
    return request<RepositoryListResponse>(`/api/v1/projects/${projectId}/repositories`)
  },
  getProjectPolicies(projectId: string) {
    return request<ProjectPolicy>(`/api/v1/projects/${projectId}/policies`)
  },
  updateProjectPolicies(projectId: string, body: ProjectPolicy) {
    return request<ProjectPolicy>(`/api/v1/projects/${projectId}/policies`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    })
  },
  getClusters() {
    return request<ClusterListResponse>('/api/v1/clusters')
  },
  getAdminResourceOverview() {
    return request<FleetResourceOverviewResponse>('/api/v1/admin/resource-overview')
  },
  createCluster(body: CreateClusterRequest) {
    return request<ClusterSummary>('/api/v1/clusters', {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  createChange(projectId: string, body: CreateChangeRequest) {
    return request<ChangeRecord>(`/api/v1/projects/${projectId}/changes`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  getChange(changeId: string) {
    return request<ChangeRecord>(`/api/v1/changes/${changeId}`)
  },
  submitChange(changeId: string) {
    return request<ChangeRecord>(`/api/v1/changes/${changeId}/submit`, {
      method: 'POST',
    })
  },
  approveChange(changeId: string) {
    return request<ChangeRecord>(`/api/v1/changes/${changeId}/approve`, {
      method: 'POST',
    })
  },
  mergeChange(changeId: string) {
    return request<ChangeRecord>(`/api/v1/changes/${changeId}/merge`, {
      method: 'POST',
    })
  },
  patchApplication(applicationId: string, body: UpdateApplicationRequest) {
    return request<Application>(`/api/v1/applications/${applicationId}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    })
  },
  getApplicationSecrets(applicationId: string) {
    return request<ApplicationSecretsResponse>(`/api/v1/applications/${applicationId}/secrets`)
  },
  updateApplicationSecrets(applicationId: string, body: UpdateApplicationSecretsRequest) {
    return request<ApplicationSecretsResponse>(`/api/v1/applications/${applicationId}/secrets`, {
      method: 'PUT',
      body: JSON.stringify(body),
    })
  },
  getApplicationSecretVersions(applicationId: string) {
    return request<ApplicationSecretVersionsResponse>(`/api/v1/applications/${applicationId}/secrets/versions`)
  },
  restoreApplicationSecretVersion(applicationId: string, version: number) {
    return request<ApplicationSecretsResponse>(`/api/v1/applications/${applicationId}/secrets/versions/${version}/restore`, {
      method: 'POST',
    })
  },
  archiveApplication(applicationId: string) {
    return request<ApplicationLifecycleResponse>(`/api/v1/applications/${applicationId}/archive`, {
      method: 'POST',
    })
  },
  deleteApplication(applicationId: string) {
    return request<ApplicationLifecycleResponse>(`/api/v1/applications/${applicationId}`, {
      method: 'DELETE',
    })
  },
  createDeployment(applicationId: string, imageTag: string, environment?: string) {
    const body: CreateDeploymentRequest = { imageTag, environment }
    return request(`/api/v1/applications/${applicationId}/deployments`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  getDeployments(applicationId: string) {
    return request<DeploymentListResponse>(`/api/v1/applications/${applicationId}/deployments`)
  },
  getDeployment(applicationId: string, deploymentId: string) {
    return request<DeploymentRecord>(`/api/v1/applications/${applicationId}/deployments/${deploymentId}`)
  },
  promoteDeployment(applicationId: string, deploymentId: string) {
    return request<DeploymentRecord>(
      `/api/v1/applications/${applicationId}/deployments/${deploymentId}/promote`,
      { method: 'POST' },
    )
  },
  abortDeployment(applicationId: string, deploymentId: string) {
    return request<DeploymentRecord>(
      `/api/v1/applications/${applicationId}/deployments/${deploymentId}/abort`,
      { method: 'POST' },
    )
  },
  getSyncStatus(applicationId: string) {
    return request<SyncStatusResponse>(`/api/v1/applications/${applicationId}/sync-status`)
  },
  syncApplicationRepository(applicationId: string) {
    return request<RepositorySyncResponse>(`/api/v1/applications/${applicationId}/sync`, {
      method: 'POST',
    })
  },
  getApplicationNetworkExposure(applicationId: string) {
    return request<NetworkExposureResponse>(`/api/v1/applications/${applicationId}/network-exposure`)
  },
  getMetrics(applicationId: string, range?: string) {
    const params = new URLSearchParams()
    if (range) {
      params.append('range', range)
    }
    const query = params.toString() ? `?${params.toString()}` : ''
    return request<ApplicationMetricsResponse>(
      `/api/v1/applications/${applicationId}/metrics${query}`,
    )
  },
  getMetricsDiagnostics(applicationId: string, range?: string) {
    const params = new URLSearchParams()
    if (range) {
      params.append('range', range)
    }
    const query = params.toString() ? `?${params.toString()}` : ''
    return request<MetricsDiagnosticsResponse>(
      `/api/v1/applications/${applicationId}/metrics/diagnostics${query}`,
    )
  },
  getRollbackPolicy(applicationId: string) {
    return request<RollbackPolicy>(`/api/v1/applications/${applicationId}/rollback-policies`)
  },
  saveRollbackPolicy(applicationId: string, body: RollbackPolicy) {
    return request<RollbackPolicy>(`/api/v1/applications/${applicationId}/rollback-policies`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  getEvents(applicationId: string) {
    return request<EventListResponse>(`/api/v1/applications/${applicationId}/events`)
  },
  getApplicationLogs(applicationId: string, tailLines = 120) {
    const params = new URLSearchParams()
    params.set('tailLines', String(tailLines))
    return request<ContainerLogsResponse>(
      `/api/v1/applications/${applicationId}/logs?${params.toString()}`,
    )
  },
  getApplicationLogTargets(applicationId: string) {
    return request<ContainerLogTargetsResponse>(`/api/v1/applications/${applicationId}/logs/targets`)
  },
  async streamApplicationLogs(applicationId: string, options: StreamApplicationLogsOptions) {
    const params = new URLSearchParams()
    params.set('podName', options.podName)
    params.set('containerName', options.containerName)
    params.set('tailLines', String(options.tailLines ?? 120))

    const headers = await buildRequestHeaders({
      Accept: 'text/event-stream',
    })
    headers.delete('Content-Type')

    const response = await fetch(
      buildApiUrl(`/api/v1/applications/${applicationId}/logs/stream?${params.toString()}`),
      {
        method: 'GET',
        headers,
        signal: options.signal,
      },
    )

    handleUnauthorizedResponse(response)

    if (!response.ok) {
      const text = await response.text()
      const payload = parseApiErrorPayload(text)
      throw new ApiError(
        payload?.error.message ?? response.statusText,
        payload?.error.code ?? 'UNKNOWN_ERROR',
        payload?.error.details,
      )
    }

    await consumeSSEStream(response, options)
  },
}
