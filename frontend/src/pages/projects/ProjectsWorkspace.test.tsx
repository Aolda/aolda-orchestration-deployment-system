import type { ComponentProps } from 'react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '../../testing/test-utils'
import { ProjectsWorkspace } from './ProjectsWorkspace'

type ProjectsWorkspaceProps = ComponentProps<typeof ProjectsWorkspace>

function buildProps(overrides: Partial<ProjectsWorkspaceProps> = {}): ProjectsWorkspaceProps {
  return {
    projectTab: 'applications',
    onProjectTabChange: vi.fn(),
    applications: <div>애플리케이션 패널</div>,
    monitoring: <div>모니터링 패널</div>,
    rules: <div>운영 규칙 패널</div>,
    ...overrides,
  }
}

describe('ProjectsWorkspace', () => {
  it('[US-PROJ-001] 프로젝트를 선택하기 전에는 상세 운영 화면 진입 안내를 보여준다', () => {
    render(<ProjectsWorkspace {...buildProps()} />)

    expect(screen.getByText('프로젝트를 선택하면 상세 운영 화면이 열립니다.')).toBeInTheDocument()
  })

  it('[US-PROJ-002] 프로젝트를 선택하면 운영 탭과 프로젝트 컨텍스트가 열린다', () => {
    render(
      <ProjectsWorkspace
        {...buildProps({
          projectName: 'Payments',
        })}
      />,
    )

    expect(screen.getByRole('tab', { name: '애플리케이션' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: '모니터링' })).toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: '개요' })).not.toBeInTheDocument()
    expect(screen.getByText('애플리케이션 패널')).toBeInTheDocument()
  })

  it('[US-PROJ-003] 비활성 프로젝트 탭 패널은 DOM에서 제거해 숨은 화면 클릭과 렌더 오류를 막는다', async () => {
    const user = userEvent.setup()
    const onProjectTabChange = vi.fn()

    const { rerender } = render(
      <ProjectsWorkspace
        {...buildProps({
          projectName: 'Payments',
          onProjectTabChange,
        })}
      />,
    )

    expect(screen.getByText('애플리케이션 패널')).toBeInTheDocument()
    expect(screen.queryByText('모니터링 패널')).not.toBeInTheDocument()
    expect(screen.queryByText('운영 규칙 패널')).not.toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: '모니터링' }))
    expect(onProjectTabChange).toHaveBeenCalledWith('monitoring')

    rerender(
      <ProjectsWorkspace
        {...buildProps({
          projectName: 'Payments',
          projectTab: 'monitoring',
          onProjectTabChange,
        })}
      />,
    )

    expect(screen.queryByText('애플리케이션 패널')).not.toBeInTheDocument()
    expect(screen.getByText('모니터링 패널')).toBeInTheDocument()
  })
})
