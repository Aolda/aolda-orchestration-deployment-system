import type { ComponentProps } from 'react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type {
  ApplicationSummary,
  ChangeRecord,
  CurrentUser,
  EnvironmentSummary,
  ProjectSummary,
} from '../../types/api'
import { render, screen, waitFor } from '../../testing/test-utils'
import { ChangesWorkspace } from './ChangesWorkspace'

type ChangesWorkspaceProps = ComponentProps<typeof ChangesWorkspace>

const baseUser: CurrentUser = {
  id: 'user-1',
  username: 'alice',
  groups: ['platform-admins'],
}

const baseProject: ProjectSummary = {
  id: 'project-a',
  name: 'Payments',
  namespace: 'payments',
  role: 'admin',
}

const baseEnvironments: EnvironmentSummary[] = [
  {
    id: 'staging',
    name: 'Staging',
    clusterId: 'cluster-seoul-1',
    writeMode: 'pull_request',
    default: true,
  },
]

const baseApplications: ApplicationSummary[] = [
  {
    id: 'project-a__payments-api',
    name: 'payments-api',
    image: 'ghcr.io/aolda/payments-api:v1.0.0',
    deploymentStrategy: 'Rollout',
    syncStatus: 'Synced',
    meshEnabled: false,
    loadBalancerEnabled: false,
  },
]

function buildChange(overrides: Partial<ChangeRecord> = {}): ChangeRecord {
  return {
    id: 'chg_001',
    projectId: 'project-a',
    applicationId: 'project-a__payments-api',
    operation: 'Redeploy',
    environment: 'staging',
    writeMode: 'pull_request',
    status: 'Draft',
    summary: 'payments-api 이미지 태그 갱신',
    diffPreview: ['image.tag: v1.0.0 -> v1.0.1'],
    createdBy: 'alice',
    request: {
      operation: 'Redeploy',
      applicationId: 'project-a__payments-api',
      environment: 'staging',
      imageTag: 'v1.0.1',
    },
    createdAt: '2026-04-13T01:00:00Z',
    updatedAt: '2026-04-13T02:00:00Z',
    ...overrides,
  }
}

function buildProps(overrides: Partial<ChangesWorkspaceProps> = {}): ChangesWorkspaceProps {
  return {
    project: baseProject,
    currentUser: baseUser,
    applications: baseApplications,
    environments: baseEnvironments,
    changes: [],
    selectedChangeId: null,
    onSelectChange: vi.fn(),
    onCreateChange: vi.fn().mockResolvedValue(undefined),
    onTrackChange: vi.fn().mockResolvedValue(undefined),
    onRefreshChanges: vi.fn().mockResolvedValue(undefined),
    onSubmitChange: vi.fn().mockResolvedValue(undefined),
    onApproveChange: vi.fn().mockResolvedValue(undefined),
    onMergeChange: vi.fn().mockResolvedValue(undefined),
    creatingChange: false,
    refreshingChanges: false,
    actionLoading: null,
    onOpenProjectChanges: vi.fn(),
    ...overrides,
  }
}

describe('ChangesWorkspace', () => {
  it('[US-CHG-001] 프로젝트 문맥이 없으면 draft 생성 대신 빈 상태를 우선 노출한다', () => {
    render(
      <ChangesWorkspace
        {...buildProps({
          project: undefined,
          applications: [],
          environments: [],
        })}
      />,
    )

    expect(screen.getByText('프로젝트를 선택해야 변경 요청 작업 공간이 열립니다')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '추적 목록 새로고침' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Draft 생성' })).toBeDisabled()
  })

  it('[US-CHG-002] viewer 는 변경 요청을 조회할 수 있지만 draft 생성은 할 수 없다', () => {
    render(
      <ChangesWorkspace
        {...buildProps({
          project: {
            ...baseProject,
            role: 'viewer',
          },
        })}
      />,
    )

    expect(screen.getByText('viewer 역할은 change draft를 생성할 수 없습니다')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Draft 생성' })).toBeDisabled()
  })

  it('[US-CHG-003] 운영자는 tracked changes 를 검색해 원하는 변경만 좁혀서 본다', async () => {
    const user = userEvent.setup()

    render(
      <ChangesWorkspace
        {...buildProps({
          selectedChangeId: 'chg_hotfix',
          changes: [
            buildChange({
              id: 'chg_old',
              summary: 'payments-api 이미지 태그 갱신',
              updatedAt: '2026-04-13T01:00:00Z',
            }),
            buildChange({
              id: 'chg_hotfix',
              applicationId: 'project-a__checkout-api',
              summary: 'checkout hotfix 반영',
              diffPreview: ['image.tag: v2.1.0 -> v2.1.1-hotfix'],
              updatedAt: '2026-04-13T04:00:00Z',
            }),
          ],
        })}
      />,
    )

    await user.type(screen.getByRole('textbox', { name: '검색' }), 'hotfix')

    expect(screen.getAllByText('checkout hotfix 반영')).toHaveLength(2)
    expect(screen.queryByText('payments-api 이미지 태그 갱신')).not.toBeInTheDocument()
  })

  it('[US-CHG-004] 첫 진입 시에는 가장 최근 tracked change 를 자동 선택한다', async () => {
    const onSelectChange = vi.fn()

    render(
      <ChangesWorkspace
        {...buildProps({
          onSelectChange,
          changes: [
            buildChange({
              id: 'chg_old',
              summary: '이전 변경',
              updatedAt: '2026-04-13T01:00:00Z',
            }),
            buildChange({
              id: 'chg_new',
              summary: '가장 최근 변경',
              updatedAt: '2026-04-13T06:00:00Z',
            }),
          ],
        })}
      />,
    )

    await waitFor(() => {
      expect(onSelectChange).toHaveBeenCalledWith('chg_new')
    })
  })

  it('[US-CHG-005] pull_request 환경의 submitted change 는 관리자 승인 후에만 반영할 수 있다', () => {
    render(
      <ChangesWorkspace
        {...buildProps({
          project: {
            ...baseProject,
            role: 'admin',
          },
          selectedChangeId: 'chg_pr',
          changes: [
            buildChange({
              id: 'chg_pr',
              status: 'Submitted',
              writeMode: 'pull_request',
              summary: 'prod 반영 승인 대기',
            }),
          ],
        })}
      />,
    )

    expect(screen.getByRole('button', { name: '승인' })).toBeEnabled()
    expect(screen.getByRole('button', { name: '반영' })).toBeDisabled()
    expect(screen.getByText('pull_request 환경은 승인 후에만 반영할 수 있습니다.')).toBeInTheDocument()
  })
})
