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
import type {
  PreviewApplicationSourceResponse,
  VerifyImageAccessRequest,
  VerifyImageAccessResponse,
  WriteMode,
} from '../types/api'

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
  onVerifyImageAccess: (request: VerifyImageAccessRequest) => Promise<VerifyImageAccessResponse>
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
  onVerifyImageAccess,
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
  const [imageAccessLoading, setImageAccessLoading] = useState(false)
  const [imageAccessError, setImageAccessError] = useState('')
  const [imageAccessResult, setImageAccessResult] = useState<VerifyImageAccessResponse | null>(null)
  const [imageAccessKey, setImageAccessKey] = useState('')
  const [repositoryAccess, setRepositoryAccess] = useState<'public' | 'private'>(
    initialState.repositoryToken.trim() ? 'private' : 'public',
  )
  const [envBulkText, setEnvBulkText] = useState('')
  const [envBulkMessage, setEnvBulkMessage] = useState('')
  const [validationMessage, setValidationMessage] = useState('')
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const repositoryUrlInputRef = useRef<HTMLInputElement | null>(null)
  const appNameInputRef = useRef<HTMLInputElement | null>(null)
  const imageInputRef = useRef<HTMLInputElement | null>(null)

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
  const normalizedEffectiveAppName = effectiveAppName.trim()
  const effectiveAppNameIsValid = !normalizedEffectiveAppName || appNamePattern.test(normalizedEffectiveAppName)
  const previewAppName = effectiveAppNameIsValid ? normalizedEffectiveAppName : ''
  const manifestPathPreview = buildManifestPath(projectId, previewAppName)
  const applicationIdPreview = buildApplicationID(projectId, previewAppName)
  const vaultPathPreview = buildVaultPath(projectId, previewAppName)
  const repositoryTokenPathPreview = buildRepositoryTokenPath(projectId, previewAppName)
  const registrySecretPathPreview = buildRegistrySecretPath(projectId, previewAppName)
  const repositoryTarget = useMemo(
    () => parseGitHubRepositoryInput(form.repositoryUrl),
    [form.repositoryUrl],
  )
  const repositoryPreviewKey = useMemo(
    () =>
      JSON.stringify({
        repositoryUrl: form.repositoryUrl.trim(),
        repositoryBranch: form.repositoryBranch.trim(),
        repositoryToken: form.repositoryToken.trim(),
        configPath: form.configPath.trim(),
      }),
    [form.repositoryUrl, form.repositoryBranch, form.repositoryToken, form.configPath],
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
    setImageAccessError('')
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
    const message = validateStep(active, form, environments, preview, previewError, repositoryAccess, repositoryConnectionReady)
    if (message) {
      setValidationMessage(message)
      focusFirstInvalidField(active, form, {
        repositoryUrlInput: repositoryUrlInputRef.current,
        appNameInput: appNameInputRef.current,
        imageInput: imageInputRef.current,
      })
      return
    }
    setValidationMessage('')
    setActive((current) => {
      if (current === 0 && form.sourceMode === 'quick') return 3
      return current < 6 ? current + 1 : current
    })
  }

  const prevStep = () => {
    setValidationMessage('')
    setActive((current) => {
      if (current === 3 && form.sourceMode === 'quick') return 0
      return current > 0 ? current - 1 : current
    })
  }

  const handleFinish = () => {
    const message = validateAllSteps(form, environments, preview, previewError, repositoryAccess, repositoryConnectionReady)
    if (message) {
      setValidationMessage(message)
      focusFirstInvalidField(active, form, {
        repositoryUrlInput: repositoryUrlInputRef.current,
        appNameInput: appNameInputRef.current,
        imageInput: imageInputRef.current,
      })
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

  const previewMatchesCurrentRepository = Boolean(preview && previewKey === repositoryPreviewKey)
  const repositoryConnectionReady = form.sourceMode !== 'github' || (previewMatchesCurrentRepository && !previewError)

  const selectedRepositoryService = useMemo(() => {
    if (!previewMatchesCurrentRepository) return null
    if (!preview) return null
    const selectedServiceID = form.repositoryServiceId.trim() || preview.selectedServiceId || ''
    if (selectedServiceID) {
      return preview.services.find((service) => service.serviceId === selectedServiceID) ?? null
    }
    return preview.services.length === 1 ? preview.services[0] : null
  }, [preview, previewMatchesCurrentRepository, form.repositoryServiceId])

  const selectedImage = form.sourceMode === 'github'
    ? selectedRepositoryService?.image ?? ''
    : form.image.trim()
  const registryServerPreview = form.registryServer.trim() || inferRegistryServer(selectedImage) || '이미지 주소에서 자동 추론'
  const currentImageAccessKey = JSON.stringify({
    image: selectedImage,
    registryServer: form.registryServer.trim(),
    registryUsername: form.registryUsername.trim(),
    registryToken: form.registryToken.trim(),
  })
  const imageAccessResultIsCurrent = Boolean(imageAccessResult && imageAccessKey === currentImageAccessKey)

  const loadPreview = useCallback(async (force = false) => {
    if (form.sourceMode !== 'github' || !form.repositoryUrl.trim()) return

    if (!force && repositoryPreviewKey === previewKey) {
      return
    }

    setPreviewLoading(true)
    setPreviewError('')
    try {
      const response = await onPreviewSource(form)
      setPreview(response)
      setPreviewKey(repositoryPreviewKey)
      if (response.selectedServiceId) {
        applyPreviewSelection(response.selectedServiceId)
      }
    } catch (error) {
      setPreview(null)
      setPreviewKey(repositoryPreviewKey)
      if (error instanceof Error) {
        setPreviewError(translatePreviewError(error.message, form.configPath))
      } else {
        setPreviewError('설정 파일을 확인하지 못했습니다.')
      }
    } finally {
      setPreviewLoading(false)
    }
  }, [form, onPreviewSource, previewKey, repositoryPreviewKey, applyPreviewSelection])

  useEffect(() => {
    if (active !== 1 || form.sourceMode !== 'github' || !form.repositoryUrl.trim()) {
      return
    }
    void loadPreview(false)
  }, [active, form.sourceMode, form.repositoryUrl, form.repositoryBranch, form.repositoryToken, form.configPath, loadPreview])

  const verifyImageAccess = async () => {
    const message = validateRegistryStep(form, preview)
    if (message) {
      setValidationMessage(message)
      return
    }
    if (!selectedImage) {
      setValidationMessage('확인할 컨테이너 이미지가 없습니다.')
      return
    }

    const request: VerifyImageAccessRequest = {
      image: selectedImage,
      registryServer: form.registryServer.trim() || undefined,
      registryUsername: form.registryUsername.trim() || undefined,
      registryToken: form.registryToken.trim() || undefined,
    }

    setImageAccessLoading(true)
    setImageAccessError('')
    setImageAccessResult(null)
    try {
      const response = await onVerifyImageAccess(request)
      setImageAccessResult(response)
      setImageAccessKey(currentImageAccessKey)
    } catch (error) {
      if (error instanceof Error) {
        setImageAccessError(error.message)
      } else {
        setImageAccessError('이미지 접근 상태를 확인하지 못했습니다.')
      }
    } finally {
      setImageAccessLoading(false)
    }
  }

  const repositoryUrlError =
    validationMessage === 'GitHub 저장소 URL을 입력하세요.' ? validationMessage : undefined
  const repositoryTokenError =
    validationMessage === 'Private 저장소는 GitHub 저장소 토큰을 입력하세요.' ? validationMessage : undefined
  const appNameError =
    validationMessage === '애플리케이션 이름을 입력하세요.' ||
    validationMessage === '애플리케이션 이름은 영문 소문자, 숫자, 하이픈만 사용할 수 있습니다.'
      ? validationMessage
      : undefined
  const imageError = validationMessage === '컨테이너 이미지 주소를 입력하세요.' ? validationMessage : undefined
  const registryCredentialError = validationMessage.includes('레지스트리') ? validationMessage : undefined
  const nextStepDisabled =
    submitting ||
    (
      active === 1 &&
      form.sourceMode === 'github' &&
      previewLoading
    )

  return (
    <Card shadow="sm" p="lg" radius="md" withBorder className="glass-panel">
      <Stepper
        active={active}
        onStepClick={(nextStepIndex) => {
          if (form.sourceMode === 'quick' && (nextStepIndex === 1 || nextStepIndex === 2)) {
            return
          }
          if (nextStepIndex <= active) {
            setValidationMessage('')
            setActive(nextStepIndex)
          }
        }}
        allowNextStepsSelect={false}
        color="lagoon.6"
        mb="xl"
      >
        <Stepper.Step label="등록 방식" description="시작 방식 선택">
          <Stack gap="md" mt="md">
            <Text size="sm" c="dimmed">
              지금 바로 이미지와 포트를 입력해서 만들 수도 있고, GitHub 저장소의 `aolda_deploy.json`을 읽어서 자동으로
              등록할 수도 있습니다.
            </Text>

            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
              <Card
                component="button"
                type="button"
                aria-pressed={form.sourceMode === 'quick'}
                withBorder
                radius="md"
                padding="lg"
                bg={form.sourceMode === 'quick' ? 'lagoon.0' : undefined}
                style={{
                  cursor: 'pointer',
                  textAlign: 'left',
                  width: '100%',
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
                component="button"
                type="button"
                aria-pressed={form.sourceMode === 'github'}
                withBorder
                radius="md"
                padding="lg"
                bg={form.sourceMode === 'github' ? 'lagoon.0' : undefined}
                style={{
                  cursor: 'pointer',
                  textAlign: 'left',
                  width: '100%',
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
              <Card withBorder radius="md" padding="lg" bg="gray.0">
                <Group justify="space-between" align="flex-start">
                  <Stack gap={4}>
                    <Text fw={800}>연결 마법사로 진행</Text>
                    <Text size="sm" c="dimmed">
                      다음 단계에서 저장소 URL과 필요 토큰을 넣고, AODS가 설정 파일과 이미지 정보를 순서대로 확인합니다.
                    </Text>
                  </Stack>
                  <Group gap="xs" wrap="wrap">
                    <Button
                      component="a"
                      href={applicationSourceGuideURL}
                      target="_blank"
                      rel="noreferrer"
                      variant="default"
                      radius="xl"
                      size="xs"
                    >
                      설정 페이지 열기
                    </Button>
                    <Button variant="default" radius="xl" size="xs" onClick={handleDownloadRepositoryConfigExample}>
                      예시 JSON 다운로드
                    </Button>
                  </Group>
                </Group>
              </Card>
            ) : null}

            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
              {form.sourceMode === 'quick' ? (
                <TextInput
                  label="애플리케이션 이름"
                  placeholder="예: payment-api"
                  required
                  ref={appNameInputRef}
                  value={form.name}
                  error={appNameError}
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
                  ref={imageInputRef}
                  value={form.image}
                  error={imageError}
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

            <Text size="xs" c="dimmed">
              앱 이름은 GitOps 경로와 애플리케이션 ID를 결정합니다. 비워두면 저장소 서비스 ID를 기준으로 생성됩니다. 예: <Code>{applicationIdPreview}</Code>
              {!effectiveAppNameIsValid ? ' 현재 입력값은 경로 preview에 반영하지 않았습니다.' : ''}
            </Text>
          </Stack>
        </Stepper.Step>

        <Stepper.Step label="GitHub 연결" description="저장소 접근 확인">
          <Stack gap="md" mt="md">
            {form.sourceMode === 'github' ? (
              <>
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
                  저장소 URL을 넣고 접근을 확인하면 AODS가 바로 <Code>{form.configPath || 'aolda_deploy.json'}</Code>까지 읽습니다.
                  public 저장소는 토큰 없이 진행하고, private 저장소만 읽기 토큰을 추가합니다.
                </Alert>

                <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
                  <TextInput
                    label="GitHub 저장소 URL"
                    placeholder="예: https://github.com/aods/example-app.git"
                    required
                    ref={repositoryUrlInputRef}
                    name="aods-github-repository-url"
                    autoComplete="off"
                    autoCapitalize="none"
                    spellCheck={false}
                    data-form-type="other"
                    data-lpignore="true"
                    data-1p-ignore="true"
                    value={form.repositoryUrl}
                    error={repositoryUrlError}
                    onChange={(event) => updateForm({ repositoryUrl: event.target.value })}
                  />
                  <Select
                    label="저장소 공개 여부"
                    data={[
                      { value: 'public', label: 'Public 저장소' },
                      { value: 'private', label: 'Private 저장소' },
                    ]}
                    value={repositoryAccess}
                    onChange={(value) => setRepositoryAccess(value === 'private' ? 'private' : 'public')}
                    allowDeselect={false}
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

                {repositoryAccess === 'private' ? (
                  <Card withBorder radius="md" padding="md" bg="gray.0">
                    <Stack gap="sm">
                      <Group justify="space-between" align="flex-start">
                        <Stack gap={2}>
                          <Text fw={700}>Private 저장소 토큰</Text>
                          <Text size="sm" c="dimmed">
                            fine-grained token에서 대상 repository를 선택하고 Contents 권한을 Read-only로 둡니다.
                          </Text>
                        </Stack>
                        <Button
                          component="a"
                          href={githubFineGrainedTokenURL}
                          target="_blank"
                          rel="noreferrer"
                          variant="default"
                          radius="xl"
                          size="xs"
                        >
                          GitHub 토큰 만들기
                        </Button>
                      </Group>
                      <PasswordInput
                        label="GitHub 저장소 토큰"
                        description="이 토큰은 aolda_deploy.json을 읽는 용도로만 저장됩니다."
                        name="aods-github-token"
                        autoComplete="new-password"
                        data-form-type="other"
                        data-lpignore="true"
                        data-1p-ignore="true"
                        value={form.repositoryToken}
                        error={repositoryTokenError}
                        onChange={(event) => updateForm({ repositoryToken: event.target.value })}
                      />
                    </Stack>
                  </Card>
                ) : (
                  <Alert color="gray" variant="light">
                    public 저장소로 진행합니다. 토큰 없이 저장소와 설정 파일을 확인합니다.
                  </Alert>
                )}

                <Group justify="space-between" align="center">
                  <Group gap="xs">
                    <Button
                      component="a"
                      href={applicationSourceGuideURL}
                      target="_blank"
                      rel="noreferrer"
                      variant="default"
                      radius="xl"
                      size="xs"
                    >
                      설정 페이지 열기
                    </Button>
                    <Button variant="default" radius="xl" size="xs" onClick={handleDownloadRepositoryConfigExample}>
                      예시 JSON 다운로드
                    </Button>
                  </Group>
                  <Button variant="light" onClick={() => void loadPreview(true)} loading={previewLoading}>
                    저장소 연결 확인
                  </Button>
                </Group>

                {previewLoading ? (
                  <Alert color="lagoon" variant="light">
                    저장소와 설정 파일을 확인하는 중입니다.
                  </Alert>
                ) : null}

                {previewError ? (
                  <Alert color="red" variant="light">
                    {previewError}
                  </Alert>
                ) : null}

                {repositoryConnectionReady && preview ? (
                  <Card withBorder radius="md" padding="md" bg="gray.0">
                    <Stack gap="xs">
                      <Group gap="xs">
                        <Badge color="green" variant="light">저장소 접근 가능</Badge>
                        <Badge color="green" variant="light">설정 파일 확인됨</Badge>
                      </Group>
                      <Text size="sm">저장소 owner: <Code>{repositoryTarget?.owner || '-'}</Code></Text>
                      <Text size="sm">저장소 이름: <Code>{repositoryTarget?.repo || '-'}</Code></Text>
                      <Text size="sm">감지된 서비스 수: <Code>{String(preview.services.length)}</Code></Text>
                    </Stack>
                  </Card>
                ) : null}
              </>
            ) : (
              <Alert color="gray" variant="light">
                빠른 생성 모드는 GitHub 저장소를 읽지 않으므로 이 단계를 건너뜁니다.
              </Alert>
            )}
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

        <Stepper.Step label="이미지 접근" description="레지스트리 인증">
          <Stack gap="md" mt="md">
            <Alert color="blue" variant="light">
              <Stack gap={4}>
                <Text fw={700}>컨테이너 이미지를 pull할 수 있게 준비합니다</Text>
                <Text size="sm">
                  public 이미지는 인증 없이 진행하고, private 이미지만 레지스트리 사용자명과 토큰을 입력합니다.
                </Text>
                <Text size="sm">
                  실제 이미지 접근성은 애플리케이션 생성 시 backend preflight에서 한 번 더 검증합니다.
                </Text>
              </Stack>
            </Alert>

            <Card withBorder radius="md" padding="md" bg="gray.0">
              <Stack gap="xs">
                <Text fw={700}>배포 이미지</Text>
                <Text size="sm">
                  {selectedImage ? <Code>{selectedImage}</Code> : '아직 선택된 이미지가 없습니다.'}
                </Text>
                <Text size="sm">레지스트리: <Code>{registryServerPreview}</Code></Text>
                {form.sourceMode === 'github' && !selectedImage ? (
                  <Text size="sm" c="red">
                    설정 파일 확인 단계에서 배포할 서비스를 먼저 선택하세요.
                  </Text>
                ) : null}
              </Stack>
            </Card>

            <Card withBorder radius="md" padding="md" bg="gray.0">
              <Stack gap="md">
                <Group justify="space-between" align="flex-start">
                  <Stack gap={2}>
                    <Text fw={700}>레지스트리 인증</Text>
                    <Text size="sm" c="dimmed">
                      GHCR private 이미지는 GitHub token에 read:packages 권한이 필요합니다.
                    </Text>
                  </Stack>
                  <Button
                    component="a"
                    href={githubRegistryTokenURL}
                    target="_blank"
                    rel="noreferrer"
                    variant="default"
                    radius="xl"
                    size="xs"
                  >
                    GHCR 토큰 만들기
                  </Button>
                </Group>

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
                    error={registryCredentialError}
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
                    error={registryCredentialError}
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
                  {!effectiveAppNameIsValid ? ' 올바른 앱 이름을 입력하면 실제 경로가 확정됩니다.' : ''}
                </Text>

                <Group justify="space-between" align="center">
                  <Stack gap={2}>
                    <Text fw={700}>이미지 pull 확인</Text>
                    <Text size="sm" c="dimmed">
                      현재 입력한 이미지와 레지스트리 인증 정보로 manifest 접근을 확인합니다.
                    </Text>
                  </Stack>
                  <Button
                    variant="light"
                    onClick={() => void verifyImageAccess()}
                    loading={imageAccessLoading}
                    disabled={!selectedImage}
                  >
                    이미지 접근 확인
                  </Button>
                </Group>

                {imageAccessResultIsCurrent ? (
                  <Alert color="green" variant="light">
                    {imageAccessResult?.message || '이미지를 가져올 수 있습니다.'} registry: <Code>{imageAccessResult?.registry || registryServerPreview}</Code>
                  </Alert>
                ) : null}

                {imageAccessResult && !imageAccessResultIsCurrent ? (
                  <Alert color="yellow" variant="light">
                    이미지 또는 인증 정보가 바뀌었습니다. 다시 확인하세요.
                  </Alert>
                ) : null}

                {imageAccessError ? (
                  <Alert color="red" variant="light">
                    {imageAccessError}
                  </Alert>
                ) : null}

                {form.registryUsername.trim() && form.registryToken.trim() ? (
                  <Alert color="green" variant="light">
                    private 이미지 pull credential을 함께 저장합니다.
                  </Alert>
                ) : (
                  <Alert color="gray" variant="light">
                    토큰 없이 public 이미지 기준으로 진행합니다. private image pull 실패가 나면 이 단계로 돌아와 토큰을 추가하세요.
                  </Alert>
                )}
              </Stack>
            </Card>
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
                  allowDeselect={false}
                  disabled
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
        {active < 6 ? (
          <Button onClick={nextStep} color="lagoon.6" disabled={nextStepDisabled}>
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
  repositoryAccess: 'public' | 'private',
  repositoryConnectionReady: boolean,
) {
  switch (step) {
    case 0:
      return validateIdentityStep(form)
    case 1:
      return validateRepositoryStep(form, preview, previewError, repositoryAccess, repositoryConnectionReady)
    case 2:
      return validatePreviewStep(form, preview, previewError)
    case 3:
      return validateRegistryStep(form, preview)
    case 4:
      return validateDeploymentStep(form, environments)
    case 5:
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
  repositoryAccess: 'public' | 'private',
  repositoryConnectionReady: boolean,
) {
  return (
    validateIdentityStep(form) ||
    validateRepositoryStep(form, preview, previewError, repositoryAccess, repositoryConnectionReady) ||
    validatePreviewStep(form, preview, previewError) ||
    validateRegistryStep(form, preview) ||
    validateDeploymentStep(form, environments) ||
    validateSecretsStep(form)
  )
}

function focusFirstInvalidField(
  step: number,
  form: CreateFormState,
  refs: {
    repositoryUrlInput: HTMLInputElement | null
    appNameInput: HTMLInputElement | null
    imageInput: HTMLInputElement | null
  },
) {
  if (step === 1 && form.sourceMode === 'github' && !form.repositoryUrl.trim()) {
    refs.repositoryUrlInput?.focus()
    return
  }
  if (step === 0 && form.sourceMode === 'quick') {
    const normalizedName = form.name.trim()
    if (!normalizedName || !appNamePattern.test(normalizedName)) {
      refs.appNameInput?.focus()
      return
    }
    if (!form.image.trim()) {
      refs.imageInput?.focus()
    }
  }
}

function validateIdentityStep(form: CreateFormState) {
  const normalizedName = form.name.trim()
  if (form.sourceMode === 'quick' && !normalizedName) {
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
  return ''
}

function validateRepositoryStep(
  form: CreateFormState,
  preview: PreviewApplicationSourceResponse | null,
  previewError: string,
  repositoryAccess: 'public' | 'private',
  repositoryConnectionReady: boolean,
) {
  if (form.sourceMode !== 'github') {
    return ''
  }
  if (!form.repositoryUrl.trim()) {
    return 'GitHub 저장소 URL을 입력하세요.'
  }
  if (repositoryAccess === 'private' && !form.repositoryToken.trim()) {
    return 'Private 저장소는 GitHub 저장소 토큰을 입력하세요.'
  }
  if (previewError) {
    return previewError
  }
  if (!preview) {
    return '저장소 연결 확인을 먼저 실행하세요.'
  }
  if (!repositoryConnectionReady) {
    return '저장소 또는 설정 파일 값이 바뀌었습니다. 저장소 연결 확인을 다시 실행하세요.'
  }
  return ''
}

function validateRegistryStep(form: CreateFormState, preview: PreviewApplicationSourceResponse | null) {
  if (form.sourceMode === 'github') {
    const selectedServiceID = form.repositoryServiceId.trim() || preview?.selectedServiceId || ''
    const selectedService = preview?.services.find((service) => service.serviceId === selectedServiceID)
      ?? (preview?.services.length === 1 ? preview.services[0] : null)
    if (!selectedService) {
      return '설정 파일에서 배포할 서비스를 먼저 선택하세요.'
    }
  }
  const hasRegistryUsername = Boolean(form.registryUsername.trim())
  const hasRegistryToken = Boolean(form.registryToken.trim())
  if (hasRegistryUsername !== hasRegistryToken) {
    return '레지스트리 사용자명과 레지스트리 토큰은 함께 입력하세요.'
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

function translatePreviewError(message: string, configPath: string) {
  const path = configPath.trim() || 'aolda_deploy.json'
  const normalized = message.toLowerCase()
  if (normalized.includes('could not be read') || normalized.includes('not found') || normalized.includes('404')) {
    return `저장소에서 ${path} 파일을 읽지 못했습니다. 저장소 URL, 브랜치, 설정 파일 경로를 확인하세요.`
  }
  if (normalized.includes('bad credentials') || normalized.includes('unauthorized') || normalized.includes('authentication') || normalized.includes('401') || normalized.includes('403')) {
    return 'GitHub 저장소를 읽을 권한이 없습니다. private 저장소라면 읽기 권한이 있는 토큰을 입력하세요.'
  }
  if (normalized.includes('rate limit')) {
    return 'GitHub API 호출 한도에 걸렸습니다. 잠시 후 다시 시도하거나 저장소 읽기 토큰을 입력하세요.'
  }
  if (normalized.includes('invalid') && normalized.includes('json')) {
    return `${path} 파일의 JSON 형식이 올바르지 않습니다. 설정 파일 문법을 확인하세요.`
  }
  if (!message.trim()) {
    return '설정 파일을 확인하지 못했습니다.'
  }
  return message
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
