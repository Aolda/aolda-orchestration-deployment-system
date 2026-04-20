import type { ChangeEvent } from 'react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  Badge,
  Button,
  Card,
  Code,
  Group,
  List,
  NumberInput,
  PasswordInput,
  Select,
  SimpleGrid,
  Stack,
  Stepper,
  Text,
  TextInput,
  Textarea,
} from '@mantine/core'
import type { PreviewApplicationSourceResponse, WriteMode } from '../types/api'

const appNamePattern = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/
const githubFineGrainedTokenURL = 'https://github.com/settings/personal-access-tokens/new'
const githubRegistryTokenURL = 'https://github.com/settings/tokens/new?scopes=read:packages'
const applicationSourceGuideURL = '/application-source-guide.html'

type WizardEnvironment = {
  id: string
  name: string
  writeMode?: WriteMode
  default?: boolean
}

export type CreateFormState = {
  sourceMode: 'quick' | 'github'
  name: string
  description: string
  image: string
  servicePort: number
  deploymentStrategy: 'Rollout' | 'Canary'
  environment: string
  repositoryUrl: string
  repositoryBranch: string
  repositoryToken: string
  repositoryServiceId: string
  configPath: string
  registryServer: string
  registryUsername: string
  registryToken: string
  secrets: { key: string; value: string }[]
}

type ApplicationWizardProps = {
  projectId?: string
  projectName?: string
  environments: WizardEnvironment[]
  allowedStrategies: readonly string[]
  initialState: CreateFormState
  onPreviewSource: (state: CreateFormState) => Promise<PreviewApplicationSourceResponse>
  onSubmit: (state: CreateFormState) => Promise<void>
  onCancel: () => void
  submitting: boolean
}

