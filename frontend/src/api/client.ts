import type {
  Application,
  ApplicationListResponse,
  ApplicationMetricsResponse,
  CreateApplicationRequest,
  CreateDeploymentRequest,
  CurrentUser,
  ErrorResponse,
  ProjectListResponse,
  SyncStatusResponse,
} from '../types/api'

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'

export class ApiError extends Error {
  code: string

  constructor(message: string, code: string) {
    super(message)
    this.code = code
  }
}

async function request<T>(path: string, init?: RequestInit) {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
    ...init,
  })

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
  createDeployment(applicationId: string, imageTag: string) {
    const body: CreateDeploymentRequest = { imageTag }
    return request(`/api/v1/applications/${applicationId}/deployments`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  getSyncStatus(applicationId: string) {
    return request<SyncStatusResponse>(`/api/v1/applications/${applicationId}/sync-status`)
  },
  getMetrics(applicationId: string) {
    return request<ApplicationMetricsResponse>(
      `/api/v1/applications/${applicationId}/metrics`,
    )
  },
}
