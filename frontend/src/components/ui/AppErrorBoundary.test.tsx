import type { ReactElement } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '../../testing/test-utils'
import { AppErrorBoundary } from './AppErrorBoundary'

function CrashingPanel(): ReactElement {
  throw new Error('drawer tab render failed')
}

describe('AppErrorBoundary', () => {
  it('렌더 오류가 나도 흰 화면 대신 복구 가능한 오류 화면을 보여준다', () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})

    render(
      <AppErrorBoundary>
        <CrashingPanel />
      </AppErrorBoundary>,
    )

    expect(screen.getByText('화면 렌더링 오류가 발생했습니다')).toBeInTheDocument()
    expect(screen.getByText('drawer tab render failed')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '화면 새로고침' })).toBeInTheDocument()

    consoleError.mockRestore()
  })
})
