import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import { render, screen, waitFor } from './testing/test-utils'

const apiMock = vi.hoisted(() => ({
  getCurrentUser: vi.fn(),
  getProjects: vi.fn(),
  getClusters: vi.fn(),
  getApplications: vi.fn(),
  getProjectHealth: vi.fn(),
  getProjectEnvironments: vi.fn(),
  getProjectPolicies: vi.fn(),
  getMetrics: vi.fn(),
  getSyncStatus: vi.fn(),
  getApplicationNetworkExposure: vi.fn(),
  getDeployments: vi.fn(),
  getDeployment: vi.fn(),
  getEvents: vi.fn(),
  getRollbackPolicy: vi.fn(),
  getApplicationLogTargets: vi.fn(),
  streamApplicationLogs: vi.fn(),
}))

vi.mock('./api/client', () => ({
  ApiError: class ApiError extends Error {
    code: string
    details?: Record<string, unknown>

    constructor(message: string, code: string, details?: Record<string, unknown>) {
      super(message)
      this.code = code
      this.details = details
    }
  },
  api: apiMock,
}))

vi.mock('./auth/oidc', () => ({
  clearEmergencyAuthSession: vi.fn(),
  clearOIDCSession: vi.fn(),
  ensureOIDCAccessToken: vi.fn(async () => undefined),
  hasEmergencyAuthSession: vi.fn(() => false),
  isEmergencyLoginEnabled: vi.fn(() => false),
  isOIDCAuthEnabled: vi.fn(() => true),
  logoutOIDCSession: vi.fn(async () => undefined),
  shouldResumeOIDCSession: vi.fn(() => true),
  startEmergencyAuthSession: vi.fn(),
}))

const demoApplication = {
  id: 'shared__demo-web',
  name: 'demo-web',
  image: 'ghcr.io/aolda/demo-web:v1',
  deploymentStrategy: 'Rollout' as const,
  syncStatus: 'Synced' as const,
  meshEnabled: false,
  loadBalancerEnabled: false,
}

const demoDeployment = {
  deploymentId: 'deploy-1',
  applicationId: demoApplication.id,
  projectId: 'shared',
  applicationName: demoApplication.name,
  environment: 'dev',
  image: 'ghcr.io/aolda/demo-web:v1',
  imageTag: 'v1',
  deploymentStrategy: 'Rollout' as const,
  status: 'Completed',
  createdAt: '2026-05-16T00:00:00.000Z',
  updatedAt: '2026-05-16T00:00:00.000Z',
}

const demoPolicy = {
  minReplicas: 1,
  allowedEnvironments: ['dev'],
  allowedDeploymentStrategies: ['Rollout' as const],
  allowedClusterTargets: ['local'],
  prodPRRequired: false,
  autoRollbackEnabled: false,
  requiredProbes: true,
}

function arrangeApi() {
  apiMock.getCurrentUser.mockResolvedValue({
    id: 'user-1',
    username: 'operator',
    displayName: 'Operator',
    groups: [],
  })
  apiMock.getProjects.mockResolvedValue({
    items: [
      {
        id: 'shared',
        name: 'Shared',
        namespace: 'shared',
        role: 'admin',
      },
    ],
  })
  apiMock.getClusters.mockResolvedValue({ items: [] })
  apiMock.getApplications.mockResolvedValue({ items: [demoApplication] })
  apiMock.getProjectEnvironments.mockResolvedValue({
    items: [
      {
        id: 'dev',
        name: 'Development',
        clusterId: 'local',
        writeMode: 'direct',
        default: true,
      },
    ],
  })
  apiMock.getProjectPolicies.mockResolvedValue(demoPolicy)
  apiMock.getProjectHealth.mockResolvedValue({
    projectId: 'shared',
    observedAt: '2026-05-16T00:00:00.000Z',
    items: [
      {
        applicationId: demoApplication.id,
        name: demoApplication.name,
        namespace: 'shared',
        status: 'Healthy',
        syncStatus: 'Synced',
        deploymentStrategy: 'Rollout',
        metrics: [],
        latestDeployment: demoDeployment,
        signals: [],
      },
    ],
  })
  apiMock.getSyncStatus.mockResolvedValue({
    applicationId: demoApplication.id,
    status: 'Synced',
    observedAt: '2026-05-16T00:00:00.000Z',
  })
  apiMock.getDeployments.mockResolvedValue({
    applicationId: demoApplication.id,
    items: [demoDeployment],
  })
  apiMock.getEvents.mockResolvedValue({
    applicationId: demoApplication.id,
    items: [],
  })
  apiMock.getRollbackPolicy.mockResolvedValue({ enabled: false })
  apiMock.getApplicationNetworkExposure.mockResolvedValue({
    applicationId: demoApplication.id,
    enabled: false,
    status: 'Internal',
  })
  apiMock.getMetrics.mockResolvedValue({
    applicationId: demoApplication.id,
    metrics: [],
  })
  apiMock.getApplicationLogTargets.mockResolvedValue({
    applicationId: demoApplication.id,
    collectedAt: '2026-05-16T00:00:00.000Z',
    items: [],
  })
}

describe('App API fan-out', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    arrangeApi()
  })

  it('첫 프로젝트 화면은 상세 health, metrics, deployments API를 호출하지 않는다', async () => {
    render(<App />)

    await screen.findByText('demo-web')

    expect(apiMock.getApplications).toHaveBeenCalledWith('shared')
    expect(apiMock.getProjectHealth).not.toHaveBeenCalled()
    expect(apiMock.getMetrics).not.toHaveBeenCalled()
    expect(apiMock.getDeployments).not.toHaveBeenCalled()
  })

  it('모니터링 탭을 열 때 project health snapshot을 지연 호출한다', async () => {
    const user = userEvent.setup()
    render(<App />)

    await screen.findByText('demo-web')
    await user.click(screen.getByRole('tab', { name: '모니터링' }))

    await waitFor(() => {
      expect(apiMock.getProjectHealth).toHaveBeenCalledWith('shared')
    })
    expect(apiMock.getMetrics).not.toHaveBeenCalled()
    expect(apiMock.getDeployments).not.toHaveBeenCalled()
  })

  it('운영 센터는 탭별로 필요한 상세 API만 호출한다', async () => {
    const user = userEvent.setup()
    render(<App />)

    await user.click(await screen.findByText('demo-web'))

    await waitFor(() => {
      expect(apiMock.getSyncStatus).toHaveBeenCalledWith(demoApplication.id)
    })
    expect(apiMock.getDeployments).toHaveBeenCalledWith(demoApplication.id)
    expect(apiMock.getMetrics).not.toHaveBeenCalled()

    await user.click(screen.getByRole('tab', { name: '관측' }))

    await waitFor(() => {
      expect(apiMock.getMetrics).toHaveBeenCalledWith(demoApplication.id, '15m')
    })
  })
})