export function ApplicationWizard({
  projectId,
  projectName,
  environments,
  allowedStrategies,
  initialState,
  onPreviewSource,
  onSubmit,
  onCancel,
  submitting,
}: ApplicationWizardProps) {
  const [active, setActive] = useState(0)
  const [form, setForm] = useState<CreateFormState>(initialState)
  const [preview, setPreview] = useState<PreviewApplicationSourceResponse | null>(null)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [previewError, setPreviewError] = useState('')
  const [previewKey, setPreviewKey] = useState('')
  const [envBulkText, setEnvBulkText] = useState('')
  const [envBulkMessage, setEnvBulkMessage] = useState('')
  const [validationMessage, setValidationMessage] = useState('')
  const fileInputRef = useRef<HTMLInputElement | null>(null)

  const selectedEnvironment = useMemo(
    () => environments.find((environment) => environment.id === form.environment) ?? null,
    [environments, form.environment],
  )
  const directEnvironments = useMemo(
    () => environments.filter((environment) => environment.writeMode !== 'pull_request'),
    [environments],
  )
  const environmentOptions = useMemo(
    () => environments.map((environment) => ({
      value: environment.id,
      label:
        environment.writeMode === 'pull_request'
          ? `${environment.name} · 변경 요청 필요`
          : environment.name,
      disabled: environment.writeMode === 'pull_request',
    })),
    [environments],
  )

  const filledSecretCount = form.secrets.filter((secret) => secret.key.trim() && secret.value.trim()).length
  const effectiveAppName = form.name || form.repositoryServiceId
  const manifestPathPreview = buildManifestPath(projectId, effectiveAppName)
  const applicationIdPreview = buildApplicationID(projectId, effectiveAppName)
  const vaultPathPreview = buildVaultPath(projectId, effectiveAppName)
  const repositoryTokenPathPreview = buildRepositoryTokenPath(projectId, effectiveAppName)
  const registrySecretPathPreview = buildRegistrySecretPath(projectId, effectiveAppName)
  const repositoryTarget = useMemo(
    () => parseGitHubRepositoryInput(form.repositoryUrl),
    [form.repositoryUrl],
  )
  const repositoryConfigExample = useMemo(
    () =>
      JSON.stringify(
        {
          services: [
            {
              serviceId: form.repositoryServiceId.trim() || effectiveAppName.trim() || 'example-app',
              image: `ghcr.io/aods/${(form.repositoryServiceId.trim() || effectiveAppName.trim() || 'example-app').replace(/\s+/g, '-').toLowerCase()}:latest`,
              port: form.sourceMode === 'quick' ? form.servicePort || 3000 : 3000,
              replicas: 1,
              strategy: 'Rollout',
            },
          ],
        },
        null,
        2,
      ),
    [effectiveAppName, form.repositoryServiceId, form.servicePort, form.sourceMode],
  )
  const currentRegistryMode = useMemo<'auto' | 'ghcr.io' | 'docker.io' | 'custom'>(() => {
    const normalized = form.registryServer.trim()
    if (normalized === '') return 'auto'
    if (normalized === 'ghcr.io') return 'ghcr.io'
    if (normalized === 'docker.io') return 'docker.io'
    return 'custom'
  }, [form.registryServer])

  const updateForm = useCallback((updates: Partial<CreateFormState>) => {
    setForm((current) => ({ ...current, ...updates }))
    setValidationMessage('')
  }, [])

  const nextStrategy = (value: string | null): CreateFormState['deploymentStrategy'] =>
    value === 'Canary' ? 'Canary' : 'Rollout'

  const handleSourceModeChange = (sourceMode: CreateFormState['sourceMode']) => {
    setForm((current) => ({ ...current, sourceMode }))
    setPreview(null)
    setPreviewError('')
    setPreviewKey('')
    setValidationMessage('')
  }

  const applyParsedSecrets = (text: string) => {
    const parsed = parseEnvEntries(text)
    if (parsed.length === 0) {
      setEnvBulkMessage('.env 형식에서 읽을 수 있는 항목이 없습니다.')
      return
    }
    updateForm({ secrets: parsed })
    setEnvBulkMessage(`${parsed.length}개 항목을 가져왔습니다.`)
  }

  const handleImportEnvFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return
    const text = await file.text()
    setEnvBulkText(text)
    applyParsedSecrets(text)
    event.target.value = ''
  }

  const nextStep = () => {
    const message = validateStep(active, form, environments, preview, previewError)
    if (message) {
      setValidationMessage(message)
      return
    }
    setValidationMessage('')
    setActive((current) => (current < 4 ? current + 1 : current))
  }

  const prevStep = () => {
    setValidationMessage('')
    setActive((current) => (current > 0 ? current - 1 : current))
  }

  const handleFinish = () => {
    const message = validateAllSteps(form, environments, preview, previewError)
    if (message) {
      setValidationMessage(message)
      return
    }
    setValidationMessage('')
    void onSubmit(form)
  }

  const handleDownloadRepositoryConfigExample = () => {
    downloadTextFile(form.configPath.trim() || 'aolda_deploy.json', repositoryConfigExample)
  }

  const handleRegistryModeChange = (value: string | null) => {
    switch (value) {
      case 'ghcr.io':
        updateForm({ registryServer: 'ghcr.io' })
        return
      case 'docker.io':
        updateForm({ registryServer: 'docker.io' })
        return
      case 'custom':
        updateForm({ registryServer: currentRegistryMode === 'custom' ? form.registryServer : '' })
        return
      default:
        updateForm({ registryServer: '' })
    }
  }

  const inferRegistryServer = (image: string) => {
    const trimmed = image.trim()
    if (trimmed.startsWith('ghcr.io/')) return 'ghcr.io'
    if (trimmed.startsWith('docker.io/')) return 'docker.io'
    return ''
  }

  const applyPreviewSelection = useCallback((serviceId: string) => {
    const selected = preview?.services.find((service) => service.serviceId === serviceId)
    if (!selected) return
    updateForm({
      repositoryServiceId: selected.serviceId,
      name: selected.serviceId,
      registryServer: form.registryServer.trim() || inferRegistryServer(selected.image),
    })
  }, [preview, form.registryServer, updateForm])

  const loadPreview = useCallback(async (force = false) => {
    if (form.sourceMode !== 'github' || !form.repositoryUrl.trim()) return

    const nextPreviewKey = JSON.stringify({
      name: form.name.trim(),
      repositoryUrl: form.repositoryUrl.trim(),
      repositoryBranch: form.repositoryBranch.trim(),
      repositoryToken: form.repositoryToken.trim(),
      repositoryServiceId: form.repositoryServiceId.trim(),
      configPath: form.configPath.trim(),
    })

    if (!force && nextPreviewKey === previewKey && preview) {
      return
    }

    setPreviewLoading(true)
    setPreviewError('')
    try {
      const response = await onPreviewSource(form)
      setPreview(response)
      setPreviewKey(nextPreviewKey)
      if (response.selectedServiceId) {
        applyPreviewSelection(response.selectedServiceId)
      }
    } catch (error) {
      setPreview(null)
      if (error instanceof Error) {
        setPreviewError(error.message)
      } else {
        setPreviewError('설정 파일을 확인하지 못했습니다.')
      }
    } finally {
      setPreviewLoading(false)
    }
  }, [form, onPreviewSource, previewKey, preview, applyPreviewSelection])

  useEffect(() => {
    if (active !== 1 || form.sourceMode !== 'github' || !form.repositoryUrl.trim()) {
      return
    }
    void loadPreview(false)
  }, [active, form.sourceMode, form.repositoryUrl, form.repositoryBranch, form.repositoryToken, form.repositoryServiceId, form.configPath, loadPreview])

  return (
    <Card shadow="sm" p="lg" radius="md" withBorder className="glass-panel">
      <Stepper
        active={active}
        onStepClick={(nextStepIndex) => {
          if (nextStepIndex <= active) {
            setValidationMessage('')
            setActive(nextStepIndex)
          }
        }}
        allowNextStepsSelect={false}
        color="lagoon.6"
        mb="xl"
      >
        <Stepper.Step label="등록 방식" description="GitHub 연결과 기본 정보">
          <Stack gap="md" mt="md">
            <Text size="sm" c="dimmed">
              지금 바로 이미지와 포트를 입력해서 만들 수도 있고, GitHub 저장소의 `aolda_deploy.json`을 읽어서 자동으로
              등록할 수도 있습니다.
            </Text>

            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
              <Card
                withBorder
                radius="md"
                padding="lg"
                bg={form.sourceMode === 'quick' ? 'lagoon.0' : undefined}
                style={{
                  cursor: 'pointer',
                  borderColor:
                    form.sourceMode === 'quick' ? 'var(--mantine-color-lagoon-6)' : undefined,
                }}
                onClick={() => handleSourceModeChange('quick')}
              >
                <Stack gap="xs">
                  <Group justify="space-between" align="flex-start">
                    <Text fw={800}>빠른 생성</Text>
                    <Badge variant="light" color="gray">직접 입력</Badge>
                  </Group>
                  <Text size="sm" c="dimmed">
                    컨테이너 이미지, 포트, 배포 환경만 정리해서 바로 GitOps 등록을 진행합니다.
                  </Text>
                </Stack>
              </Card>

              <Card
                withBorder
                radius="md"
                padding="lg"
                bg={form.sourceMode === 'github' ? 'lagoon.0' : undefined}
                style={{
                  cursor: 'pointer',
                  borderColor:
                    form.sourceMode === 'github' ? 'var(--mantine-color-lagoon-6)' : undefined,
                }}
                onClick={() => handleSourceModeChange('github')}
              >
                <Stack gap="xs">
                  <Group justify="space-between" align="flex-start">
                    <Text fw={800}>GitHub에서 읽기</Text>
                    <Badge variant="light" color="lagoon">권장</Badge>
                  </Group>
                  <Text size="sm" c="dimmed">
                    GitHub 저장소 URL만 넣으면 `aolda_deploy.json`을 읽어서 이미지, 포트, 레플리카, 전략을 자동 채웁니다.
                    private 저장소일 때만 토큰을 추가하면 됩니다.
                  </Text>
                </Stack>
              </Card>
            </SimpleGrid>

            {form.sourceMode === 'github' ? (
              <Stack gap="md">
                <div
                  aria-hidden="true"
                  style={{
                    position: 'absolute',
                    opacity: 0,
                    pointerEvents: 'none',
                    width: 0,
                    height: 0,
                    overflow: 'hidden',
                  }}
                >
                  <input type="text" name="username" autoComplete="username" tabIndex={-1} />
                  <input type="password" name="password" autoComplete="current-password" tabIndex={-1} />
                </div>

                <Alert color="lagoon" variant="light">
                  저장소 기본 브랜치의 <Code>{form.configPath || 'aolda_deploy.json'}</Code> 파일을 읽습니다.
                  서비스 ID와 애플리케이션 이름은 다음 단계에서 읽은 결과를 기준으로 자동 정합니다.
                </Alert>

                <Card withBorder radius="md" padding="lg" bg="gray.0">
                  <Stack gap="md">
                    <Group justify="space-between" align="flex-start">
                      <Stack gap={2}>
                        <Text fw={800}>AODS 연결 설정</Text>
                        <Text size="sm" c="dimmed">
                          저장소 토큰, 이미지 Pull 토큰, <Code>aolda_deploy.json</Code>, 모노레포 구성, 이미지 태그 운영 규칙은
                          설정 가이드 페이지로 모아 두었습니다.
                        </Text>
                      </Stack>
                      <Group gap="xs" wrap="wrap">
                        <Button
                          component="a"
                          href={applicationSourceGuideURL}
                          target="_blank"
                          rel="noreferrer"
                          variant="filled"
                          color="lagoon.6"
                          radius="xl"
                          size="xs"
                        >
                          설정 페이지 열기
                        </Button>
                        <Button variant="default" radius="xl" size="xs" onClick={handleDownloadRepositoryConfigExample}>
                          예시 JSON 다운로드
                        </Button>
                        <Button
                          component="a"
                          href={githubFineGrainedTokenURL}
                          target="_blank"
                          rel="noreferrer"
                          variant="default"
                          radius="xl"
                          size="xs"
                        >
                          GitHub 토큰 발급
                        </Button>
                        <Button
                          component="a"
                          href={githubRegistryTokenURL}
                          target="_blank"
                          rel="noreferrer"
                          variant="default"
                          radius="xl"
                          size="xs"
                        >
                          GHCR 토큰 발급
                        </Button>
                      </Group>
                    </Group>

                    <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
                      <Alert color="blue" variant="light">
                        <Stack gap={4}>
                          <Text fw={700}>입력 전에 먼저 볼 것</Text>
                          <Text size="sm">
                            public 저장소는 URL만 넣어도 되고, private 저장소면 저장소 읽기 토큰이 필요합니다.
                          </Text>
                          <Text size="sm">
                            private 이미지면 레지스트리 사용자명과 레지스트리 토큰도 함께 입력해야 합니다.
                          </Text>
                          <Text size="sm">
                            <Code>{form.configPath || 'aolda_deploy.json'}</Code> 파일이 실제 브랜치에 있어야 합니다.
                          </Text>
                        </Stack>
                      </Alert>

                      <Alert color="lagoon" variant="light">
                        <Stack gap={4}>
                          <Text fw={700}>같은 레포에 프론트와 백이 같이 있으면</Text>
                          <Text size="sm">
                            <Code>services</Code> 안에 <Code>web</Code>, <Code>api</Code> 처럼 서비스별 항목을 나누고,
                            AODS에서는 앱을 각각 따로 등록합니다.
                          </Text>
                          <Text size="sm">
                            이미지 태그는 <Code>latest</Code> 덮어쓰기보다 새 tag를 계속 발급하는 방식을 권장합니다.
                          </Text>
                          <Text size="sm">
                            자세한 예시는 설정 페이지에서 바로 확인할 수 있습니다.
                          </Text>
                        </Stack>
                      </Alert>
                    </SimpleGrid>

                    <List spacing="xs" size="sm">
                      <List.Item>
                        저장소 owner: <Code>{repositoryTarget?.owner || '저장소 owner'}</Code>
                      </List.Item>
                      <List.Item>
                        저장소 이름: <Code>{repositoryTarget?.repo || '저장소 이름'}</Code>
                      </List.Item>
                      <List.Item>
                        레지스트리 주소: <Code>{form.registryServer.trim() || 'ghcr.io 또는 이미지 주소에서 자동 추론'}</Code>
                      </List.Item>
                    </List>
                  </Stack>
                </Card>

                <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
                  <TextInput
                    label="GitHub 저장소 URL"
                    placeholder="예: https://github.com/aods/example-app.git"
                    required
                    name="aods-github-repository-url"
                    autoComplete="off"
                    autoCapitalize="none"
                    spellCheck={false}
                    data-form-type="other"
                    data-lpignore="true"
                    data-1p-ignore="true"
                    value={form.repositoryUrl}
                    onChange={(event) => updateForm({ repositoryUrl: event.target.value })}
                  />
                  <PasswordInput
                    label="GitHub 저장소 토큰 (선택)"
                    description="public 저장소면 비워 두고, private 저장소일 때만 입력합니다."
                    name="aods-github-token"
                    autoComplete="new-password"
                    data-form-type="other"
                    data-lpignore="true"
                    data-1p-ignore="true"
                    value={form.repositoryToken}
                    onChange={(event) => updateForm({ repositoryToken: event.target.value })}
                  />
                </SimpleGrid>

                <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
                  <TextInput
                    label="브랜치"
                    placeholder="기본값: main"
                    name="aods-github-branch"
                    autoComplete="off"
                    autoCapitalize="none"
                    spellCheck={false}
                    data-form-type="other"
                    data-lpignore="true"
                    data-1p-ignore="true"
                    value={form.repositoryBranch}
                    onChange={(event) => updateForm({ repositoryBranch: event.target.value })}
                  />
                  <TextInput
                    label="설정 파일 경로"
                    name="aods-github-config-path"
                    autoComplete="off"
                    autoCapitalize="none"
                    spellCheck={false}
                    data-form-type="other"
                    data-lpignore="true"
                    data-1p-ignore="true"
                    value={form.configPath}
                    onChange={(event) => updateForm({ configPath: event.target.value })}
                  />
                </SimpleGrid>

                <Alert color="gray" variant="light">
                  <Stack gap={4}>
                    <Text size="sm">
                      지금 단계에서는 저장소 URL만 맞게 넣으면 됩니다. AODS가 먼저
                      <Code>{form.configPath || 'aolda_deploy.json'}</Code>을 읽고 서비스 목록을 확인합니다.
                    </Text>
                    <Text size="sm">
                      단일 서비스면 그 <Code>serviceId</Code>가 자동 저장되고, 서비스가 여러 개면 다음 단계에서 하나를 선택하면 됩니다.
                    </Text>
                    <Text size="sm">
                      값 입력 방법이 헷갈리면
                      {' '}
                      <Text component="a" href={applicationSourceGuideURL} target="_blank" rel="noreferrer" span c="lagoon.7" fw={700}>
                        설정 페이지
                      </Text>
                      에서 예시를 보고 그대로 넣으면 됩니다.
                    </Text>
                  </Stack>
                </Alert>
              </Stack>
            ) : null}

            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
              {form.sourceMode === 'quick' ? (
                <TextInput
                  label="애플리케이션 이름"
                  placeholder="예: payment-api"
                  required
                  value={form.name}
                  onChange={(event) => updateForm({ name: event.target.value })}
                />
              ) : (
                <Alert color="blue" variant="light">
                  애플리케이션 이름은 사용자가 직접 넣지 않습니다. 단일 서비스면 자동 확정하고, 여러 서비스면 다음 단계에서 선택한
                  <Code>serviceId</Code>를 그대로 사용합니다.
                </Alert>
              )}
              {form.sourceMode === 'quick' ? (
                <TextInput
                  label="컨테이너 이미지"
                  placeholder="예: ghcr.io/aolda/payment-api:1.0.0"
                  required
                  value={form.image}
                  onChange={(event) => updateForm({ image: event.target.value })}
                />
              ) : (
                <Alert color="blue" variant="light">
                  이미지, 포트, 레플리카, 배포 전략은 저장소의 <Code>{form.configPath || 'aolda_deploy.json'}</Code>에서 읽습니다.
                </Alert>
              )}
            </SimpleGrid>

            <Textarea
              label="설명"
              placeholder="애플리케이션의 역할과 책임"
              value={form.description}
              onChange={(event) => updateForm({ description: event.target.value })}
            />

            <Card withBorder radius="md" padding="md" bg="gray.0">
              <Stack gap="md">
                <Stack gap={2}>
                  <Text fw={700}>컨테이너 레지스트리 인증</Text>
                  <Text size="sm" c="dimmed">
                    public 이미지는 비워 두면 됩니다. private 이미지면 레지스트리 사용자명과 토큰을 입력하면 앱별
                    docker registry Secret을 같이 생성합니다.
                  </Text>
                </Stack>

                <SimpleGrid cols={{ base: 1, sm: 3 }} spacing="md">
                  <Select
                    label="레지스트리 종류"
                    data={[
                      { value: 'auto', label: '자동 추론' },
                      { value: 'ghcr.io', label: 'GHCR (ghcr.io)' },
                      { value: 'docker.io', label: 'Docker Hub (docker.io)' },
                      { value: 'custom', label: '직접 입력' },
                    ]}
                    value={currentRegistryMode}
                    onChange={handleRegistryModeChange}
                  />
                  <TextInput
                    label="레지스트리 사용자명"
                    name="aods-registry-username"
                    autoComplete="off"
                    autoCapitalize="none"
                    spellCheck={false}
                    data-form-type="other"
                    data-lpignore="true"
                    data-1p-ignore="true"
                    value={form.registryUsername}
                    onChange={(event) => updateForm({ registryUsername: event.target.value })}
                  />
                  <PasswordInput
                    label="레지스트리 토큰"
                    name="aods-registry-token"
                    autoComplete="new-password"
                    data-form-type="other"
                    data-lpignore="true"
                    data-1p-ignore="true"
                    value={form.registryToken}
                    onChange={(event) => updateForm({ registryToken: event.target.value })}
                  />
                </SimpleGrid>

                {currentRegistryMode === 'custom' ? (
                  <TextInput
                    label="직접 입력 레지스트리 주소"
                    placeholder="예: registry.example.com"
                    name="aods-registry-server-custom"
                    autoComplete="off"
                    autoCapitalize="none"
                    spellCheck={false}
                    data-form-type="other"
                    data-lpignore="true"
                    data-1p-ignore="true"
                    value={form.registryServer}
                    onChange={(event) => updateForm({ registryServer: event.target.value })}
                  />
                ) : null}

                <Text size="xs" c="dimmed">
                  레지스트리 토큰은 앱 환경변수 및 GitHub 저장소 토큰과 분리해서 <Code>{registrySecretPathPreview}</Code> 경로에 저장합니다.
                </Text>
                <Text size="xs" c="dimmed">
                  GHCR private 이미지를 쓰면
                  {' '}
                  <Text component="a" href={githubRegistryTokenURL} target="_blank" rel="noreferrer" span c="lagoon.7" fw={700}>
                    GHCR 토큰 발급 페이지
                  </Text>
                  또는
                  {' '}
                  <Text component="a" href={applicationSourceGuideURL} target="_blank" rel="noreferrer" span c="lagoon.7" fw={700}>
                    설정 페이지
                  </Text>
                  에서 발급 방법을 확인할 수 있습니다.
                </Text>
              </Stack>
            </Card>

            <Text size="xs" c="dimmed">
              앱 이름은 GitOps 경로와 애플리케이션 ID를 결정합니다. 비워두면 저장소 서비스 ID를 기준으로 생성됩니다. 예: <Code>{applicationIdPreview}</Code>
            </Text>
          </Stack>
        </Stepper.Step>

        <Stepper.Step label="설정 파일 확인" description="저장소 서비스 확인">
          <Stack gap="md" mt="md">
            {form.sourceMode === 'github' ? (
              <>
                <Alert color="blue" variant="light">
                  <Stack gap={4}>
                    <Text fw={700}>JSON을 먼저 확인합니다</Text>
                    <Text size="sm">
                      AODS가 <Code>{form.configPath || 'aolda_deploy.json'}</Code>을 읽어서 서비스가 몇 개인지 확인합니다.
                    </Text>
                    <Text size="sm">
                      서비스가 하나면 그 <Code>serviceId</Code>를 자동으로 저장하고, 여러 개면 여기서 선택한 뒤 다음 단계로 넘어갑니다.
                    </Text>
                    <Text size="sm">
                      각 서비스는 앱을 각각 등록하면서 비밀값도 서비스별로 따로 넣는 구조로 생각하면 됩니다.
                    </Text>
                  </Stack>
                </Alert>

                <Group justify="space-between">
                  <Text fw={700}>저장소 서비스 미리보기</Text>
                  <Button variant="light" size="xs" onClick={() => void loadPreview(true)} loading={previewLoading}>
                    다시 확인
                  </Button>
                </Group>

                {previewLoading ? (
                  <Alert color="lagoon" variant="light">
                    설정 파일을 읽는 중입니다.
                  </Alert>
                ) : null}

                {previewError ? (
                  <Alert color="red" variant="light">
                    {previewError}
                  </Alert>
                ) : null}

                {preview && !previewLoading ? (
                  <Stack gap="md">
                    <Card withBorder radius="md" bg="gray.0">
                      <Stack gap="xs">
                        <Text fw={700}>확인 결과</Text>
                        <Text size="sm">설정 파일 경로: <Code>{preview.configPath}</Code></Text>
                        <Text size="sm">감지된 서비스 수: <Code>{String(preview.services.length)}</Code></Text>
                        <Text size="sm">
                          선택된 서비스 ID: <Code>{form.repositoryServiceId.trim() || preview.selectedServiceId || '아직 선택 전'}</Code>
                        </Text>
                      </Stack>
                    </Card>

                    <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
                      {preview.services.map((service) => {
                        const activeService = form.repositoryServiceId.trim() === service.serviceId
                        return (
                          <Card
                            key={service.serviceId}
                            withBorder
                            radius="md"
                            padding="md"
                            bg={activeService ? 'lagoon.0' : undefined}
                            style={{
                              borderColor: activeService ? 'var(--mantine-color-lagoon-6)' : undefined,
                            }}
                          >
                            <Stack gap="xs">
                              <Group justify="space-between" align="flex-start">
                                <Stack gap={2}>
                                  <Text fw={800}>{service.serviceId}</Text>
                                  <Text size="sm" c="dimmed">{service.image}</Text>
                                </Stack>
                                <Badge variant="light" color={activeService ? 'lagoon' : 'gray'}>
                                  {activeService ? '선택됨' : '감지됨'}
                                </Badge>
                              </Group>
                              <Text size="sm">포트: {service.port}</Text>
                              <Text size="sm">레플리카: {service.replicas}</Text>
                              <Text size="sm">전략: {service.strategy || 'Rollout'}</Text>
                              <Group justify="space-between" align="center">
                                <Text size="xs" c="dimmed">
                                  이 서비스를 선택하면 serviceId와 애플리케이션 이름을 같이 확정합니다.
                                </Text>
                                <Button
                                  size="xs"
                                  variant={activeService ? 'light' : 'filled'}
                                  onClick={() => applyPreviewSelection(service.serviceId)}
                                >
                                  {activeService ? '선택 완료' : '이 서비스 선택'}
                                </Button>
                              </Group>
                            </Stack>
                          </Card>
                        )
                      })}
                    </SimpleGrid>
                  </Stack>
                ) : null}
              </>
            ) : (
              <Alert color="gray" variant="light">
                빠른 생성 모드에서는 저장소 설정 파일을 읽지 않으므로 이 단계를 건너뛰고 다음 단계에서 배포 설정을 직접 입력합니다.
              </Alert>
            )}
          </Stack>
        </Stepper.Step>

        <Stepper.Step label="배포 설정" description="직접 생성 가능한 환경 선택">
          <Stack gap="md" mt="md">
            <Alert color="blue" variant="light">
              <Stack gap={4}>
                <Text fw={700}>현재 프로젝트 정책</Text>
                <Text size="sm">
                  허용 전략: {allowedStrategies.join(', ') || '없음'}
                </Text>
                <Text size="sm">
                  직접 생성 가능 환경: {directEnvironments.map((environment) => environment.name).join(', ') || '없음'}
                </Text>
              </Stack>
            </Alert>

            {directEnvironments.length === 0 ? (
              <Alert color="yellow" variant="light">
                현재 프로젝트에는 직접 생성 가능한 환경이 없습니다. 이 경우 애플리케이션 직접 생성이 아니라 변경 요청 흐름으로 진행해야 합니다.
              </Alert>
            ) : null}

            {form.sourceMode === 'quick' ? (
              <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
                <NumberInput
                  label="서비스 포트"
                  required
                  min={1}
                  max={65535}
                  value={form.servicePort}
                  onChange={(value) => updateForm({ servicePort: Number(value) || 8080 })}
                />
                <Select
                  label="배포 전략"
                  data={allowedStrategies}
                  value={form.deploymentStrategy}
                  onChange={(value) => updateForm({ deploymentStrategy: nextStrategy(value) })}
                />
              </SimpleGrid>
            ) : (
              <Alert color="lagoon" variant="light">
                GitHub 등록 모드에서는 서비스 포트, 배포 전략, 레플리카 수를 <Code>{form.configPath || 'aolda_deploy.json'}</Code> 기준으로 자동 적용합니다.
              </Alert>
            )}

            <Select
              label="배포 환경"
              data={environmentOptions}
              value={form.environment}
              onChange={(value) => updateForm({ environment: value || '' })}
            />

            <Text size="xs" c="dimmed">
              `pull_request` 환경은 직접 생성 대상이 아니라 승인된 변경 요청 흐름에서 처리합니다.
              {selectedEnvironment ? ` 현재 선택: ${selectedEnvironment.name}` : ''}
            </Text>
          </Stack>
        </Stepper.Step>

        <Stepper.Step label="비밀값" description="Vault 저장 정보">
          <Stack gap="md" mt="md">
            <Text size="sm" c="dimmed">
              입력한 값은 생성 시 Vault에 저장됩니다. 지금 없어도 생성은 가능하며, 필요한 경우 나중에 다시 연결할 수 있습니다.
            </Text>
            <div
              aria-hidden="true"
              style={{
                position: 'absolute',
                opacity: 0,
                pointerEvents: 'none',
                width: 0,
                height: 0,
                overflow: 'hidden',
              }}
            >
              <input type="text" name="username" autoComplete="username" tabIndex={-1} />
              <input type="password" name="password" autoComplete="current-password" tabIndex={-1} />
            </div>
            <Textarea
              label=".env 일괄 입력"
              placeholder={'예:\nDB_HOST=db.internal\nDB_PASSWORD=secret-value\nAPI_KEY="abc123"'}
              minRows={6}
              value={envBulkText}
              onChange={(event) => setEnvBulkText(event.target.value)}
            />
            <Group>
              <Button variant="light" onClick={() => applyParsedSecrets(envBulkText)}>
                .env 내용 적용
              </Button>
              <Button variant="default" onClick={() => fileInputRef.current?.click()}>
                환경 변수 파일 가져오기
              </Button>
              <input
                ref={fileInputRef}
                type="file"
                accept=".env,.txt,text/plain"
                style={{ display: 'none' }}
                onChange={handleImportEnvFile}
              />
            </Group>
            <Text size="xs" c="dimmed">
              `.env`, `.env.example`, `application-prod.env`, `.txt` 파일을 읽습니다. `KEY=value`, `export KEY=value`, 주석(`#`) 형식을 자동으로 파싱합니다.
            </Text>
            {envBulkMessage ? (
              <Text size="sm" c="dimmed">{envBulkMessage}</Text>
            ) : null}

            {form.secrets.length === 0 ? (
              <Alert color="gray" variant="light">
                아직 입력된 비밀값이 없습니다. 필요하면 아래에서 직접 추가하거나 `.env` 파일을 불러오세요.
              </Alert>
            ) : null}

            {form.secrets.map((secret, index) => (
              <Group key={`${secret.key}-${index}`} grow align="flex-end">
                <TextInput
                  label={index === 0 ? '키' : undefined}
                  placeholder="예: DB_PASSWORD"
                  name={`aods-secret-key-${index}`}
                  autoComplete="off"
                  autoCapitalize="none"
                  spellCheck={false}
                  data-form-type="other"
                  data-lpignore="true"
                  data-1p-ignore="true"
                  value={secret.key}
                  onChange={(event) => {
                    const nextSecrets = [...form.secrets]
                    nextSecrets[index].key = event.target.value
                    updateForm({ secrets: nextSecrets })
                  }}
                />
                <TextInput
                  label={index === 0 ? '값' : undefined}
                  placeholder="비밀값"
                  type="password"
                  name={`aods-secret-value-${index}`}
                  autoComplete="new-password"
                  autoCapitalize="none"
                  spellCheck={false}
                  data-form-type="other"
                  data-lpignore="true"
                  data-1p-ignore="true"
                  value={secret.value}
                  onChange={(event) => {
                    const nextSecrets = [...form.secrets]
                    nextSecrets[index].value = event.target.value
                    updateForm({ secrets: nextSecrets })
                  }}
                />
                <Button
                  color="red"
                  variant="light"
                  onClick={() => updateForm({ secrets: form.secrets.filter((_, itemIndex) => itemIndex !== index) })}
                  disabled={form.secrets.length === 1}
                >
                  삭제
                </Button>
              </Group>
            ))}

            <Button
              variant="outline"
              onClick={() => updateForm({ secrets: [...form.secrets, { key: '', value: '' }] })}
            >
              + 비밀값 추가
            </Button>
          </Stack>
        </Stepper.Step>

        <Stepper.Completed>
          <Stack gap="md" mt="md">
            <Text fw={700} size="lg">생성 전 최종 확인</Text>
            <Text c="dimmed">
              {projectName ? `${projectName} 프로젝트에` : '선택한 프로젝트에'} 아래 구성이 반영됩니다.
            </Text>

            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
              <Card withBorder radius="md" bg="gray.0">
                <Stack gap="xs">
                  <Text fw={700}>등록 요약</Text>
                  <Text size="sm">방식: {form.sourceMode === 'github' ? 'GitHub에서 읽기' : '빠른 생성'}</Text>
                  <Text size="sm">이름: {form.name || (form.sourceMode === 'github' ? '설정 파일 확인 후 자동 확정' : '(미입력)')}</Text>
                  <Text size="sm">이미지: {form.sourceMode === 'github' ? 'aolda_deploy.json에서 자동 채움' : form.image || '(미입력)'}</Text>
                  <Text size="sm">포트: {form.sourceMode === 'github' ? 'aolda_deploy.json에서 자동 채움' : form.servicePort || '(미입력)'}</Text>
                  <Text size="sm">전략: {form.sourceMode === 'github' ? 'aolda_deploy.json에서 자동 채움' : form.deploymentStrategy}</Text>
                  <Text size="sm">환경: {selectedEnvironment?.name || form.environment || '(미선택)'}</Text>
                </Stack>
              </Card>

              <Card withBorder radius="md" bg="gray.0">
                <Stack gap="xs">
                  <Text fw={700}>GitOps 반영 미리보기</Text>
                  <Text size="sm">애플리케이션 ID</Text>
                  <Code>{applicationIdPreview}</Code>
                  <Text size="sm">매니페스트 경로</Text>
                  <Code>{manifestPathPreview}</Code>
                  {filledSecretCount > 0 ? (
                    <>
                      <Text size="sm">Vault 경로</Text>
                      <Code>{vaultPathPreview}</Code>
                    </>
                  ) : null}
                  {form.sourceMode === 'github' && form.repositoryToken.trim() ? (
                    <>
                      <Text size="sm">GitHub 저장소 토큰 경로</Text>
                      <Code>{repositoryTokenPathPreview}</Code>
                    </>
                  ) : null}
                  {form.registryToken.trim() ? (
                    <>
                      <Text size="sm">레지스트리 토큰 경로</Text>
                      <Code>{registrySecretPathPreview}</Code>
                    </>
                  ) : null}
                </Stack>
              </Card>
            </SimpleGrid>

            {form.sourceMode === 'github' ? (
              <Card withBorder radius="md" bg="gray.0">
                <Stack gap="xs">
                  <Group gap="xs">
                    <Badge variant="light">GitHub 연결</Badge>
                    <Text fw={700}>{form.repositoryServiceId || form.name || '설정 파일 확인 후 자동 확정'}</Text>
                  </Group>
                  <Text size="sm">저장소 URL: {form.repositoryUrl || '(미입력)'}</Text>
                  <Text size="sm">저장소 토큰: {form.repositoryToken.trim() ? '입력됨' : '없음 (public 저장소 기준)'}</Text>
                  <Text size="sm">브랜치: {form.repositoryBranch || 'main'}</Text>
                  <Text size="sm">서비스 ID: {form.repositoryServiceId || '설정 파일 확인 후 자동 확정'}</Text>
                  <Text size="sm">설정 파일: {form.configPath || 'aolda_deploy.json'}</Text>
                </Stack>
              </Card>
            ) : null}

            <Alert color="lagoon" variant="light">
              <List spacing="xs" size="sm">
                <List.Item>GitHub 기본 브랜치에 앱 디렉터리와 Flux child manifest를 생성합니다.</List.Item>
                <List.Item>애플리케이션 식별자는 <Code>{applicationIdPreview}</Code> 규칙으로 저장됩니다.</List.Item>
                <List.Item>
                  비밀값 {filledSecretCount}개{filledSecretCount > 0 ? `를 ${vaultPathPreview} 경로로 저장합니다.` : '은 이번 생성에서 저장하지 않습니다.'}
                </List.Item>
                {form.sourceMode === 'github' && form.repositoryToken.trim() ? (
                  <List.Item>GitHub 저장소 토큰은 앱 환경변수와 분리해서 <Code>{repositoryTokenPathPreview}</Code> 경로에 저장합니다.</List.Item>
                ) : null}
                {form.sourceMode === 'github' && !form.repositoryToken.trim() ? (
                  <List.Item>GitHub 저장소 토큰은 입력하지 않았으므로 별도 Vault 경로를 만들지 않습니다.</List.Item>
                ) : null}
                {form.registryToken.trim() ? (
                  <List.Item>레지스트리 토큰은 Kubernetes image pull credential 용도로 <Code>{registrySecretPathPreview}</Code> 경로에 저장합니다.</List.Item>
                ) : null}
              </List>
            </Alert>
          </Stack>
        </Stepper.Completed>
      </Stepper>

      {validationMessage ? (
        <Alert color="red" variant="light" mb="md">
          {validationMessage}
        </Alert>
      ) : null}

      <Group justify="flex-end" mt="xl">
        <Button variant="default" onClick={onCancel} disabled={submitting}>
          취소
        </Button>
        {active !== 0 ? (
          <Button variant="default" onClick={prevStep} disabled={submitting}>
            이전
          </Button>
        ) : null}
        {active < 4 ? (
          <Button onClick={nextStep} color="lagoon.6" disabled={submitting}>
            다음 단계
          </Button>
        ) : (
          <Button onClick={handleFinish} loading={submitting} color="lagoon.8">
            GitHub 반영 시작
          </Button>
        )}
      </Group>
    </Card>
  )
}

