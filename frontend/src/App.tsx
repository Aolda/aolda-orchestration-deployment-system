import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import {
  Alert,
  Badge,
  Button,
  Checkbox,
  Drawer,
  Divider,
  Grid,
  Group,
  Loader,
  Modal,
  NumberInput,
  PasswordInput,
  ScrollArea,
  Select,
  SegmentedControl,
  SimpleGrid,
  Stack,
  Switch,
  Table,
  Tabs,
  Text,
  Textarea,
  TextInput,
  Tooltip,
  UnstyledButton,
} from '@mantine/core'
import { notifications } from '@mantine/notifications'
import {
  IconActivity,
  IconArrowLeft,
  IconBox,
  IconChevronRight,
  IconCloudCheck,
  IconAlertTriangle,
  IconGitBranch,
  IconHistory,
  IconLock,
  IconPlus,
  IconRefresh,
  IconRocket,
  IconShieldCheck,
  IconSettings,
  IconCpu,
  IconDatabase,
  IconBolt,
  IconExternalLink,
  IconUser,
} from '@tabler/icons-react'
import { ApiError, api } from './api/client'
import classes from './App.module.css'
import { ApplicationWizard, type CreateFormState } from './components/ApplicationWizard'
import {
  clearEmergencyAuthSession,
  clearOIDCSession,
  ensureOIDCAccessToken,
  hasEmergencyAuthSession,
  isEmergencyLoginEnabled,
  isOIDCAuthEnabled,
  logoutOIDCSession,
  startEmergencyAuthSession,
  shouldResumeOIDCSession,
} from './auth/oidc'
import type {
  ApplicationResources,
  ApplicationHealthSnapshot,
  ApplicationSecretsResponse,
  ApplicationSecretVersionsResponse,
  ApplicationMetricsResponse,
  ApplicationSummary,
  ChangeRecord,
  ContainerLogEvent,
  ContainerLogTargetsResponse,
  CreateClusterRequest,
  CurrentUser,
  CreateChangeRequest,
  CreateProjectRequest,
  DeploymentRecord,
  EnvironmentSummary,
  EventListResponse,
  FleetResourceOverviewResponse,
  HealthSignal,
  MetricSeries,
  NetworkExposureResponse,
  ProjectPolicy,
  ProjectSummary,
  RepositoryPollStatus,
  RollbackPolicy,
  SecretEntry,
  SyncStatus,
  SyncStatusResponse,
  ClusterSummary,
  VerifyImageAccessRequest,
} from './types/api'
import { AppShell as PortalShell } from './app/layout/AppShell'
import type { GlobalSection } from './app/layout/navigation'
import { ProjectsWorkspace, type ProjectTab } from './pages/projects/ProjectsWorkspace'
import { ChangesWorkspace } from './pages/changes/ChangesWorkspace'
import { ClustersPage } from './pages/clusters/ClustersPage'
import { MePage } from './pages/me/MePage'
import { StatePanel } from './components/ui/StatePanel'

const platformAdminAuthorities = new Set(parseAuthorityList(import.meta.env.VITE_AODS_PLATFORM_ADMIN_AUTHORITIES ?? 'aods:platform:admin'))
const localLoginUsername = 'admin'
const localLoginPassword = 'qwe1356@'
const supportedDeploymentStrategies = ['Rollout'] as const
const repositoryPollIntervalOptions = [
  { value: '60', label: '1분' },
  { value: '300', label: '5분' },
  { value: '600', label: '10분' },
]
const projectRefreshIntervalMs = 15000
const applicationDetailsRefreshIntervalMs = 15000
const externalInternetConnectionURL = 'https://itda.aoldacloud.com/login'
const showProjectComposer = false
const showRollbackPolicyControls = false
const showServiceMeshControls = false
const showEmergencyActionControls = false
const showApplicationLifecycleControls = false
const cpuLimitPresetOptions = [
  { value: '500m', label: '기본 상한 500m' },
  { value: '1000m', label: '확장 상한 1000m' },
]
const memoryLimitPresetOptions = [
  { value: '512Mi', label: '기본 상한 512Mi' },
  { value: '1Gi', label: '확장 상한 1Gi' },
]

function translateCreateApplicationError(message: string) {
  if (message === 'repositoryServiceId is required when the descriptor defines multiple services') {
    return '이 저장소에는 서비스가 여러 개 있습니다. 설정 파일 확인 단계에서 aolda_deploy.json 안의 serviceId 하나를 선택하세요.'
  }
  if (message === 'aolda_deploy.json could not be read from the repository') {
    return '저장소에서 aolda_deploy.json 파일을 읽지 못했습니다. 저장소 URL, 브랜치, 설정 파일 경로를 다시 확인하세요.'
  }
  if (message === 'aolda_deploy.json format is invalid') {
    return 'aolda_deploy.json 형식이 올바르지 않습니다. 설명 페이지의 예시 구조와 비교해서 확인하세요.'
  }
  return message
}

function translatePreviewSourceError(error: ApiError) {
  if (error.code === 'ROUTE_NOT_FOUND' || error.message === 'Route was not found.') {
    return '백엔드를 다시 시작하세요. 현재 실행 중인 서버에는 설정 파일 확인 API가 아직 반영되지 않았습니다.'
  }
  return error.message
}

function translateImageAccessError(error: ApiError) {
  if (error.code === 'ROUTE_NOT_FOUND' || error.message === 'Route was not found.') {
    return '백엔드를 다시 시작하세요. 현재 실행 중인 서버에는 이미지 접근 확인 API가 아직 반영되지 않았습니다.'
  }
  if (error.code === 'IMAGE_AUTH_REQUIRED') {
    return '현재 입력한 정보로는 이미지를 가져올 수 없습니다. private 이미지라면 레지스트리 사용자명과 read 권한 토큰을 확인하세요.'
  }
  if (error.code === 'IMAGE_NOT_FOUND') {
    return '이미지를 찾지 못했습니다. 이미지 이름과 태그가 실제 레지스트리에 있는지 확인하세요.'
  }
  if (error.code === 'INVALID_IMAGE_REFERENCE') {
    return '컨테이너 이미지 주소 형식이 올바르지 않습니다.'
  }
  return error.message
}

function translateAdminOverviewError(error: unknown) {
  if (error instanceof ApiError) {
    if (error.code === 'ROUTE_NOT_FOUND' || error.message === 'Route was not found.') {
      return '백엔드를 다시 시작하세요. 현재 실행 중인 서버에는 어드민 리소스 개요 API가 아직 반영되지 않았습니다.'
    }
    if (error.code === 'FORBIDDEN') {
      return 'platform admin만 전체 리소스 효율 화면을 볼 수 있습니다.'
    }
    return error.message || '전체 리소스 효율 정보를 불러오지 못했습니다.'
  }
  return '전체 리소스 효율 정보를 불러오지 못했습니다.'
}

function translateApplicationLogsError(error: unknown) {
  if (error instanceof ApiError) {
    if (error.code === 'ROUTE_NOT_FOUND' || error.message === 'Route was not found.') {
      return '백엔드를 다시 시작하세요. 현재 실행 중인 서버에는 컨테이너 로그 API가 아직 반영되지 않았습니다.'
    }
    if (error.code === 'INVALID_REQUEST' && error.message.includes('kubernetes api is not configured')) {
      return '백엔드의 Kubernetes API 설정이 아직 연결되지 않았습니다.'
    }
    if (error.code === 'INVALID_REQUEST' && error.message.includes('selected pod or container was not found')) {
      return '선택한 pod 또는 container가 이미 교체되었습니다. 로그 대상을 다시 불러옵니다.'
    }
    if (isStaleApplicationLogsError(error)) {
      return 'Pod가 교체되어 기존 로그 대상이 사라졌습니다. 새 로그 대상을 다시 불러옵니다.'
    }
    if (error.code === 'INTEGRATION_ERROR') {
      const cause = apiErrorDetailCause(error)
      return cause
        ? `Kubernetes 로그 조회 실패: ${cause}`
        : 'Kubernetes에서 컨테이너 로그를 가져오지 못했습니다. 잠시 후 다시 시도하세요.'
    }
    return error.message || '컨테이너 로그를 불러오지 못했습니다.'
  }
  return '컨테이너 로그를 불러오지 못했습니다.'
}

function apiErrorDetailCause(error: ApiError) {
  const cause = typeof error.details?.error === 'string' ? error.details.error.trim() : ''
  if (!cause) {
    return ''
  }
  return cause.length > 180 ? `${cause.slice(0, 177)}...` : cause
}

function translateRepositoryPollControlError(error: unknown, fallback: string) {
  if (error instanceof ApiError) {
    if (error.code === 'ROUTE_NOT_FOUND' || error.message === 'Route was not found.') {
      return '백엔드를 다시 시작하세요. 현재 실행 중인 서버에는 저장소 sync API가 아직 반영되지 않았습니다.'
    }

    const cause = apiErrorDetailCause(error)
    if (error.code === 'INTEGRATION_ERROR' && cause) {
      const lowerCause = cause.toLowerCase()
      if (lowerCause.includes('repository token was empty')) {
        return '저장소 토큰이 비어 있습니다. Vault의 repository token 값을 확인하세요.'
      }
      if (lowerCause.includes('read repository secret') || lowerCause.includes('send vault request') || lowerCause.includes('vault')) {
        return `저장소 토큰을 읽지 못했습니다. Vault 연결 상태를 확인하세요. (${cause})`
      }
      return `저장소 연동 실패: ${cause}`
    }

    return error.message || fallback
  }

  return fallback
}

function isStaleApplicationLogsError(error: unknown) {
  if (!(error instanceof ApiError)) {
    return false
  }
  if (error.code === 'INVALID_REQUEST' && error.message.includes('selected pod or container was not found')) {
    return true
  }
  const text = [error.message, apiErrorDetailCause(error)].join(' ').toLowerCase()
  return (
    (error.code === 'INTEGRATION_ERROR' || error.code === 'STREAM_ERROR') &&
    (
      text.includes('not found') ||
      text.includes('404') ||
      text.includes('the server could not find the requested resource')
    )
  )
}

function translateApplicationNetworkError(error: unknown) {
  if (error instanceof ApiError) {
    const detail = typeof error.details?.error === 'string' ? error.details.error : ''
    if (
      error.code === 'INVALID_REQUEST' &&
      (detail.includes('unknown field "meshEnabled"') || detail.includes('unknown field "loadBalancerEnabled"'))
    ) {
      return '백엔드를 다시 시작하세요. 현재 실행 중인 서버에는 네트워크 노출 정책 API가 아직 반영되지 않았습니다.'
    }
    if (error.code === 'ROUTE_NOT_FOUND' || error.message === 'Route was not found.') {
      return '백엔드를 다시 시작하세요. 현재 실행 중인 서버에는 애플리케이션 수정 API가 아직 반영되지 않았습니다.'
    }
    return error.message || '트래픽 설정을 저장하지 못했습니다.'
  }
  return '트래픽 설정을 저장하지 못했습니다.'
}

function translateApplicationSecretsError(error: unknown) {
  if (error instanceof ApiError) {
    if (error.code === 'ROUTE_NOT_FOUND' || error.message === 'Route was not found.') {
      return '백엔드를 다시 시작하세요. 현재 실행 중인 서버에는 환경 변수 관리 API가 아직 반영되지 않았습니다.'
    }
    if (error.code === 'FORBIDDEN') {
      return 'deployer 이상 권한에서만 환경 변수를 조회하거나 수정할 수 있습니다.'
    }
    if (error.code === 'CHANGE_REVIEW_REQUIRED') {
      return '이 환경은 직접 수정이 막혀 있습니다. 변경 요청 흐름에서 처리해야 합니다.'
    }
    return error.message || '환경 변수 정보를 처리하지 못했습니다.'
  }
  return '환경 변수 정보를 처리하지 못했습니다.'
}

// --- Internal Components ---

const Sparkline = ({ points, color }: { points: { value: number | null }[]; color: string }) => {
  const validPoints = points.filter((p) => p.value !== null) as { value: number }[]
  if (validPoints.length < 2) return <div style={{ height: '100%', width: '100%', background: '#f1f5f9', borderRadius: '4px' }} />

  const min = Math.min(...validPoints.map((p) => p.value))
  const max = Math.max(...validPoints.map((p) => p.value))
  const range = max - min || 1
  const width = 100
  const height = 40
  const padding = 2

  const pathData = validPoints
    .map((p, i) => {
      const x = (i / (validPoints.length - 1)) * width
      const y = height - ((p.value - min) / range) * (height - padding * 2) - padding
      return `${i === 0 ? 'M' : 'L'} ${x} ${y}`
    })
    .join(' ')

  return (
    <svg viewBox={`0 0 ${width} ${height}`} style={{ width: '100%', height: '100%', display: 'block' }}>
      <path d={pathData} fill="none" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

const SyncStatusBadge = ({ status }: { status: SyncStatus }) => {
  const config = {
    Synced: { color: 'green', label: '정상', icon: <IconCloudCheck size={14} /> },
    Syncing: { color: 'yellow', label: '동기화 중', icon: <IconRefresh size={14} className={classes.liveIndicator} /> },
    Degraded: { color: 'red', label: '문제 발생', icon: <IconBox size={14} /> },
    Unknown: { color: 'gray', label: '확인 불가', icon: <IconBox size={14} /> },
  }
  const { color, label, icon } = config[status] || config.Unknown
  return (
    <Badge variant="light" color={color} leftSection={icon} radius="sm">
      {label}
    </Badge>
  )
}

const MetricCard = ({
  label,
  value,
  unit,
  points,
  color,
  active,
  onClick,
  description,
}: {
  label: string
  value: string
  unit: string
  points?: { value: number | null }[]
  color: string
  active?: boolean
  onClick?: () => void
  description?: string
}) => (
  <div 
    className={`${classes.metricCard} ${active ? classes.activeMetricCard : ''}`} 
    onClick={onClick}
    style={{ cursor: onClick ? 'pointer' : 'default' }}
  >
    <Text className={classes.metricLabel}>{label}</Text>
    <Group align="baseline" gap={4}>
      <Text className={classes.metricValue}>{value}</Text>
      {unit && <Text size="xs" c="dimmed" fw={700}>{unit}</Text>}
    </Group>
    {description ? (
      <Text size="xs" c="dimmed" mt={4}>{description}</Text>
    ) : null}
    {points && (
      <div className={classes.sparklineWrapper}>
        <Sparkline points={points} color={color} />
      </div>
    )}
  </div>
)

const HelpTooltipLabel = ({ label, description }: { label: string; description: string }) => (
  <Group gap={6} align="center" wrap="nowrap">
    <Text size="xs" c="dimmed" fw={700}>{label}</Text>
    <Tooltip label={description} multiline w={280} withArrow position="top-start">
      <Text
        component="span"
        size="xs"
        fw={800}
        c="lagoon.6"
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
          width: '18px',
          height: '18px',
          borderRadius: '999px',
          border: '1px solid #bfdbfe',
          background: '#eff6ff',
          cursor: 'help',
          flexShrink: 0,
        }}
      >
        ?
      </Text>
    </Tooltip>
  </Group>
)

type ApplicationCatalogSignalState = 'available' | 'empty' | 'failed'
type LoadBalancerExposureStepState = 'done' | 'active' | 'pending' | 'error'

type LoadBalancerExposureStep = {
  title: string
  owner: 'AODS' | 'Flux' | '클러스터' | '네트워크'
  detail: ReactNode
  state: LoadBalancerExposureStepState
}

type ApplicationCatalogSignal = {
  metrics: MetricSeries[]
  metricsState: ApplicationCatalogSignalState
  latestDeployment: DeploymentRecord | null
  deploymentState: ApplicationCatalogSignalState
  healthSignals: HealthSignal[]
}

function formatCPUCoreValue(value?: number) {
  if (value == null) return '확인 불가'
  return `${Math.round(value * 1000)}m`
}

function formatCPUAllocationValue(value?: number) {
  if (value == null) return '미설정'
  return `${Math.round(value * 1000)}m`
}

function formatMemoryMiBValue(value?: number) {
  if (value == null) return '확인 불가'
  return `${value.toFixed(value >= 100 ? 0 : 1)} MiB`
}

function formatMemoryAllocationValue(value?: number) {
  if (value == null) return '미설정'
  return `${value.toFixed(value >= 100 ? 0 : 1)} MiB`
}

function formatUtilizationValue(value?: number) {
  if (value == null) return '확인 불가'
  return `${value.toFixed(value >= 100 ? 0 : 1)}%`
}

const defaultApplicationResources: ApplicationResources = {
  requests: {
    cpu: '250m',
    memory: '256Mi',
  },
  limits: {
    cpu: '500m',
    memory: '512Mi',
  },
}

function resolveApplicationResourcesDraft(resources?: ApplicationResources): ApplicationResources {
  return {
    requests: {
      cpu: defaultApplicationResources.requests?.cpu || '',
      memory: defaultApplicationResources.requests?.memory || '',
    },
    limits: {
      cpu: resources?.limits?.cpu || defaultApplicationResources.limits?.cpu || '',
      memory: resources?.limits?.memory || defaultApplicationResources.limits?.memory || '',
    },
  }
}

type ApplicationNetworkDraft = {
  meshEnabled: boolean
  loadBalancerEnabled: boolean
}

type SecretDraftEntry = {
  key: string
  value: string
}

const defaultApplicationNetworkDraft: ApplicationNetworkDraft = {
  meshEnabled: false,
  loadBalancerEnabled: false,
}

function resolveApplicationNetworkDraft(application?: {
  meshEnabled?: boolean
  loadBalancerEnabled?: boolean
} | null): ApplicationNetworkDraft {
  return {
    meshEnabled: application?.meshEnabled ?? false,
    loadBalancerEnabled: application?.loadBalancerEnabled ?? false,
  }
}

// --- Login Form Component ---

