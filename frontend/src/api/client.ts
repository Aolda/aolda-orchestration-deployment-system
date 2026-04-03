import type {
  Application,
  ApplicationListResponse,
  ApplicationMetricsResponse,
  ChangeRecord,
  ClusterListResponse,
  CreateApplicationRequest,
  CreateChangeRequest,
  CreateDeploymentRequest,
  CurrentUser,
  DeploymentRecord,
  DeploymentListResponse,
  EnvironmentListResponse,
  ErrorResponse,
  EventListResponse,
  ProjectListResponse,
  ProjectPolicy,
  RepositoryListResponse,
  RollbackPolicy,
  SyncStatusResponse,
  UpdateApplicationRequest,
} from '../types/api'
import {
  clearOIDCSession,
  ensureOIDCAccessToken,
  isOIDCAuthEnabled,
} from '../auth/oidc'

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'

export class ApiError extends Error {
  code: string

  constructor(message: string, code: string) {
    super(message)
    this.code = code
  }
}

async function request<T>(path: string, init?: RequestInit) {
  const headers = new Headers(init?.headers ?? {})
  if (!headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  if (isOIDCAuthEnabled() && !headers.has('Authorization')) {
    const accessToken = await ensureOIDCAccessToken()
    if (accessToken) {
      headers.set('Authorization', `Bearer ${accessToken}`)
    }
  }

  const response = await fetch(`${apiBaseUrl}${path}`, {
    ...init,
    headers,
  })

  if (response.status === 401 && isOIDCAuthEnabled()) {
    clearOIDCSession()
  }

  const text = await response.text()
  const payload = text ? (JSON.parse(text) as T | ErrorResponse) : null

  if (!response.ok) {
    const errorPayload = payload as ErrorResponse | null
    throw new ApiError(
      errorPayload?.error.message ?? response.statusText,
      errorPayload?.error.code ?? 'UNKNOWN_ERROR',
    )
  }

  return payload as T
}

export const api = {
  getCurrentUser() {
    return request<CurrentUser>('/api/v1/me')
  },
  getProjects() {
    return request<ProjectListResponse>('/api/v1/projects')
  },
  getApplications(projectId: string) {
    return request<ApplicationListResponse>(`/api/v1/projects/${projectId}/applications`)
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
}