function validateStep(
  step: number,
  form: CreateFormState,
  environments: WizardEnvironment[],
  preview: PreviewApplicationSourceResponse | null,
  previewError: string,
) {
  switch (step) {
    case 0:
      return validateIdentityStep(form)
    case 1:
      return validatePreviewStep(form, preview, previewError)
    case 2:
      return validateDeploymentStep(form, environments)
    case 3:
      return validateSecretsStep(form)
    default:
      return ''
  }
}

function validateAllSteps(
  form: CreateFormState,
  environments: WizardEnvironment[],
  preview: PreviewApplicationSourceResponse | null,
  previewError: string,
) {
  return (
    validateIdentityStep(form) ||
    validatePreviewStep(form, preview, previewError) ||
    validateDeploymentStep(form, environments) ||
    validateSecretsStep(form)
  )
}

function validateIdentityStep(form: CreateFormState) {
  const normalizedName = form.name.trim()
  if (form.sourceMode === 'github') {
    if (!form.repositoryUrl.trim()) {
      return 'GitHub 저장소 URL을 입력하세요.'
    }
  } else if (!normalizedName) {
    return '애플리케이션 이름을 입력하세요.'
  }
  if (normalizedName && !appNamePattern.test(normalizedName)) {
    return '애플리케이션 이름은 영문 소문자, 숫자, 하이픈만 사용할 수 있습니다.'
  }
  if (form.repositoryServiceId.trim() && !appNamePattern.test(form.repositoryServiceId.trim())) {
    return '설정 파일에서 읽은 서비스 ID는 영문 소문자, 숫자, 하이픈만 사용할 수 있습니다.'
  }
  if (form.sourceMode === 'quick' && !form.image.trim()) {
    return '컨테이너 이미지 주소를 입력하세요.'
  }
  const hasRegistryServer = Boolean(form.registryServer.trim())
  const hasRegistryUsername = Boolean(form.registryUsername.trim())
  const hasRegistryToken = Boolean(form.registryToken.trim())
  if (hasRegistryUsername !== hasRegistryToken) {
    return '레지스트리 사용자명과 레지스트리 토큰은 함께 입력하세요.'
  }
  if (hasRegistryServer && !hasRegistryUsername) {
    return '레지스트리 주소를 직접 넣는 경우 사용자명과 토큰도 함께 입력하세요.'
  }
  return ''
}

