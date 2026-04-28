import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '../testing/test-utils'
import { ApplicationWizard, type CreateFormState } from './ApplicationWizard'

function buildInitialState(overrides: Partial<CreateFormState> = {}): CreateFormState {
  return {
    sourceMode: 'github',
    name: '',
    description: '',
    image: '',
    servicePort: 8080,
    deploymentStrategy: 'Rollout',
    environment: 'dev',
    repositoryUrl: '',
    repositoryBranch: '',
    repositoryToken: '',
    repositoryServiceId: '',
    configPath: 'aolda_deploy.json',
    registryServer: '',
    registryUsername: '',
    registryToken: '',
    secrets: [{ key: '', value: '' }],
    ...overrides,
  }
}

describe('ApplicationWizard', () => {
  it('[US-APP-001] 기본으로 공개 저장소 기준의 GitHub 등록 흐름을 보여준다', async () => {
    const user = userEvent.setup()

    render(
      <ApplicationWizard
        projectId="shared"
        projectName="공용 프로젝트"
        environments={[
          { id: 'shared', name: '공용', writeMode: 'direct', default: true },
          { id: 'prod', name: '운영', writeMode: 'pull_request' },
        ]}
        allowedStrategies={['Rollout', 'Canary']}
        initialState={buildInitialState()}
        onPreviewSource={vi.fn().mockResolvedValue({
          configPath: 'aolda_deploy.json',
          services: [{ serviceId: 'example-app', image: 'ghcr.io/aods/example-app:sha-abc1234', port: 3000, replicas: 1, strategy: 'Rollout' }],
          selectedServiceId: 'example-app',
          requiresServiceSelection: false,
        })}
        onSubmit={vi.fn().mockResolvedValue(undefined)}
        onCancel={vi.fn()}
        submitting={false}
      />,
    )

    expect(screen.getByText('GitHub에서 읽기')).toBeInTheDocument()
    expect(screen.getByText('연결 마법사로 진행')).toBeInTheDocument()
    expect(screen.getByText('설정 페이지 열기')).toBeInTheDocument()
    expect(screen.getByText('예시 JSON 다운로드')).toBeInTheDocument()
    expect(screen.getByText('GitHub 연결')).toBeInTheDocument()
    expect(screen.getByText('이미지 접근')).toBeInTheDocument()
    expect(screen.queryByLabelText('레지스트리 사용자명')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('저장소 내 서비스 ID')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('애플리케이션 이름')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '다음 단계' }))

    expect(await screen.findByPlaceholderText('예: https://github.com/aods/example-app.git')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Public 저장소')).toBeInTheDocument()
    expect(screen.getByDisplayValue('aolda_deploy.json')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장소 연결 확인' })).toBeInTheDocument()
  })

  it('[US-APP-002] GitHub 등록에서 저장소 URL 없이 다음 단계로 넘어가면 즉시 검증 메시지를 보여준다', async () => {
    const user = userEvent.setup()

    render(
      <ApplicationWizard
        projectId="shared"
        environments={[{ id: 'shared', name: '공용', writeMode: 'direct', default: true }]}
        allowedStrategies={['Rollout']}
        initialState={buildInitialState()}
        onPreviewSource={vi.fn().mockResolvedValue({
          configPath: 'aolda_deploy.json',
          services: [{ serviceId: 'example-app', image: 'ghcr.io/aods/example-app:sha-abc1234', port: 3000, replicas: 1, strategy: 'Rollout' }],
          selectedServiceId: 'example-app',
          requiresServiceSelection: false,
        })}
        onSubmit={vi.fn().mockResolvedValue(undefined)}
        onCancel={vi.fn()}
        submitting={false}
      />,
    )

    await user.click(screen.getByRole('button', { name: '다음 단계' }))
    await user.click(screen.getByRole('button', { name: '다음 단계' }))

    expect(screen.getAllByText('GitHub 저장소 URL을 입력하세요.').length).toBeGreaterThan(0)
    expect(screen.getByPlaceholderText('예: https://github.com/aods/example-app.git')).toHaveFocus()
  })

  it('[US-APP-003] 공개 저장소는 토큰 없이도 다음 단계로 이동할 수 있다', async () => {
    const user = userEvent.setup()

    render(
      <ApplicationWizard
        projectId="shared"
        environments={[{ id: 'shared', name: '공용', writeMode: 'direct', default: true }]}
        allowedStrategies={['Rollout']}
        initialState={buildInitialState({
          repositoryUrl: 'https://github.com/aods/example-app.git',
        })}
        onPreviewSource={vi.fn().mockResolvedValue({
          configPath: 'aolda_deploy.json',
          services: [{ serviceId: 'example-app', image: 'ghcr.io/aods/example-app:sha-abc1234', port: 3000, replicas: 1, strategy: 'Rollout' }],
          selectedServiceId: 'example-app',
          requiresServiceSelection: false,
        })}
        onSubmit={vi.fn().mockResolvedValue(undefined)}
        onCancel={vi.fn()}
        submitting={false}
      />,
    )

    await user.click(screen.getByRole('button', { name: '다음 단계' }))
    expect(await screen.findByText('저장소 접근 가능')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '다음 단계' }))

    expect(await screen.findByText('저장소 서비스 미리보기')).toBeInTheDocument()
    expect(screen.getByText('확인 결과')).toBeInTheDocument()
    expect(screen.queryByText('GitHub 저장소 URL을 입력하세요.')).not.toBeInTheDocument()
  })

  it('[US-APP-004] 레지스트리 사용자명과 토큰을 하나만 입력하면 검증 메시지를 보여준다', async () => {
    const user = userEvent.setup()

    render(
      <ApplicationWizard
        projectId="shared"
        environments={[{ id: 'shared', name: '공용', writeMode: 'direct', default: true }]}
        allowedStrategies={['Rollout']}
        initialState={buildInitialState({
          repositoryUrl: 'https://github.com/aods/example-app.git',
          name: 'example-app',
          registryUsername: 'octocat',
        })}
        onPreviewSource={vi.fn().mockResolvedValue({
          configPath: 'aolda_deploy.json',
          services: [{ serviceId: 'example-app', image: 'ghcr.io/aods/example-app:sha-abc1234', port: 3000, replicas: 1, strategy: 'Rollout' }],
          selectedServiceId: 'example-app',
          requiresServiceSelection: false,
        })}
        onSubmit={vi.fn().mockResolvedValue(undefined)}
        onCancel={vi.fn()}
        submitting={false}
      />,
    )

    await user.click(screen.getByRole('button', { name: '다음 단계' }))
    expect(await screen.findByText('저장소 접근 가능')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '다음 단계' }))
    await user.click(screen.getByRole('button', { name: '다음 단계' }))
    await user.click(screen.getByRole('button', { name: '다음 단계' }))

    expect(screen.getAllByText('레지스트리 사용자명과 레지스트리 토큰은 함께 입력하세요.').length).toBeGreaterThan(0)
  })

  it('[US-APP-005] 비밀값 입력칸은 브라우저 자동완성 방지 속성을 가진다', async () => {
    const user = userEvent.setup()

    render(
      <ApplicationWizard
        projectId="shared"
        environments={[{ id: 'shared', name: '공용', writeMode: 'direct', default: true }]}
        allowedStrategies={['Rollout']}
        initialState={buildInitialState({
          repositoryUrl: 'https://github.com/aods/example-app.git',
        })}
        onPreviewSource={vi.fn().mockResolvedValue({
          configPath: 'aolda_deploy.json',
          services: [{ serviceId: 'example-app', image: 'ghcr.io/aods/example-app:sha-abc1234', port: 3000, replicas: 1, strategy: 'Rollout' }],
          selectedServiceId: 'example-app',
          requiresServiceSelection: false,
        })}
        onSubmit={vi.fn().mockResolvedValue(undefined)}
        onCancel={vi.fn()}
        submitting={false}
      />,
    )

    await user.click(screen.getByRole('button', { name: '다음 단계' }))
    expect(await screen.findByText('저장소 접근 가능')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '다음 단계' }))
    expect(await screen.findByText('저장소 서비스 미리보기')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '다음 단계' }))
    await user.click(screen.getByRole('button', { name: '다음 단계' }))
    await user.click(screen.getByRole('button', { name: '다음 단계' }))

    expect(screen.getByLabelText('키')).toHaveAttribute('name', 'aods-secret-key-0')
    expect(screen.getByLabelText('키')).toHaveAttribute('autocomplete', 'off')
    expect(screen.getByLabelText('값')).toHaveAttribute('name', 'aods-secret-value-0')
    expect(screen.getByLabelText('값')).toHaveAttribute('autocomplete', 'new-password')
  })

  it('[US-APP-006] 설정 파일 확인 단계에서 여러 서비스를 보여주고 선택할 수 있다', async () => {
    const user = userEvent.setup()

    render(
      <ApplicationWizard
        projectId="shared"
        environments={[{ id: 'shared', name: '공용', writeMode: 'direct', default: true }]}
        allowedStrategies={['Rollout']}
        initialState={buildInitialState({
          repositoryUrl: 'https://github.com/aods/example-app.git',
        })}
        onPreviewSource={vi.fn().mockResolvedValue({
          configPath: 'aolda_deploy.json',
          services: [
            { serviceId: 'example-web', image: 'ghcr.io/aods/example-web:sha-abc1234', port: 3000, replicas: 1, strategy: 'Rollout' },
            { serviceId: 'example-api', image: 'ghcr.io/aods/example-api:sha-abc1234', port: 8080, replicas: 1, strategy: 'Canary' },
          ],
          requiresServiceSelection: true,
        })}
        onSubmit={vi.fn().mockResolvedValue(undefined)}
        onCancel={vi.fn()}
        submitting={false}
      />,
    )

    await user.click(screen.getByRole('button', { name: '다음 단계' }))
    expect(await screen.findByText('저장소 접근 가능')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '다음 단계' }))

    expect(await screen.findByText('감지된 서비스 수:')).toBeInTheDocument()
    expect(screen.getByText('example-web')).toBeInTheDocument()
    expect(screen.getByText('example-api')).toBeInTheDocument()

    await user.click(screen.getAllByRole('button', { name: '이 서비스 선택' })[0])

    expect(screen.getByText('선택 완료')).toBeInTheDocument()
  })

  it('[US-APP-007] 빠른 생성은 설정 파일 확인 단계를 건너뛰고 배포 설정으로 이동한다', async () => {
    const user = userEvent.setup()

    render(
      <ApplicationWizard
        projectId="shared"
        environments={[{ id: 'shared', name: '공용', writeMode: 'direct', default: true }]}
        allowedStrategies={['Rollout']}
        initialState={buildInitialState({
          sourceMode: 'quick',
          name: 'payment-api',
          image: 'ghcr.io/aods/payment-api:1.0.0',
        })}
        onPreviewSource={vi.fn()}
        onSubmit={vi.fn().mockResolvedValue(undefined)}
        onCancel={vi.fn()}
        submitting={false}
      />,
    )

    await user.click(screen.getByRole('button', { name: '다음 단계' }))

    expect(screen.getByText('컨테이너 이미지를 pull할 수 있게 준비합니다')).toBeInTheDocument()
    expect(screen.getByText('배포 이미지')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '다음 단계' }))

    expect(screen.getByText('현재 프로젝트 정책')).toBeInTheDocument()
    expect(screen.getByText('서비스 포트')).toBeInTheDocument()
    expect(screen.queryByText('빠른 생성 모드에서는 저장소 설정 파일을 읽지 않으므로')).not.toBeInTheDocument()
  })
})