const LoginForm = ({
  oidcEnabled,
  allowEmergencyLogin,
  onLogin,
  onEmergencyLogin,
}: {
  oidcEnabled: boolean
  allowEmergencyLogin: boolean
  onLogin: () => Promise<void>
  onEmergencyLogin: (username: string, password: string) => Promise<void>
}) => {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState<'oidc' | 'emergency' | 'password' | null>(null)

  const runLogin = async (mode: 'oidc' | 'emergency' | 'password') => {
    setSubmitting(mode)
    setError('')
    try {
      if (mode === 'emergency') {
        await onEmergencyLogin(username, password)
      } else {
        await onLogin()
      }
    } catch (loginError) {
      setSubmitting(null)
      setError(loginError instanceof Error ? loginError.message : '로그인을 시작하지 못했습니다.')
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (oidcEnabled) {
      void runLogin('emergency')
      return
    }

    if (username === localLoginUsername && password === localLoginPassword) {
      void runLogin('password')
      return
    }

    setError('아이디 또는 비밀번호가 올바르지 않습니다.')
  }

  return (
    <div className={classes.authShell}>
      <div className={classes.loginCard}>
        <Stack gap="xl">
          <div style={{ textAlign: 'center' }}>
            <Text
              fw={900}
              style={{
                fontSize: '3rem',
                letterSpacing: '-0.06em',
                lineHeight: 1,
              }}
            >
              AODS
            </Text>
            <Text size="sm" c="dimmed" mt={8}>
              내부 배포 관리 플랫폼
            </Text>
          </div>

          {oidcEnabled && allowEmergencyLogin ? (
            <Stack gap="lg">
              <form onSubmit={handleSubmit} autoComplete="off">
                <Stack gap="md">
                  {error ? <Alert color="red">{error}</Alert> : null}
                  <TextInput
                    label="아이디"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    required
                    leftSection={<IconUser size={16} />}
                    name="aods-login-username"
                    autoComplete="off"
                    autoCapitalize="none"
                    spellCheck={false}
                  />
                  <PasswordInput
                    label="비밀번호"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    required
                    leftSection={<IconLock size={16} />}
                    name="aods-login-password"
                    autoComplete="new-password"
                  />
                  <Button fullWidth size="md" color="lagoon.6" type="submit" mt="xs" loading={submitting === 'emergency'}>
                    로그인
                  </Button>
                </Stack>
              </form>

              <Divider label="또는" labelPosition="center" />

              <Stack gap="xs">
                <Button fullWidth size="md" variant="default" onClick={() => void runLogin('oidc')} loading={submitting === 'oidc'}>
                  Keycloak으로 로그인
                </Button>
              </Stack>
            </Stack>
          ) : oidcEnabled ? (
            <Stack gap="md">
              <Alert color="blue" variant="light">
                사내 SSO(Keycloak)로 로그인한 뒤 `aods` 클라이언트 권한을 기준으로 포털 접근 범위를 결정합니다.
              </Alert>
              {error ? <Alert color="red">{error}</Alert> : null}
              <Button fullWidth size="md" color="lagoon.6" onClick={() => void runLogin('oidc')} loading={submitting === 'oidc'}>
                Keycloak로 로그인
              </Button>
            </Stack>
          ) : (
            <form onSubmit={handleSubmit} autoComplete="off">
              <Stack gap="md">
                <TextInput
                  label="아이디"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                  leftSection={<IconUser size={16} />}
                  name="aods-login-username"
                  autoComplete="off"
                  autoCapitalize="none"
                  spellCheck={false}
                />
                <PasswordInput
                  label="비밀번호"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  leftSection={<IconLock size={16} />}
                  name="aods-login-password"
                  autoComplete="new-password"
                />
                {error ? <Alert color="red">{error}</Alert> : null}
                <Button fullWidth size="md" color="lagoon.6" type="submit" mt="md" loading={submitting === 'password'}>
                  로그인
                </Button>
              </Stack>
            </form>
          )}
        </Stack>
      </div>
    </div>
  )
}

type ProjectSettingsPanelProps = {
  project: ProjectSummary | undefined
  environments: EnvironmentSummary[]
  projectPolicy: ProjectPolicy | null
  canEditPolicies: boolean
  savingPolicies: boolean
  onSavePolicies: (policy: ProjectPolicy) => void
  applicationCount: number
  canDeleteProject: boolean
  isProtectedProject: boolean
  deletingProject: boolean
  pendingProjectDelete: boolean
  onRequestProjectDelete: () => void
  onCancelProjectDelete: () => void
  onConfirmProjectDelete: () => void
}

type ChangeActionKind = 'submit' | 'approve' | 'merge'
type LifecycleActionKind = 'archive' | 'delete'

const ProjectSettingsPanel = ({
  project,
  environments,
  projectPolicy,
  canEditPolicies,
  savingPolicies,
  onSavePolicies,
  applicationCount,
  canDeleteProject,
  isProtectedProject,
  deletingProject,
  pendingProjectDelete,
  onRequestProjectDelete,
  onCancelProjectDelete,
  onConfirmProjectDelete,
}: ProjectSettingsPanelProps) => {
  const [policyDraft, setPolicyDraft] = useState<ProjectPolicy | null>(() => cloneProjectPolicy(projectPolicy))

  return (
  <Stack gap="xl" className={classes.projectSettingsPanel}>
    <div className={classes.surfaceCard}>
      <Text className={classes.sectionEyebrow} mb="md">기본 정보</Text>
      <Alert color="gray" variant="light" radius="md" mb="md">
        프로젝트 이름은 영문 소문자 slug 규칙으로 생성되며, 프로젝트 ID와 Kubernetes namespace도 같은 값을 사용합니다. 이 식별자는 생성 후 변경할 수 없습니다.
      </Alert>
      <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
        <div>
          <HelpTooltipLabel label="프로젝트 이름" description="프로젝트 식별에 사용하는 영문 slug이며, 내부 ID와 동일하게 동작합니다." />
          <Text size="sm" fw={800}>{project?.name || '-'}</Text>
        </div>
        <div>
          <HelpTooltipLabel label="네임스페이스" description="배포 리소스와 애플리케이션을 논리적으로 묶는 기본 공간입니다." />
          <Text size="sm" fw={800}>{project?.namespace || '-'}</Text>
        </div>
      </SimpleGrid>
    </div>

    <div className={classes.surfaceCard}>
      <Text className={classes.sectionEyebrow} mb="md">운영 환경</Text>
      {environments.length > 0 ? (
        <ScrollArea className={classes.projectSettingsTableScroll} offsetScrollbars>
          <Table striped highlightOnHover className={classes.projectSettingsTable} miw={640}>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>
                  <HelpTooltipLabel label="이름" description="배포와 운영에서 구분하는 환경 이름입니다. 예: dev, staging, prod" />
                </Table.Th>
                <Table.Th>
                  <HelpTooltipLabel label="클러스터" description="이 운영 환경이 실제로 반영되는 대상 클러스터입니다." />
                </Table.Th>
                <Table.Th>
                  <HelpTooltipLabel label="반영 방식" description="직접 반영이면 바로 적용되고, 변경 요청이면 승인 흐름을 거쳐 반영됩니다." />
                </Table.Th>
                <Table.Th>
                  <HelpTooltipLabel label="기본" description="새 애플리케이션 생성이나 배포 시 기본 선택으로 잡히는 환경인지 표시합니다." />
                </Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {environments.map((environment) => (
                <Table.Tr key={environment.id}>
                  <Table.Td>{environment.name}</Table.Td>
                  <Table.Td>{environment.clusterId}</Table.Td>
                  <Table.Td>{environment.writeMode === 'pull_request' ? '변경 요청' : '직접 반영'}</Table.Td>
                  <Table.Td>
                    <Badge color={environment.default ? 'lagoon.6' : 'gray'} variant="light">
                      {environment.default ? '기본' : '일반'}
                    </Badge>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </ScrollArea>
      ) : (
        <Text size="sm" c="dimmed">운영 환경 정보를 아직 불러오지 못했습니다.</Text>
      )}
    </div>

    <div className={classes.surfaceCard}>
      <Text className={classes.sectionEyebrow} mb="md">배포 정책</Text>
      {policyDraft ? (
        <Stack gap="md">
          <Text size="sm" c="dimmed">
            아래 정책 값은 이 화면에서 바로 수정할 수 있습니다. 저장하면 현재 프로젝트의 운영 가드레일에 즉시 반영됩니다.
          </Text>
          <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
          <div>
            <HelpTooltipLabel label="최소 복제본 수" description="서비스가 유지해야 하는 최소 replica 기준값입니다." />
            <NumberInput
              min={0}
              value={policyDraft.minReplicas}
              onChange={(value) => {
                const nextValue = typeof value === 'number' ? value : Number(value || 0)
                setPolicyDraft((current) => (current ? { ...current, minReplicas: Number.isFinite(nextValue) ? nextValue : 0 } : current))
              }}
              disabled={!canEditPolicies}
            />
          </div>
          <div>
            <HelpTooltipLabel label="프로브 필수" description="liveness, readiness 같은 헬스체크 설정을 필수로 요구하는지 나타냅니다." />
            <Switch
              checked={policyDraft.requiredProbes}
              onChange={(event) => {
                const checked = event.currentTarget.checked
                setPolicyDraft((current) => (current ? { ...current, requiredProbes: checked } : current))
              }}
              disabled={!canEditPolicies}
            />
          </div>
          <div>
            <HelpTooltipLabel label="운영 환경 변경 요청 필수" description="운영 성격 환경에는 직접 반영 대신 승인 기반 변경 요청이 필요한지 보여줍니다." />
            <Switch
              checked={policyDraft.prodPRRequired}
              onChange={(event) => {
                const checked = event.currentTarget.checked
                setPolicyDraft((current) => (current ? { ...current, prodPRRequired: checked } : current))
              }}
              disabled={!canEditPolicies}
            />
          </div>
          {showRollbackPolicyControls ? (
            <div>
              <HelpTooltipLabel label="자동 롤백" description="배포 이상 징후가 생기면 이전 안정 버전으로 자동 복구할지 나타냅니다." />
              <Switch
                checked={policyDraft.autoRollbackEnabled}
                onChange={(event) => {
                  const checked = event.currentTarget.checked
                  setPolicyDraft((current) => (current ? { ...current, autoRollbackEnabled: checked } : current))
                }}
                disabled={!canEditPolicies}
              />
            </div>
          ) : null}
          <div>
            <HelpTooltipLabel label="허용 환경" description="이 프로젝트가 배포 대상으로 사용할 수 있는 운영 환경 목록입니다." />
            <Text size="sm" fw={800}>{policyDraft.allowedEnvironments.join(', ') || '-'}</Text>
            <Text size="xs" c="dimmed">현재 플랫폼 기본값으로 고정되어 있으며 이 화면에서 수정하지 않습니다.</Text>
          </div>
          <div>
            <HelpTooltipLabel label="허용 배포 전략" description="이 프로젝트에서 선택 가능한 배포 방식 목록입니다." />
            <Text size="sm" fw={800}>{supportedDeploymentStrategies.join(', ')}</Text>
            <Text size="xs" c="dimmed">현재 프론트에서는 Rollout만 지원하도록 고정되어 있습니다.</Text>
          </div>
          <div style={{ gridColumn: '1 / -1' }}>
            <HelpTooltipLabel label="허용 클러스터 대상" description="이 프로젝트가 배포될 수 있는 클러스터 범위를 정의합니다." />
            <Text size="sm" fw={800}>{policyDraft.allowedClusterTargets.join(', ') || '-'}</Text>
            <Text size="xs" c="dimmed">현재 플랫폼 기본값으로 고정되어 있으며 이 화면에서 수정하지 않습니다.</Text>
          </div>
          </SimpleGrid>

          <Group justify="space-between" align="center">
            {!canEditPolicies ? (
              <Text size="sm" c="dimmed">admin 역할만 정책 변경을 저장할 수 있습니다.</Text>
            ) : (
              <Text size="sm" c="dimmed">허용 환경, 배포 전략, 클러스터 대상은 현재 플랫폼 기본값으로 고정됩니다.</Text>
            )}
            <Group>
              <Button
                variant="default"
                disabled={!canEditPolicies}
                onClick={() => {
                  setPolicyDraft(cloneProjectPolicy(projectPolicy))
                }}
              >
                되돌리기
              </Button>
              <Button
                color="lagoon.6"
                loading={savingPolicies}
                disabled={!canEditPolicies || !policyDraft}
                onClick={() => {
                  if (policyDraft) {
                    onSavePolicies(policyDraft)
                  }
                }}
              >
                정책 저장
              </Button>
            </Group>
          </Group>
        </Stack>
      ) : (
        <Text size="sm" c="dimmed">프로젝트 정책 정보를 아직 불러오지 못했습니다.</Text>
      )}
    </div>

    <div className={classes.surfaceCard}>
      <Group justify="space-between" align="start" mb="md">
        <div>
          <Text className={classes.sectionEyebrow}>위험 구역</Text>
          <Text fw={800}>프로젝트 삭제</Text>
          <Text size="sm" c="dimmed" mt={6}>
            프로젝트 카탈로그 엔트리와 애플리케이션 {applicationCount}개, 연결된 앱 시크릿을 함께 정리합니다.
          </Text>
        </div>
        <Badge color={isProtectedProject ? 'gray' : canDeleteProject ? 'red' : 'gray'} variant="light">
          {isProtectedProject ? '보호됨' : canDeleteProject ? '삭제 가능' : '권한 필요'}
        </Badge>
      </Group>

      {isProtectedProject ? (
        <Alert color="gray" radius="md" icon={<IconLock size={16} />}>
          공용 프로젝트는 보호 대상이라 삭제할 수 없습니다.
        </Alert>
      ) : pendingProjectDelete ? (
        <Alert color="red" radius="md" icon={<IconAlertTriangle size={16} />}>
          <Text size="sm">
            이 작업은 되돌릴 수 없습니다. 프로젝트 자체와 하위 애플리케이션 운영 흔적이 함께 정리됩니다.
          </Text>
          <Group mt="md">
            <Button color="red" loading={deletingProject} onClick={onConfirmProjectDelete}>
              프로젝트 삭제 확인
            </Button>
            <Button variant="default" onClick={onCancelProjectDelete}>
              취소
            </Button>
          </Group>
        </Alert>
      ) : (
        <Stack gap="xs">
          <Button color="red" variant="light" disabled={!canDeleteProject} onClick={onRequestProjectDelete}>
            프로젝트 삭제
          </Button>
          {!canDeleteProject ? (
            <Text size="sm" c="dimmed">platform admin만 프로젝트 삭제를 실행할 수 있습니다.</Text>
          ) : null}
        </Stack>
      )}
    </div>
  </Stack>
)}

// --- Main App Component ---

export default function App() {
  const oidcAuthEnabled = isOIDCAuthEnabled()
  const emergencyLoginEnabled = isEmergencyLoginEnabled()
  const [isLoggedIn, setIsLoggedIn] = useState(() => (oidcAuthEnabled ? shouldResumeOIDCSession() || hasEmergencyAuthSession() : false))
  const [activeSection, setActiveSection] = useState<GlobalSection>('projects')
  const [projectTab, setProjectTab] = useState<ProjectTab>('applications')
  const [projectComposerOpen, setProjectComposerOpen] = useState(false)
  const [clusterComposerOpen, setClusterComposerOpen] = useState(false)
  const [currentUser, setCurrentUser] = useState<CurrentUser | null>(null)
  const [projects, setProjects] = useState<ProjectSummary[]>([])
  const [clusters, setClusters] = useState<ClusterSummary[]>([])
  const [adminResourceOverview, setAdminResourceOverview] = useState<FleetResourceOverviewResponse | null>(null)
  const [adminResourceOverviewLoading, setAdminResourceOverviewLoading] = useState(false)
  const [adminResourceOverviewError, setAdminResourceOverviewError] = useState<string | null>(null)
  const [bootstrapWarnings, setBootstrapWarnings] = useState<string[]>([])
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)
  const [applications, setApplications] = useState<ApplicationSummary[]>([])
  const [environments, setEnvironments] = useState<EnvironmentSummary[]>([])
  const [projectPolicy, setProjectPolicy] = useState<ProjectPolicy | null>(null)
  const [projectDataLoaded, setProjectDataLoaded] = useState(false)
  const [projectDataWarnings, setProjectDataWarnings] = useState<string[]>([])
  const [trackedChanges, setTrackedChanges] = useState<ChangeRecord[]>([])
  const [selectedChangeId, setSelectedChangeId] = useState<string | null>(null)
  const [selectedAppId, setSelectedAppId] = useState<string | null>(null)
  const [applicationDrawerTab, setApplicationDrawerTab] = useState('status')
  const [selectedDeploymentId, setSelectedDeploymentId] = useState<string | null>(null)
  const [selectedDeploymentDetail, setSelectedDeploymentDetail] = useState<DeploymentRecord | null>(null)
  const [loading, setLoading] = useState(true)
  const [appDetailsLoaded, setAppDetailsLoaded] = useState(false)
  const [appDetailWarnings, setAppDetailWarnings] = useState<string[]>([])
  const [selectedDeploymentLoaded, setSelectedDeploymentLoaded] = useState(false)

  // Application Details State
  const [appDetails, setAppDetails] = useState<{
    metrics: ApplicationMetricsResponse | null
    syncStatus: SyncStatusResponse | null
    networkExposure: NetworkExposureResponse | null
    deployments: DeploymentRecord[]
    events: EventListResponse['items']
    rollbackPolicy: RollbackPolicy | null
    logTargets: ContainerLogTargetsResponse | null
  }>({ metrics: null, syncStatus: null, networkExposure: null, deployments: [], events: [], rollbackPolicy: null, logTargets: null })
  const [applicationSecrets, setApplicationSecrets] = useState<ApplicationSecretsResponse | null>(null)
  const [applicationSecretVersions, setApplicationSecretVersions] = useState<ApplicationSecretVersionsResponse | null>(null)
  const [applicationSecretsLoaded, setApplicationSecretsLoaded] = useState(false)
  const [applicationSecretsError, setApplicationSecretsError] = useState<string | null>(null)
  const [projectInsightMetrics, setProjectInsightMetrics] = useState<MetricSeries[]>([])
  const [applicationCatalogSignals, setApplicationCatalogSignals] = useState<Record<string, ApplicationCatalogSignal>>({})

  const [wizardOpened, setWizardOpened] = useState(false)
  const [projectSettingsOpened, setProjectSettingsOpened] = useState(false)
  const [creatingApplication, setCreatingApplication] = useState(false)
  const [creatingProject, setCreatingProject] = useState(false)
  const [deletingProject, setDeletingProject] = useState(false)
  const [creatingCluster, setCreatingCluster] = useState(false)
  const [creatingChange, setCreatingChange] = useState(false)
  const [refreshingChanges, setRefreshingChanges] = useState(false)
  const [savingProjectPolicy, setSavingProjectPolicy] = useState(false)
  const [changeActionLoading, setChangeActionLoading] = useState<{
    changeId: string
    action: ChangeActionKind
  } | null>(null)
  const [savingRollbackPolicy, setSavingRollbackPolicy] = useState(false)
  const [savingApplicationNetwork, setSavingApplicationNetwork] = useState(false)
  const [savingApplicationResources, setSavingApplicationResources] = useState(false)
  const [savingApplicationSecrets, setSavingApplicationSecrets] = useState(false)
  const [savingRepositoryPollInterval, setSavingRepositoryPollInterval] = useState(false)
  const [syncingRepositoryPoll, setSyncingRepositoryPoll] = useState(false)
  const [loadBalancerConfirmOpened, setLoadBalancerConfirmOpened] = useState(false)
  const [loadBalancerConfirmAcknowledged, setLoadBalancerConfirmAcknowledged] = useState(false)
  const [emergencyActionLoading, setEmergencyActionLoading] = useState<'abort' | 'rollback' | null>(null)
  const [promotingDeploymentId, setPromotingDeploymentId] = useState<string | null>(null)
  const [lifecycleActionLoading, setLifecycleActionLoading] = useState<LifecycleActionKind | null>(null)
  const [metricRange, setMetricRange] = useState('15m')
  const [selectedLogPodName, setSelectedLogPodName] = useState('')
  const [selectedLogContainerName, setSelectedLogContainerName] = useState('')
  const [liveLogEvents, setLiveLogEvents] = useState<ContainerLogEvent[]>([])
  const [liveLogStatus, setLiveLogStatus] = useState<'idle' | 'connecting' | 'streaming' | 'closed' | 'error'>('idle')
  const [liveLogError, setLiveLogError] = useState<string | null>(null)
  const [refreshingApplicationLogs, setRefreshingApplicationLogs] = useState(false)
  const [logStreamRefreshKey, setLogStreamRefreshKey] = useState(0)
  const [deployEnvironment, setDeployEnvironment] = useState('')
  const [pendingDangerAction, setPendingDangerAction] = useState<'abort' | 'rollback' | null>(null)
  const [pendingLifecycleAction, setPendingLifecycleAction] = useState<LifecycleActionKind | null>(null)
  const [pendingProjectDelete, setPendingProjectDelete] = useState(false)
  const [projectDraft, setProjectDraft] = useState<CreateProjectRequest>({
    id: '',
    name: '',
    description: '',
  })
  const [clusterDraft, setClusterDraft] = useState<CreateClusterRequest>({
    id: '',
    name: '',
    description: '',
    default: false,
  })
  const [rollbackPolicyDraft, setRollbackPolicyDraft] = useState<RollbackPolicy>({
    enabled: false,
  })
  const [applicationResourcesDraft, setApplicationResourcesDraft] = useState<ApplicationResources>(
    defaultApplicationResources,
  )
  const [applicationNetworkDraft, setApplicationNetworkDraft] = useState<ApplicationNetworkDraft>(
    defaultApplicationNetworkDraft,
  )
  const [secretValueDrafts, setSecretValueDrafts] = useState<Record<string, string>>({})
  const [secretDeleteDrafts, setSecretDeleteDrafts] = useState<string[]>([])
  const [newSecretRows, setNewSecretRows] = useState<SecretDraftEntry[]>([{ key: '', value: '' }])
  const [secretBulkText, setSecretBulkText] = useState('')
  const [secretBulkMessage, setSecretBulkMessage] = useState('')
  const [restoringSecretVersion, setRestoringSecretVersion] = useState<number | null>(null)
  const [repositoryPollIntervalDraft, setRepositoryPollIntervalDraft] = useState('300')
  const projectRefreshSeq = useRef(0)
  const appDetailsRequestSeq = useRef(0)
  const logTargetsRequestSeq = useRef(0)
  const selectedAppIdRef = useRef<string | null>(null)
  const previousSelectedAppRef = useRef<string | null>(null)
  const previousDeploymentAppRef = useRef<string | null>(null)
  const previousResourceAppRef = useRef<string | null>(null)
  const previousNetworkAppRef = useRef<string | null>(null)
  const previousSelectedProjectRef = useRef<string | null>(null)
  const previousRepositoryPollAppRef = useRef<string | null>(null)
  const previousRepositoryPollServerValueRef = useRef<string>('300')
  const isPlatformAdmin = hasPlatformAdmin(currentUser)
  const visibleGlobalSections = useMemo<GlobalSection[]>(
    () => (isPlatformAdmin ? ['projects', 'clusters', 'me'] : ['projects', 'me']),
    [isPlatformAdmin],
  )

  useEffect(() => {
    if (!visibleGlobalSections.includes(activeSection)) {
      setActiveSection('projects')
    }
  }, [activeSection, visibleGlobalSections])

  const resetSessionState = useCallback(() => {
    setCurrentUser(null)
    setProjects([])
    setClusters([])
    setAdminResourceOverview(null)
    setAdminResourceOverviewLoading(false)
    setAdminResourceOverviewError(null)
    setBootstrapWarnings([])
    setSelectedProjectId(null)
    setApplications([])
    setEnvironments([])
    setProjectPolicy(null)
    setProjectDataLoaded(false)
    setProjectDataWarnings([])
    setTrackedChanges([])
    setSelectedChangeId(null)
    setSelectedAppId(null)
    setSelectedDeploymentId(null)
    setSelectedDeploymentDetail(null)
    setAppDetailsLoaded(false)
    setAppDetailWarnings([])
    setSelectedDeploymentLoaded(false)
    setAppDetails({
      metrics: null,
      syncStatus: null,
      networkExposure: null,
      deployments: [],
      events: [],
      rollbackPolicy: null,
      logTargets: null,
    })
    setApplicationSecrets(null)
    setApplicationSecretVersions(null)
    setApplicationSecretsLoaded(false)
    setApplicationSecretsError(null)
    setSelectedLogPodName('')
    setSelectedLogContainerName('')
    setLiveLogEvents([])
    setLiveLogStatus('idle')
    setLiveLogError(null)
    setRefreshingApplicationLogs(false)
    setLogStreamRefreshKey(0)
    setApplicationResourcesDraft(defaultApplicationResources)
    setApplicationNetworkDraft(defaultApplicationNetworkDraft)
    setSecretValueDrafts({})
    setSecretDeleteDrafts([])
    setNewSecretRows([{ key: '', value: '' }])
    setSecretBulkText('')
    setSecretBulkMessage('')
    setSavingApplicationSecrets(false)
    setRestoringSecretVersion(null)
    setRepositoryPollIntervalDraft('300')
    setSavingRepositoryPollInterval(false)
    setSyncingRepositoryPoll(false)
    previousResourceAppRef.current = null
    previousNetworkAppRef.current = null
    previousRepositoryPollAppRef.current = null
    previousRepositoryPollServerValueRef.current = '300'
    setProjectInsightMetrics([])
    setApplicationCatalogSignals({})
    setWizardOpened(false)
    setProjectSettingsOpened(false)
    setProjectComposerOpen(false)
    setClusterComposerOpen(false)
    setPendingProjectDelete(false)
    setPendingDangerAction(null)
    setPendingLifecycleAction(null)
    setCreatingApplication(false)
    setCreatingProject(false)
    setDeletingProject(false)
    setCreatingCluster(false)
    setCreatingChange(false)
    setRefreshingChanges(false)
    setSavingProjectPolicy(false)
    setChangeActionLoading(null)
    setSavingRollbackPolicy(false)
    setSavingApplicationNetwork(false)
    setSavingApplicationResources(false)
    setEmergencyActionLoading(null)
    setPromotingDeploymentId(null)
    setLifecycleActionLoading(null)
    setLoading(true)
  }, [])

  const handleLogin = useCallback(async () => {
    clearEmergencyAuthSession()
    if (oidcAuthEnabled) {
      await ensureOIDCAccessToken()
    }
    setLoading(true)
    setIsLoggedIn(true)
  }, [oidcAuthEnabled])

  const handleEmergencyLogin = useCallback(async (username: string, password: string) => {
    startEmergencyAuthSession(username, password)
    setLoading(true)
    setIsLoggedIn(true)
  }, [])

  const handleLogout = useCallback(async () => {
    resetSessionState()
    setIsLoggedIn(false)
    clearEmergencyAuthSession()
    if (oidcAuthEnabled) {
      await logoutOIDCSession()
    }
  }, [oidcAuthEnabled, resetSessionState])

  // Fetch Bootstrap Data
  useEffect(() => {
    if (!isLoggedIn) return
    const fetchProjects = async () => {
      try {
        const [meRes, projectRes] = await Promise.allSettled([
          api.getCurrentUser(),
          api.getProjects(),
        ])
        const warnings: string[] = []
        let resolvedUser: CurrentUser | null = null
        if (meRes.status === 'fulfilled') {
          resolvedUser = meRes.value
          setCurrentUser(meRes.value)
        } else {
          warnings.push('사용자 정보를 불러오지 못했습니다.')
        }
        if (projectRes.status !== 'fulfilled') {
          throw projectRes.reason
        }

        if (hasPlatformAdmin(resolvedUser)) {
          try {
            const clusterRes = await api.getClusters()
            setClusters(clusterRes.items)
          } catch {
            warnings.push('클러스터 카탈로그를 불러오지 못했습니다.')
          }
        } else {
          setClusters([])
        }

        const visibleProjects = filterVisibleProjects(projectRes.value.items)
        setBootstrapWarnings(warnings)
        setProjects(visibleProjects)
        setSelectedProjectId((current) => {
          if (current && visibleProjects.some((project) => project.id === current)) {
            return current
          }
          return visibleProjects[0]?.id ?? null
        })
      } catch (error) {
        if (oidcAuthEnabled) {
          clearOIDCSession()
          clearEmergencyAuthSession()
          resetSessionState()
          setIsLoggedIn(false)
        }
        notifications.show({
          title: oidcAuthEnabled ? '로그인 또는 초기화 실패' : '오류',
          message: error instanceof Error ? error.message : '프로젝트 목록을 가져오지 못했습니다.',
          color: 'red',
        })
      } finally {
        setLoading(false)
      }
    }
    fetchProjects()
  }, [isLoggedIn, oidcAuthEnabled, resetSessionState])

  const refreshAdminResourceOverview = useCallback(async () => {
    if (!isLoggedIn || !isPlatformAdmin) {
      setAdminResourceOverview(null)
      setAdminResourceOverviewError(null)
      setAdminResourceOverviewLoading(false)
      return
    }

    setAdminResourceOverviewLoading(true)
    setAdminResourceOverviewError(null)
    try {
      const response = await api.getAdminResourceOverview()
      setAdminResourceOverview(response)
    } catch (error) {
      setAdminResourceOverview(null)
      setAdminResourceOverviewError(translateAdminOverviewError(error))
    } finally {
      setAdminResourceOverviewLoading(false)
    }
  }, [isLoggedIn, isPlatformAdmin])

  useEffect(() => {
    if (!isLoggedIn || !isPlatformAdmin || activeSection !== 'clusters') {
      return
    }
    void refreshAdminResourceOverview()
  }, [activeSection, isLoggedIn, isPlatformAdmin, refreshAdminResourceOverview])

  const refreshProjectData = useCallback(async (projectId: string) => {
    const requestSeq = ++projectRefreshSeq.current
    try {
      const warnings: string[] = []
      const [appRes, healthRes, envRes, policyRes] = await Promise.allSettled([
        api.getApplications(projectId),
        api.getProjectHealth(projectId),
        api.getProjectEnvironments(projectId),
        api.getProjectPolicies(projectId),
      ])
      if (requestSeq !== projectRefreshSeq.current) {
        return
      }

      if (appRes.status === 'fulfilled') {
        const healthByApplication =
          healthRes.status === 'fulfilled'
            ? new Map(healthRes.value.items.map((item) => [item.applicationId, item]))
            : new Map<string, ApplicationHealthSnapshot>()
        setApplications(
          appRes.value.items.map((application) => ({
            ...application,
            syncStatus: healthByApplication.get(application.id)?.syncStatus ?? application.syncStatus,
          })),
        )
        if (appRes.value.items.length === 0) {
          setProjectInsightMetrics([])
          setApplicationCatalogSignals({})
        } else if (healthRes.status === 'fulfilled') {
          const aggregatedSeries: MetricSeries[] = []
          const nextSignals: Record<string, ApplicationCatalogSignal> = {}
          let missingHealthSnapshot = false

          for (const application of appRes.value.items) {
            const health = healthByApplication.get(application.id)
            if (!health) {
              missingHealthSnapshot = true
              nextSignals[application.id] = {
                metrics: [],
                metricsState: 'failed',
                latestDeployment: null,
                deploymentState: 'failed',
                healthSignals: [
                  {
                    key: 'health',
                    status: 'Unavailable',
                    message: '프로젝트 health snapshot에서 이 애플리케이션 항목을 찾지 못했습니다.',
                  },
                ],
              }
              continue
            }

            aggregatedSeries.push(...health.metrics)
            nextSignals[application.id] = {
              metrics: health.metrics,
              metricsState: metricsStateFromHealth(health),
              latestDeployment: health.latestDeployment ?? null,
              deploymentState: deploymentStateFromHealth(health),
              healthSignals: health.signals,
            }
          }

          if (missingHealthSnapshot) {
            warnings.push('일부 애플리케이션 health snapshot을 찾지 못했습니다.')
          }

          setProjectInsightMetrics(aggregateMetricSeries(aggregatedSeries))
          setApplicationCatalogSignals(nextSignals)
        } else {
          setProjectInsightMetrics([])
          setApplicationCatalogSignals(
            Object.fromEntries(
              appRes.value.items.map((application) => [
                application.id,
                {
                  metrics: [],
                  metricsState: 'failed' as const,
                  latestDeployment: null,
                  deploymentState: 'failed' as const,
                  healthSignals: [
                    {
                      key: 'health',
                      status: 'Unavailable',
                      message: '프로젝트 health snapshot을 불러오지 못했습니다.',
                    },
                  ],
                },
              ]),
            ),
          )
          warnings.push('프로젝트 health snapshot을 불러오지 못했습니다.')
          console.error('Failed to refresh project health', healthRes.reason)
        }
      } else {
        setApplicationCatalogSignals({})
        setProjectInsightMetrics([])
        warnings.push('애플리케이션 목록을 불러오지 못했습니다.')
        console.error('Failed to refresh applications', appRes.reason)
      }

      if (envRes.status === 'fulfilled') {
        setEnvironments(envRes.value.items)
      } else {
        warnings.push('운영 환경을 불러오지 못했습니다.')
        console.error('Failed to refresh environments', envRes.reason)
      }

      if (policyRes.status === 'fulfilled') {
        setProjectPolicy(policyRes.value)
      } else {
        warnings.push('프로젝트 정책을 불러오지 못했습니다.')
        console.error('Failed to refresh project policies', policyRes.reason)
      }

      setProjectDataWarnings(warnings)
      setProjectDataLoaded(true)

      if (
        appRes.status === 'rejected' &&
        envRes.status === 'rejected' &&
        policyRes.status === 'rejected'
      ) {
        notifications.show({ title: '오류', message: '데이터를 가져오지 못했습니다.', color: 'red' })
      }
    } catch (err) {
      console.error('Failed to refresh project data', err)
      setProjectDataWarnings(['프로젝트 작업 공간 데이터를 새로고침하지 못했습니다.'])
      setProjectDataLoaded(true)
    }
  }, [])

  const refreshTrackedChanges = useCallback(async (projectId: string, quiet = false) => {
    const cachedChanges = readProjectTrackedChanges(projectId)
    setTrackedChanges(cachedChanges)
    setSelectedChangeId((current) => {
      if (current && cachedChanges.some((change) => change.id === current)) {
        return current
      }
      return cachedChanges[0]?.id ?? null
    })

    if (cachedChanges.length === 0) {
      return
    }

    if (!quiet) {
      setRefreshingChanges(true)
    }

    try {
      const refreshedResults = await Promise.allSettled(
        cachedChanges.map((change) => api.getChange(change.id)),
      )
      const refreshedChanges = sortChangeRecords(
        cachedChanges.map((cachedChange) => {
          const latest = refreshedResults.find(
            (result): result is PromiseFulfilledResult<ChangeRecord> =>
              result.status === 'fulfilled' && result.value.id === cachedChange.id,
          )
          return latest?.value ?? cachedChange
        }),
      ).filter((change) => change.projectId === projectId)

      writeProjectTrackedChanges(projectId, refreshedChanges)
      setTrackedChanges(refreshedChanges)
      setSelectedChangeId((current) => {
        if (current && refreshedChanges.some((change) => change.id === current)) {
          return current
        }
        return refreshedChanges[0]?.id ?? null
      })
    } catch (err) {
      console.error('Failed to refresh tracked changes', err)
      if (!quiet) {
        notifications.show({
          title: '새로고침 실패',
          message: '추적 중인 변경 요청을 새로고침하지 못했습니다.',
          color: 'red',
        })
      }
    } finally {
      if (!quiet) {
        setRefreshingChanges(false)
      }
    }
  }, [])

  // Fetch Applications when project changes
  useEffect(() => {
    if (!selectedProjectId) return
    setProjectDataLoaded(false)
    setProjectDataWarnings([])
    refreshProjectData(selectedProjectId)
    const ival = setInterval(() => refreshProjectData(selectedProjectId), projectRefreshIntervalMs)
    return () => clearInterval(ival)
  }, [refreshProjectData, selectedProjectId])

  useEffect(() => {
    if (!selectedProjectId) {
      setTrackedChanges([])
      setSelectedChangeId(null)
      return
    }
    void refreshTrackedChanges(selectedProjectId, true)
  }, [refreshTrackedChanges, selectedProjectId])

  useEffect(() => {
    setPendingProjectDelete(false)
  }, [selectedProjectId])

  // Fetch Application Details when app changes or sidebar opens
  const fetchAppDetails = useCallback(async (appId: string) => {
    const requestSeq = ++appDetailsRequestSeq.current
    try {
      const warnings: string[] = []
      const [metrics, syncStatus, networkExposure, deployments, events, rollback] = await Promise.allSettled([
        api.getMetrics(appId, metricRange),
        api.getSyncStatus(appId),
        api.getApplicationNetworkExposure(appId),
        api.getDeployments(appId),
        api.getEvents(appId),
        api.getRollbackPolicy(appId),
      ])

      if (requestSeq !== appDetailsRequestSeq.current) {
        return
      }

      setAppDetails((current) => ({
        metrics: metrics.status === 'fulfilled' ? metrics.value : current.metrics,
        syncStatus: syncStatus.status === 'fulfilled' ? syncStatus.value : current.syncStatus,
        networkExposure: networkExposure.status === 'fulfilled' ? networkExposure.value : current.networkExposure,
        deployments: deployments.status === 'fulfilled' ? deployments.value.items : current.deployments,
        events: events.status === 'fulfilled' ? events.value.items : current.events,
        rollbackPolicy: rollback.status === 'fulfilled' ? rollback.value : current.rollbackPolicy,
        logTargets: current.logTargets,
      }))

      if (metrics.status === 'rejected') {
        warnings.push('metrics를 불러오지 못했습니다.')
      }
      if (syncStatus.status === 'rejected') {
        warnings.push('sync 상태를 불러오지 못했습니다.')
      }
      if (networkExposure.status === 'rejected') {
        warnings.push('외부 공개 상태를 불러오지 못했습니다.')
      }
      if (deployments.status === 'rejected') {
        warnings.push('배포 이력을 불러오지 못했습니다.')
      }
      if (events.status === 'rejected') {
        warnings.push('이벤트를 불러오지 못했습니다.')
      }
      if (rollback.status === 'rejected') {
        warnings.push('롤백 정책을 불러오지 못했습니다.')
      }

      if (rollback.status === 'fulfilled') {
        setRollbackPolicyDraft({
          enabled: rollback.value.enabled,
          maxErrorRate: rollback.value.maxErrorRate,
          maxLatencyP95Ms: rollback.value.maxLatencyP95Ms,
          minRequestRate: rollback.value.minRequestRate,
        })
      }
      setAppDetailWarnings(warnings)
      setAppDetailsLoaded(true)
    } catch (err) {
      console.error('Failed to fetch app details', err)
      setAppDetailWarnings(['운영 센터 데이터를 불러오지 못했습니다.'])
      setAppDetailsLoaded(true)
    }
  }, [metricRange])

  const refreshApplicationLogTargets = useCallback(async (
    appId: string,
    options: { restartStream?: boolean; quiet?: boolean } = {},
  ) => {
    const requestSeq = ++logTargetsRequestSeq.current
    setRefreshingApplicationLogs(true)
    if (options.restartStream) {
      setLiveLogEvents([])
      setLiveLogStatus('connecting')
      setLiveLogError(null)
    }

    try {
      const response = await api.getApplicationLogTargets(appId)
      if (requestSeq !== logTargetsRequestSeq.current || selectedAppIdRef.current !== appId) {
        return
      }
      setAppDetails((current) => ({
        ...current,
        logTargets: response,
      }))
      setAppDetailWarnings((current) => current.filter((warning) => !warning.startsWith('로그 대상: ')))
      if (options.restartStream) {
        setLogStreamRefreshKey((current) => current + 1)
      }
      if (!options.quiet) {
        notifications.show({
          title: '로그 대상 업데이트 완료',
          message: '현재 pod/container 목록을 다시 불러왔습니다.',
          color: 'green',
        })
      }
    } catch (error) {
      if (requestSeq !== logTargetsRequestSeq.current || selectedAppIdRef.current !== appId) {
        return
      }
      const message = translateApplicationLogsError(error)
      setLiveLogStatus('error')
      setLiveLogError(message)
      setAppDetailWarnings((current) => [
        ...current.filter((warning) => !warning.startsWith('로그 대상: ')),
        `로그 대상: ${message}`,
      ])
      if (!options.quiet) {
        notifications.show({
          title: '로그 업데이트 실패',
          message,
          color: 'red',
        })
      }
    } finally {
      if (requestSeq === logTargetsRequestSeq.current && selectedAppIdRef.current === appId) {
        setRefreshingApplicationLogs(false)
      }
    }
  }, [])

  useEffect(() => {
    selectedAppIdRef.current = selectedAppId
    logTargetsRequestSeq.current += 1
    setAppDetails((current) => ({ ...current, logTargets: null }))
    setRefreshingApplicationLogs(false)
    setLogStreamRefreshKey(0)
  }, [selectedAppId])

  useEffect(() => {
    if (selectedAppId) {
      setApplicationDrawerTab('status')
    }
  }, [selectedAppId])

  useEffect(() => {
    if (!selectedAppId || applicationDrawerTab !== 'observability') {
      return
    }
    void refreshApplicationLogTargets(selectedAppId, { quiet: true })
  }, [applicationDrawerTab, refreshApplicationLogTargets, selectedAppId])

  useEffect(() => {
    if (!selectedAppId) return
    setAppDetailsLoaded(false)
    setAppDetailWarnings([])
    fetchAppDetails(selectedAppId)
    const ival = setInterval(() => fetchAppDetails(selectedAppId), applicationDetailsRefreshIntervalMs)
    return () => clearInterval(ival)
  }, [selectedAppId, metricRange, fetchAppDetails])

  useEffect(() => {
    const nextDefaultEnvironment =
      environments.find((environment) => environment.default)?.id ?? environments[0]?.id ?? ''
    if (selectedAppId !== previousSelectedAppRef.current) {
      previousSelectedAppRef.current = selectedAppId
      setPendingDangerAction(null)
      setDeployEnvironment(nextDefaultEnvironment)
      return
    }

    setDeployEnvironment((current) => {
      if (current && environments.some((environment) => environment.id === current)) {
        return current
      }
      return nextDefaultEnvironment
    })
  }, [environments, selectedAppId])

  useEffect(() => {
    if (!selectedAppId) {
      previousDeploymentAppRef.current = null
      setSelectedDeploymentId(null)
      setSelectedDeploymentDetail(null)
      setSelectedDeploymentLoaded(false)
      return
    }

    if (selectedAppId !== previousDeploymentAppRef.current) {
      previousDeploymentAppRef.current = selectedAppId
      setSelectedDeploymentId(null)
      setSelectedDeploymentDetail(null)
      setSelectedDeploymentLoaded(false)
      return
    }

    const nextDeploymentId = appDetails.deployments[0]?.deploymentId ?? null
    setSelectedDeploymentId((current) => {
      if (current && appDetails.deployments.some((deployment) => deployment.deploymentId === current)) {
        return current
      }
      return nextDeploymentId
    })
  }, [appDetails.deployments, selectedAppId])

  const fetchSelectedDeploymentDetail = useCallback(async (applicationId: string, deploymentId: string) => {
    setSelectedDeploymentLoaded(false)
    try {
      const detail = await api.getDeployment(applicationId, deploymentId)
      setSelectedDeploymentDetail(detail)
    } catch (error) {
      console.error('Failed to fetch deployment detail', error)
      notifications.show({
        title: '배포 상세 조회 실패',
        message: '선택한 배포 상세 정보를 가져오지 못했습니다.',
        color: 'red',
      })
    } finally {
      setSelectedDeploymentLoaded(true)
    }
  }, [])

  useEffect(() => {
    if (!selectedAppId || !selectedDeploymentId) {
      setSelectedDeploymentDetail(null)
      setSelectedDeploymentLoaded(false)
      return
    }
    void fetchSelectedDeploymentDetail(selectedAppId, selectedDeploymentId)
  }, [fetchSelectedDeploymentDetail, selectedAppId, selectedDeploymentId])

  useEffect(() => {
    if (!selectedAppId) {
      setSelectedLogPodName('')
      setSelectedLogContainerName('')
      setLiveLogEvents([])
      setLiveLogStatus('idle')
      setLiveLogError(null)
      return
    }

    const targets = appDetails.logTargets?.items ?? []
    if (targets.length === 0) {
      setSelectedLogPodName('')
      setSelectedLogContainerName('')
      setLiveLogEvents([])
      setLiveLogStatus('idle')
      setLiveLogError(null)
      return
    }

    const nextPod = targets.find((target) => target.podName === selectedLogPodName) ?? targets[0]
    const nextContainer =
      nextPod.containers.find((container) => container.name === selectedLogContainerName) ??
      nextPod.containers.find((container) => container.default) ??
      nextPod.containers[0]

    if (nextPod.podName !== selectedLogPodName) {
      setSelectedLogPodName(nextPod.podName)
    }
    if ((nextContainer?.name ?? '') !== selectedLogContainerName) {
      setSelectedLogContainerName(nextContainer?.name ?? '')
    }
  }, [appDetails.logTargets, selectedAppId, selectedLogContainerName, selectedLogPodName])

  const selectedProject = useMemo(() => projects.find((p) => p.id === selectedProjectId), [projects, selectedProjectId])
  const selectedApp = useMemo(() => applications.find((a) => a.id === selectedAppId), [applications, selectedAppId])
  const cpuLimitOptions = useMemo(
    () => buildResourceLimitOptions(applicationResourcesDraft.limits?.cpu, cpuLimitPresetOptions),
    [applicationResourcesDraft.limits?.cpu],
  )
  const memoryLimitOptions = useMemo(
    () => buildResourceLimitOptions(applicationResourcesDraft.limits?.memory, memoryLimitPresetOptions),
    [applicationResourcesDraft.limits?.memory],
  )
  const selectedLogTarget = useMemo(
    () => appDetails.logTargets?.items.find((target) => target.podName === selectedLogPodName) ?? null,
    [appDetails.logTargets, selectedLogPodName],
  )
  const selectedLogContainer = useMemo(
    () => selectedLogTarget?.containers.find((container) => container.name === selectedLogContainerName) ?? null,
    [selectedLogContainerName, selectedLogTarget],
  )
  const isProtectedProject = isSharedProject(selectedProject)
  const selectedProjectRole = selectedProject?.role
  const canDeployInProject = canRoleDeploy(selectedProjectRole)
  const canAdminProject = canRoleAdmin(selectedProjectRole)
  const canDeleteProject = isPlatformAdmin && !isProtectedProject
  const resetSecretEditor = useCallback((response?: ApplicationSecretsResponse | null) => {
    setSecretValueDrafts({})
    setSecretDeleteDrafts([])
    setNewSecretRows([{ key: '', value: '' }])
    setSecretBulkText('')
    setSecretBulkMessage('')
    if (response) {
      setApplicationSecrets(response)
    }
  }, [])
  const refreshApplicationSecrets = useCallback(async (appId: string) => {
    setApplicationSecretsLoaded(false)
    setApplicationSecretsError(null)
    try {
      const [secretsResult, versionsResult] = await Promise.allSettled([
        api.getApplicationSecrets(appId),
        api.getApplicationSecretVersions(appId),
      ])
      if (secretsResult.status === 'rejected') {
        throw secretsResult.reason
      }
      resetSecretEditor(secretsResult.value)
      if (versionsResult.status === 'fulfilled') {
        setApplicationSecretVersions(versionsResult.value)
      } else {
        setApplicationSecretVersions(null)
      }
    } catch (error) {
      setApplicationSecretsError(translateApplicationSecretsError(error))
      setApplicationSecrets(null)
      setApplicationSecretVersions(null)
    } finally {
      setApplicationSecretsLoaded(true)
    }
  }, [resetSecretEditor])

  useEffect(() => {
    if (!selectedAppId) {
      setApplicationSecrets(null)
      setApplicationSecretVersions(null)
      setApplicationSecretsLoaded(false)
      setApplicationSecretsError(null)
      resetSecretEditor(null)
      return
    }
    if (!canDeployInProject) {
      setApplicationSecrets(null)
      setApplicationSecretVersions(null)
      setApplicationSecretsLoaded(true)
      setApplicationSecretsError(null)
      resetSecretEditor(null)
      return
    }
    void refreshApplicationSecrets(selectedAppId)
  }, [canDeployInProject, refreshApplicationSecrets, resetSecretEditor, selectedAppId])

  const persistedLoadBalancerEnabled = selectedApp?.loadBalancerEnabled ?? false
  const networkSyncStatus = appDetails.syncStatus?.status ?? selectedApp?.syncStatus
  const loadBalancerExposureWorkflow = useMemo(
    () => buildLoadBalancerExposureWorkflow(
      persistedLoadBalancerEnabled,
      applicationNetworkDraft.loadBalancerEnabled,
      networkSyncStatus,
      appDetails.networkExposure,
    ),
    [appDetails.networkExposure, applicationNetworkDraft.loadBalancerEnabled, networkSyncStatus, persistedLoadBalancerEnabled],
  )
  const defaultEnvironment = useMemo(
    () => environments.find((environment) => environment.default) ?? environments[0],
    [environments],
  )
  const defaultCreateEnvironment = useMemo(
    () =>
      environments.find((environment) => environment.default && environment.writeMode !== 'pull_request') ??
      environments.find((environment) => environment.writeMode !== 'pull_request') ??
      null,
    [environments],
  )
  const selectedDeployEnvironment = useMemo(
    () => environments.find((environment) => environment.id === deployEnvironment) ?? defaultEnvironment,
    [defaultEnvironment, deployEnvironment, environments],
  )
  const isProtectedDeployTarget = selectedDeployEnvironment?.writeMode === 'pull_request'

  useEffect(() => {
    if (!selectedAppId) {
      previousResourceAppRef.current = null
      setApplicationResourcesDraft(defaultApplicationResources)
      return
    }
    if (selectedAppId !== previousResourceAppRef.current) {
      previousResourceAppRef.current = selectedAppId
      setApplicationResourcesDraft(resolveApplicationResourcesDraft(selectedApp?.resources))
    }
  }, [selectedApp?.resources, selectedAppId])

  useEffect(() => {
    if (!selectedAppId) {
      previousNetworkAppRef.current = null
      setLoadBalancerConfirmOpened(false)
      setLoadBalancerConfirmAcknowledged(false)
      setApplicationNetworkDraft(defaultApplicationNetworkDraft)
      return
    }
    if (selectedAppId !== previousNetworkAppRef.current) {
      previousNetworkAppRef.current = selectedAppId
      setLoadBalancerConfirmOpened(false)
      setLoadBalancerConfirmAcknowledged(false)
      setApplicationNetworkDraft(resolveApplicationNetworkDraft({
        meshEnabled: selectedApp?.meshEnabled,
        loadBalancerEnabled: selectedApp?.loadBalancerEnabled,
      }))
    }
  }, [selectedApp?.loadBalancerEnabled, selectedApp?.meshEnabled, selectedAppId])

  const handleLoadBalancerDraftChange = (checked: boolean) => {
    if (!checked) {
      setApplicationNetworkDraft((current) => ({
        ...current,
        loadBalancerEnabled: false,
      }))
      setLoadBalancerConfirmOpened(false)
      setLoadBalancerConfirmAcknowledged(false)
      return
    }

    setLoadBalancerConfirmAcknowledged(false)
    setLoadBalancerConfirmOpened(true)
  }

  const confirmLoadBalancerDraft = () => {
    setApplicationNetworkDraft((current) => ({
      ...current,
      loadBalancerEnabled: true,
    }))
    setLoadBalancerConfirmOpened(false)
  }

  useEffect(() => {
    if (!selectedAppId || !selectedLogPodName || !selectedLogContainerName) {
      setLiveLogEvents([])
      setLiveLogStatus('idle')
      setLiveLogError(null)
      return
    }

    const controller = new AbortController()
    setLiveLogEvents([])
    setLiveLogStatus('connecting')
    setLiveLogError(null)

    void api.streamApplicationLogs(selectedAppId, {
      podName: selectedLogPodName,
      containerName: selectedLogContainerName,
      tailLines: 120,
      signal: controller.signal,
      onEvent: (event) => {
        setLiveLogEvents((current) => [...current, event].slice(-400))
        setLiveLogStatus('streaming')
      },
    }).then(() => {
      if (!controller.signal.aborted) {
        setLiveLogStatus('closed')
      }
    }).catch((error) => {
      if (controller.signal.aborted) {
        return
      }

      if (isStaleApplicationLogsError(error)) {
        setLiveLogStatus('connecting')
        setLiveLogError('선택한 pod/container가 교체되어 로그 대상을 다시 찾는 중입니다.')
        void refreshApplicationLogTargets(selectedAppId, { restartStream: true, quiet: true })
        return
      }

      setLiveLogStatus('error')
      setLiveLogError(translateApplicationLogsError(error))
    })

    return () => controller.abort()
  }, [logStreamRefreshKey, refreshApplicationLogTargets, selectedAppId, selectedLogContainerName, selectedLogPodName])

  useEffect(() => {
    if (selectedProjectId === previousSelectedProjectRef.current) {
      return
    }

    previousSelectedProjectRef.current = selectedProjectId
    setProjectTab(defaultProjectTab())
  }, [selectedProject, selectedProjectId])

  const projectNotice = !selectedProject ? null : !canDeployInProject ? (
    <StatePanel
      kind="forbidden"
      title="조회 전용 프로젝트입니다"
      description="현재 역할은 viewer라서 새 애플리케이션 생성, 배포, change draft 생성을 직접 실행할 수 없습니다."
    />
  ) : environments.some((environment) => environment.writeMode === 'pull_request') ? (
    <StatePanel
      kind="partial"
      title="보호 환경이 포함된 프로젝트입니다"
      description="pull_request 환경에서는 직접 배포보다 change request 흐름이 우선됩니다. 승인 규칙이 있는 환경을 먼저 확인하세요."
    />
  ) : null

  const handleCreateApp = async (form: CreateFormState) => {
    if (!selectedProjectId) return
    if (!canDeployInProject) {
      notifications.show({
        title: '권한 없음',
        message: 'viewer 역할은 새 애플리케이션을 생성할 수 없습니다.',
        color: 'yellow',
      })
      return
    }

    setCreatingApplication(true)
    try {
      const created = await api.createApplication(selectedProjectId, {
        name: form.name.trim() || undefined,
        description: form.description.trim() || undefined,
        image: form.sourceMode === 'quick' ? form.image.trim() : undefined,
        servicePort: form.sourceMode === 'quick' ? form.servicePort : undefined,
        deploymentStrategy: form.sourceMode === 'quick' ? 'Rollout' : undefined,
        environment: form.environment || defaultCreateEnvironment?.id || 'shared',
        secrets: form.secrets
          .filter((secret) => secret.key.trim() && secret.value.trim())
          .map((secret) => ({
            key: secret.key.trim(),
            value: secret.value,
          })),
        repositoryUrl: form.sourceMode === 'github' ? form.repositoryUrl.trim() || undefined : undefined,
        repositoryBranch:
          form.sourceMode === 'github' ? form.repositoryBranch.trim() || undefined : undefined,
        repositoryToken: form.sourceMode === 'github' ? form.repositoryToken.trim() || undefined : undefined,
        repositoryServiceId:
          form.sourceMode === 'github' ? form.repositoryServiceId.trim() || undefined : undefined,
        configPath: form.sourceMode === 'github' ? form.configPath.trim() || undefined : undefined,
        registryServer: form.registryServer.trim() || undefined,
        registryUsername: form.registryUsername.trim() || undefined,
        registryToken: form.registryToken.trim() || undefined,
      })
      notifications.show({
        title: '애플리케이션 생성 완료',
        message:
          form.sourceMode === 'github'
            ? `${created.name} 애플리케이션을 GitHub 설정 파일 기준으로 등록했습니다. 서비스 ID: ${created.repositoryServiceId || '확인 필요'}`
            : `${created.name} 애플리케이션을 등록했습니다.`,
        color: 'green',
      })
      setWizardOpened(false)
      await refreshProjectData(selectedProjectId)
      setProjectTab('applications')
      setSelectedAppId(created.id)
    } catch (error) {
      let message = '요청이 거부되었습니다.'
      if (error instanceof ApiError) {
        if (error.code === 'CHANGE_REVIEW_REQUIRED') {
          message = '선택한 환경은 직접 생성이 막혀 있습니다. 변경 요청 흐름을 사용하세요.'
        } else if (error.code === 'DUPLICATE_APPLICATION') {
          message = '같은 이름의 애플리케이션이 이미 있습니다.'
        } else if (error.code === 'INVALID_REQUEST') {
          message = translateCreateApplicationError(error.message)
        }
      }
      notifications.show({ title: '생성 실패', message, color: 'red' })
    } finally {
      setCreatingApplication(false)
    }
  }

  const wizardInitialState = useMemo<CreateFormState>(() => {
    return {
      sourceMode: 'github',
      name: '',
      description: '',
      image: '',
      servicePort: 80,
      deploymentStrategy: 'Rollout',
      environment: defaultCreateEnvironment?.id || '',
      repositoryUrl: '',
      repositoryBranch: '',
      repositoryToken: '',
      repositoryServiceId: '',
      configPath: 'aolda_deploy.json',
      registryServer: '',
      registryUsername: '',
      registryToken: '',
      secrets: [{ key: '', value: '' }],
    }
  }, [defaultCreateEnvironment])

  const handlePreviewAppSource = async (form: CreateFormState) => {
    if (!selectedProjectId) {
      throw new Error('프로젝트를 먼저 선택하세요.')
    }

    try {
      return await api.previewApplicationSource(selectedProjectId, {
        name: form.name.trim() || undefined,
        repositoryUrl: form.repositoryUrl.trim(),
        repositoryBranch: form.repositoryBranch.trim() || undefined,
        repositoryToken: form.repositoryToken.trim() || undefined,
        repositoryServiceId: form.repositoryServiceId.trim() || undefined,
        configPath: form.configPath.trim() || undefined,
      })
    } catch (error) {
      if (error instanceof ApiError) {
        throw new Error(translatePreviewSourceError(error))
      }
      throw error
    }
  }

  const handleVerifyAppImageAccess = async (request: VerifyImageAccessRequest) => {
    if (!selectedProjectId) {
      throw new Error('프로젝트를 먼저 선택하세요.')
    }

    try {
      return await api.verifyImageAccess(selectedProjectId, request)
    } catch (error) {
      if (error instanceof ApiError) {
        throw new Error(translateImageAccessError(error))
      }
      throw error
    }
  }

  const handleCreateProject = async () => {
    if (!isPlatformAdmin) {
      notifications.show({
        title: '권한 없음',
        message: 'platform admin만 프로젝트를 생성할 수 있습니다.',
        color: 'yellow',
      })
      return
    }
    if (!projectDraft.name.trim()) {
      notifications.show({
        title: '입력 확인 필요',
        message: '프로젝트 이름(slug)을 입력하세요.',
        color: 'yellow',
      })
      return
    }

    setCreatingProject(true)
    try {
      const created = await api.createProject({
        id: projectDraft.id.trim(),
        name: projectDraft.name.trim(),
        description: projectDraft.description?.trim() || undefined,
      })
      const projectRes = await api.getProjects()
      setProjects(filterVisibleProjects(projectRes.items))
      setSelectedProjectId(created.id)
      setProjectDraft({ id: '', name: '', description: '' })
      setProjectComposerOpen(false)
      notifications.show({
        title: '프로젝트 생성 완료',
        message: `${created.name} 프로젝트를 생성했습니다.`,
        color: 'green',
      })
    } catch {
      notifications.show({
        title: '프로젝트 생성 실패',
        message: '프로젝트를 생성하지 못했습니다.',
        color: 'red',
      })
    } finally {
      setCreatingProject(false)
    }
  }

  const handleUpdateProjectPolicy = async (policy: ProjectPolicy) => {
    if (!selectedProjectId) return

    setSavingProjectPolicy(true)
    try {
      const saved = await api.updateProjectPolicies(selectedProjectId, {
        ...policy,
        allowedEnvironments: projectPolicy?.allowedEnvironments ?? policy.allowedEnvironments,
        allowedDeploymentStrategies: [...supportedDeploymentStrategies],
        allowedClusterTargets: projectPolicy?.allowedClusterTargets ?? policy.allowedClusterTargets,
      })
      setProjectPolicy(saved)
      notifications.show({
        title: '정책 저장 완료',
        message: '프로젝트 운영 규칙을 업데이트했습니다.',
        color: 'green',
      })
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '프로젝트 운영 규칙을 저장하지 못했습니다.'
      notifications.show({
        title: '정책 저장 실패',
        message,
        color: 'red',
      })
    } finally {
      setSavingProjectPolicy(false)
    }
  }

  const handleDeleteProject = async () => {
    if (!selectedProjectId || !selectedProject) return
    if (!isPlatformAdmin) {
      notifications.show({
        title: '권한 없음',
        message: 'platform admin만 프로젝트를 삭제할 수 있습니다.',
        color: 'yellow',
      })
      return
    }
    if (isSharedProject(selectedProject)) {
      notifications.show({
        title: '삭제 불가',
        message: '공용 프로젝트는 삭제할 수 없습니다.',
        color: 'yellow',
      })
      return
    }

    setDeletingProject(true)
    try {
      const deleted = await api.deleteProject(selectedProjectId)
      const projectRes = await api.getProjects()
      const visibleProjects = filterVisibleProjects(projectRes.items)
      const nextSelectedProjectId = visibleProjects[0]?.id ?? null

      setProjects(visibleProjects)
      setApplications([])
      setEnvironments([])
      setProjectPolicy(null)
      setProjectInsightMetrics([])
      setApplicationCatalogSignals({})
      setProjectDataLoaded(false)
      setTrackedChanges([])
      setSelectedChangeId(null)
      setSelectedAppId(null)
      setSelectedDeploymentId(null)
      setSelectedDeploymentDetail(null)
      setProjectSettingsOpened(false)
      setPendingProjectDelete(false)
      setProjectTab(defaultProjectTab())
      setActiveSection('projects')
      setSelectedProjectId(nextSelectedProjectId)

      notifications.show({
        title: '프로젝트 삭제 완료',
        message: `${deleted.name} 프로젝트를 삭제했습니다.`,
        color: 'green',
      })
    } catch (err) {
      const message =
        err instanceof ApiError
          ? err.code === 'PROJECT_DELETE_PROTECTED'
            ? '공용 프로젝트는 삭제할 수 없습니다.'
            : err.code === 'FORBIDDEN'
              ? 'platform admin만 프로젝트를 삭제할 수 있습니다.'
              : err.message
          : '프로젝트를 삭제하지 못했습니다.'

      notifications.show({
        title: '프로젝트 삭제 실패',
        message,
        color: 'red',
      })
    } finally {
      setDeletingProject(false)
    }
  }

  const handleCreateCluster = async () => {
    if (!isPlatformAdmin) {
      notifications.show({
        title: '권한 없음',
        message: 'platform admin만 클러스터를 생성할 수 있습니다.',
        color: 'yellow',
      })
      return
    }
    if (!clusterDraft.id.trim() || !clusterDraft.name.trim()) {
      notifications.show({
        title: '입력 확인 필요',
        message: '클러스터 ID와 이름을 입력하세요.',
        color: 'yellow',
      })
      return
    }

    setCreatingCluster(true)
    try {
      await api.createCluster({
        id: clusterDraft.id.trim(),
        name: clusterDraft.name.trim(),
        description: clusterDraft.description?.trim() || undefined,
        default: clusterDraft.default,
      })
      const clusterRes = await api.getClusters()
      setClusters(clusterRes.items)
      setClusterDraft({ id: '', name: '', description: '', default: false })
      if (activeSection === 'clusters') {
        await refreshAdminResourceOverview()
      }
      notifications.show({
        title: '클러스터 생성 완료',
        message: '클러스터 카탈로그를 갱신했습니다.',
        color: 'green',
      })
    } catch {
      notifications.show({
        title: '클러스터 생성 실패',
        message: '클러스터를 생성하지 못했습니다.',
        color: 'red',
      })
    } finally {
      setCreatingCluster(false)
    }
  }

  const rememberTrackedChange = useCallback((record: ChangeRecord) => {
    const nextTrackedChanges = upsertChangeRecord(readProjectTrackedChanges(record.projectId), record)
    writeProjectTrackedChanges(record.projectId, nextTrackedChanges)

    if (record.projectId === selectedProjectId) {
      setTrackedChanges(nextTrackedChanges)
      setSelectedChangeId(record.id)
    }
  }, [selectedProjectId])

  const handleCreateChange = async (request: CreateChangeRequest) => {
    if (!selectedProjectId) return
    if (!canDeployInProject) {
      notifications.show({
        title: '권한 없음',
        message: 'viewer 역할은 변경 요청 draft를 생성할 수 없습니다.',
        color: 'yellow',
      })
      throw new Error('permission denied')
    }
    setCreatingChange(true)
    try {
      const created = await api.createChange(selectedProjectId, request)
      rememberTrackedChange(created)
      notifications.show({
        title: 'Draft 생성 완료',
        message: `${created.summary} change를 추적 목록에 추가했습니다.`,
        color: 'green',
      })
    } catch {
      notifications.show({
        title: 'Draft 생성 실패',
        message: '변경 요청 draft를 생성하지 못했습니다.',
        color: 'red',
      })
      throw new Error('create change failed')
    } finally {
      setCreatingChange(false)
    }
  }

  const handleTrackChange = async (changeId: string) => {
    if (!selectedProjectId) return
    try {
      const record = await api.getChange(changeId)
      if (record.projectId !== selectedProjectId) {
        notifications.show({
          title: '프로젝트 불일치',
          message: '현재 선택한 프로젝트의 change만 이 화면에서 추적할 수 있습니다.',
          color: 'yellow',
        })
        return
      }
      rememberTrackedChange(record)
      notifications.show({
        title: '불러오기 완료',
        message: `${record.id} change를 추적 목록에 추가했습니다.`,
        color: 'green',
      })
    } catch {
      notifications.show({
        title: '불러오기 실패',
        message: '해당 change ID를 조회하지 못했습니다.',
        color: 'red',
      })
      throw new Error('track change failed')
    }
  }

  const applyChangeTransition = async (
    changeId: string,
    action: ChangeActionKind,
    transition: () => Promise<ChangeRecord>,
  ) => {
    setChangeActionLoading({ changeId, action })
    try {
      const updated = await transition()
      rememberTrackedChange(updated)
      if (updated.status === 'Merged' && selectedProjectId) {
        await refreshProjectData(selectedProjectId)
      }
      notifications.show({
        title: '변경 요청 갱신 완료',
        message: `${updated.id} 상태가 ${updated.status}로 변경되었습니다.`,
        color: 'green',
      })
    } catch {
      notifications.show({
        title: '변경 요청 처리 실패',
        message: '요청한 change 액션을 완료하지 못했습니다.',
        color: 'red',
      })
      throw new Error('change transition failed')
    } finally {
      setChangeActionLoading(null)
    }
  }

  const handleSubmitChange = async (changeId: string) => {
    await applyChangeTransition(changeId, 'submit', () => api.submitChange(changeId))
  }

  const handleApproveChange = async (changeId: string) => {
    await applyChangeTransition(changeId, 'approve', () => api.approveChange(changeId))
  }

  const handleMergeChange = async (changeId: string) => {
    await applyChangeTransition(changeId, 'merge', () => api.mergeChange(changeId))
  }

  const handlePromoteDeployment = async (deploymentId: string) => {
    if (!selectedAppId) return
    if (!canDeployInProject) {
      notifications.show({
        title: '권한 없음',
        message: 'viewer 역할은 카나리아 승격을 실행할 수 없습니다.',
        color: 'yellow',
      })
      return
    }
    setPromotingDeploymentId(deploymentId)
    try {
      await api.promoteDeployment(selectedAppId, deploymentId)
      notifications.show({
        title: '승격 완료',
        message: '선택한 카나리아 배포를 다음 단계로 승격했습니다.',
        color: 'green',
      })
      await fetchAppDetails(selectedAppId)
      await fetchSelectedDeploymentDetail(selectedAppId, deploymentId)
    } catch {
      notifications.show({
        title: '승격 실패',
        message: '선택한 배포를 승격하지 못했습니다.',
        color: 'red',
      })
    } finally {
      setPromotingDeploymentId(null)
    }
  }

  const handleSaveRollbackPolicy = async () => {
    if (!selectedAppId) return
    if (!canDeployInProject) {
      notifications.show({
        title: '권한 없음',
        message: 'viewer 역할은 롤백 정책을 저장할 수 없습니다.',
        color: 'yellow',
      })
      return
    }
    setSavingRollbackPolicy(true)
    try {
      const saved = await api.saveRollbackPolicy(selectedAppId, rollbackPolicyDraft)
      setAppDetails((current) => ({ ...current, rollbackPolicy: saved }))
      setRollbackPolicyDraft(saved)
      notifications.show({ title: '성공', message: '롤백 정책이 저장되었습니다.', color: 'green' })
    } catch {
      notifications.show({ title: '저장 실패', message: '롤백 정책을 저장하지 못했습니다.', color: 'red' })
    } finally {
      setSavingRollbackPolicy(false)
    }
  }

  const handleSaveApplicationResources = async () => {
    if (!selectedAppId) return
    if (!canAdminProject) {
      notifications.show({
        title: '권한 없음',
        message: '프로젝트 admin만 리소스 할당을 수정할 수 있습니다.',
        color: 'yellow',
      })
      return
    }

    setSavingApplicationResources(true)
    try {
      const saved = await api.patchApplication(selectedAppId, {
        resources: {
          requests: {
            cpu: defaultApplicationResources.requests?.cpu || '',
            memory: defaultApplicationResources.requests?.memory || '',
          },
          limits: {
            cpu: applicationResourcesDraft.limits?.cpu?.trim() || defaultApplicationResources.limits?.cpu || '',
            memory: applicationResourcesDraft.limits?.memory?.trim() || defaultApplicationResources.limits?.memory || '',
          },
        },
      })

      setApplications((current) =>
        current.map((application) =>
          application.id === saved.id
            ? {
                ...application,
                image: saved.image,
                deploymentStrategy: saved.deploymentStrategy as 'Rollout' | 'Canary',
                syncStatus: saved.syncStatus ?? application.syncStatus,
                resources: saved.resources,
                meshEnabled: saved.meshEnabled,
                loadBalancerEnabled: saved.loadBalancerEnabled,
              }
            : application,
        ),
      )
      setApplicationResourcesDraft(resolveApplicationResourcesDraft(saved.resources))
      notifications.show({
        title: '리소스 할당 저장 완료',
        message: '애플리케이션 기본 요청값과 상한선을 갱신했습니다.',
        color: 'green',
      })
      if (selectedProjectId) {
        await refreshProjectData(selectedProjectId)
      }
      await fetchAppDetails(selectedAppId)
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '리소스 할당을 저장하지 못했습니다.'
      notifications.show({
        title: '리소스 저장 실패',
        message,
        color: 'red',
      })
    } finally {
      setSavingApplicationResources(false)
    }
  }

  const handleSaveApplicationNetwork = async () => {
    if (!selectedAppId) return
    if (!canAdminProject) {
      notifications.show({
        title: '권한 없음',
        message: '프로젝트 admin만 네트워크 노출 정책을 수정할 수 있습니다.',
        color: 'yellow',
      })
      return
    }
    if (selectedApp?.deploymentStrategy === 'Canary' && applicationNetworkDraft.loadBalancerEnabled) {
      notifications.show({
        title: '설정 확인 필요',
        message: '카나리아 배포는 LoadBalancer 직접 노출을 함께 사용할 수 없습니다.',
        color: 'yellow',
      })
      return
    }

    setSavingApplicationNetwork(true)
    try {
      const saved = await api.patchApplication(selectedAppId, {
        meshEnabled: selectedApp?.meshEnabled ?? applicationNetworkDraft.meshEnabled,
        loadBalancerEnabled: applicationNetworkDraft.loadBalancerEnabled,
      })

      setApplications((current) =>
        current.map((application) =>
          application.id === saved.id
            ? {
                ...application,
                image: saved.image,
                deploymentStrategy: saved.deploymentStrategy as 'Rollout' | 'Canary',
                syncStatus: saved.syncStatus ?? application.syncStatus,
                resources: saved.resources,
                meshEnabled: saved.meshEnabled,
                loadBalancerEnabled: saved.loadBalancerEnabled,
              }
            : application,
        ),
      )
      setApplicationNetworkDraft(resolveApplicationNetworkDraft(saved))
      notifications.show({
        title: '트래픽 설정 저장 완료',
        message: 'LoadBalancer 노출 정책을 갱신했습니다.',
        color: 'green',
      })
      if (selectedProjectId) {
        await refreshProjectData(selectedProjectId)
      }
      await fetchAppDetails(selectedAppId)
    } catch (error) {
      const message = translateApplicationNetworkError(error)
      notifications.show({
        title: '트래픽 설정 저장 실패',
        message,
        color: 'red',
      })
    } finally {
      setSavingApplicationNetwork(false)
    }
  }

  const handleApplySecretBulkText = () => {
    const parsed = parseEnvEntries(secretBulkText)
    if (parsed.length === 0) {
      setSecretBulkMessage('.env 형식에서 읽을 수 있는 항목이 없습니다.')
      return
    }

    const existingKeys = new Set((applicationSecrets?.items ?? []).map((item) => item.key))
    setSecretValueDrafts((current) => {
      const next = { ...current }
      for (const entry of parsed) {
        if (existingKeys.has(entry.key)) {
          next[entry.key] = entry.value
        }
      }
      return next
    })
    setNewSecretRows((current) => {
      const byKey = new Map<string, string>()
      for (const row of current) {
        const key = row.key.trim()
        if (key && !existingKeys.has(key)) {
          byKey.set(key, row.value)
        }
      }
      for (const entry of parsed) {
        if (!existingKeys.has(entry.key)) {
          byKey.set(entry.key, entry.value)
        }
      }
      const rows = Array.from(byKey.entries()).map(([key, value]) => ({ key, value }))
      return [...rows, { key: '', value: '' }]
    })
    setSecretBulkMessage(`${parsed.length}개 항목을 편집 내용에 반영했습니다.`)
  }

  const handleSaveApplicationSecrets = async () => {
    if (!selectedAppId) return
    if (!canDeployInProject) {
      notifications.show({
        title: '권한 없음',
        message: 'viewer 역할은 환경 변수를 수정할 수 없습니다.',
        color: 'yellow',
      })
      return
    }

    const deleteSet = new Set(secretDeleteDrafts)
    const setByKey = new Map<string, string>()
    for (const [key, value] of Object.entries(secretValueDrafts)) {
      const normalizedKey = key.trim()
      if (normalizedKey && value !== '' && !deleteSet.has(normalizedKey)) {
        setByKey.set(normalizedKey, value)
      }
    }
    for (const row of newSecretRows) {
      const normalizedKey = row.key.trim()
      if (normalizedKey && row.value !== '' && !deleteSet.has(normalizedKey)) {
        setByKey.set(normalizedKey, row.value)
      }
    }

    const setEntries: SecretEntry[] = Array.from(setByKey.entries()).map(([key, value]) => ({ key, value }))
    const deleteEntries = Array.from(deleteSet)
    if (setEntries.length === 0 && deleteEntries.length === 0) {
      notifications.show({
        title: '변경 없음',
        message: '교체하거나 삭제할 환경 변수를 먼저 지정하세요.',
        color: 'gray',
      })
      return
    }

    setSavingApplicationSecrets(true)
    try {
      const response = await api.updateApplicationSecrets(selectedAppId, {
        set: setEntries,
        delete: deleteEntries,
      })
      resetSecretEditor(response)
      await refreshApplicationSecrets(selectedAppId)
      notifications.show({
        title: '환경 변수 저장 완료',
        message: 'Vault 값과 GitOps Secret 연결 상태를 갱신했습니다. 실행 중인 Pod에는 다음 rollout부터 반영됩니다.',
        color: 'green',
      })
      await fetchAppDetails(selectedAppId)
    } catch (error) {
      notifications.show({
        title: '환경 변수 저장 실패',
        message: translateApplicationSecretsError(error),
        color: 'red',
      })
    } finally {
      setSavingApplicationSecrets(false)
    }
  }

  const handleRestoreApplicationSecretVersion = async (version: number) => {
    if (!selectedAppId) return
    if (!canDeployInProject) {
      notifications.show({
        title: '권한 없음',
        message: 'viewer 역할은 환경 변수 버전을 복원할 수 없습니다.',
        color: 'yellow',
      })
      return
    }

    setRestoringSecretVersion(version)
    try {
      const response = await api.restoreApplicationSecretVersion(selectedAppId, version)
      resetSecretEditor(response)
      await refreshApplicationSecrets(selectedAppId)
      notifications.show({
        title: '환경 변수 버전 복원 완료',
        message: `Vault version ${version} 값으로 새 버전을 만들었습니다. 실행 중인 Pod에는 다음 rollout부터 반영됩니다.`,
        color: 'green',
      })
      await fetchAppDetails(selectedAppId)
    } catch (error) {
      notifications.show({
        title: '환경 변수 버전 복원 실패',
        message: translateApplicationSecretsError(error),
        color: 'red',
      })
    } finally {
      setRestoringSecretVersion(null)
    }
  }

  const handleSaveRepositoryPollInterval = async () => {
    if (!selectedAppId) return
    if (!canDeployInProject) {
      notifications.show({
        title: '권한 없음',
        message: 'viewer 역할은 저장소 polling 주기를 수정할 수 없습니다.',
        color: 'yellow',
      })
      return
    }
    if (!repositoryPoll?.enabled) {
      notifications.show({
        title: '설정 불가',
        message: '이 애플리케이션은 저장소 polling 대상이 아닙니다.',
        color: 'yellow',
      })
      return
    }

    const nextIntervalSeconds = Number(repositoryPollIntervalDraft)
    if (!Number.isFinite(nextIntervalSeconds) || ![60, 300, 600].includes(nextIntervalSeconds)) {
      notifications.show({
        title: '주기 확인 필요',
        message: 'polling 주기는 1분, 5분, 10분 중 하나만 선택할 수 있습니다.',
        color: 'yellow',
      })
      return
    }
    if (repositoryPoll.intervalSeconds === nextIntervalSeconds) {
      notifications.show({
        title: '변경 없음',
        message: `저장소 확인 주기가 이미 ${formatRepositoryPollInterval(nextIntervalSeconds)}입니다.`,
        color: 'gray',
      })
      return
    }

    setSavingRepositoryPollInterval(true)
    try {
      await api.patchApplication(selectedAppId, {
        repositoryPollIntervalSeconds: nextIntervalSeconds,
      })
      notifications.show({
        title: 'Polling 주기 저장 완료',
        message: `저장소 확인 주기를 ${formatRepositoryPollInterval(nextIntervalSeconds)}로 변경했습니다.`,
        color: 'green',
      })
      await fetchAppDetails(selectedAppId)
    } catch (error) {
      notifications.show({
        title: 'Polling 주기 저장 실패',
        message: translateRepositoryPollControlError(error, 'polling 주기를 저장하지 못했습니다.'),
        color: 'red',
      })
    } finally {
      setSavingRepositoryPollInterval(false)
    }
  }

  const handleSyncRepositoryNow = async () => {
    if (!selectedAppId) return
    if (!canDeployInProject) {
      notifications.show({
        title: '권한 없음',
        message: 'viewer 역할은 수동 sync를 실행할 수 없습니다.',
        color: 'yellow',
      })
      return
    }
    if (!repositoryPoll?.enabled) {
      notifications.show({
        title: '실행 불가',
        message: '이 애플리케이션은 저장소 polling 대상이 아닙니다.',
        color: 'yellow',
      })
      return
    }

    setSyncingRepositoryPoll(true)
    try {
      const response = await api.syncApplicationRepository(selectedAppId)
      notifications.show({
        title: '저장소 sync 완료',
        message: response.message,
        color: 'green',
      })
      if (selectedProjectId) {
        await refreshProjectData(selectedProjectId)
      }
      await fetchAppDetails(selectedAppId)
    } catch (error) {
      notifications.show({
        title: '저장소 sync 실패',
        message: translateRepositoryPollControlError(error, '저장소 sync를 실행하지 못했습니다.'),
        color: 'red',
      })
    } finally {
      setSyncingRepositoryPoll(false)
    }
  }

  const handleAbortLatestDeployment = async () => {
    const latestDeployment = appDetails.deployments[0]
    if (!selectedAppId || !latestDeployment) return
    if (!canDeployInProject) {
      notifications.show({
        title: '권한 없음',
        message: 'viewer 역할은 긴급 조치를 실행할 수 없습니다.',
        color: 'yellow',
      })
      return
    }
    setEmergencyActionLoading('abort')
    try {
      await api.abortDeployment(selectedAppId, latestDeployment.deploymentId)
      notifications.show({ title: '성공', message: '현재 배포를 중단했습니다.', color: 'green' })
      setPendingDangerAction(null)
      await fetchAppDetails(selectedAppId)
    } catch {
      notifications.show({ title: '중단 실패', message: '현재 배포를 중단하지 못했습니다.', color: 'red' })
    } finally {
      setEmergencyActionLoading(null)
    }
  }

  const handleRollbackToPreviousRevision = async () => {
    const previousDeployment = appDetails.deployments[1]
    if (!selectedAppId || !previousDeployment) return
    if (!canDeployInProject) {
      notifications.show({
        title: '권한 없음',
        message: 'viewer 역할은 긴급 조치를 실행할 수 없습니다.',
        color: 'yellow',
      })
      return
    }
    setEmergencyActionLoading('rollback')
    try {
      await api.createDeployment(selectedAppId, previousDeployment.imageTag, previousDeployment.environment)
      notifications.show({ title: '성공', message: '직전 버전으로 롤백을 요청했습니다.', color: 'green' })
      setPendingDangerAction(null)
      await fetchAppDetails(selectedAppId)
    } catch {
      notifications.show({ title: '롤백 실패', message: '직전 버전 롤백을 요청하지 못했습니다.', color: 'red' })
    } finally {
      setEmergencyActionLoading(null)
    }
  }

  const handleArchiveApplication = async () => {
    if (!selectedAppId || !selectedProjectId) return
    if (!canAdminProject) {
      notifications.show({
        title: '권한 없음',
        message: '애플리케이션 보관은 프로젝트 관리자만 실행할 수 있습니다.',
        color: 'yellow',
      })
      return
    }
    setLifecycleActionLoading('archive')
    try {
      await api.archiveApplication(selectedAppId)
      notifications.show({
        title: '애플리케이션 보관 완료',
        message: '애플리케이션을 archive 상태로 전환했습니다.',
        color: 'green',
      })
      setPendingLifecycleAction(null)
      setSelectedAppId(null)
      await refreshProjectData(selectedProjectId)
    } catch {
      notifications.show({
        title: '보관 실패',
        message: '애플리케이션을 보관 처리하지 못했습니다.',
        color: 'red',
      })
    } finally {
      setLifecycleActionLoading(null)
    }
  }

  const handleDeleteApplication = async () => {
    if (!selectedAppId || !selectedProjectId) return
    if (!canAdminProject) {
      notifications.show({
        title: '권한 없음',
        message: '애플리케이션 삭제는 프로젝트 관리자만 실행할 수 있습니다.',
        color: 'yellow',
      })
      return
    }
    setLifecycleActionLoading('delete')
    try {
      await api.deleteApplication(selectedAppId)
      notifications.show({
        title: '애플리케이션 삭제 완료',
        message: '애플리케이션과 연결된 manifest를 제거했습니다.',
        color: 'green',
      })
      setPendingLifecycleAction(null)
      setSelectedAppId(null)
      await refreshProjectData(selectedProjectId)
    } catch {
      notifications.show({
        title: '삭제 실패',
        message: '애플리케이션을 삭제하지 못했습니다.',
        color: 'red',
      })
    } finally {
      setLifecycleActionLoading(null)
    }
  }

  const sectionMeta = useMemo(() => {
    switch (activeSection) {
      case 'changes':
        return {
          breadcrumbs: ['AODS', '변경 요청'],
          title: '변경 요청',
          description: '변경 draft, 제출, 승인, 반영 흐름을 관리합니다.',
        }
      case 'clusters':
        return {
          breadcrumbs: ['AODS', '클러스터'],
          title: '클러스터',
          description: '배포 대상 클러스터 카탈로그와 기본 상태를 확인합니다.',
        }
      case 'me':
        return {
          breadcrumbs: ['AODS', '내 정보'],
          title: '내 정보',
          description: '현재 로그인 사용자와 접근 가능한 프로젝트를 확인합니다.',
        }
      default:
        return {
          breadcrumbs: ['AODS', '프로젝트', selectedProject?.name ?? '선택 없음'],
          title: selectedProject?.name ?? '프로젝트',
          description:
            selectedProject?.description || '프로젝트 애플리케이션과 모니터링, 운영 규칙을 관리합니다.',
        }
    }
  }, [activeSection, selectedProject])

  const projectCatalog = (
    null
  )

  const projectMonitoringContent = (
    <Stack gap="lg">
      <Group justify="space-between" align="center">
        <div>
          <Text className={classes.sectionEyebrow}>프로젝트 모니터링</Text>
          <Text fw={800}>CPU, 메모리, 트래픽 기준으로 현재 부하 확인</Text>
          <Text size="sm" c="dimmed" mt={6}>
            최근 15분 기준으로 프로젝트 전체와 애플리케이션별 실측 지표를 모아 보여줍니다.
          </Text>
        </div>
        <Badge color="gray" variant="light" radius="sm">
          최근 15분 기준
        </Badge>
      </Group>

      {!projectDataLoaded ? (
        <StatePanel
          kind="loading"
          title="모니터링 지표를 불러오는 중"
          description="프로젝트 전체와 애플리케이션별 실측 지표를 수집하고 있습니다."
        />
      ) : (
        <>
          {projectInsightMetrics.length > 0 ? (
            <SimpleGrid cols={{ base: 1, sm: 2, xl: 3 }} spacing="lg">
              <MetricCard
                label="프로젝트 CPU"
                value={formatMetricSeriesValue(projectInsightMetrics, 'cpu_usage')}
                unit={metricSeriesUnit(projectInsightMetrics, 'cpu_usage')}
                points={findMetricSeries(projectInsightMetrics, 'cpu_usage')?.points}
                color="#1d66d6"
              />
              <MetricCard
                label="프로젝트 메모리"
                value={formatMetricSeriesValue(projectInsightMetrics, 'memory_usage')}
                unit={metricSeriesUnit(projectInsightMetrics, 'memory_usage')}
                points={findMetricSeries(projectInsightMetrics, 'memory_usage')?.points}
                color="#0b3d7f"
              />
              <MetricCard
                label="프로젝트 트래픽"
                value={formatMetricSeriesValue(projectInsightMetrics, 'request_rate')}
                unit={metricSeriesUnit(projectInsightMetrics, 'request_rate')}
                points={findMetricSeries(projectInsightMetrics, 'request_rate')?.points}
                color="#10b981"
              />
            </SimpleGrid>
          ) : (
            <StatePanel
              kind={projectDataWarnings.length > 0 ? 'partial' : 'empty'}
              title="프로젝트 집계 지표가 없습니다"
              description={
                projectDataWarnings.length > 0
                  ? projectDataWarnings.join(' ')
                  : '메트릭 연동이 설정되지 않았거나 아직 수집된 값이 없습니다.'
              }
            />
          )}

          {applications.length > 0 ? (
            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="lg">
              {applications.map((app) => {
                const signal = applicationCatalogSignals[app.id]
                const syncIssue = applicationSyncIssue(app, signal)

                return (
                  <div key={app.id} className={classes.surfaceCard}>
                    <Stack gap="md">
                      <Group justify="space-between" align="start">
                        <div>
                          <Text className={classes.cardTitle}>{app.name}</Text>
                          <Text className={classes.cardMeta}>애플리케이션별 실측 지표</Text>
                        </div>
                        <SyncStatusBadge status={app.syncStatus} />
                      </Group>

                      {syncIssue ? (
                        <Alert color={applicationSyncIssueColor(app.syncStatus)} variant="light" icon={<IconAlertTriangle size={16} />}>
                          <Text size="sm" fw={800}>{app.syncStatus === 'Unknown' ? 'Sync 확인 불가 사유' : 'Sync 상태 사유'}</Text>
                          <Text size="sm" style={{ wordBreak: 'break-word' }}>{syncIssue}</Text>
                        </Alert>
                      ) : null}

                      {hasApplicationMetrics(signal) ? (
                        <SimpleGrid cols={{ base: 1, sm: 3 }} spacing="sm">
                          <div className={classes.applicationCatalogMetric}>
                            <Group justify="space-between" align="center">
                              <Group gap={6}>
                                <IconCpu size={14} color="#64748b" />
                                <Text size="xs" fw={700} c="dimmed">CPU</Text>
                              </Group>
                              <Text size="xs" fw={800}>{formatApplicationMetricSummary(signal, 'cpu_usage')}</Text>
                            </Group>
                          </div>
                          <div className={classes.applicationCatalogMetric}>
                            <Group justify="space-between" align="center">
                              <Group gap={6}>
                                <IconDatabase size={14} color="#64748b" />
                                <Text size="xs" fw={700} c="dimmed">메모리</Text>
                              </Group>
                              <Text size="xs" fw={800}>{formatApplicationMetricSummary(signal, 'memory_usage')}</Text>
                            </Group>
                          </div>
                          <div className={classes.applicationCatalogMetric}>
                            <Group justify="space-between" align="center">
                              <Group gap={6}>
                                <IconBolt size={14} color="#64748b" />
                                <Text size="xs" fw={700} c="dimmed">트래픽</Text>
                              </Group>
                              <Text size="xs" fw={800}>{formatApplicationMetricSummary(signal, 'request_rate')}</Text>
                            </Group>
                          </div>
                        </SimpleGrid>
                      ) : (
                        <Group gap="xs">
                          <Badge color={applicationMetricBadgeColor(signal)} variant="light" radius="sm">
                            {applicationMetricBadgeLabel(signal)}
                          </Badge>
                          <Text size="sm" c="dimmed">
                            {applicationMetricHelperText(signal)}
                          </Text>
                        </Group>
                      )}

                      <Group justify="space-between" align="center" className={classes.applicationCatalogFooter}>
                        <Text size="sm" fw={700} c="lagoon.7">
                          상세 모니터링이 필요하면 운영 센터로 이동
                        </Text>
                        <Button size="xs" variant="light" color="lagoon.6" onClick={() => setSelectedAppId(app.id)}>
                          운영 센터 열기
                        </Button>
                      </Group>
                    </Stack>
                  </div>
                )
              })}
            </SimpleGrid>
          ) : (
            <StatePanel
              kind="empty"
              title="모니터링할 애플리케이션이 없습니다"
              description="애플리케이션이 생성되면 CPU, 메모리, 트래픽 지표를 이 탭에서 모아 볼 수 있습니다."
            />
          )}
        </>
      )}
    </Stack>
  )

  const projectApplicationsContent = (
    <Stack gap="lg">
      <Group justify="space-between" align="center">
        <div>
          <Text className={classes.sectionEyebrow}>애플리케이션 운영 목록</Text>
          <Text fw={800}>애플리케이션별 현재 운영 상태</Text>
          <Text size="sm" c="dimmed" mt={6}>
            앱 개수, sync 상태, 최근 배포 태그와 최근 상태를 한눈에 보고 필요한 애플리케이션만 상세 운영 화면으로 들어갈 수 있습니다.
          </Text>
        </div>
        <Button
          leftSection={<IconPlus size={16} />}
          color="lagoon.6"
          radius="md"
          onClick={() => setWizardOpened(true)}
          disabled={!canDeployInProject}
        >
          새 애플리케이션
        </Button>
      </Group>

      {projectDataLoaded ? (
        <SimpleGrid cols={{ base: 1, sm: 2, xl: 5 }} spacing="sm">
          <div className={classes.statBadge}>
            <Text className={classes.statLabel}>전체 애플리케이션</Text>
            <Text className={classes.statValue}>{applications.length}</Text>
          </div>
          <div className={classes.statBadge}>
            <Text className={classes.statLabel} style={{ color: '#15803d' }}>정상</Text>
            <Text className={classes.statValue}>{countApplicationsByStatus(applications, 'Synced')}</Text>
          </div>
          <div className={classes.statBadge}>
            <Text className={classes.statLabel} style={{ color: '#ca8a04' }}>동기화 중</Text>
            <Text className={classes.statValue}>{countApplicationsByStatus(applications, 'Syncing')}</Text>
          </div>
          <div className={classes.statBadge}>
            <Text className={classes.statLabel} style={{ color: '#dc2626' }}>주의 필요</Text>
            <Text className={classes.statValue}>{countApplicationsByStatus(applications, 'Degraded')}</Text>
          </div>
          <div className={classes.statBadge}>
            <Text className={classes.statLabel}>확인 불가</Text>
            <Text className={classes.statValue}>{countApplicationsByStatus(applications, 'Unknown')}</Text>
          </div>
        </SimpleGrid>
      ) : null}

      {!projectDataLoaded ? (
        <StatePanel
          kind="loading"
          title="애플리케이션 목록을 불러오는 중"
          description="프로젝트의 앱 카탈로그와 배포 전략 정보를 가져오고 있습니다."
        />
      ) : applications.length > 0 ? (
        <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="lg">
          {applications.map((app) => {
            const signal = applicationCatalogSignals[app.id]
            const syncIssue = applicationSyncIssue(app, signal)
            return (
            <UnstyledButton
              key={app.id}
              onClick={() => setSelectedAppId(app.id)}
              className={`${classes.appItem} ${classes.applicationCatalogCard} ${selectedAppId === app.id ? classes.selectCardActive : ''}`}
            >
              <div className={classes.surfaceCard}>
                <Stack gap="md">
                  <Group justify="space-between" align="start">
                    <Stack gap={2}>
                      <Text className={classes.cardTitle}>{app.name}</Text>
                      <Text className={classes.cardMeta}>{app.image}</Text>
                    </Stack>
                    <Stack gap="xs" align="end">
                      <SyncStatusBadge status={app.syncStatus} />
                      <Badge color="gray" variant="outline" radius="sm">
                        {formatLatestDeploymentLabel(signal)}
                      </Badge>
                    </Stack>
                  </Group>

                  <Text size="sm" c="dimmed" className={classes.applicationCatalogSummary}>
                    {buildApplicationCatalogSummary(app, selectedProject?.namespace || 'default', signal)}
                  </Text>

                  {syncIssue ? (
                    <Alert color={applicationSyncIssueColor(app.syncStatus)} variant="light" icon={<IconAlertTriangle size={16} />}>
                      <Text size="sm" fw={800}>{app.syncStatus === 'Unknown' ? 'Sync 확인 불가 사유' : 'Sync 상태 사유'}</Text>
                      <Text size="sm" style={{ wordBreak: 'break-word' }}>{syncIssue}</Text>
                    </Alert>
                  ) : null}

                  <Group gap="xs">
                    <Badge color={app.loadBalancerEnabled ? 'blue' : 'gray'} variant="light" radius="sm">
                      {formatLoadBalancerBadgeLabel(app.loadBalancerEnabled)}
                    </Badge>
                  </Group>

                  <SimpleGrid cols={{ base: 2, lg: 4 }} spacing="sm">
                    <div className={classes.applicationCatalogStat}>
                      <Text size="xs" c="dimmed" fw={700}>배포 전략</Text>
                      <Text size="sm" fw={800} c="lagoon.9">
                        {app.deploymentStrategy === 'Canary' ? '카나리아' : '롤아웃'}
                      </Text>
                    </div>
                    <div className={classes.applicationCatalogStat}>
                      <Text size="xs" c="dimmed" fw={700}>네임스페이스</Text>
                      <Text size="sm" fw={800} c="lagoon.9">{selectedProject?.namespace || 'default'}</Text>
                    </div>
                    <div className={classes.applicationCatalogStat}>
                      <Text size="xs" c="dimmed" fw={700}>최근 배포</Text>
                      <Text size="sm" fw={800} c="lagoon.9">{formatLatestDeploymentLabel(signal)}</Text>
                    </div>
                    <div className={classes.applicationCatalogStat}>
                      <Text size="xs" c="dimmed" fw={700}>최근 상태</Text>
                      <Text size="sm" fw={800} c="lagoon.9">{formatLatestDeploymentStatus(signal)}</Text>
                    </div>
                  </SimpleGrid>

                  <Group justify="space-between" align="center" className={classes.applicationCatalogFooter}>
                    <Text size="sm" fw={700} c="lagoon.7">
                      클릭해서 상세 운영 보기
                    </Text>
                    <IconChevronRight size={18} color="#1d66d6" />
                  </Group>
                </Stack>
              </div>
            </UnstyledButton>
          )})}
        </SimpleGrid>
      ) : (
        <StatePanel
          kind="empty"
          title="애플리케이션이 없습니다"
          description={
            canDeployInProject
              ? '아직 생성된 애플리케이션이 없습니다. 새 애플리케이션을 생성해 운영 흐름을 시작하세요.'
              : '표시할 애플리케이션이 없습니다. viewer 역할은 새 애플리케이션을 직접 만들 수 없습니다.'
          }
        />
      )}
    </Stack>
  )

  const projectRulesContent = (
    <Stack gap="lg">
      <Group justify="space-between" align="center">
        <div>
          <Text className={classes.sectionEyebrow}>프로젝트 운영 규칙</Text>
          <Text fw={800}>프로젝트 운영 규칙</Text>
          <Text size="sm" c="dimmed" mt={6}>
            각 항목 옆 `?` 아이콘에 마우스를 올리면 해당 값이 무엇을 뜻하는지 바로 설명을 볼 수 있습니다.
          </Text>
        </div>
        <Button variant="light" color="lagoon.6" onClick={() => setProjectSettingsOpened(true)}>
          프로젝트 설정 열기
        </Button>
      </Group>
      {!canAdminProject ? (
        <StatePanel
          kind="forbidden"
          title="프로젝트 규칙은 관리자 승인 범위입니다"
          description="현재 화면에서는 규칙을 조회할 수 있지만, 정책 변경 승인과 넓은 운영 권한은 admin 역할에서만 처리됩니다."
        />
      ) : null}
      {projectPolicy === null ? (
        <StatePanel
          kind={projectDataWarnings.length > 0 ? 'partial' : 'loading'}
          title={projectDataWarnings.length > 0 ? '프로젝트 정책을 완전히 받지 못했습니다' : '프로젝트 정책을 불러오는 중'}
          description={
            projectDataWarnings.length > 0
              ? projectDataWarnings.join(' ')
              : '운영 환경, 허용 전략, 클러스터 대상 정책 정보를 수집하고 있습니다.'
          }
        />
      ) : null}
      <ProjectSettingsPanel
        key={[selectedProject?.id ?? 'no-project', JSON.stringify(projectPolicy ?? null)].join(':')}
        project={selectedProject}
        environments={environments}
        projectPolicy={projectPolicy}
        canEditPolicies={canAdminProject}
        savingPolicies={savingProjectPolicy}
        onSavePolicies={(policy) => void handleUpdateProjectPolicy(policy)}
        applicationCount={applications.length}
        canDeleteProject={canDeleteProject}
        isProtectedProject={isProtectedProject}
        deletingProject={deletingProject}
        pendingProjectDelete={pendingProjectDelete}
        onRequestProjectDelete={() => setPendingProjectDelete(true)}
        onCancelProjectDelete={() => setPendingProjectDelete(false)}
        onConfirmProjectDelete={() => void handleDeleteProject()}
      />
    </Stack>
  )

  const changesSectionContent = (
    <ChangesWorkspace
      key={selectedProjectId ?? 'no-project'}
      project={selectedProject}
      currentUser={currentUser}
      applications={applications}
      environments={environments}
      changes={trackedChanges}
      selectedChangeId={selectedChangeId}
      onSelectChange={setSelectedChangeId}
      onCreateChange={handleCreateChange}
      onTrackChange={handleTrackChange}
      onRefreshChanges={async () => {
        if (selectedProjectId) {
          await refreshTrackedChanges(selectedProjectId)
        }
      }}
      onSubmitChange={handleSubmitChange}
      onApproveChange={handleApproveChange}
      onMergeChange={handleMergeChange}
      creatingChange={creatingChange}
      refreshingChanges={refreshingChanges}
      actionLoading={changeActionLoading}
      onOpenProjectChanges={() => {
        setActiveSection('projects')
        setProjectTab(defaultProjectTab())
      }}
    />
  )

  const clusterActions = isPlatformAdmin ? (
    <Group gap="xs">
      <Button
        size="xs"
        variant="default"
        color="gray"
        leftSection={<IconRefresh size={14} />}
        loading={adminResourceOverviewLoading}
        onClick={() => {
          void refreshAdminResourceOverview()
        }}
      >
        리소스 새로고침
      </Button>
      <Button
        size="xs"
        variant={clusterComposerOpen ? 'default' : 'light'}
        color="lagoon.6"
        onClick={() => setClusterComposerOpen((current) => !current)}
      >
        새 클러스터
      </Button>
    </Group>
  ) : undefined

  const clusterCreationPanel = clusterComposerOpen ? (
    <div className={classes.surfaceCard}>
      <Stack gap="md">
        <div>
          <Text fw={800}>클러스터 생성</Text>
          <Text size="sm" c="dimmed" mt={4}>
            platform admin만 cluster catalog와 bootstrap scaffold를 추가할 수 있습니다.
          </Text>
        </div>
        <SimpleGrid cols={{ base: 1, md: 3 }} spacing="md">
          <TextInput
            label="클러스터 ID"
            placeholder="예: live"
            value={clusterDraft.id}
            onChange={(event) => {
              const value = event.currentTarget.value
              setClusterDraft((current) => ({ ...current, id: value }))
            }}
          />
          <TextInput
            label="클러스터 이름"
            placeholder="예: Live Cluster"
            value={clusterDraft.name}
            onChange={(event) => {
              const value = event.currentTarget.value
              setClusterDraft((current) => ({ ...current, name: value }))
            }}
          />
          <TextInput
            label="설명"
            placeholder="선택 사항"
            value={clusterDraft.description ?? ''}
            onChange={(event) => {
              const value = event.currentTarget.value
              setClusterDraft((current) => ({ ...current, description: value }))
            }}
          />
        </SimpleGrid>
        <Group justify="space-between">
          <Switch
            label="기본 클러스터로 지정"
            checked={clusterDraft.default ?? false}
            onChange={(event) => {
              const checked = event.currentTarget.checked
              setClusterDraft((current) => ({ ...current, default: checked }))
            }}
          />
          <Group>
            <Button color="lagoon.6" loading={creatingCluster} onClick={handleCreateCluster}>
              클러스터 생성
            </Button>
            <Button variant="default" onClick={() => setClusterComposerOpen(false)}>
              닫기
            </Button>
          </Group>
        </Group>
      </Stack>
    </div>
  ) : null

  const sectionContent = (() => {
    switch (activeSection) {
      case 'changes':
        return changesSectionContent
      case 'clusters':
        if (!isPlatformAdmin) {
          return (
            <StatePanel
              kind="forbidden"
              title="platform admin 전용 화면입니다"
              description="클러스터 카탈로그와 전체 리소스 효율 화면은 platform admin 권한에서만 접근할 수 있습니다."
            />
          )
        }
        return (
          <ClustersPage
            clusters={clusters}
            loading={loading}
            errorMessage={bootstrapWarnings.find((message) => message.includes('클러스터')) ?? null}
            showAdminOverview={isPlatformAdmin}
            adminOverview={adminResourceOverview}
            adminOverviewLoading={adminResourceOverviewLoading}
            adminOverviewError={adminResourceOverviewError}
            actions={clusterActions}
            creationPanel={clusterCreationPanel}
          />
        )
      case 'me':
        return (
          <MePage
            user={currentUser}
            projects={projects}
            loading={loading}
            errorMessage={bootstrapWarnings.find((message) => message.includes('사용자')) ?? null}
          />
        )
      default:
        return (
          <Stack gap="lg">
            {bootstrapWarnings.length > 0 ? (
              <StatePanel
                kind="partial"
                title="초기 프로젝트 컨텍스트를 일부만 불러왔습니다"
                description={bootstrapWarnings.join(' ')}
              />
            ) : null}
            <ProjectsWorkspace
              projectName={selectedProject?.name}
              projectDescription={selectedProject?.description}
              projectNamespace={selectedProject?.namespace}
              projectRole={selectedProject ? formatProjectRole(selectedProject.role) : undefined}
              projectNotice={projectNotice}
              projectCatalog={projectCatalog}
              projectTab={projectTab}
              onProjectTabChange={setProjectTab}
              applications={projectApplicationsContent}
              monitoring={projectMonitoringContent}
              rules={projectRulesContent}
            />
          </Stack>
        )
    }
  })()

  const latestDeployment = appDetails.deployments[0]
  const previousDeployment = appDetails.deployments[1]
  const latestEvent = appDetails.events[0]
  const repositoryPoll = appDetails.syncStatus?.repositoryPoll ?? null
  const repositoryPollIntervalChanged = repositoryPoll?.enabled
    ? String(repositoryPoll.intervalSeconds) !== repositoryPollIntervalDraft
    : false
  const latestFailedDeployment = appDetails.deployments.find(
    (deployment) => deployment.status === 'Aborted' || deployment.status === 'Failed',
  )
  const logTargetsWarning = appDetailWarnings.find((warning) => warning.startsWith('로그 대상: '))
  const liveLogText = liveLogEvents.map((event) => event.rawLine || event.message).join('\n')
  const selectedLogResourceStatus = selectedLogContainer?.resourceStatus ?? null
  const logPodOptions = (appDetails.logTargets?.items ?? []).map((target) => ({
    value: target.podName,
    label: target.phase ? `${target.podName} · ${target.phase}` : target.podName,
  }))
  const logContainerOptions = (selectedLogTarget?.containers ?? []).map((container) => ({
    value: container.name,
    label: container.default ? `${container.name} · 기본` : container.name,
  }))

  useEffect(() => {
    const serverValue = repositoryPoll?.enabled && repositoryPoll.intervalSeconds > 0
      ? String(repositoryPoll.intervalSeconds)
      : '300'
    if (!selectedAppId) {
      setRepositoryPollIntervalDraft('300')
      previousRepositoryPollAppRef.current = null
      previousRepositoryPollServerValueRef.current = '300'
      return
    }
    if (selectedAppId !== previousRepositoryPollAppRef.current) {
      previousRepositoryPollAppRef.current = selectedAppId
      previousRepositoryPollServerValueRef.current = serverValue
      setRepositoryPollIntervalDraft(serverValue)
      return
    }

    if (repositoryPollIntervalDraft === previousRepositoryPollServerValueRef.current && repositoryPollIntervalDraft !== serverValue) {
      setRepositoryPollIntervalDraft(serverValue)
    }
    previousRepositoryPollServerValueRef.current = serverValue
  }, [repositoryPoll?.enabled, repositoryPoll?.intervalSeconds, repositoryPollIntervalDraft, selectedAppId])

  if (!isLoggedIn) return <LoginForm oidcEnabled={oidcAuthEnabled} allowEmergencyLogin={emergencyLoginEnabled} onLogin={handleLogin} onEmergencyLogin={handleEmergencyLogin} />

  if (loading) return (
    <div style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <Loader size="xl" color="lagoon.6" />
    </div>
  )

  return (
    <>
      <Modal
        opened={loadBalancerConfirmOpened}
        onClose={() => {
          setLoadBalancerConfirmOpened(false)
          setLoadBalancerConfirmAcknowledged(false)
        }}
        centered
        radius="lg"
        size="lg"
        title="LoadBalancer 노출 요청 확인"
      >
        <Stack gap="md">
          <Text size="sm" c="dimmed">
            이 설정은 외부 공개 완료가 아니라 `Service type=LoadBalancer` 요청을 저장하는 단계입니다. 이후 floating IP와 외부 라우팅 연결은 AODS 밖의 네트워크 계층에서 이어집니다.
          </Text>
          <Group gap="xs">
            <Badge color="orange" variant="light" radius="sm">비용 영향</Badge>
            <Badge color="blue" variant="light" radius="sm">외부 연결</Badge>
            <Badge color="gray" variant="light" radius="sm">관리자 확인</Badge>
          </Group>
          <Alert color="orange" radius="md" icon={<IconAlertTriangle size={16} />}>
            저장 후 바로 공인 접속이 보장되지는 않습니다. 클러스터 반영, LoadBalancer 준비, floating IP 연결 순서로 후속 확인이 필요합니다.
          </Alert>
          <Checkbox
            checked={loadBalancerConfirmAcknowledged}
            onChange={(event) => setLoadBalancerConfirmAcknowledged(event.currentTarget.checked)}
            label="비용과 외부 노출 영향, floating IP 후속 확인이 필요하다는 점을 확인했습니다."
          />
          <Group justify="flex-end">
            <Button
              variant="subtle"
              color="gray"
              onClick={() => {
                setLoadBalancerConfirmOpened(false)
                setLoadBalancerConfirmAcknowledged(false)
              }}
            >
              취소
            </Button>
            <Button
              color="lagoon.6"
              disabled={!loadBalancerConfirmAcknowledged}
              onClick={confirmLoadBalancerDraft}
            >
              LB 요청 켜기
            </Button>
          </Group>
        </Stack>
      </Modal>

      <PortalShell
        activeSection={activeSection}
        onSectionChange={setActiveSection}
        visibleSections={visibleGlobalSections}
        breadcrumbs={sectionMeta.breadcrumbs}
        title={sectionMeta.title}
        description={sectionMeta.description}
        topBarActions={
          <Group gap="sm">
            {activeSection === 'projects' && selectedProject ? (
              <Button variant="light" color="gray" size="xs" onClick={() => setProjectSettingsOpened(true)}>
                프로젝트 설정
              </Button>
            ) : null}
            <Button
              variant="light"
              color="gray"
              size="xs"
              leftSection={<IconArrowLeft size={14} />}
              onClick={() => void handleLogout()}
            >
              로그아웃
            </Button>
          </Group>
        }
        metaBadges={
          activeSection === 'projects' && selectedProject ? (
            <>
              <Badge color="lagoon.6" variant="light" radius="sm">
                {selectedProject.namespace.toUpperCase()}
              </Badge>
              <Badge color="gray" variant="light" radius="sm">
                {selectedProject.id}
              </Badge>
            </>
          ) : undefined
        }
        userLabel={currentUser?.displayName || currentUser?.username || '관리자'}
        roleLabel={activeSection === 'projects' && selectedProject ? formatProjectRole(selectedProject.role) : undefined}
        projects={projects.map((project) => ({
          id: project.id,
          name: project.name,
          namespace: project.namespace,
          role: formatProjectRole(project.role),
        }))}
        selectedProjectId={selectedProjectId}
        onProjectSelect={(projectId) => {
          setSelectedProjectId(projectId)
          setProjectTab(defaultProjectTab())
          setActiveSection('projects')
        }}
        canCreateProject={showProjectComposer && isPlatformAdmin}
        onCreateProject={() => {
          setActiveSection('projects')
          setProjectTab(defaultProjectTab())
          setProjectComposerOpen(true)
        }}
      >
        <div className={classes.workspacePage}>{sectionContent}</div>
      </PortalShell>

      {/* Application Operations Drawer */}
      <Drawer
        opened={!!selectedAppId}
        onClose={() => {
          setSelectedAppId(null)
          setApplicationDrawerTab('status')
        }}
        position="right"
        size="75%"
        title={
          <Group gap="sm">
            <IconRocket size={20} color="#1d66d6" />
            <Text fw={900}>{selectedApp?.name} 운영 센터</Text>
          </Group>
        }
        styles={{ title: { fontSize: '1.2rem' }, body: { padding: 0 } }}
      >
        <ScrollArea h="calc(100vh - 80px)">
          <Tabs
            value={applicationDrawerTab}
            onChange={(value) => setApplicationDrawerTab(value ?? 'status')}
            keepMounted={false}
            color="lagoon.6"
            styles={{ tab: { padding: '16px 20px' } }}
          >
            <Tabs.List>
              <Tabs.Tab value="status" leftSection={<IconActivity size={16} />}>상태</Tabs.Tab>
              <Tabs.Tab value="deploy" leftSection={<IconRocket size={16} />}>배포</Tabs.Tab>
              <Tabs.Tab value="secrets" leftSection={<IconLock size={16} />}>환경 변수</Tabs.Tab>
              <Tabs.Tab value="observability" leftSection={<IconDatabase size={16} />}>관측</Tabs.Tab>
              <Tabs.Tab value="history" leftSection={<IconHistory size={16} />}>배포 이력</Tabs.Tab>
              <Tabs.Tab value="rules" leftSection={<IconShieldCheck size={16} />}>운영 규칙</Tabs.Tab>
            </Tabs.List>

            <Tabs.Panel value="status" p="xl">
              <Stack gap="xl">
                {appDetailWarnings.length > 0 ? (
                  <StatePanel
                    kind="partial"
                    title="운영 센터 데이터를 일부만 불러왔습니다"
                    description={appDetailWarnings.join(' ')}
                  />
                ) : null}

                {!appDetailsLoaded ? (
                  <StatePanel
                    kind="loading"
                    title="운영 상태를 불러오는 중"
                    description="선택한 애플리케이션의 배포 상태와 관측 신호를 수집하고 있습니다."
                  />
                ) : (
                  <>
                    <SimpleGrid cols={{ base: 1, sm: 2, xl: 4 }} spacing="md">
                      <div className={classes.statBadge}>
                        <Text className={classes.statLabel}>GitOps 동기화</Text>
                        <Text className={classes.statValueSmall}>
                          {formatSyncStatusLabel(appDetails.syncStatus?.status ?? selectedApp?.syncStatus)}
                        </Text>
                      </div>
                      <div className={classes.statBadge}>
                        <Text className={classes.statLabel}>최근 배포 태그</Text>
                        <Text className={classes.statValueSmall}>
                          {latestDeployment?.imageTag ?? '없음'}
                        </Text>
                      </div>
                      <div className={classes.statBadge}>
                        <Text className={classes.statLabel}>대상 환경</Text>
                        <Text className={classes.statValueSmall}>
                          {selectedDeployEnvironment?.name ?? '-'}
                        </Text>
                      </div>
                      <div className={classes.statBadge}>
                        <Text className={classes.statLabel}>권한 범위</Text>
                        <Text className={classes.statValueSmall}>
                          {canDeployInProject ? (canAdminProject ? '관리자 운영' : '배포 운영') : '조회 전용'}
                        </Text>
                      </div>
                      {repositoryPoll?.enabled ? (
                        <div className={classes.statBadge}>
                          <Text className={classes.statLabel}>최근 저장소 확인</Text>
                          <Text className={classes.statValueSmall}>
                            {formatRepositoryPollCheckedAt(repositoryPoll)}
                          </Text>
                        </div>
                      ) : null}
                      {repositoryPoll?.enabled ? (
                        <div className={classes.statBadge}>
                          <Text className={classes.statLabel}>저장소 확인 주기</Text>
                          <Text className={classes.statValueSmall}>
                            {formatRepositoryPollInterval(repositoryPoll.intervalSeconds)}
                          </Text>
                        </div>
                      ) : null}
                      {repositoryPoll?.enabled ? (
                        <div className={classes.statBadge}>
                          <Text className={classes.statLabel}>다음 저장소 확인</Text>
                          <Text className={classes.statValueSmall}>
                            {formatRepositoryPollNextAt(repositoryPoll)}
                          </Text>
                        </div>
                      ) : null}
                    </SimpleGrid>

                    <StatePanel
                      kind={!canDeployInProject ? 'forbidden' : isProtectedDeployTarget ? 'partial' : 'empty'}
                      title={
                        !canDeployInProject
                          ? '현재 앱은 조회 전용 모드입니다'
                          : isProtectedDeployTarget
                            ? '보호 환경 반영 상태를 보고 있습니다'
                            : '레포 반영 상태를 보고 있습니다'
                      }
                      description={
                        !canDeployInProject
                          ? 'viewer 역할에서는 배포와 정책 저장이 막혀 있습니다.'
                          : isProtectedDeployTarget
                            ? `${selectedDeployEnvironment?.name ?? '선택한'} 환경은 change request 승인 후 Git 반영과 Flux 동기화 순서로 배포됩니다.`
                            : `${selectedDeployEnvironment?.name ?? '선택한'} 환경은 레포에서 image tag 또는 descriptor를 바꾸면 Flux가 반영합니다. 이 화면에서는 실행이 아니라 상태만 확인합니다.`
                      }
                    />
                    <Alert
                      color={runtimeReadinessColor(appDetails.syncStatus?.status ?? selectedApp?.syncStatus, latestDeployment)}
                      radius="md"
                      icon={<IconCloudCheck size={16} />}
                    >
                      {describeRuntimeReadiness(appDetails.syncStatus?.status ?? selectedApp?.syncStatus, latestDeployment)}
                    </Alert>
                    {repositoryPoll?.enabled ? (
                      <Alert
                        color={repositoryPoll.lastResult === 'Error' ? 'red' : 'gray'}
                        radius="md"
                        icon={<IconRefresh size={16} />}
                      >
                        {describeRepositoryPoll(repositoryPoll)}
                      </Alert>
                    ) : null}
                    {repositoryPoll?.enabled ? (
                      <div className={classes.surfaceCard}>
                        <Group justify="space-between" align="end" gap="md" wrap="wrap">
                          <div>
                            <Text className={classes.sectionEyebrow}>저장소 Sync 제어</Text>
                            <Text size="sm" c="dimmed">
                              Argo CD의 수동 sync처럼 지금 바로 저장소를 재확인하거나, 자동 polling 주기를 1분·5분·10분으로 제한해 운영할 수 있습니다. 주기 저장은 GitOps repo commit/push 이후 완료됩니다.
                            </Text>
                          </div>
                          <Group gap="sm" align="end" wrap="wrap">
                            <Select
                              label="Polling 주기"
                              data={repositoryPollIntervalOptions}
                              value={repositoryPollIntervalDraft}
                              onChange={(value) => setRepositoryPollIntervalDraft(value ?? '300')}
                              allowDeselect={false}
                              w={160}
                              disabled={!canDeployInProject}
                            />
                            <Button
                              variant="default"
                              loading={savingRepositoryPollInterval}
                              disabled={!canDeployInProject || !repositoryPollIntervalChanged}
                              onClick={handleSaveRepositoryPollInterval}
                            >
                              주기 저장
                            </Button>
                            <Button
                              leftSection={<IconRefresh size={16} />}
                              loading={syncingRepositoryPoll}
                              disabled={!canDeployInProject}
                              onClick={handleSyncRepositoryNow}
                            >
                              지금 Sync
                            </Button>
                          </Group>
                        </Group>
                        {!canDeployInProject ? (
                          <Text size="sm" c="dimmed" mt="sm">
                            viewer 역할은 저장소 sync와 polling 주기 변경을 실행할 수 없습니다.
                          </Text>
                        ) : null}
                      </div>
                    ) : null}

                    <div className={classes.surfaceCard}>
                      <Text className={classes.sectionEyebrow} mb="md">배포 진행 상태</Text>
                      <div className={classes.progressList}>
                        <div className={`${classes.progressItem} ${deploymentStageClass(latestDeployment ? 'complete' : 'pending', classes)}`}>
                          <div className={classes.progressMarker}><IconGitBranch size={16} /></div>
                          <div>
                            <Text className={classes.progressTitle}>Git 변경 기록됨</Text>
                            <Text className={classes.progressDetail}>
                              {latestDeployment
                                ? `${latestDeployment.imageTag} 버전 요청이 기록되었습니다.`
                                : '배포 이력이 아직 없습니다.'}
                            </Text>
                          </div>
                        </div>
                        <div className={`${classes.progressItem} ${deploymentStageClass(syncStageState(appDetails.syncStatus?.status), classes)}`}>
                          <div className={classes.progressMarker}><IconRefresh size={16} /></div>
                          <div>
                            <Text className={classes.progressTitle}>Flux 동기화</Text>
                            <Text className={classes.progressDetail}>
                              {appDetails.syncStatus?.message || '동기화 상태를 아직 수집하지 못했습니다.'}
                            </Text>
                          </div>
                        </div>
                        <div className={`${classes.progressItem} ${deploymentStageClass(rolloutStageState(latestDeployment), classes)}`}>
                          <div className={classes.progressMarker}><IconBox size={16} /></div>
                          <div>
                            <Text className={classes.progressTitle}>런타임 반영</Text>
                            <Text className={classes.progressDetail}>
                              {rolloutStageMessage(latestDeployment)}
                            </Text>
                          </div>
                        </div>
                      </div>
                    </div>

                    <div className={classes.surfaceCard}>
                      <Text className={classes.sectionEyebrow} mb="md">현재 신호</Text>
                      {latestEvent || latestFailedDeployment || appDetails.syncStatus?.message ? (
                        <Stack gap="sm">
                          {latestFailedDeployment ? (
                            <Alert color="red" radius="md" icon={<IconAlertTriangle size={16} />}>
                              최근 실패 배포: {latestFailedDeployment.imageTag} · {latestFailedDeployment.message || latestFailedDeployment.status}
                            </Alert>
                          ) : null}
                          {latestEvent ? (
                            <Alert color="lagoon.6" radius="md" icon={<IconActivity size={16} />}>
                              최근 이벤트: {latestEvent.type} · {latestEvent.message}
                            </Alert>
                          ) : null}
                          {appDetails.syncStatus?.message ? (
                            <Alert color="gray" radius="md" icon={<IconRefresh size={16} />}>
                              Sync 메모: {appDetails.syncStatus.message}
                            </Alert>
                          ) : null}
                        </Stack>
                      ) : (
                        <StatePanel
                          kind="empty"
                          withCard={false}
                          title="표시할 상태 신호가 없습니다"
                          description="배포가 진행되거나 시스템 이벤트가 쌓이면 현재 상태 요약이 여기에 표시됩니다."
                        />
                      )}
                    </div>
                  </>
                )}
              </Stack>
            </Tabs.Panel>

            <Tabs.Panel value="deploy" p="xl">
              <Stack gap="lg">
                <div className={classes.surfaceCard}>
                  <Text fw={800} mb={4}>배포 기준과 반영 흐름</Text>
                  <Text size="sm" c="dimmed" mb="lg">
                    이미지 태그 변경은 이 화면에서 직접 실행하지 않습니다. 앱 레포 또는 GitOps 레포 변경을 Flux가 반영하는 구조입니다.
                  </Text>
                  {environments.length === 0 ? (
                    <StatePanel
                      kind="empty"
                      withCard={false}
                      title="배포 대상 환경이 없습니다"
                      description="프로젝트에 환경이 정의되면 환경별 반영 정책과 Flux 상태를 이 화면에서 확인할 수 있습니다."
                    />
                  ) : (
                    <Stack gap="md">
                      <Select
                        label="확인할 환경"
                        value={selectedDeployEnvironment?.id ?? null}
                        data={environments.map((environment) => ({
                          value: environment.id,
                          label: `${environment.name} · ${environment.writeMode === 'pull_request' ? '변경 요청' : '직접 반영'}`,
                        }))}
                        onChange={(value) => setDeployEnvironment(value ?? '')}
                        allowDeselect={false}
                      />
                      <Alert
                        color={!canDeployInProject ? 'violet' : isProtectedDeployTarget ? 'yellow' : 'lagoon.6'}
                        radius="md"
                        icon={!canDeployInProject ? <IconLock size={16} /> : <IconGitBranch size={16} />}
                      >
                        {!selectedDeployEnvironment
                          ? '환경을 먼저 선택하세요.'
                          : !canDeployInProject
                            ? 'viewer 역할은 배포 상태 조회만 가능합니다.'
                            : isProtectedDeployTarget
                              ? `${selectedDeployEnvironment.name} 환경은 change request 승인 후 Git 반영과 Flux 동기화로 배포됩니다.`
                              : `${selectedDeployEnvironment.name} 환경은 레포의 image tag 또는 aolda_deploy.json 변경을 Flux가 그대로 반영합니다.`}
                      </Alert>
                      <SimpleGrid cols={{ base: 1, md: 3 }} spacing="md">
                        <div className={classes.statBadge}>
                          <Text className={classes.statLabel}>현재 이미지</Text>
                          <Text className={classes.statValueSmall}>{selectedApp?.image ?? '확인 불가'}</Text>
                        </div>
                        <div className={classes.statBadge}>
                          <Text className={classes.statLabel}>최근 배포 태그</Text>
                          <Text className={classes.statValueSmall}>{latestDeployment?.imageTag ?? '없음'}</Text>
                        </div>
                        <div className={classes.statBadge}>
                          <Text className={classes.statLabel}>Flux 상태</Text>
                          <Text className={classes.statValueSmall}>{formatSyncStatusLabel(appDetails.syncStatus?.status ?? selectedApp?.syncStatus)}</Text>
                        </div>
                      </SimpleGrid>
                      <Alert color="gray" radius="md" icon={<IconDatabase size={16} />}>
                        권장 순서: 앱 레포에서 새 이미지를 빌드하고, GitOps 기준 파일의 tag 또는 descriptor를 갱신한 뒤, 이 화면에서
                        Flux 동기화와 배포 이력을 확인하세요.
                      </Alert>
                    </Stack>
                  )}
                </div>

                <div className={classes.surfaceCard}>
                  <Text className={classes.sectionEyebrow} mb="md">배포 진행률</Text>
                  <div className={classes.progressList}>
                    <div className={`${classes.progressItem} ${deploymentStageClass(appDetails.deployments.length > 0 ? 'complete' : 'pending', classes)}`}>
                      <div className={classes.progressMarker}><IconGitBranch size={16} /></div>
                      <div>
                        <Text className={classes.progressTitle}>Git 커밋 반영</Text>
                        <Text className={classes.progressDetail}>
                          {latestDeployment
                            ? `${latestDeployment.imageTag} 버전 요청이 저장되었습니다.`
                            : '배포 이력이 아직 없습니다.'}
                        </Text>
                      </div>
                    </div>
                    <div className={`${classes.progressItem} ${deploymentStageClass(syncStageState(appDetails.syncStatus?.status), classes)}`}>
                      <div className={classes.progressMarker}><IconRefresh size={16} /></div>
                      <div>
                        <Text className={classes.progressTitle}>Flux 동기화</Text>
                        <Text className={classes.progressDetail}>
                          {appDetails.syncStatus?.message || '동기화 상태를 아직 수집하지 못했습니다.'}
                        </Text>
                      </div>
                    </div>
                    <div className={`${classes.progressItem} ${deploymentStageClass(rolloutStageState(latestDeployment), classes)}`}>
                      <div className={classes.progressMarker}><IconBox size={16} /></div>
                      <div>
                        <Text className={classes.progressTitle}>런타임 반영</Text>
                        <Text className={classes.progressDetail}>
                          {rolloutStageMessage(latestDeployment)}
                        </Text>
                      </div>
                    </div>
                  </div>
                </div>

                {latestFailedDeployment ? (
                  <StatePanel
                    kind="error"
                    title="최근 실패 배포가 있습니다"
                    description={latestFailedDeployment.message || `${latestFailedDeployment.imageTag} 배포가 정상 종료되지 않았습니다.`}
                  />
                ) : null}
              </Stack>
            </Tabs.Panel>

            <Tabs.Panel value="secrets" p="xl">
              <Stack gap="lg">
                {!canDeployInProject ? (
                  <StatePanel
                    kind="forbidden"
                    title="deployer 이상 권한에서만 환경 변수를 수정할 수 있습니다"
                    description="Vault 환경 변수 키 목록과 값 교체는 배포 권한이 있는 사용자에게만 열립니다."
                  />
                ) : !applicationSecretsLoaded ? (
                  <StatePanel
                    kind="loading"
                    title="환경 변수 정보를 불러오는 중"
                    description="Vault Secret 경로와 등록된 키 목록을 확인하고 있습니다."
                  />
                ) : applicationSecretsError ? (
                  <StatePanel
                    kind="partial"
                    title="환경 변수 정보를 불러오지 못했습니다"
                    description={applicationSecretsError}
                  />
                ) : (
                  <>
                    <SimpleGrid cols={{ base: 1, md: 5 }} spacing="md">
                      <div className={classes.statBadge}>
                        <Text className={classes.statLabel}>Vault 경로</Text>
                        <Text className={classes.statValueSmall} style={{ wordBreak: 'break-all' }}>
                          {applicationSecrets?.secretPath || '-'}
                        </Text>
                      </div>
                      <div className={classes.statBadge}>
                        <Text className={classes.statLabel}>연결 상태</Text>
                        <Text className={classes.statValueSmall}>
                          {applicationSecrets?.configured ? 'ExternalSecret 연결됨' : '아직 연결 없음'}
                        </Text>
                      </div>
                      <div className={classes.statBadge}>
                        <Text className={classes.statLabel}>등록된 키</Text>
                        <Text className={classes.statValueSmall}>{applicationSecrets?.items.length ?? 0}</Text>
                      </div>
                      <div className={classes.statBadge}>
                        <Text className={classes.statLabel}>현재 버전</Text>
                        <Text className={classes.statValueSmall}>
                          {applicationSecrets?.currentVersion ? `v${applicationSecrets.currentVersion}` : '-'}
                        </Text>
                      </div>
                      <div className={classes.statBadge}>
                        <Text className={classes.statLabel}>최근 변경</Text>
                        <Text className={classes.statValueSmall}>
                          {formatDateTimeValue(applicationSecrets?.updatedAt)}
                        </Text>
                      </div>
                    </SimpleGrid>

                    <Alert color="yellow" variant="light" icon={<IconAlertTriangle size={16} />}>
                      Vault 값은 버전으로 남지만, Kubernetes envFrom 값은 실행 중인 Pod에 즉시 주입되지 않습니다. 저장하거나 복원한 값은 다음 rollout 또는 Pod 재시작부터 적용됩니다.
                    </Alert>

                    <Alert color="blue" variant="light" icon={<IconLock size={16} />}>
                      새 환경 변수는 아래 새 환경 변수 영역에서 키와 값을 입력한 뒤 <b>Vault 환경 변수 저장</b>을 누르면 반영됩니다. 값은 저장 후 다시 표시하지 않습니다.
                    </Alert>

                    <div className={classes.surfaceCard}>
                      <Group justify="space-between" align="flex-start" mb="md">
                        <Stack gap={2}>
                          <Text fw={800}>Vault 버전 히스토리</Text>
                          <Text size="sm" c="dimmed">KV v2 metadata 기준으로 버전만 표시하고 값은 노출하지 않습니다.</Text>
                        </Stack>
                        <Badge color={applicationSecrets?.versioningEnabled ? 'teal' : 'gray'} variant="light">
                          {applicationSecrets?.versioningEnabled ? 'KV v2 버전 관리' : '버전 기록 없음'}
                        </Badge>
                      </Group>
                      <Table verticalSpacing="sm">
                        <Table.Thead>
                          <Table.Tr>
                            <Table.Th>버전</Table.Th>
                            <Table.Th>생성 시각</Table.Th>
                            <Table.Th>변경자</Table.Th>
                            <Table.Th>키</Table.Th>
                            <Table.Th>상태</Table.Th>
                            <Table.Th />
                          </Table.Tr>
                        </Table.Thead>
                        <Table.Tbody>
                          {(applicationSecretVersions?.items ?? []).length > 0 ? (
                            (applicationSecretVersions?.items ?? []).map((item) => {
                              const disabled = item.current || item.deleted || item.destroyed || restoringSecretVersion !== null
                              const status = item.destroyed ? 'destroyed' : item.deleted ? 'deleted' : item.current ? 'current' : 'available'
                              return (
                                <Table.Tr key={item.version}>
                                  <Table.Td><Text fw={800}>v{item.version}</Text></Table.Td>
                                  <Table.Td><Text size="sm">{formatDateTimeValue(item.createdAt)}</Text></Table.Td>
                                  <Table.Td><Text size="sm">{item.updatedBy || '-'}</Text></Table.Td>
                                  <Table.Td><Text size="sm">{item.keyCount ?? '-'}</Text></Table.Td>
                                  <Table.Td>
                                    <Badge color={item.current ? 'teal' : item.deleted || item.destroyed ? 'red' : 'gray'} variant="light">
                                      {formatSecretVersionStatus(status)}
                                    </Badge>
                                  </Table.Td>
                                  <Table.Td>
                                    <Button
                                      size="xs"
                                      variant="light"
                                      leftSection={<IconHistory size={14} />}
                                      disabled={disabled}
                                      loading={restoringSecretVersion === item.version}
                                      onClick={() => void handleRestoreApplicationSecretVersion(item.version)}
                                    >
                                      복원
                                    </Button>
                                  </Table.Td>
                                </Table.Tr>
                              )
                            })
                          ) : (
                            <Table.Tr>
                              <Table.Td colSpan={6}>
                                <Text size="sm" c="dimmed">아직 표시할 Vault 버전 히스토리가 없습니다.</Text>
                              </Table.Td>
                            </Table.Tr>
                          )}
                        </Table.Tbody>
                      </Table>
                    </div>

                    <div className={classes.surfaceCard}>
                      <Group justify="space-between" align="flex-start" mb="md">
                        <Stack gap={2}>
                          <Text fw={800}>기존 환경 변수</Text>
                          <Text size="sm" c="dimmed">값은 표시하지 않고, 새 값을 입력한 키만 교체합니다.</Text>
                        </Stack>
                        <Badge color="gray" variant="light">값 숨김</Badge>
                      </Group>
                      <Table verticalSpacing="sm">
                        <Table.Thead>
                          <Table.Tr>
                            <Table.Th>키</Table.Th>
                            <Table.Th>새 값</Table.Th>
                            <Table.Th>삭제</Table.Th>
                          </Table.Tr>
                        </Table.Thead>
                        <Table.Tbody>
                          {(applicationSecrets?.items ?? []).length > 0 ? (
                            (applicationSecrets?.items ?? []).map((item) => {
                              const deleting = secretDeleteDrafts.includes(item.key)
                              return (
                                <Table.Tr key={item.key}>
                                  <Table.Td>
                                    <Text fw={800}>{item.key}</Text>
                                  </Table.Td>
                                  <Table.Td>
                                    <PasswordInput
                                      placeholder="새 값을 입력하면 교체"
                                      value={secretValueDrafts[item.key] ?? ''}
                                      disabled={deleting}
                                      onChange={(event) =>
                                        setSecretValueDrafts((current) => ({
                                          ...current,
                                          [item.key]: event.currentTarget.value,
                                        }))
                                      }
                                    />
                                  </Table.Td>
                                  <Table.Td>
                                    <Checkbox
                                      checked={deleting}
                                      onChange={(event) => {
                                        const checked = event.currentTarget.checked
                                        setSecretDeleteDrafts((current) =>
                                          checked
                                            ? Array.from(new Set([...current, item.key]))
                                            : current.filter((key) => key !== item.key),
                                        )
                                      }}
                                    />
                                  </Table.Td>
                                </Table.Tr>
                              )
                            })
                          ) : (
                            <Table.Tr>
                              <Table.Td colSpan={3}>
                                <Text size="sm" c="dimmed">아직 등록된 환경 변수 키가 없습니다.</Text>
                              </Table.Td>
                            </Table.Tr>
                          )}
                        </Table.Tbody>
                      </Table>
                    </div>

                    <div className={classes.surfaceCard}>
                      <Stack gap="md">
                        <Group justify="space-between" align="center">
                          <Stack gap={2}>
                            <Text fw={800}>새 환경 변수</Text>
                            <Text size="sm" c="dimmed">새 키를 추가하거나 `.env` 내용을 편집 내용에 반영합니다.</Text>
                          </Stack>
                          <Button
                            variant="default"
                            leftSection={<IconPlus size={16} />}
                            onClick={() => setNewSecretRows((current) => [...current, { key: '', value: '' }])}
                          >
                            키 추가
                          </Button>
                        </Group>
                        <Stack gap="sm">
                          {newSecretRows.map((row, index) => (
                            <Group key={index} grow align="flex-end">
                              <TextInput
                                label={index === 0 ? '키' : undefined}
                                placeholder="DATABASE_URL"
                                value={row.key}
                                onChange={(event) =>
                                  setNewSecretRows((current) =>
                                    current.map((item, itemIndex) =>
                                      itemIndex === index ? { ...item, key: event.currentTarget.value } : item,
                                    ),
                                  )
                                }
                              />
                              <PasswordInput
                                label={index === 0 ? '값' : undefined}
                                placeholder="값 입력"
                                value={row.value}
                                onChange={(event) =>
                                  setNewSecretRows((current) =>
                                    current.map((item, itemIndex) =>
                                      itemIndex === index ? { ...item, value: event.currentTarget.value } : item,
                                    ),
                                  )
                                }
                              />
                              <Button
                                variant="subtle"
                                color="red"
                                disabled={newSecretRows.length === 1}
                                onClick={() => setNewSecretRows((current) => current.filter((_, itemIndex) => itemIndex !== index))}
                              >
                                제거
                              </Button>
                            </Group>
                          ))}
                        </Stack>
                        <Textarea
                          label=".env 일괄 입력"
                          autosize
                          minRows={4}
                          placeholder={'DB_HOST=db.internal\nDB_PASSWORD=secret-value'}
                          value={secretBulkText}
                          onChange={(event) => setSecretBulkText(event.currentTarget.value)}
                        />
                        <Group justify="space-between" align="center">
                          <Text size="sm" c="dimmed">{secretBulkMessage || '기존 키와 이름이 같으면 새 값 입력란에 반영됩니다.'}</Text>
                          <Button variant="light" onClick={handleApplySecretBulkText}>
                            .env 반영
                          </Button>
                        </Group>
                        <Button
                          fullWidth
                          color="lagoon.6"
                          leftSection={<IconLock size={16} />}
                          loading={savingApplicationSecrets}
                          onClick={handleSaveApplicationSecrets}
                        >
                          Vault 환경 변수 저장
                        </Button>
                      </Stack>
                    </div>
                  </>
                )}
              </Stack>
            </Tabs.Panel>

            <Tabs.Panel value="observability" p="xl">
              <Stack gap="xl">
                <Group justify="space-between" align="center">
                  <Text className={classes.sectionEyebrow}>관측 데이터</Text>
                  <SegmentedControl
                    value={metricRange}
                    onChange={setMetricRange}
                    data={[
                      { label: '5분', value: '5m' },
                      { label: '15분', value: '15m' },
                      { label: '1시간', value: '1h' },
                    ]}
                    color="lagoon.6"
                    radius="md"
                  />
                </Group>

                {!appDetailsLoaded ? (
                  <StatePanel
                    kind="loading"
                    title="관측 데이터를 불러오는 중"
                    description="metrics, 이벤트, pod/container 로그를 수집하고 있습니다."
                  />
                ) : (
                  <>
                    {appDetails.metrics?.metrics?.length ? (
                      <SimpleGrid cols={{ base: 1, sm: 2, xl: 4 }} spacing="xl">
                        <MetricCard
                          label="CPU 사용량"
                          value={formatLatestMetric(appDetails.metrics, 'cpu_usage')}
                          unit={latestMetricUnit(appDetails.metrics, 'cpu_usage')}
                          points={appDetails.metrics?.metrics?.find((metric) => metric.key === 'cpu_usage')?.points}
                          color="#1d66d6"
                          description={`${metricRange} 범위의 마지막 수집값`}
                        />
                        <MetricCard
                          label="메모리 사용량"
                          value={formatLatestMetric(appDetails.metrics, 'memory_usage')}
                          unit="MiB"
                          points={appDetails.metrics?.metrics?.find((metric) => metric.key === 'memory_usage')?.points}
                          color="#0b3d7f"
                          description={`${metricRange} 범위의 마지막 수집값`}
                        />
                        <MetricCard
                          label="요청량"
                          value={formatLatestMetric(appDetails.metrics, 'request_rate')}
                          unit="rpm"
                          points={appDetails.metrics?.metrics?.find((metric) => metric.key === 'request_rate')?.points}
                          color="#10b981"
                          description="분당 요청 수 기준"
                        />
                        <MetricCard
                          label="지연시간 P95"
                          value={formatLatestMetric(appDetails.metrics, 'latency_p95')}
                          unit="ms"
                          points={appDetails.metrics?.metrics?.find((metric) => metric.key === 'latency_p95')?.points}
                          color="#f59e0b"
                          description="95번째 백분위 응답 시간"
                        />
                      </SimpleGrid>
                    ) : (
                      <StatePanel
                        kind={appDetailWarnings.some((warning) => warning.includes('metrics')) ? 'partial' : 'empty'}
                        title="metrics 데이터가 없습니다"
                        description={
                          appDetailWarnings.some((warning) => warning.includes('metrics'))
                            ? 'metrics 조회에 실패했습니다. 범위를 바꾸거나 잠시 후 다시 시도하세요.'
                            : '아직 시계열 metric이 수집되지 않았습니다.'
                        }
                      />
                    )}

                    <div className={classes.surfaceCard}>
                      <Group justify="space-between" align="flex-start" mb="md">
                        <div>
                          <Text className={classes.sectionEyebrow}>Pod / Container 로그</Text>
                          <Text size="sm" c="dimmed" mt={6}>
                            선택한 pod/container 기준 최근 120줄과 이후 신규 로그를 실시간으로 보여줍니다.
                          </Text>
                        </div>
                        <Group gap="xs">
                          <Button
                            size="xs"
                            variant="light"
                            color="lagoon.6"
                            leftSection={<IconRefresh size={14} />}
                            loading={refreshingApplicationLogs}
                            disabled={!selectedAppId}
                            onClick={() => {
                              if (selectedAppId) {
                                void refreshApplicationLogTargets(selectedAppId, { restartStream: true })
                              }
                            }}
                          >
                            로그 업데이트
                          </Button>
                          <Badge
                            variant="light"
                            color={
                              liveLogStatus === 'streaming'
                                ? 'green'
                                : liveLogStatus === 'connecting'
                                  ? 'yellow'
                                  : liveLogStatus === 'error'
                                    ? 'red'
                                    : 'gray'
                            }
                          >
                            {liveLogStatus === 'streaming'
                              ? '실시간 수신 중'
                              : liveLogStatus === 'connecting'
                                ? '연결 중'
                                : liveLogStatus === 'error'
                                  ? '스트림 오류'
                                  : liveLogStatus === 'closed'
                                    ? '스트림 종료'
                                    : '대기 중'}
                          </Badge>
                        </Group>
                      </Group>

                      {(appDetails.logTargets?.items.length ?? 0) > 0 ? (
                        <Stack gap="md">
                          <Grid gutter="md">
                            <Grid.Col span={{ base: 12, md: 6 }}>
                              <Select
                                label="Pod"
                                value={selectedLogPodName || null}
                                data={logPodOptions}
                                onChange={(value) => setSelectedLogPodName(value ?? '')}
                                allowDeselect={false}
                              />
                            </Grid.Col>
                            <Grid.Col span={{ base: 12, md: 6 }}>
                              <Select
                                label="Container"
                                value={selectedLogContainerName || null}
                                data={logContainerOptions}
                                onChange={(value) => setSelectedLogContainerName(value ?? '')}
                                allowDeselect={false}
                              />
                            </Grid.Col>
                          </Grid>

                          <Group gap="xs">
                            <Badge color={selectedLogTarget?.phase === 'Running' ? 'green' : 'gray'} variant="light">
                              {selectedLogTarget?.phase || '상태 미상'}
                            </Badge>
                            <Badge color={selectedLogContainer?.ready ? 'green' : 'yellow'} variant="light">
                              {selectedLogContainer?.ready ? '준비 완료' : '준비 중'}
                            </Badge>
                            <Badge color={(selectedLogContainer?.restartCount ?? 0) > 0 ? 'orange' : 'gray'} variant="outline">
                              재시작 {selectedLogContainer?.restartCount ?? 0}회
                            </Badge>
                          </Group>

                          {selectedLogResourceStatus ? (
                            <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
                              <div className={classes.statBadge}>
                                <Group gap={8} align="center" mb={8}>
                                  <IconCpu size={16} color="#1d66d6" />
                                  <Text className={classes.statLabel}>CPU 사용량 대비 할당</Text>
                                </Group>
                                <Text className={classes.statValueSmall}>
                                  사용 {formatCPUCoreValue(selectedLogResourceStatus.cpuUsageCores)}
                                </Text>
                                <Text size="sm" c="dimmed" mt={6}>
                                  요청 {formatCPUAllocationValue(selectedLogResourceStatus.cpuRequestCores)} · 제한 {formatCPUAllocationValue(selectedLogResourceStatus.cpuLimitCores)}
                                </Text>
                                <Text size="xs" c="dimmed" mt={8}>
                                  요청 기준 {formatUtilizationValue(selectedLogResourceStatus.cpuRequestUtilization)} · 제한 기준 {formatUtilizationValue(selectedLogResourceStatus.cpuLimitUtilization)}
                                </Text>
                              </div>
                              <div className={classes.statBadge}>
                                <Group gap={8} align="center" mb={8}>
                                  <IconDatabase size={16} color="#0b3d7f" />
                                  <Text className={classes.statLabel}>메모리 사용량 대비 할당</Text>
                                </Group>
                                <Text className={classes.statValueSmall}>
                                  사용 {formatMemoryMiBValue(selectedLogResourceStatus.memoryUsageMiB)}
                                </Text>
                                <Text size="sm" c="dimmed" mt={6}>
                                  요청 {formatMemoryAllocationValue(selectedLogResourceStatus.memoryRequestMiB)} · 제한 {formatMemoryAllocationValue(selectedLogResourceStatus.memoryLimitMiB)}
                                </Text>
                                <Text size="xs" c="dimmed" mt={8}>
                                  요청 기준 {formatUtilizationValue(selectedLogResourceStatus.memoryRequestUtilization)} · 제한 기준 {formatUtilizationValue(selectedLogResourceStatus.memoryLimitUtilization)}
                                </Text>
                              </div>
                            </SimpleGrid>
                          ) : (
                            <Alert color="gray" radius="md" icon={<IconDatabase size={16} />}>
                              선택한 컨테이너의 request/limit 또는 현재 usage 데이터를 아직 수집하지 못했습니다.
                            </Alert>
                          )}

                          {liveLogError ? (
                            <Alert color="red" radius="md" icon={<IconAlertTriangle size={16} />}>
                              {liveLogError}
                            </Alert>
                          ) : null}

                          <ScrollArea h={320} offsetScrollbars>
                            <pre
                              style={{
                                margin: 0,
                                padding: '14px 16px',
                                borderRadius: '14px',
                                background: '#0f172a',
                                color: '#e2e8f0',
                                fontSize: '12px',
                                lineHeight: 1.6,
                                whiteSpace: 'pre-wrap',
                                wordBreak: 'break-word',
                                fontFamily:
                                  'ui-monospace, SFMono-Regular, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, monospace',
                              }}
                            >
                              {liveLogText || '실시간 로그를 기다리는 중입니다.'}
                            </pre>
                          </ScrollArea>
                        </Stack>
                      ) : (
                        <StatePanel
                          kind={logTargetsWarning ? 'partial' : 'empty'}
                          withCard={false}
                          title="표시할 pod/container 로그가 없습니다"
                          description={
                            logTargetsWarning
                              ? logTargetsWarning.replace(/^로그 대상:\s*/, '')
                              : '실행 중인 pod가 아직 없습니다.'
                          }
                        />
                      )}
                    </div>

                    <Grid gutter="lg">
                      <Grid.Col span={{ base: 12, xl: 7 }}>
                        <div className={classes.surfaceCard}>
                          <Text className={classes.sectionEyebrow}>최근 시스템 이벤트</Text>
                          {appDetails.events.length === 0 ? (
                            <StatePanel
                              kind={appDetailWarnings.some((warning) => warning.includes('이벤트')) ? 'partial' : 'empty'}
                              withCard={false}
                              title="표시할 이벤트가 없습니다"
                              description={
                                appDetailWarnings.some((warning) => warning.includes('이벤트'))
                                  ? '이벤트 조회에 실패했습니다.'
                                  : '이벤트 스트림이 비어 있습니다.'
                              }
                            />
                          ) : (
                            <div className={classes.progressList}>
                              {appDetails.events.map((ev, idx) => (
                                <div key={idx} className={classes.progressItem}>
                                  <div className={classes.progressMarker}>
                                    <IconActivity size={16} />
                                  </div>
                                  <div>
                                    <Text className={classes.progressTitle}>{ev.type}</Text>
                                    <Text className={classes.progressDetail}>{ev.message}</Text>
                                    <Text size="xs" c="dimmed" mt={4}>{new Date(ev.createdAt).toLocaleString()}</Text>
                                  </div>
                                </div>
                              ))}
                            </div>
                          )}
                        </div>
                      </Grid.Col>

                      <Grid.Col span={{ base: 12, xl: 5 }}>
                        <Stack gap="lg">
                          <StatePanel
                            kind={
                              latestFailedDeployment
                                ? 'error'
                                : appDetails.syncStatus?.status === 'Degraded'
                                  ? 'error'
                                  : appDetails.syncStatus?.status === 'Syncing'
                                    ? 'partial'
                                    : 'empty'
                            }
                            title="현재 운영 메모"
                            description={
                              latestFailedDeployment?.message
                                ? `최근 실패 배포 원인: ${latestFailedDeployment.message}`
                                : latestDeployment?.message
                                  || appDetails.syncStatus?.message
                                  || '현재 수집된 추가 운영 메모가 없습니다.'
                            }
                          />
                          <div className={classes.surfaceCard}>
                            <Text className={classes.sectionEyebrow}>현재 연동 상태</Text>
                            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
                              <div className={classes.statBadge}>
                                <Text className={classes.statLabel}>Sync 상태</Text>
                                <Text className={classes.statValueSmall}>
                                  {appDetails.syncStatus?.status ?? '확인 불가'}
                                </Text>
                              </div>
                              <div className={classes.statBadge}>
                                <Text className={classes.statLabel}>최근 배포 상태</Text>
                                <Text className={classes.statValueSmall}>
                                  {latestDeployment?.status ?? '없음'}
                                </Text>
                              </div>
                            </SimpleGrid>
                          </div>
                        </Stack>
                      </Grid.Col>
                    </Grid>
                  </>
                )}
              </Stack>
            </Tabs.Panel>

            <Tabs.Panel value="history" p="xl">
              <Stack gap="md">
                {latestFailedDeployment ? (
                  <Alert color="red" radius="md" icon={<IconAlertTriangle size={16} />}>
                    최근 실패 이력: {latestFailedDeployment.imageTag} · {latestFailedDeployment.message || latestFailedDeployment.status}
                  </Alert>
                ) : null}

                {appDetails.deployments.length > 0 ? (
                  <>
                    <Alert color="gray" radius="md" icon={<IconHistory size={16} />}>
                      배포 이력의 상태는 AODS가 기록한 배포 레코드 기준입니다. 실제 런타임 준비 여부는 상태 탭의 준비 상태와 관측 탭의 로그를 함께 확인하세요.
                    </Alert>
                    <Table striped highlightOnHover>
                      <Table.Thead>
                        <Table.Tr>
                          <Table.Th>배포 ID</Table.Th>
                          <Table.Th>환경</Table.Th>
                          <Table.Th>이미지 태그</Table.Th>
                          <Table.Th>상태</Table.Th>
                          <Table.Th>메시지</Table.Th>
                          <Table.Th>완료 시각</Table.Th>
                        </Table.Tr>
                      </Table.Thead>
                      <Table.Tbody>
                        {appDetails.deployments.map((deployment) => (
                          <Table.Tr
                            key={deployment.deploymentId}
                            style={{
                              cursor: 'pointer',
                              backgroundColor:
                                selectedDeploymentId === deployment.deploymentId ? 'rgba(29, 102, 214, 0.06)' : undefined,
                            }}
                            onClick={() => setSelectedDeploymentId(deployment.deploymentId)}
                          >
                            <Table.Td><Text size="xs" ff="monospace">{deployment.deploymentId.slice(0, 8)}</Text></Table.Td>
                            <Table.Td>{deployment.environment}</Table.Td>
                            <Table.Td><Badge variant="outline" size="sm">{deployment.imageTag}</Badge></Table.Td>
                            <Table.Td>
                              <Badge color={deploymentStatusColor(deployment.status)}>{formatDeploymentStatusLabel(deployment.status)}</Badge>
                            </Table.Td>
                            <Table.Td>
                              <Text size="xs" c="dimmed">
                                {deployment.message || deployment.rolloutPhase || '-'}
                              </Text>
                            </Table.Td>
                            <Table.Td><Text size="xs" c="dimmed">{new Date(deployment.updatedAt).toLocaleString('ko-KR')}</Text></Table.Td>
                          </Table.Tr>
                        ))}
                      </Table.Tbody>
                    </Table>

                    {!selectedDeploymentId ? (
                      <StatePanel
                        kind="empty"
                        title="배포를 선택하면 상세가 열립니다"
                        description="배포 레코드를 클릭하면 실제 반영 이미지와 상태 메타데이터를 확인할 수 있습니다."
                      />
                    ) : !selectedDeploymentLoaded ? (
                      <StatePanel
                        kind="loading"
                        title="배포 상세를 불러오는 중"
                        description="선택한 배포 레코드의 실제 반영 정보와 세부 메타데이터를 조회하고 있습니다."
                      />
                    ) : selectedDeploymentDetail ? (
                      <div className={classes.surfaceCard}>
                        <Group justify="space-between" align="start" mb="md">
                          <div>
                            <Text fw={800}>선택한 배포 상세</Text>
                            <Text size="sm" c="dimmed" mt={4}>
                              {selectedDeploymentDetail.deploymentId} · {selectedDeploymentDetail.imageTag}
                            </Text>
                          </div>
                          <Badge color={deploymentStatusColor(selectedDeploymentDetail.status)}>
                            {formatDeploymentStatusLabel(selectedDeploymentDetail.status)}
                          </Badge>
                        </Group>
                        <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
                          <div className={classes.statBadge}>
                            <Text className={classes.statLabel}>배포 방식</Text>
                            <Text className={classes.statValueSmall}>{selectedDeploymentDetail.deploymentStrategy}</Text>
                          </div>
                          <div className={classes.statBadge}>
                            <Text className={classes.statLabel}>환경</Text>
                            <Text className={classes.statValueSmall}>{selectedDeploymentDetail.environment}</Text>
                          </div>
                          <div className={classes.statBadge}>
                            <Text className={classes.statLabel}>반영 이미지</Text>
                            <Text className={classes.statValueSmall}>{selectedDeploymentDetail.image}</Text>
                          </div>
                          <div className={classes.statBadge}>
                            <Text className={classes.statLabel}>커밋 SHA</Text>
                            <Text className={classes.statValueSmall}>
                              {selectedDeploymentDetail.commitSha || '-'}
                            </Text>
                          </div>
                          <div className={classes.statBadge}>
                            <Text className={classes.statLabel}>기록 시각</Text>
                            <Text className={classes.statValueSmall}>
                              {new Date(selectedDeploymentDetail.createdAt).toLocaleString('ko-KR')}
                            </Text>
                          </div>
                          <div className={classes.statBadge}>
                            <Text className={classes.statLabel}>마지막 갱신</Text>
                            <Text className={classes.statValueSmall}>
                              {new Date(selectedDeploymentDetail.updatedAt).toLocaleString('ko-KR')}
                            </Text>
                          </div>
                        </SimpleGrid>
                        <Text size="sm" c="dimmed" mt="md">
                          {selectedDeploymentDetail.message || '추가 배포 메모가 없습니다.'}
                        </Text>
                        {selectedDeploymentDetail.deploymentStrategy === 'Canary' ? (
                          <div className={classes.surfaceCard} style={{ marginTop: '16px' }}>
                            <Text className={classes.sectionEyebrow} mb="md">카나리아 상세</Text>
                            <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
                              <div className={classes.statBadge}>
                                <Text className={classes.statLabel}>Rollout Phase</Text>
                                <Text className={classes.statValueSmall}>{selectedDeploymentDetail.rolloutPhase || '-'}</Text>
                              </div>
                              <div className={classes.statBadge}>
                                <Text className={classes.statLabel}>Current Step</Text>
                                <Text className={classes.statValueSmall}>
                                  {selectedDeploymentDetail.currentStep ?? '-'}
                                </Text>
                              </div>
                              <div className={classes.statBadge}>
                                <Text className={classes.statLabel}>Canary Weight</Text>
                                <Text className={classes.statValueSmall}>
                                  {selectedDeploymentDetail.canaryWeight ?? '-'}
                                </Text>
                              </div>
                              <div className={classes.statBadge}>
                                <Text className={classes.statLabel}>Stable Revision</Text>
                                <Text className={classes.statValueSmall}>
                                  {selectedDeploymentDetail.stableRevision || '-'}
                                </Text>
                              </div>
                              <div className={classes.statBadge}>
                                <Text className={classes.statLabel}>Canary Revision</Text>
                                <Text className={classes.statValueSmall}>
                                  {selectedDeploymentDetail.canaryRevision || '-'}
                                </Text>
                              </div>
                            </SimpleGrid>
                          </div>
                        ) : null}
                        {selectedApp?.deploymentStrategy === 'Canary' ? (
                          <Group mt="md">
                            <Button
                              color="lagoon.6"
                              variant="light"
                              loading={promotingDeploymentId === selectedDeploymentDetail.deploymentId}
                              disabled={
                                !canDeployInProject ||
                                ['Completed', 'Promoted', 'Aborted', 'Failed'].includes(selectedDeploymentDetail.status)
                              }
                              onClick={() => void handlePromoteDeployment(selectedDeploymentDetail.deploymentId)}
                            >
                              카나리아 승격
                            </Button>
                            {!canDeployInProject ? (
                              <Text size="sm" c="dimmed">viewer 역할은 승격을 실행할 수 없습니다.</Text>
                            ) : null}
                          </Group>
                        ) : null}
                      </div>
                    ) : (
                      <StatePanel
                        kind="partial"
                        title="배포 상세를 표시하지 못했습니다"
                        description="선택한 deployment ID의 상세 응답을 아직 받지 못했습니다."
                      />
                    )}
                  </>
                ) : (
                  <StatePanel
                    kind={appDetailWarnings.some((warning) => warning.includes('배포 이력')) ? 'partial' : 'empty'}
                    title="배포 이력이 없습니다"
                    description={
                      appDetailWarnings.some((warning) => warning.includes('배포 이력'))
                        ? '배포 이력 조회에 실패했습니다.'
                        : '첫 배포가 시작되면 이미지 태그, 상태, 메시지가 이력 표에 쌓입니다.'
                    }
                  />
                )}
              </Stack>
            </Tabs.Panel>

            <Tabs.Panel value="rules" p="xl">
              <Stack gap="lg">
                {!canAdminProject ? (
                  <StatePanel
                    kind="forbidden"
                    title="project admin만 규칙을 수정할 수 있습니다"
                    description="현재 탭에서는 외부 노출과 리소스 상한선을 조회할 수만 있고 저장은 admin 역할에서만 허용됩니다."
                  />
                ) : null}

                {showRollbackPolicyControls ? (
                  <div className={classes.surfaceCard}>
                    <Group justify="space-between" mb="md">
                      <Stack gap={0}>
                        <Text fw={800}>자동 롤백 정책</Text>
                        <Text size="sm" c="dimmed">장애 발생 시 자동으로 이전 안정 버전으로 되돌립니다.</Text>
                      </Stack>
                      <Switch
                        size="lg"
                        color="lagoon.6"
                        checked={rollbackPolicyDraft.enabled}
                        disabled={!canDeployInProject}
                        onChange={(event) => {
                          const checked = event.currentTarget.checked
                          setRollbackPolicyDraft((current) => ({
                            ...current,
                            enabled: checked,
                          }))
                        }}
                      />
                    </Group>
                    <SimpleGrid cols={2} spacing="md">
                      <NumberInput
                        label="최대 에러율 (%)"
                        value={rollbackPolicyDraft.maxErrorRate ?? undefined}
                        disabled={!canDeployInProject}
                        onChange={(value) =>
                          setRollbackPolicyDraft((current) => ({
                            ...current,
                            maxErrorRate: toOptionalNumber(value),
                          }))
                        }
                      />
                      <NumberInput
                        label="최대 지연시간 P95 (ms)"
                        value={rollbackPolicyDraft.maxLatencyP95Ms ?? undefined}
                        disabled={!canDeployInProject}
                        onChange={(value) =>
                          setRollbackPolicyDraft((current) => ({
                            ...current,
                            maxLatencyP95Ms: toOptionalNumber(value),
                          }))
                        }
                      />
                    </SimpleGrid>
                    <Button
                      fullWidth
                      variant="light"
                      color="lagoon.6"
                      mt="xl"
                      leftSection={<IconSettings size={16} />}
                      loading={savingRollbackPolicy}
                      disabled={!canDeployInProject}
                      onClick={handleSaveRollbackPolicy}
                    >
                      사용자 정의 규칙 저장
                    </Button>
                  </div>
                ) : null}

                <div className={classes.surfaceCard}>
                  <Stack gap="xs" mb="md">
                    <Text fw={800}>외부 노출</Text>
                    <Text size="sm" c="dimmed">
                      기본은 내부 서비스 운영이며, 필요할 때만 Kubernetes LoadBalancer 요청을 저장합니다. 외부 공개는 이후 네트워크 절차까지 확인해야 합니다.
                    </Text>
                    {selectedApp?.deploymentStrategy === 'Canary' ? (
                      <Alert color="blue" radius="md" icon={<IconGitBranch size={16} />}>
                        카나리아 배포는 이 화면에서 LoadBalancer 직접 노출을 변경할 수 없습니다.
                      </Alert>
                    ) : null}
                    {!canAdminProject ? (
                      <Alert color="yellow" radius="md" icon={<IconLock size={16} />}>
                        현재 역할에서는 네트워크 노출 정책을 수정할 수 없습니다. project admin만 변경할 수 있습니다.
                      </Alert>
                    ) : null}
                  </Stack>
                  <SimpleGrid cols={{ base: 1, md: showServiceMeshControls ? 2 : 1 }} spacing="md">
                    {showServiceMeshControls ? (
                      <div className={classes.statBadge}>
                        <Group justify="space-between" align="center" mb="xs">
                          <div>
                            <Text className={classes.statLabel}>Istio mesh 사용</Text>
                            <Text size="sm" c="dimmed">
                              사이드카, VirtualService, DestinationRule 기반으로 서비스 메시를 사용합니다.
                            </Text>
                          </div>
                          <Switch
                            size="lg"
                            color="lagoon.6"
                            checked={applicationNetworkDraft.meshEnabled}
                            disabled={!canAdminProject || selectedApp?.deploymentStrategy === 'Canary'}
                            onChange={(event) => {
                              const checked = event.currentTarget.checked
                              setApplicationNetworkDraft((current) => ({
                                ...current,
                                meshEnabled: checked,
                              }))
                            }}
                          />
                        </Group>
                        <Text size="sm" c="lagoon.8" fw={700}>
                          {applicationNetworkDraft.meshEnabled ? 'Istio 리소스를 함께 생성합니다.' : '기본 Kubernetes Service 경로만 사용합니다.'}
                        </Text>
                      </div>
                    ) : null}
                    <div className={classes.statBadge}>
                      <Group justify="space-between" align="center" mb="xs">
                        <div>
                          <Text className={classes.statLabel}>LoadBalancer 노출 요청</Text>
                          <Text size="sm" c="dimmed">
                            이 애플리케이션 Service를 `LoadBalancer` 타입으로 요청합니다.
                          </Text>
                        </div>
                        <Switch
                          size="lg"
                          color="lagoon.6"
                          checked={applicationNetworkDraft.loadBalancerEnabled}
                          disabled={!canAdminProject || selectedApp?.deploymentStrategy === 'Canary'}
                          onChange={(event) => {
                            const checked = event.currentTarget.checked
                            handleLoadBalancerDraftChange(checked)
                          }}
                        />
                      </Group>
                      <Text size="sm" c="lagoon.8" fw={700}>
                        {applicationNetworkDraft.loadBalancerEnabled ? '외부 연결용 LoadBalancer 요청을 저장합니다. floating IP 연결은 다음 단계에서 확인합니다.' : 'ClusterIP로 내부에서만 접근합니다.'}
                      </Text>
                    </div>
                  </SimpleGrid>
                  {applicationNetworkDraft.loadBalancerEnabled ? (
                    <Stack gap="md" mt="lg">
                      <Alert color="orange" radius="md" icon={<IconAlertTriangle size={16} />}>
                        `LoadBalancer`를 켠다고 바로 외부 오픈이 끝나는 것은 아닙니다. AODS는 Service 타입과 GitOps 반영까지 관리하고, 실제 floating IP 연결과 외부 라우팅은 이후 네트워크 계층에서 마무리됩니다.
                      </Alert>
                      <div
                        style={{
                          padding: '18px',
                          borderRadius: '18px',
                          border: '1px solid #fed7aa',
                          background: '#fff7ed',
                        }}
                      >
                        <Group justify="space-between" align="flex-start" mb="md">
                          <div>
                            <Text fw={800}>외부 공개 처리 단계</Text>
                            <Text size="sm" c="dimmed">
                              AODS가 직접 반영하는 단계와, 이후 floating IP가 연결되는 후속 절차를 같이 보여줍니다.
                            </Text>
                          </div>
                          <Badge color="orange" variant="light" radius="sm">비용 영향 있음</Badge>
                        </Group>
                        <div className={classes.progressList}>
                          {loadBalancerExposureWorkflow.map((step) => (
                            <div
                              key={step.title}
                              className={`${classes.progressItem} ${
                                step.state === 'done'
                                  ? classes.progressComplete
                                  : step.state === 'error'
                                    ? classes.progressError
                                  : step.state === 'active'
                                    ? classes.progressActive
                                    : classes.progressPending
                              }`}
                            >
                              <div className={classes.progressMarker}>
                                {step.state === 'done' ? (
                                  <IconCloudCheck size={16} />
                                ) : step.state === 'error' ? (
                                  <IconAlertTriangle size={16} />
                                ) : step.state === 'active' ? (
                                  <IconRefresh size={16} />
                                ) : (
                                  <IconChevronRight size={16} />
                                )}
                              </div>
                              <div style={{ flex: 1 }}>
                                <Group justify="space-between" gap="sm" align="center" mb={4}>
                                  <Text className={classes.progressTitle}>{step.title}</Text>
                                  <Badge
                                    color={
                                      step.owner === 'AODS'
                                        ? 'lagoon.6'
                                        : step.owner === 'Flux'
                                          ? 'blue'
                                          : step.owner === '클러스터'
                                            ? 'teal'
                                            : 'orange'
                                    }
                                    variant="light"
                                    radius="sm"
                                  >
                                    {step.owner}
                                  </Badge>
                                </Group>
                                <div className={classes.progressDetail}>{step.detail}</div>
                              </div>
                            </div>
                          ))}
                        </div>
                        <Divider my="md" />
                        <Group justify="space-between" align="center" gap="md">
                          <Stack gap={2}>
                            <Text fw={800}>외부 인터넷 연결</Text>
                            <Text size="sm" c="dimmed">
                              인터넷에서 접속 가능한 진입점으로 이동합니다.
                            </Text>
                          </Stack>
                          <Button
                            component="a"
                            href={externalInternetConnectionURL}
                            target="_blank"
                            rel="noreferrer"
                            variant="filled"
                            color="lagoon.6"
                            leftSection={<IconExternalLink size={16} />}
                          >
                            외부 인터넷 연결
                          </Button>
                        </Group>
                      </div>
                    </Stack>
                  ) : (
                    <Alert color="blue" radius="md" icon={<IconBolt size={16} />} mt="lg">
                      현재는 내부 전용(ClusterIP) 상태입니다. 비용과 외부 공개 영향이 필요한 서비스만 LoadBalancer 요청을 켜고 저장하세요.
                    </Alert>
                  )}
                  <Button
                    fullWidth
                    variant="light"
                    color="lagoon.6"
                    mt="xl"
                    leftSection={<IconBolt size={16} />}
                    loading={savingApplicationNetwork}
                    disabled={!canAdminProject}
                    onClick={handleSaveApplicationNetwork}
                  >
                    외부 노출 설정 저장
                  </Button>
                </div>

                <div className={classes.surfaceCard}>
                  <Stack gap="xs" mb="md">
                    <Text fw={800}>리소스 할당</Text>
                    <Text size="sm" c="dimmed">
                      새 애플리케이션은 기본 요청값으로 시작합니다. 이 화면에서는 프로젝트 admin이 CPU와 메모리 상한선만 프리셋으로 선택할 수 있습니다.
                    </Text>
                    {!selectedApp?.resources ? (
                      <Alert color="gray" radius="md" icon={<IconCpu size={16} />}>
                        아직 이 앱에 명시적으로 저장된 리소스 설정이 없습니다. 저장하면 기본 요청값과 선택한 상한선이 desired state에 반영됩니다.
                      </Alert>
                    ) : null}
                    {!canAdminProject ? (
                      <Alert color="yellow" radius="md" icon={<IconLock size={16} />}>
                        현재 역할에서는 리소스 할당을 수정할 수 없습니다. project admin만 리소스 상한선을 변경할 수 있습니다.
                      </Alert>
                    ) : null}
                  </Stack>
                  <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
                    <div className={classes.statBadge}>
                      <Text className={classes.statLabel}>기본 CPU 요청</Text>
                      <Text className={classes.statValueSmall}>{defaultApplicationResources.requests?.cpu ?? '250m'}</Text>
                      <Text size="sm" c="dimmed">모든 앱이 기본적으로 확보하는 CPU 요청값입니다.</Text>
                    </div>
                    <div className={classes.statBadge}>
                      <Text className={classes.statLabel}>기본 메모리 요청</Text>
                      <Text className={classes.statValueSmall}>{defaultApplicationResources.requests?.memory ?? '256Mi'}</Text>
                      <Text size="sm" c="dimmed">모든 앱이 기본적으로 확보하는 메모리 요청값입니다.</Text>
                    </div>
                    <Select
                      label="CPU 상한선"
                      description="기본 요청 250m 위에서 사용할 최대 CPU를 선택합니다."
                      data={cpuLimitOptions}
                      value={applicationResourcesDraft.limits?.cpu ?? defaultApplicationResources.limits?.cpu ?? null}
                      allowDeselect={false}
                      disabled={!canAdminProject}
                      onChange={(value) =>
                        setApplicationResourcesDraft((current) => ({
                          ...current,
                          limits: {
                            ...current.limits,
                            cpu: value ?? defaultApplicationResources.limits?.cpu ?? '',
                          },
                        }))
                      }
                    />
                    <Select
                      label="메모리 상한선"
                      description="기본 요청 256Mi 위에서 사용할 최대 메모리를 선택합니다."
                      data={memoryLimitOptions}
                      value={applicationResourcesDraft.limits?.memory ?? defaultApplicationResources.limits?.memory ?? null}
                      allowDeselect={false}
                      disabled={!canAdminProject}
                      onChange={(value) =>
                        setApplicationResourcesDraft((current) => ({
                          ...current,
                          limits: {
                            ...current.limits,
                            memory: value ?? defaultApplicationResources.limits?.memory ?? '',
                          },
                        }))
                      }
                    />
                  </SimpleGrid>
                  <Button
                    fullWidth
                    variant="light"
                    color="lagoon.6"
                    mt="xl"
                    leftSection={<IconCpu size={16} />}
                    loading={savingApplicationResources}
                    disabled={!canAdminProject}
                    onClick={handleSaveApplicationResources}
                  >
                    리소스 할당 저장
                  </Button>
                </div>

                {showEmergencyActionControls && pendingDangerAction ? (
                  <StatePanel
                    kind="error"
                    title={pendingDangerAction === 'abort' ? '배포 강제 중단을 확인하세요' : '직전 버전 롤백을 확인하세요'}
                    description={
                      pendingDangerAction === 'abort'
                        ? '현재 진행 중인 배포를 즉시 중단합니다. 중간 상태가 남을 수 있으니 상황을 확인한 뒤 진행하세요.'
                        : `직전 배포 ${previousDeployment?.imageTag ?? '-'} 기준으로 새 롤백 배포를 생성합니다.`
                    }
                    action={
                      <Group mt="xs">
                        <Button
                          color="red"
                          size="xs"
                          loading={emergencyActionLoading === pendingDangerAction}
                          onClick={() => {
                            if (pendingDangerAction === 'abort') {
                              void handleAbortLatestDeployment()
                            } else {
                              void handleRollbackToPreviousRevision()
                            }
                          }}
                        >
                          확인 후 실행
                        </Button>
                        <Button variant="default" size="xs" onClick={() => setPendingDangerAction(null)}>
                          취소
                        </Button>
                      </Group>
                    }
                  />
                ) : null}

                {showEmergencyActionControls ? (
                  <div className={classes.surfaceCard} style={{ background: '#fef2f2', borderColor: '#fee2e2' }}>
                    <Group gap="sm" mb="sm">
                      <IconShieldCheck size={20} color="#dc2626" />
                      <Text fw={800} c="#b91c1c">긴급 조치</Text>
                    </Group>
                    <Text size="xs" c="#991b1b" mb="md">현재 활성 배포를 중단하거나 즉시 롤백이 필요할 때 사용하세요.</Text>
                    <Group grow>
                      <Button
                        variant="white"
                        color="red"
                        size="xs"
                        loading={emergencyActionLoading === 'abort'}
                        disabled={!latestDeployment || !canDeployInProject}
                        onClick={() => setPendingDangerAction('abort')}
                      >
                        배포 강제 중단
                      </Button>
                      <Button
                        variant="filled"
                        color="red"
                        size="xs"
                        loading={emergencyActionLoading === 'rollback'}
                        disabled={!previousDeployment || !canDeployInProject}
                        onClick={() => setPendingDangerAction('rollback')}
                      >
                        직전 버전 롤백
                      </Button>
                    </Group>
                  </div>
                ) : null}

                {showApplicationLifecycleControls && pendingLifecycleAction ? (
                  <StatePanel
                    kind="error"
                    title={pendingLifecycleAction === 'archive' ? '애플리케이션 보관을 확인하세요' : '애플리케이션 삭제를 확인하세요'}
                    description={
                      pendingLifecycleAction === 'archive'
                        ? 'archive는 앱을 목록에서 숨기지만 기록과 최종 secret은 남길 수 있습니다.'
                        : 'delete는 앱 manifest와 연결 secret cleanup까지 포함하는 더 파괴적인 작업입니다.'
                    }
                    action={
                      <Group mt="xs">
                        <Button
                          color="red"
                          size="xs"
                          loading={lifecycleActionLoading === pendingLifecycleAction}
                          onClick={() => {
                            if (pendingLifecycleAction === 'archive') {
                              void handleArchiveApplication()
                            } else {
                              void handleDeleteApplication()
                            }
                          }}
                        >
                          확인 후 실행
                        </Button>
                        <Button variant="default" size="xs" onClick={() => setPendingLifecycleAction(null)}>
                          취소
                        </Button>
                      </Group>
                    }
                  />
                ) : null}

                {showApplicationLifecycleControls ? (
                  <div className={classes.surfaceCard}>
                    <Group justify="space-between" mb="sm" align="start">
                      <div>
                        <Text fw={800}>애플리케이션 lifecycle</Text>
                        <Text size="sm" c="dimmed" mt={4}>
                          보관과 삭제는 프로젝트 관리자만 실행할 수 있습니다.
                        </Text>
                      </div>
                      <Badge color={canAdminProject ? 'lagoon.6' : 'gray'} variant="light">
                        {canAdminProject ? 'Admin only' : '권한 부족'}
                      </Badge>
                    </Group>
                    <Group grow>
                      <Button
                        variant="light"
                        color="yellow"
                        disabled={!canAdminProject}
                        loading={lifecycleActionLoading === 'archive'}
                        onClick={() => setPendingLifecycleAction('archive')}
                      >
                        애플리케이션 보관
                      </Button>
                      <Button
                        color="red"
                        disabled={!canAdminProject}
                        loading={lifecycleActionLoading === 'delete'}
                        onClick={() => setPendingLifecycleAction('delete')}
                      >
                        애플리케이션 삭제
                      </Button>
                    </Group>
                    {!canAdminProject ? (
                      <Text size="sm" c="dimmed" mt="md">
                        deployer는 배포 운영까지만 가능하고, archive/delete는 admin 역할에서만 허용됩니다.
                      </Text>
                    ) : null}
                  </div>
                ) : null}
              </Stack>
            </Tabs.Panel>
          </Tabs>
        </ScrollArea>
      </Drawer>

      <Drawer
        opened={showProjectComposer && projectComposerOpen}
        onClose={() => setProjectComposerOpen(false)}
        position="right"
        size="min(720px, calc(100vw - 24px))"
        title={<Text fw={900} size="lg">새 프로젝트 생성</Text>}
      >
        <Stack gap="lg">
          <div>
            <Text className={classes.sectionEyebrow}>프로젝트 카탈로그 등록</Text>
            <Text size="sm" c="dimmed" mt={6}>
              platform admin만 새 프로젝트를 추가할 수 있습니다. 프로젝트 이름은 영문 소문자 slug 규칙으로 만들고, 같은 값이 프로젝트 ID와 Kubernetes namespace로 함께 사용됩니다.
            </Text>
          </div>

          <Alert color="lagoon.6" radius="md" icon={<IconPlus size={16} />}>
            기본 환경과 정책은 서버 기본값으로 초기화됩니다. 프로젝트를 만든 뒤 설정 화면에서 운영 규칙은 바로 수정할 수 있지만, 이름과 namespace는 생성 후 고정됩니다.
          </Alert>

          <TextInput
            label="프로젝트 이름"
            description="영문 소문자, 숫자, 하이픈만 사용합니다. 이 값이 프로젝트 ID와 namespace로 같이 쓰입니다."
            placeholder="예: billing-api"
            value={projectDraft.name}
            onChange={(event) => {
              const value = normalizeProjectSlugInput(event.currentTarget.value)
              setProjectDraft((current) => ({ ...current, id: value, name: value }))
            }}
          />
          <TextInput
            label="생성될 네임스페이스"
            value={projectDraft.name}
            readOnly
            placeholder="프로젝트 이름을 입력하면 동일한 값으로 생성됩니다."
          />
          <TextInput
            label="설명"
            placeholder="선택 사항"
            value={projectDraft.description ?? ''}
            onChange={(event) => {
              const value = event.currentTarget.value
              setProjectDraft((current) => ({ ...current, description: value }))
            }}
          />

          <Group grow>
            <Button variant="default" onClick={() => setProjectComposerOpen(false)}>
              닫기
            </Button>
            <Button color="lagoon.6" loading={creatingProject} onClick={handleCreateProject}>
              프로젝트 생성
            </Button>
          </Group>
        </Stack>
      </Drawer>

      {/* New Application Wizard Drawer */}
      <Drawer
        opened={wizardOpened}
        onClose={() => setWizardOpened(false)}
        position="right"
        size="min(860px, calc(100vw - 24px))"
        title={<Text fw={900} size="lg">새 애플리케이션 생성</Text>}
      >
        <ApplicationWizard
          key={[
            selectedProjectId || 'no-project',
            wizardOpened ? 'open' : 'closed',
            wizardInitialState.sourceMode,
            wizardInitialState.repositoryUrl || 'no-repo',
            wizardInitialState.environment || 'no-env',
          ].join(':')}
          projectId={selectedProjectId || undefined}
          projectName={selectedProject?.name}
          environments={environments}
          allowedStrategies={supportedDeploymentStrategies}
          initialState={wizardInitialState}
          onPreviewSource={handlePreviewAppSource}
          onVerifyImageAccess={handleVerifyAppImageAccess}
          onSubmit={handleCreateApp}
          onCancel={() => setWizardOpened(false)}
          submitting={creatingApplication}
        />
      </Drawer>

      <Drawer
        opened={projectSettingsOpened}
        onClose={() => {
          setProjectSettingsOpened(false)
          setPendingProjectDelete(false)
        }}
        position="right"
        size="min(1120px, calc(100vw - 24px))"
        title={<Text fw={900} size="lg">{selectedProject?.name || '프로젝트'} 설정</Text>}
      >
        <ProjectSettingsPanel
          project={selectedProject}
          environments={environments}
          projectPolicy={projectPolicy}
          canEditPolicies={canAdminProject}
          savingPolicies={savingProjectPolicy}
          onSavePolicies={(policy) => void handleUpdateProjectPolicy(policy)}
          applicationCount={applications.length}
          canDeleteProject={canDeleteProject}
          isProtectedProject={isProtectedProject}
          deletingProject={deletingProject}
          pendingProjectDelete={pendingProjectDelete}
          onRequestProjectDelete={() => setPendingProjectDelete(true)}
          onCancelProjectDelete={() => setPendingProjectDelete(false)}
          onConfirmProjectDelete={() => void handleDeleteProject()}
        />
      </Drawer>
    </>
  )
}

function formatLatestMetric(metrics: ApplicationMetricsResponse | null, key: string): string {
  const series = metrics?.metrics?.find(m => m.key === key)
  if (!series || series.points.length === 0) return '데이터 없음'
  const val = [...series.points].reverse().find(p => p.value !== null)?.value
  if (val == null) return '데이터 없음'
  if (key === 'cpu_usage') {
    return val < 1 ? `${(val * 1000).toFixed(1)}` : val.toFixed(2)
  }
  return val.toFixed(1)
}

function latestMetricUnit(metrics: ApplicationMetricsResponse | null, key: string) {
  const series = metrics?.metrics?.find((metric) => metric.key === key)
  const val = series ? [...series.points].reverse().find((point) => point.value !== null)?.value : null
  if (key === 'cpu_usage' && val != null && val < 1) {
    return 'mCPU'
  }
  return series?.unit || ''
}

function aggregateMetricSeries(series: MetricSeries[]): MetricSeries[] {
  const grouped = new Map<string, MetricSeries[]>()
  for (const item of series) {
    const current = grouped.get(item.key) ?? []
    current.push(item)
    grouped.set(item.key, current)
  }

  return Array.from(grouped.entries()).map(([key, items]) => {
    const template = items[0]
    const maxLength = Math.max(...items.map((item) => item.points.length))
    const points = Array.from({ length: maxLength }, (_, index) => {
      const candidates = items
        .map((item) => item.points[index])
        .filter((point): point is MetricSeries['points'][number] => Boolean(point))
      const timestamp = candidates[0]?.timestamp ?? new Date().toISOString()
      const values = candidates
        .map((point) => point.value)
        .filter((value): value is number => value !== null)
      const total = values.reduce((sum, value) => sum + value, 0)
      return {
        timestamp,
        value: values.length > 0 ? total : null,
      }
    })
    return {
      key,
      label: template.label,
      unit: template.unit,
      points,
    }
  })
}

function findMetricSeries(series: MetricSeries[], key: string) {
  return series.find((item) => item.key === key)
}

function latestMetricNumber(series: MetricSeries) {
  const value = [...series.points].reverse().find((point) => point.value !== null)?.value
  return value ?? null
}

function hasMetricValues(series: MetricSeries) {
  return series.points.some((point) => point.value !== null)
}

function findHealthSignal(health: ApplicationHealthSnapshot, key: string): HealthSignal | undefined {
  return health.signals.find((signal) => signal.key === key)
}

function findCatalogHealthSignal(signal: ApplicationCatalogSignal | undefined, key: string): HealthSignal | undefined {
  return signal?.healthSignals.find((item) => item.key === key)
}

function applicationSyncIssue(app: ApplicationSummary, signal?: ApplicationCatalogSignal) {
  const syncSignal = findCatalogHealthSignal(signal, 'sync')
  const fallbackSignal = findCatalogHealthSignal(signal, 'health')

  if (app.syncStatus === 'Unknown') {
    return syncSignal?.message || fallbackSignal?.message || 'Flux sync 상태를 확인할 수 없습니다.'
  }
  if (app.syncStatus === 'Degraded') {
    return syncSignal?.message || 'Flux sync 상태가 degraded 입니다.'
  }
  if (syncSignal?.status === 'Unavailable' || syncSignal?.status === 'Critical') {
    return syncSignal.message
  }
  return ''
}

function applicationSyncIssueColor(status: SyncStatus) {
  if (status === 'Degraded') return 'red'
  if (status === 'Unknown') return 'gray'
  return 'yellow'
}

function metricsStateFromHealth(health: ApplicationHealthSnapshot): ApplicationCatalogSignalState {
  const signal = findHealthSignal(health, 'metrics')
  if (signal?.status === 'Unavailable') return 'failed'
  if (health.metrics.some((series) => hasMetricValues(series))) return 'available'
  return 'empty'
}

function deploymentStateFromHealth(health: ApplicationHealthSnapshot): ApplicationCatalogSignalState {
  const signal = findHealthSignal(health, 'deployment')
  if (signal?.status === 'Unavailable') return 'failed'
  return health.latestDeployment ? 'available' : 'empty'
}

function hasApplicationMetrics(signal?: ApplicationCatalogSignal) {
  if (!signal || signal.metricsState !== 'available') return false
  return signal.metrics.some((series) => hasMetricValues(series))
}

function formatApplicationMetricSummary(signal: ApplicationCatalogSignal | undefined, key: string) {
  if (!signal) return '불러오는 중'
  if (signal.metricsState === 'failed') return '조회 실패'
  if (signal.metricsState === 'empty') return '연동 없음'

  const series = findMetricSeries(signal.metrics, key)
  if (!series) return '데이터 없음'
  const value = latestMetricNumber(series)
  if (value === null) return '데이터 없음'

  switch (key) {
    case 'cpu_usage':
      return value < 1 ? `${(value * 1000).toFixed(0)} mCPU` : `${value.toFixed(2)} cores`
    case 'memory_usage':
      return `${value.toFixed(1)} MiB`
    case 'request_rate':
      return `${value.toFixed(1)} RPM`
    default:
      return value.toFixed(1)
  }
}

function formatLatestDeploymentLabel(signal?: ApplicationCatalogSignal) {
  if (!signal) return '불러오는 중'
  if (signal.deploymentState === 'failed') return '조회 실패'
  if (!signal.latestDeployment) return '이력 없음'
  return signal.latestDeployment.imageTag
}

function formatLatestDeploymentStatus(signal?: ApplicationCatalogSignal) {
  if (!signal) return '불러오는 중'
  if (signal.deploymentState === 'failed') return '조회 실패'
  if (!signal.latestDeployment) return '배포 없음'
  return signal.latestDeployment.status
}

function applicationMetricBadgeColor(signal?: ApplicationCatalogSignal) {
  if (!signal) return 'gray'
  if (signal.metricsState === 'failed') return 'red'
  if (signal.metricsState === 'empty') return 'gray'
  return 'green'
}

function applicationMetricBadgeLabel(signal?: ApplicationCatalogSignal) {
  if (!signal) return '지표 불러오는 중'
  if (signal.metricsState === 'failed') return '지표 조회 실패'
  if (signal.metricsState === 'empty') return '메트릭 연동 없음'
  return '실측 지표 수집 중'
}

function applicationMetricHelperText(signal?: ApplicationCatalogSignal) {
  if (!signal) return '애플리케이션 요약 정보를 불러오는 중입니다.'
  if (signal.metricsState === 'failed') return '이 애플리케이션의 메트릭 요약을 불러오지 못했습니다.'
  if (signal.metricsState === 'empty') return '현재 수집된 실측 메트릭이 없습니다.'
  return '실측 메트릭 값이 들어오면 카드에서 바로 확인할 수 있습니다.'
}

function formatMetricSeriesValue(series: MetricSeries[], key: string) {
  const item = findMetricSeries(series, key)
  const value = item ? latestMetricNumber(item) : null
  if (value === null) return '데이터 없음'

  switch (key) {
    case 'cpu_usage':
      return value < 1 ? `${(value * 1000).toFixed(1)}` : value.toFixed(2)
    default:
      return value.toFixed(1)
  }
}

function metricSeriesUnit(series: MetricSeries[], key: string) {
  const item = findMetricSeries(series, key)
  const value = item ? latestMetricNumber(item) : null
  if (key === 'cpu_usage' && value != null && value < 1) {
    return 'mCPU'
  }
  return item?.unit || ''
}

function buildApplicationCatalogSummary(
  app: ApplicationSummary,
  namespace: string,
  signal?: ApplicationCatalogSignal,
) {
  const parts = [
    `${namespace} 네임스페이스에서 ${app.deploymentStrategy === 'Canary' ? '카나리아' : '롤아웃'} 전략으로 운영 중입니다.`,
    describeLoadBalancerExposure(app.loadBalancerEnabled, app.syncStatus),
  ]

  if (!signal) {
    parts.push('최근 배포 요약을 불러오는 중입니다.')
    return parts.join(' ')
  }

  if (signal.latestDeployment) {
    parts.push(`최근 배포는 ${signal.latestDeployment.imageTag}이고 현재 상태는 ${signal.latestDeployment.status}입니다.`)
  } else if (signal.deploymentState === 'failed') {
    parts.push('최근 배포 요약을 불러오지 못했습니다.')
  } else {
    parts.push('아직 기록된 배포 이력이 없습니다.')
  }

  return parts.join(' ')
}

function formatLoadBalancerBadgeLabel(enabled: boolean) {
  return enabled ? 'LB 요청' : '내부 전용'
}

function buildResourceLimitOptions(
  selectedValue: string | undefined,
  presetOptions: Array<{ value: string; label: string }>,
) {
  const normalized = selectedValue?.trim()
  if (!normalized) {
    return presetOptions
  }
  if (presetOptions.some((option) => option.value === normalized)) {
    return presetOptions
  }
  return [{ value: normalized, label: `현재 저장값 ${normalized}` }, ...presetOptions]
}

function filterVisibleProjects(projects: ProjectSummary[]) {
  return projects.filter((project) => isSharedProject(project))
}

function describeLoadBalancerExposure(enabled: boolean, syncStatus?: SyncStatus) {
  if (!enabled) {
    return '현재는 내부 전용 서비스로 운영 중입니다.'
  }
  if (syncStatus === 'Synced') {
    return 'LoadBalancer 요청이 클러스터에 반영되었습니다. floating IP와 외부 라우팅 연결은 별도 확인이 필요합니다.'
  }
  if (syncStatus === 'Syncing') {
    return 'LoadBalancer 요청을 클러스터에 반영 중입니다.'
  }
  return 'LoadBalancer 요청이 저장되어 있으며, 클러스터 반영과 외부 네트워크 연결 확인이 남아 있습니다.'
}

function collectExposureAddresses(exposure?: NetworkExposureResponse | null) {
  return exposure?.addresses?.filter((value) => value.trim() !== '') ?? []
}

function formatExposurePortMappings(exposure?: NetworkExposureResponse | null) {
  const ports = exposure?.ports ?? []
  return ports
    .filter((port) => port.port > 0)
    .map((port) => {
      const externalPort = `${port.port}${port.protocol ? `/${port.protocol}` : ''}`
      const targetPort = port.targetPort?.trim()
      if (targetPort && targetPort !== String(port.port)) {
        return `${externalPort} -> ${targetPort}`
      }
      return externalPort
    })
}

function buildLoadBalancerExposureWorkflow(
  persistedEnabled: boolean,
  draftEnabled: boolean,
  syncStatus?: SyncStatus,
  exposure?: NetworkExposureResponse | null,
): LoadBalancerExposureStep[] {
  const exposureRequested = persistedEnabled || draftEnabled
  const addresses = collectExposureAddresses(exposure)
  const portMappings = formatExposurePortMappings(exposure)

  let clusterDetail: ReactNode = !exposureRequested
    ? '클러스터가 Service type=LoadBalancer를 요청받으면 이 단계가 시작됩니다.'
    : persistedEnabled && syncStatus === 'Synced'
      ? '클러스터 또는 인프라 계층에서 실제 LoadBalancer와 연결 포인트를 준비하는 단계입니다.'
      : '클러스터 반영이 끝난 뒤 LoadBalancer 준비 여부를 확인합니다.'
  let clusterState: LoadBalancerExposureStepState =
    !exposureRequested ? 'pending' : persistedEnabled && syncStatus === 'Synced' ? 'active' : 'pending'
  let networkDetail: ReactNode = exposureRequested
    ? 'AODS 밖의 외부 네트워크 계층에서 floating IP, 도메인, 보안 정책 연결을 마무리해야 합니다.'
    : '외부 공개가 필요할 때만 floating IP와 외부 라우팅 절차를 진행합니다.'
  let networkState: LoadBalancerExposureStepState = 'pending'

  if (persistedEnabled && exposure) {
    clusterDetail = exposure.message?.trim() || clusterDetail
    if (exposure.lastEvent?.reason) {
      clusterDetail = `${clusterDetail} 최근 이벤트: ${exposure.lastEvent.reason}.`
    }

    switch (exposure.status) {
      case 'Ready':
        clusterState = 'done'
        networkState = 'active'
        networkDetail = addresses.length > 0
          ? (
            <>
              클러스터에서 LoadBalancer 주소{' '}
              <strong className={classes.progressHighlight}>
                {addresses.join(', ')}
              </strong>
              {portMappings.length > 0 ? (
                <>
                  {' '}가 준비되었습니다. 현재 노출 포트는{' '}
                  <strong className={classes.progressHighlight}>
                    {portMappings.join(', ')}
                  </strong>
                  입니다.
                </>
              ) : (
                ' 가 준비되었습니다.'
              )}
              {' '}이후 외부 네트워크 계층에서 floating IP, 도메인, 라우팅 연결을 마무리하세요.
            </>
          )
          : '클러스터 LoadBalancer 준비는 끝났습니다. 이후 외부 네트워크 계층에서 floating IP와 라우팅 연결을 마무리하세요.'
        break
      case 'Provisioning':
        clusterState = 'active'
        networkState = 'pending'
        networkDetail = '클러스터에서 외부 주소를 준비하는 중입니다. 준비가 끝나면 floating IP와 외부 라우팅 연결 단계로 넘어갑니다.'
        break
      case 'Error':
        clusterState = 'error'
        networkState = 'pending'
        networkDetail = '클러스터 LoadBalancer 준비 오류를 먼저 해결해야 floating IP와 외부 라우팅 연결을 진행할 수 있습니다.'
        break
      case 'Pending':
        clusterState = syncStatus === 'Synced' ? 'active' : 'pending'
        networkState = 'pending'
        networkDetail = 'Service type=LoadBalancer 반영과 외부 주소 준비가 끝난 뒤 floating IP와 외부 라우팅 연결을 진행합니다.'
        break
      default:
        clusterState = 'pending'
        networkState = 'pending'
        break
    }
  }

  return [
    {
      title: 'AODS 설정 저장',
      owner: 'AODS',
      detail: persistedEnabled
        ? '현재 애플리케이션 metadata에 LoadBalancer 요청이 저장되어 있습니다.'
        : draftEnabled
          ? '트래픽 설정 저장을 누르면 LoadBalancer 요청이 GitOps 대상에 반영됩니다.'
          : '현재는 내부 전용 정책이라 외부 공개 요청이 없습니다.',
      state: persistedEnabled ? 'done' : draftEnabled ? 'active' : 'pending',
    },
    {
      title: 'Flux / 클러스터 반영',
      owner: 'Flux',
      detail: !exposureRequested
        ? 'LoadBalancer 요청이 저장되면 Service 타입 반영이 시작됩니다.'
        : persistedEnabled
          ? syncStatus === 'Synced'
            ? '클러스터 쪽 desired state 반영은 끝났습니다.'
            : '현재 GitOps 반영 또는 sync 상태를 확인하는 단계입니다.'
          : '아직 저장 전이라 클러스터 반영이 시작되지 않았습니다.',
      state: !exposureRequested ? 'pending' : persistedEnabled ? (syncStatus === 'Synced' ? 'done' : 'active') : 'pending',
    },
    {
      title: 'LoadBalancer 준비',
      owner: '클러스터',
      detail: clusterDetail,
      state: clusterState,
    },
    {
      title: 'Floating IP / 외부 라우팅 연결',
      owner: '네트워크',
      detail: networkDetail,
      state: networkState,
    },
  ]
}

function formatRepositoryPollCheckedAt(poll?: RepositoryPollStatus | null) {
  if (!poll?.enabled) {
    return '사용 안 함'
  }
  if (!poll.lastCheckedAt) {
    return '아직 없음'
  }
  return formatDateTimeValue(poll.lastCheckedAt)
}

function formatRepositoryPollNextAt(poll?: RepositoryPollStatus | null) {
  if (!poll?.enabled) {
    return '사용 안 함'
  }
  if (!poll.nextScheduledAt) {
    return '계산 중'
  }
  return formatDateTimeValue(poll.nextScheduledAt)
}

function describeRepositoryPoll(poll?: RepositoryPollStatus | null) {
  if (!poll?.enabled) {
    return '이 애플리케이션은 저장소 polling 대상이 아닙니다.'
  }

  const interval = formatRepositoryPollInterval(poll.intervalSeconds)
  const source = poll.source ? `${poll.source} 기준` : '저장소 기준'

  if (poll.lastResult === 'Error') {
    const lastChecked = poll.lastCheckedAt ? formatDateTimeValue(poll.lastCheckedAt) : '최근 확인 시각 없음'
    return `${source} ${interval} 간격으로 확인합니다. 최근 polling은 ${lastChecked}에 실패했고, 원인은 ${poll.lastError || '알 수 없음'} 입니다.`
  }

  if (!poll.lastCheckedAt) {
    return `${source} ${interval} 간격으로 확인합니다. 첫 polling을 아직 수행하지 않았습니다.`
  }

  const lastChecked = formatDateTimeValue(poll.lastCheckedAt)
  const nextScheduled = poll.nextScheduledAt ? formatDateTimeValue(poll.nextScheduledAt) : '계산 중'
  const successText = poll.lastSucceededAt ? ` 최근 성공은 ${formatDateTimeValue(poll.lastSucceededAt)}입니다.` : ''

  return `${source} ${interval} 간격으로 확인합니다. 최근 저장소 확인은 ${lastChecked}, 다음 예정 시각은 ${nextScheduled}입니다.${successText}`
}

function formatRepositoryPollInterval(intervalSeconds?: number) {
  if (!intervalSeconds || intervalSeconds <= 0) {
    return '미정'
  }
  if (intervalSeconds % 3600 === 0) {
    return `${intervalSeconds / 3600}시간`
  }
  if (intervalSeconds % 60 === 0) {
    return `${intervalSeconds / 60}분`
  }
  return `${intervalSeconds}초`
}

function formatDateTimeValue(value?: string) {
  if (!value) {
    return '-'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return '-'
  }
  return new Intl.DateTimeFormat('ko-KR', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(date)
}

function countApplicationsByStatus(applications: ApplicationSummary[], status: SyncStatus) {
  return applications.filter((application) => application.syncStatus === status).length
}

function canRoleDeploy(role?: ProjectSummary['role']) {
  return role === 'admin' || role === 'deployer'
}

function canRoleAdmin(role?: ProjectSummary['role']) {
  return role === 'admin'
}

function hasPlatformAdmin(user: CurrentUser | null) {
  return Boolean(user?.groups?.some((group) => platformAdminAuthorities.has(group)))
}

function parseAuthorityList(raw: string) {
  const seen = new Set<string>()
  return raw
    .split(',')
    .map((value) => value.trim())
    .filter((value) => {
      if (!value || seen.has(value)) {
        return false
      }
      seen.add(value)
      return true
    })
}

function cloneProjectPolicy(policy: ProjectPolicy | null) {
  if (!policy) return null
  return {
    ...policy,
    allowedEnvironments: [...policy.allowedEnvironments],
    allowedDeploymentStrategies: [...supportedDeploymentStrategies],
    allowedClusterTargets: [...policy.allowedClusterTargets],
  }
}

function normalizeProjectSlugInput(raw: string) {
  return raw
    .toLowerCase()
    .replace(/[_\s]+/g, '-')
    .replace(/[^a-z0-9-]/g, '')
    .replace(/-+/g, '-')
    .replace(/^-+/, '')
    .replace(/-+$/, '')
}

function isSharedProject(project: ProjectSummary | undefined | null) {
  return project?.namespace?.trim().toLowerCase() === 'shared'
}

function defaultProjectTab(): ProjectTab {
  return 'applications'
}

function sortChangeRecords(changes: ChangeRecord[]) {
  return [...changes].sort((left, right) => {
    const updatedDiff = Date.parse(right.updatedAt) - Date.parse(left.updatedAt)
    if (updatedDiff !== 0) {
      return updatedDiff
    }
    return Date.parse(right.createdAt) - Date.parse(left.createdAt)
  })
}

function upsertChangeRecord(changes: ChangeRecord[], record: ChangeRecord) {
  return sortChangeRecords([
    ...changes.filter((change) => change.id !== record.id),
    record,
  ])
}

function readChangeCache() {
  if (typeof window === 'undefined') {
    return {} as Record<string, ChangeRecord[]>
  }
  try {
    const raw = window.sessionStorage.getItem('aods:tracked-changes:v1')
    if (!raw) {
      return {} as Record<string, ChangeRecord[]>
    }
    return JSON.parse(raw) as Record<string, ChangeRecord[]>
  } catch {
    return {} as Record<string, ChangeRecord[]>
  }
}

function readProjectTrackedChanges(projectId: string) {
  return sortChangeRecords(readChangeCache()[projectId] ?? [])
}

function writeProjectTrackedChanges(projectId: string, changes: ChangeRecord[]) {
  if (typeof window === 'undefined') {
    return
  }
  const cache = readChangeCache()
  cache[projectId] = sortChangeRecords(changes)
  window.sessionStorage.setItem('aods:tracked-changes:v1', JSON.stringify(cache))
}

function deploymentStatusColor(status: string) {
  switch (status) {
    case 'Completed':
    case 'Promoted':
      return 'green'
    case 'Aborted':
    case 'Failed':
      return 'red'
    case 'Created':
    case 'Running':
      return 'yellow'
    default:
      return 'gray'
  }
}

function formatDeploymentStatusLabel(status?: string) {
  switch (status) {
    case 'Completed':
      return '완료'
    case 'Promoted':
      return '승격 완료'
    case 'Aborted':
      return '중단됨'
    case 'Failed':
      return '실패'
    case 'Created':
      return '기록됨'
    case 'Running':
      return '진행 중'
    default:
      return status || '없음'
  }
}

function formatSyncStatusLabel(status?: SyncStatus) {
  switch (status) {
    case 'Synced':
      return '동기화 완료'
    case 'Syncing':
      return '동기화 중'
    case 'Degraded':
      return '문제 발생'
    case 'Unknown':
      return '확인 불가'
    default:
      return '확인 불가'
  }
}

function runtimeReadinessColor(syncStatus?: SyncStatus, deployment?: DeploymentRecord) {
  if (syncStatus === 'Degraded' || deployment?.status === 'Failed' || deployment?.status === 'Aborted') return 'red'
  if (syncStatus === 'Synced' && (deployment?.status === 'Completed' || deployment?.status === 'Promoted')) return 'green'
  if (syncStatus === 'Syncing' || deployment?.status === 'Created' || deployment?.status === 'Running') return 'yellow'
  return 'gray'
}

function describeRuntimeReadiness(syncStatus?: SyncStatus, deployment?: DeploymentRecord) {
  if (!deployment) {
    return '배포 이력이 아직 없어 runtime 준비 상태를 판단할 수 없습니다.'
  }
  if (syncStatus === 'Degraded' || deployment.status === 'Failed' || deployment.status === 'Aborted') {
    return deployment.message || 'GitOps 동기화 또는 최근 배포에 문제가 있습니다. 관측 탭의 이벤트와 로그를 확인하세요.'
  }
  if (syncStatus === 'Synced' && (deployment.status === 'Completed' || deployment.status === 'Promoted')) {
    return 'GitOps 동기화와 최근 배포 완료 기록이 모두 정상입니다. 실제 트래픽과 로그는 관측 탭에서 이어서 확인하세요.'
  }
  if (syncStatus === 'Synced') {
    return 'GitOps 동기화는 완료됐지만 최근 배포가 아직 완료 상태로 기록되지 않았습니다.'
  }
  if (syncStatus === 'Syncing') {
    return 'Flux가 desired state를 반영하는 중입니다. 완료 후 배포 이력과 관측 신호를 다시 확인하세요.'
  }
  return '동기화 상태를 아직 확인하지 못했습니다. 백엔드와 Flux 연동 상태를 확인하세요.'
}

function formatSecretVersionStatus(status: string) {
  switch (status) {
    case 'destroyed':
      return '파기됨'
    case 'deleted':
      return '삭제됨'
    case 'current':
      return '현재'
    case 'available':
      return '복원 가능'
    default:
      return status
  }
}

function formatProjectRole(role: ProjectSummary['role']) {
  switch (role) {
    case 'admin':
      return '관리자'
    case 'deployer':
      return '배포자'
    default:
      return '조회자'
  }
}

function syncStageState(status?: SyncStatus) {
  switch (status) {
    case 'Synced':
      return 'complete'
    case 'Syncing':
      return 'active'
    case 'Degraded':
      return 'error'
    default:
      return 'pending'
  }
}

function rolloutStageState(deployment?: DeploymentRecord) {
  if (!deployment) return 'pending'
  if (deployment.status === 'Aborted') return 'error'
  if (deployment.status === 'Completed' || deployment.status === 'Promoted') return 'complete'
  if (deployment.status === 'Created') return 'active'
  return 'pending'
}

function rolloutStageMessage(deployment?: DeploymentRecord) {
  if (!deployment) return '대기 중...'
  if (deployment.rolloutPhase) return deployment.rolloutPhase
  if (deployment.message) return deployment.message
  if (deployment.status === 'Completed' || deployment.status === 'Promoted') {
    return '새 버전 반영이 완료되었습니다.'
  }
  if (deployment.status === 'Aborted') {
    return '배포가 중단되었습니다.'
  }
  return '추가 롤아웃 상태를 확인할 수 없습니다.'
}

function deploymentStageClass(
  state: 'complete' | 'active' | 'pending' | 'error',
  styleClasses: Record<string, string>,
) {
  switch (state) {
    case 'complete':
      return styleClasses.progressComplete
    case 'active':
      return styleClasses.progressActive
    case 'error':
      return styleClasses.progressError
    default:
      return styleClasses.progressPending
  }
}

function parseEnvEntries(text: string): SecretDraftEntry[] {
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

function toOptionalNumber(value: string | number) {
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : undefined
  }
  if (value.trim() === '') {
    return undefined
  }
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}