function validateDeploymentStep(form: CreateFormState, environments: WizardEnvironment[]) {
  if (environments.every((environment) => environment.writeMode === 'pull_request')) {
    return '직접 생성 가능한 배포 환경이 없습니다. 변경 요청 흐름을 사용해야 합니다.'
  }
  if (form.sourceMode === 'quick' && (form.servicePort < 1 || form.servicePort > 65535)) {
    return '서비스 포트는 1 이상 65535 이하여야 합니다.'
  }
  if (!form.environment) {
    return '배포 환경을 선택하세요.'
  }
  const selectedEnvironment = environments.find((environment) => environment.id === form.environment)
  if (selectedEnvironment?.writeMode === 'pull_request') {
    return '선택한 환경은 직접 생성 대상이 아닙니다. 변경 요청 흐름을 사용하세요.'
  }
  return ''
}

function validatePreviewStep(
  form: CreateFormState,
  preview: PreviewApplicationSourceResponse | null,
  previewError: string,
) {
  if (form.sourceMode !== 'github') {
    return ''
  }
  if (previewError) {
    return previewError
  }
  if (!preview) {
    return '설정 파일 확인 결과가 아직 없습니다. 잠시 후 다시 시도하세요.'
  }
  if (preview.requiresServiceSelection && !form.repositoryServiceId.trim()) {
    return '서비스가 여러 개입니다. 다음 단계로 가기 전에 serviceId 하나를 선택하세요.'
  }
  return ''
}

