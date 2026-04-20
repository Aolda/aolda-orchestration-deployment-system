import type { ComponentProps } from 'react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '../../testing/test-utils'
import { SidebarNav } from './SidebarNav'

type SidebarNavProps = ComponentProps<typeof SidebarNav>

function buildProps(overrides: Partial<SidebarNavProps> = {}): SidebarNavProps {
  return {
    activeSection: 'projects',
    onSectionChange: vi.fn(),
    projects: [
      {
        id: 'project-a',
        name: '공용 프로젝트',
        namespace: 'shared',
        role: 'admin',
      },
      {
        id: 'project-b',
        name: 'Project B',
        namespace: 'project-b',
        role: 'deployer',
      },
    ],
    selectedProjectId: 'project-a',
    onProjectSelect: vi.fn(),
    canCreateProject: false,
    ...overrides,
  }
}

describe('SidebarNav', () => {
  it('[US-NAV-001] 글로벌 사이드바는 프로젝트 메뉴 아래에 접근 가능한 프로젝트 목록을 노출한다', async () => {
    const user = userEvent.setup()
    const onSectionChange = vi.fn()
    const onProjectSelect = vi.fn()

    render(
      <SidebarNav
        {...buildProps({
          onSectionChange,
          onProjectSelect,
        })}
      />,
    )

    expect(screen.getByText('접근 가능한 프로젝트')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /공용 프로젝트/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Project B/i })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Project B/i }))

    expect(onProjectSelect).toHaveBeenCalledWith('project-b')
    expect(onSectionChange).not.toHaveBeenCalled()
  })

  it('[US-NAV-002] 플랫폼 관리자는 프로젝트 사이드바에서 새 프로젝트 액션을 연다', async () => {
    const user = userEvent.setup()
    const onCreateProject = vi.fn()

    render(
      <SidebarNav
        {...buildProps({
          canCreateProject: true,
          onCreateProject,
        })}
      />,
    )

    await user.click(screen.getByRole('button', { name: '새 프로젝트' }))

    expect(onCreateProject).toHaveBeenCalledTimes(1)
  })

  it('[US-NAV-003] 새 프로젝트 액션은 프로젝트 목록보다 먼저 노출된다', () => {
    render(
      <SidebarNav
        {...buildProps({
          canCreateProject: true,
          onCreateProject: vi.fn(),
        })}
      />,
    )

    const createButton = screen.getByRole('button', { name: '새 프로젝트' })
    const firstProjectButton = screen.getByRole('button', { name: /공용 프로젝트/i })

    expect(createButton.compareDocumentPosition(firstProjectButton) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })
})
