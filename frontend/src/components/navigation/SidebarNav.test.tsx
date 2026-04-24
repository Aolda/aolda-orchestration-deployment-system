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
        id: 'shared',
        name: '공용 프로젝트',
        namespace: 'shared',
        role: 'admin',
      },
    ],
    selectedProjectId: 'shared',
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
    expect(screen.queryByRole('button', { name: '새 프로젝트' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /공용 프로젝트/i }))

    expect(onProjectSelect).toHaveBeenCalledWith('shared')
    expect(onSectionChange).not.toHaveBeenCalled()
  })

  it('프로젝트 생성 액션이 비활성화되면 새 프로젝트 버튼을 숨긴다', () => {
    render(<SidebarNav {...buildProps({ canCreateProject: false })} />)

    expect(screen.queryByRole('button', { name: '새 프로젝트' })).not.toBeInTheDocument()
  })

  it('허용된 글로벌 섹션만 사이드바에 노출한다', () => {
    render(<SidebarNav {...buildProps({ visibleSections: ['projects', 'me'] })} />)

    expect(screen.getByText('프로젝트')).toBeInTheDocument()
    expect(screen.getByText('내 정보')).toBeInTheDocument()
    expect(screen.queryByText('클러스터')).not.toBeInTheDocument()
  })
})