function validateSecretsStep(form: CreateFormState) {
  const seenKeys = new Set<string>()

  for (const secret of form.secrets) {
    const key = secret.key.trim()
    const value = secret.value.trim()

    if (!key && !value) {
      continue
    }
    if (!key || !value) {
      return '비밀값 행은 키와 값을 모두 입력하거나 비워 두세요.'
    }
    if (seenKeys.has(key)) {
      return `비밀값 키 ${key} 이 중복되었습니다.`
    }
    seenKeys.add(key)
  }

  return ''
}

function buildManifestPath(projectId: string | undefined, appName: string) {
  const normalizedProjectId = projectId || '{projectId}'
  const normalizedAppName = appName.trim() || '{appName}'
  return `apps/${normalizedProjectId}/${normalizedAppName}`
}

function buildApplicationID(projectId: string | undefined, appName: string) {
  const normalizedProjectId = projectId || '{projectId}'
  const normalizedAppName = appName.trim() || '{appName}'
  return `${normalizedProjectId}__${normalizedAppName}`
}

function buildVaultPath(projectId: string | undefined, appName: string) {
  const normalizedProjectId = projectId || '{projectId}'
  const normalizedAppName = appName.trim() || '{appName}'
  return `secret/aods/apps/${normalizedProjectId}/${normalizedAppName}/prod`
}

