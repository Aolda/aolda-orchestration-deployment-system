import { Component, type ErrorInfo, type ReactNode } from 'react'
import { Alert, Button, Stack, Text } from '@mantine/core'
import { IconAlertTriangle, IconRefresh } from '@tabler/icons-react'

type AppErrorBoundaryProps = {
  children: ReactNode
}

type AppErrorBoundaryState = {
  error: Error | null
}

export class AppErrorBoundary extends Component<AppErrorBoundaryProps, AppErrorBoundaryState> {
  state: AppErrorBoundaryState = { error: null }

  static getDerivedStateFromError(error: Error): AppErrorBoundaryState {
    return { error }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('AODS UI render crashed', error, errorInfo.componentStack)
  }

  private reloadPage = () => {
    window.location.reload()
  }

  render() {
    if (!this.state.error) {
      return this.props.children
    }

    return (
      <div style={{ minHeight: '100vh', display: 'grid', placeItems: 'center', padding: 24, background: '#f8fafc' }}>
        <Stack gap="md" maw={720}>
          <Alert color="red" radius="md" icon={<IconAlertTriangle size={18} />} title="화면 렌더링 오류가 발생했습니다">
            <Stack gap={6}>
              <Text size="sm">
                클릭 중 화면이 비어 보이지 않도록 오류 화면으로 전환했습니다. 새로고침 후 같은 동작에서 반복되면 콘솔의 오류 메시지를 확인해 주세요.
              </Text>
              <Text size="xs" c="dimmed" style={{ wordBreak: 'break-word' }}>
                {this.state.error.message || '알 수 없는 렌더링 오류'}
              </Text>
            </Stack>
          </Alert>
          <Button leftSection={<IconRefresh size={16} />} color="lagoon.6" onClick={this.reloadPage}>
            화면 새로고침
          </Button>
        </Stack>
      </div>
    )
  }
}