function buildRepositoryTokenPath(projectId: string | undefined, appName: string) {
  const normalizedProjectId = projectId || '{projectId}'
  const normalizedAppName = appName.trim() || '{appName}'
  return `secret/aods/apps/${normalizedProjectId}/${normalizedAppName}/repository`
}

function buildRegistrySecretPath(projectId: string | undefined, appName: string) {
  const normalizedProjectId = projectId || '{projectId}'
  const normalizedAppName = appName.trim() || '{appName}'
  return `secret/aods/apps/${normalizedProjectId}/${normalizedAppName}/registry`
}

function parseGitHubRepositoryInput(value: string) {
  const trimmed = value.trim()
  if (!trimmed) {
    return null
  }

  const normalized = trimmed
    .replace(/^https?:\/\//, '')
    .replace(/^git@/, '')
    .replace(/:/, '/')
    .replace(/\.git$/, '')

  const parts = normalized.split('/').filter(Boolean)
  if (parts.length < 3 || parts[0] !== 'github.com') {
    return null
  }

  return {
    owner: parts[1],
    repo: parts[2],
  }
}

function downloadTextFile(filename: string, content: string) {
  const blob = new Blob([content], { type: 'application/json;charset=utf-8' })
  const objectURL = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = objectURL
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(objectURL)
}

function parseEnvEntries(text: string): { key: string; value: string }[] {
  const byKey = new Map<string, string>()

  for (const rawLine of text.split(/\r?\n/)) {
    const line = rawLine.trim()
    if (!line || line.startsWith('#')) {
      continue
    }

    const normalized = line.startsWith('export ') ? line.slice(7).trim() : line
    const separatorIndex = normalized.indexOf('=')
    if (separatorIndex <= 0) {
      continue
    }

    const key = normalized.slice(0, separatorIndex).trim()
    if (!key) {
      continue
    }

    let value = normalized.slice(separatorIndex + 1).trim()
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1)
    }

    value = value
      .replace(/\\n/g, '\n')
      .replace(/\\r/g, '\r')
      .replace(/\\t/g, '\t')
      .replace(/\\"/g, '"')
      .replace(/\\'/g, "'")

    byKey.set(key, value)
  }

  return Array.from(byKey.entries()).map(([key, value]) => ({ key, value }))
}
