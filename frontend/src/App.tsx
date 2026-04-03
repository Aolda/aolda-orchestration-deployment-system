import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  AppShell,
  Avatar,
  Badge,
  Button,
  Container,
  Drawer,
  Grid,
  Group,
  Loader,
  NumberInput,
  PasswordInput,
  ScrollArea,
  SegmentedControl,
  SimpleGrid,
  Stack,
  Switch,
  Table,
  Tabs,
  Text,
  TextInput,
  UnstyledButton,
} from '@mantine/core'
import { notifications } from '@mantine/notifications'
import {
  IconActivity,
  IconArrowLeft,
  IconBox,
  IconCloudCheck,
  IconGitBranch,
  IconHistory,
  IconLayoutDashboard,
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
import { api } from './api/client'
import classes from './App.module.css'
import { ApplicationWizard, type CreateFormState } from './components/ApplicationWizard'
import type {
  ApplicationMetricsResponse,
  ApplicationSummary,
  CurrentUser,
  DeploymentRecord,
  EnvironmentSummary,
  EventListResponse,
  MetricSeries,
  ProjectPolicy,
  ProjectSummary,
  RepositorySummary,
  RollbackPolicy,
  SyncStatus,
  SyncStatusResponse,
} from './types/api'

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

const MetricCard = ({ label, value, unit, points, color, active, onClick }: { label: string; value: string; unit: string; points?: { value: number | null }[]; color: string; active?: boolean; onClick?: () => void }) => (
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
    {points && (
      <div className={classes.sparklineWrapper}>
        <Sparkline points={points} color={color} />
      </div>
    )}
  </div>
)

const MetricDataTable = ({ series }: { series: { key: string; label: string; unit: string; points: { timestamp: string; value: number | null }[] } | undefined }) => {
  if (!series || !series.points) return null

  const reversedPoints = [...series.points].reverse().filter(p => p.value !== null) as { timestamp: string; value: number }[]

  return (
    <div className={classes.surfaceCard} style={{ marginTop: '20px' }}>
      <Group justify="space-between" mb="md">
        <Text fw={800} size="sm">{series.label} 상세 데이터</Text>
        <Badge variant="light" color="lagoon.6">{series.unit}</Badge>
      </Group>
      <ScrollArea h={300} offsetScrollbars>
        <Table className={classes.dataTable}>
          <Table.Thead>
            <Table.Tr>
              <Table.Th style={{ width: '220px' }}>수집 시각</Table.Th>
              <Table.Th>실시간 수치</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {reversedPoints.map((p, idx: number) => (
              <Table.Tr key={idx}>
                <Table.Td>
                  <Text size="xs" ff="monospace">
                    {new Date(p.timestamp).toLocaleTimeString('ko-KR', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                  </Text>
                </Table.Td>
                <Table.Td>
                  <Text size="sm" fw={700} c="lagoon.9">
                    {p.value?.toFixed(keyToDecimal(series.key))}
                  </Text>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </ScrollArea>
    </div>
  )
}

function keyToDecimal(key: string): number {
  if (key === 'cpu_usage') return 3
  if (key === 'error_rate') return 2
  return 1
}

// --- Login Form Component ---

const LoginForm = ({ onLogin }: { onLogin: () => void }) => {
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('admin')
  const [error, setError] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (username === 'admin' && password === 'admin') {
      onLogin()
    } else {
      setError('아이디 또는 비밀번호가 올바르지 않습니다.')
    }
  }

  return (
    <div className={classes.authShell}>
      <div className={classes.loginCard}>
        <Stack gap="xl">
          <div style={{ textAlign: 'center' }}>
            <Badge size="lg" variant="light" color="lagoon.6" mb="md">AOLDA PORTAL</Badge>
            <Text size="xl" fw={900}>내부 배포 관리 플랫폼</Text>
            <Text size="sm" c="dimmed" mt={4}>관리자 계정으로 로그인이 필요합니다.</Text>
          </div>

          <form onSubmit={handleSubmit}>
            <Stack gap="md">
              <TextInput label="사용자 아이디" value={username} onChange={(e) => setUsername(e.target.value)} placeholder="admin" required leftSection={<IconUser size={16} />} />
              <PasswordInput label="비밀번호" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="admin" required leftSection={<IconLock size={16} />} />
              {error && <Alert color="red">{error}</Alert>}
              <Button fullWidth size="md" color="lagoon.6" type="submit" mt="md">로그인</Button>
            </Stack>
          </form>
        </Stack>
      </div>
    </div>
  )
}

// --- Main App Component ---

export default function App() {
  const [isLoggedIn, setIsLoggedIn] = useState(false)
  const [currentUser, setCurrentUser] = useState<CurrentUser | null>(null)
  const [projects, setProjects] = useState<ProjectSummary[]>([])
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)
  const [applications, setApplications] = useState<ApplicationSummary[]>([])
  const [environments, setEnvironments] = useState<EnvironmentSummary[]>([])
  const [repositories, setRepositories] = useState<RepositorySummary[]>([])
  const [projectPolicy, setProjectPolicy] = useState<ProjectPolicy | null>(null)
  const [selectedAppId, setSelectedAppId] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  // Application Details State
  const [appDetails, setAppDetails] = useState<{
    metrics: ApplicationMetricsResponse | null
    syncStatus: SyncStatusResponse | null
    deployments: DeploymentRecord[]
    events: EventListResponse['items']
    rollbackPolicy: RollbackPolicy | null
  }>({ metrics: null, syncStatus: null, deployments: [], events: [], rollbackPolicy: null })
  const [projectInsightMetrics, setProjectInsightMetrics] = useState<MetricSeries[]>([])

  const [wizardOpened, setWizardOpened] = useState(false)
  const [projectSettingsOpened, setProjectSettingsOpened] = useState(false)
  const [isDeploying, setIsDeploying] = useState(false)
  const [savingRollbackPolicy, setSavingRollbackPolicy] = useState(false)
  const [emergencyActionLoading, setEmergencyActionLoading] = useState<'abort' | 'rollback' | null>(null)
  const [metricRange, setMetricRange] = useState('15m')
  const [selectedMetricKey, setSelectedMetricKey] = useState<string>('cpu_usage')
  const [deployImageTag, setDeployImageTag] = useState('')
  const [rollbackPolicyDraft, setRollbackPolicyDraft] = useState<RollbackPolicy>({
    enabled: false,
  })
  const projectRefreshSeq = useRef(0)
  const appDetailsRequestSeq = useRef(0)

  // Fetch Bootstrap Data
  useEffect(() => {
    if (!isLoggedIn) return
    const fetchProjects = async () => {
      try {
        const [meRes, projectRes] = await Promise.allSettled([api.getCurrentUser(), api.getProjects()])
        if (meRes.status === 'fulfilled') {
          setCurrentUser(meRes.value)
        }
        if (projectRes.status !== 'fulfilled') {
          throw projectRes.reason
        }
        setProjects(projectRes.value.items)
        setSelectedProjectId((current) => {
          if (current && projectRes.value.items.some((project) => project.id === current)) {
            return current
          }
          return projectRes.value.items[0]?.id ?? null
        })
      } catch {
        notifications.show({ title: '오류', message: '프로젝트 목록을 가져오지 못했습니다.', color: 'red' })
      } finally {
        setLoading(false)
      }
    }
    fetchProjects()
  }, [isLoggedIn])

  // Fetch Applications when project changes
  useEffect(() => {
    if (!selectedProjectId) return
    const fetchApps = async () => {
      const requestSeq = ++projectRefreshSeq.current
      try {
        const [appRes, envRes, repoRes, policyRes] = await Promise.allSettled([
          api.getApplications(selectedProjectId),
          api.getProjectEnvironments(selectedProjectId),
          api.getProjectRepositories(selectedProjectId),
          api.getProjectPolicies(selectedProjectId),
        ])
        if (requestSeq !== projectRefreshSeq.current) {
          return
        }

        if (appRes.status === 'fulfilled') {
          setApplications(appRes.value.items)

          const metricsResponses = await Promise.allSettled(
            appRes.value.items.map((application) => api.getMetrics(application.id, '15m')),
          )
          const series = metricsResponses
            .filter(
              (result): result is PromiseFulfilledResult<ApplicationMetricsResponse> =>
                result.status === 'fulfilled',
            )
            .flatMap((result) => result.value.metrics)
          setProjectInsightMetrics(aggregateMetricSeries(series))
        } else {
          console.error('Failed to refresh applications', appRes.reason)
        }

        if (envRes.status === 'fulfilled') {
          setEnvironments(envRes.value.items)
        } else {
          console.error('Failed to refresh environments', envRes.reason)
        }

        if (repoRes.status === 'fulfilled') {
          setRepositories(repoRes.value.items)
        } else {
          console.error('Failed to refresh repositories', repoRes.reason)
        }

        if (policyRes.status === 'fulfilled') {
          setProjectPolicy(policyRes.value)
        } else {
          console.error('Failed to refresh project policies', policyRes.reason)
        }

        if (
          appRes.status === 'rejected' &&
          envRes.status === 'rejected' &&
          repoRes.status === 'rejected' &&
          policyRes.status === 'rejected'
        ) {
          notifications.show({ title: '오류', message: '데이터를 가져오지 못했습니다.', color: 'red' })
        }
      } catch (err) {
        console.error('Failed to refresh project data', err)
      }
    }
    fetchApps()
    const ival = setInterval(fetchApps, 5000)
    return () => clearInterval(ival)
  }, [selectedProjectId])

  // Fetch Application Details when app changes or sidebar opens
  const fetchAppDetails = useCallback(async (appId: string) => {
    const requestSeq = ++appDetailsRequestSeq.current
    try {
      const [metrics, syncStatus, deployments, events, rollback] = await Promise.allSettled([
        api.getMetrics(appId, metricRange),
        api.getSyncStatus(appId),
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
        deployments: deployments.status === 'fulfilled' ? deployments.value.items : current.deployments,
        events: events.status === 'fulfilled' ? events.value.items : current.events,
        rollbackPolicy: rollback.status === 'fulfilled' ? rollback.value : current.rollbackPolicy,
      }))

      if (rollback.status === 'fulfilled') {
        setRollbackPolicyDraft({
          enabled: rollback.value.enabled,
          maxErrorRate: rollback.value.maxErrorRate,
          maxLatencyP95Ms: rollback.value.maxLatencyP95Ms,
          minRequestRate: rollback.value.minRequestRate,
        })
      }
    } catch (err) {
      console.error('Failed to fetch app details', err)
    }
  }, [metricRange])

  useEffect(() => {
    if (!selectedAppId) return
    fetchAppDetails(selectedAppId)
    const ival = setInterval(() => fetchAppDetails(selectedAppId), 5000)
    return () => clearInterval(ival)
  }, [selectedAppId, metricRange, fetchAppDetails])

  useEffect(() => {
    setDeployImageTag('')
  }, [selectedAppId])

  const selectedProject = useMemo(() => projects.find((p) => p.id === selectedProjectId), [projects, selectedProjectId])
  const selectedApp = useMemo(() => applications.find((a) => a.id === selectedAppId), [applications, selectedAppId])

  const handleCreateApp = async (form: CreateFormState) => {
    if (!selectedProjectId) return
    try {
      await api.createApplication(selectedProjectId, {
        name: form.name,
        image: form.image,
        servicePort: form.servicePort,
        deploymentStrategy: form.deploymentStrategy,
        environment: form.environment || 'dev',
        secrets: form.secrets.filter(s => s.key && s.value)
      })
      notifications.show({ title: '성공', message: '애플리케이션이 생성되었습니다.', color: 'green' })
      setWizardOpened(false)
      const res = await api.getApplications(selectedProjectId)
      setApplications(res.items)
    } catch {
      notifications.show({ title: '생성 실패', message: '요청이 거부되었습니다.', color: 'red' })
    }
  }

  const handleDeploy = async (tag: string) => {
    if (!selectedAppId) return
    setIsDeploying(true)
    try {
      await api.createDeployment(selectedAppId, tag)
      notifications.show({ title: '성공', message: '배포가 시작되었습니다.', color: 'green' })
      setDeployImageTag('')
      await fetchAppDetails(selectedAppId)
    } catch {
      notifications.show({ title: '배포 실패', message: '이미지 태그를 확인해주세요.', color: 'red' })
    } finally {
      setIsDeploying(false)
    }
  }

  const handleSaveRollbackPolicy = async () => {
    if (!selectedAppId) return
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

  const handleAbortLatestDeployment = async () => {
    const latestDeployment = appDetails.deployments[0]
    if (!selectedAppId || !latestDeployment) return
    setEmergencyActionLoading('abort')
    try {
      await api.abortDeployment(selectedAppId, latestDeployment.deploymentId)
      notifications.show({ title: '성공', message: '현재 배포를 중단했습니다.', color: 'green' })
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
    setEmergencyActionLoading('rollback')
    try {
      await api.createDeployment(selectedAppId, previousDeployment.imageTag, previousDeployment.environment)
      notifications.show({ title: '성공', message: '직전 버전으로 롤백을 요청했습니다.', color: 'green' })
      await fetchAppDetails(selectedAppId)
    } catch {
      notifications.show({ title: '롤백 실패', message: '직전 버전 롤백을 요청하지 못했습니다.', color: 'red' })
    } finally {
      setEmergencyActionLoading(null)
    }
  }

  if (!isLoggedIn) return <LoginForm onLogin={() => setIsLoggedIn(true)} />

  if (loading) return (
    <div style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <Loader size="xl" color="lagoon.6" />
    </div>
  )

  return (
    <AppShell
      header={{ height: 72 }}
      padding="md"
    >
      <AppShell.Header style={{ borderBottom: '1px solid #e2e8f0', background: 'rgba(255, 255, 255, 0.8)', backdropFilter: 'blur(12px)' }}>
        <Container fluid h="100%" px="xl">
          <Group justify="space-between" h="100%">
            <Group gap="xs">
              <div style={{ background: '#1d66d6', color: 'white', padding: '6px 10px', borderRadius: '8px', fontWeight: 900 }}>AODS</div>
              <div>
                <Text fw={800} size="sm">Aolda Orchestration System</Text>
                <Text size="xs" c="dimmed">Internal Deployment Console</Text>
              </div>
            </Group>
            <Group gap="lg">
              <Group gap="xs">
                <Avatar radius="xl" size="sm" color="lagoon.6"><IconUser size={16} /></Avatar>
                <Stack gap={0}>
                  <Text size="xs" fw={700}>
                    {currentUser?.displayName || currentUser?.username || '관리자'}
                  </Text>
                  {currentUser?.id ? (
                    <Text size="xs" c="dimmed">{currentUser.id}</Text>
                  ) : null}
                </Stack>
              </Group>
              <Button variant="light" color="gray" size="xs" leftSection={<IconArrowLeft size={14} />} onClick={() => setIsLoggedIn(false)}>로그아웃</Button>
            </Group>
          </Group>
        </Container>
      </AppShell.Header>

      <AppShell.Main>
        <Container fluid className={classes.page} px="xl">
          <Stack gap="xl">
            {/* Masthead */}
            <div className={classes.masthead}>
              <Text className={classes.kicker}>CLUSTER DASHBOARD</Text>
              <Text className={classes.title}>플랫폼 운영 현황</Text>
              <Text className={classes.description}>실시간 GitOps 동기화 상태와 인프라 메트릭을 추적합니다.</Text>
            </div>

            {/* Project Selection Tabs */}
            <Group justify="space-between">
              <Tabs variant="pills" value={selectedProjectId} onChange={setSelectedProjectId} color="lagoon.6">
                <Tabs.List>
                  {projects.map((p) => (
                    <Tabs.Tab key={p.id} value={p.id} leftSection={<IconLayoutDashboard size={16} />}>
                      {p.name}
                    </Tabs.Tab>
                  ))}
                </Tabs.List>
              </Tabs>
              <Button leftSection={<IconPlus size={16} />} color="lagoon.6" radius="md" onClick={() => setWizardOpened(true)}>새 애플리케이션</Button>
            </Group>

            {/* Application Section with Insights Sidebar */}
            <Grid gutter="xl">
              <Grid.Col span={{ base: 12, lg: 8 }}>
                <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="lg">
                  {applications.map((app) => (
                    <UnstyledButton key={app.id} onClick={() => setSelectedAppId(app.id)} className={classes.appItem}>
                      <div className={classes.surfaceCard}>
                        <Stack gap="md">
                          <Group justify="space-between" align="start">
                            <Stack gap={2}>
                              <Text className={classes.cardTitle}>{app.name}</Text>
                              <Text className={classes.cardMeta}>{app.image}</Text>
                            </Stack>
                            <SyncStatusBadge status={app.syncStatus} />
                          </Group>

                          <SimpleGrid cols={2} spacing="sm">
                            <div style={{ background: '#f8fafc', padding: '12px', borderRadius: '12px', border: '1px solid #f1f5f9' }}>
                              <Text size="xs" c="dimmed" fw={700}>STRATEGY</Text>
                              <Text size="sm" fw={800} c="lagoon.9">{app.deploymentStrategy === 'Canary' ? '카나리아' : '표준 배포'}</Text>
                            </div>
                            <div style={{ background: '#f8fafc', padding: '12px', borderRadius: '12px', border: '1px solid #f1f5f9' }}>
                              <Text size="xs" c="dimmed" fw={700}>NAMESPACE</Text>
                              <Text size="sm" fw={800} c="lagoon.9">{selectedProject?.namespace || 'default'}</Text>
                            </div>
                          </SimpleGrid>
                        </Stack>
                      </div>
                    </UnstyledButton>
                  ))}
                </SimpleGrid>
              </Grid.Col>

              <Grid.Col span={{ base: 12, lg: 4 }}>
                <div className={classes.sidebarSection}>
                  <div className={classes.insightsCard}>
                    <Group justify="space-between" mb="lg">
                      <Text fw={800} size="sm" className={classes.sectionEyebrow} style={{ margin: 0 }}>Project Insights</Text>
                      <Badge variant="dot" color={projectHealthColor(applications)}>
                        {projectHealthLabel(applications)}
                      </Badge>
                    </Group>

                    <SimpleGrid cols={2} spacing="sm" mb="xl">
                      <div className={classes.statBadge}>
                        <Text className={classes.statLabel}>Total Apps</Text>
                        <Text className={classes.statValue}>{applications.length}</Text>
                      </div>
                      <div className={classes.statBadge}>
                        <Text className={classes.statLabel}>Synced</Text>
                        <Text className={classes.statValue}>{applications.filter(a => a.syncStatus === 'Synced').length}</Text>
                      </div>
                    </SimpleGrid>

                    <Stack gap="lg">
                      <div>
                        <Group justify="space-between" mb={8}>
                          <Group gap={6}>
                            <IconCpu size={14} color="#64748b" />
                            <Text size="xs" fw={700} c="dimmed">CPU ALLOCATION</Text>
                          </Group>
                          <Text size="xs" fw={800}>{formatInsightValue(projectInsightMetrics, 'cpu_usage')}</Text>
                        </Group>
                        <div className={classes.metricProgress}>
                          <div className={classes.metricBar} style={{ width: `${metricSeriesWidth(projectInsightMetrics, 'cpu_usage')}%` }}></div>
                        </div>
                      </div>

                      <div>
                        <Group justify="space-between" mb={8}>
                          <Group gap={6}>
                            <IconDatabase size={14} color="#64748b" />
                            <Text size="xs" fw={700} c="dimmed">MEMORY USAGE</Text>
                          </Group>
                          <Text size="xs" fw={800}>{formatInsightValue(projectInsightMetrics, 'memory_usage')}</Text>
                        </Group>
                        <div className={classes.metricProgress}>
                          <div className={classes.metricBar} style={{ width: `${metricSeriesWidth(projectInsightMetrics, 'memory_usage')}%`, background: '#f59e0b' }}></div>
                        </div>
                      </div>

                      <div>
                        <Group justify="space-between" mb={8}>
                          <Group gap={6}>
                            <IconBolt size={14} color="#64748b" />
                            <Text size="xs" fw={700} c="dimmed">TRAFFIC SCALE</Text>
                          </Group>
                          <Text size="xs" fw={800}>{formatInsightValue(projectInsightMetrics, 'request_rate')}</Text>
                        </Group>
                        <div className={classes.metricProgress}>
                          <div className={classes.metricBar} style={{ width: `${metricSeriesWidth(projectInsightMetrics, 'request_rate')}%`, background: '#10b981' }}></div>
                        </div>
                      </div>
                    </Stack>
                  </div>

                  <div className={classes.surfaceCard} style={{ padding: '24px' }}>
                    <Text fw={800} size="sm" mb="md">Operational Shortcuts</Text>
                    <Stack gap="xs">
                      <Button
                        variant="light"
                        color="gray"
                        fullWidth
                        justify="space-between"
                        rightSection={<IconExternalLink size={14} />}
                        radius="md"
                        onClick={() => setProjectSettingsOpened(true)}
                      >
                        Project Repository
                      </Button>
                      <Button variant="light" color="gray" fullWidth justify="space-between" rightSection={<IconExternalLink size={14} />} radius="md">
                        Project Documentation
                      </Button>
                      <Button
                        variant="light"
                        color="gray"
                        fullWidth
                        justify="space-between"
                        rightSection={<IconSettings size={14} />}
                        radius="md"
                        onClick={() => setProjectSettingsOpened(true)}
                      >
                        Project Settings
                      </Button>
                    </Stack>
                  </div>
                </div>
              </Grid.Col>
            </Grid>
          </Stack>
        </Container>
      </AppShell.Main>

      {/* Application Operations Drawer */}
      <Drawer
        opened={!!selectedAppId}
        onClose={() => {
          setSelectedAppId(null)
          setDeployImageTag('')
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
          <Tabs defaultValue="status" color="lagoon.6" styles={{ tab: { padding: '16px 20px' } }}>
            <Tabs.List>
              <Tabs.Tab value="status" leftSection={<IconActivity size={16} />}>상태 및 지표</Tabs.Tab>
              <Tabs.Tab value="deploy" leftSection={<IconRocket size={16} />}>배포 제어</Tabs.Tab>
              <Tabs.Tab value="history" leftSection={<IconHistory size={16} />}>배포 이력</Tabs.Tab>
              <Tabs.Tab value="rules" leftSection={<IconShieldCheck size={16} />}>운영 규칙</Tabs.Tab>
            </Tabs.List>

            <Tabs.Panel value="status" p="xl">
              <Stack gap="xl">
                <Group justify="space-between" align="center">
                  <Text className={classes.sectionEyebrow}>인프라 실시간 지표</Text>
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
                
                <SimpleGrid cols={4} spacing="xl">
                  <MetricCard
                    label="CPU USAGE"
                    value={formatLatestMetric(appDetails.metrics, 'cpu_usage')}
                    unit={latestMetricUnit(appDetails.metrics, 'cpu_usage')}
                    points={appDetails.metrics?.metrics?.find(m => m.key === 'cpu_usage')?.points}
                    color="#1d66d6"
                    active={selectedMetricKey === 'cpu_usage'}
                    onClick={() => setSelectedMetricKey('cpu_usage')}
                  />
                  <MetricCard
                    label="MEMORY"
                    value={formatLatestMetric(appDetails.metrics, 'memory_usage')}
                    unit="MiB"
                    points={appDetails.metrics?.metrics?.find(m => m.key === 'memory_usage')?.points}
                    color="#0b3d7f"
                    active={selectedMetricKey === 'memory_usage'}
                    onClick={() => setSelectedMetricKey('memory_usage')}
                  />
                  <MetricCard
                    label="TRAFFIC"
                    value={formatLatestMetric(appDetails.metrics, 'request_rate')}
                    unit="rpm"
                    points={appDetails.metrics?.metrics?.find(m => m.key === 'request_rate')?.points}
                    color="#10b981"
                    active={selectedMetricKey === 'request_rate'}
                    onClick={() => setSelectedMetricKey('request_rate')}
                  />
                  <MetricCard
                    label="LATENCY P95"
                    value={formatLatestMetric(appDetails.metrics, 'latency_p95')}
                    unit="ms"
                    points={appDetails.metrics?.metrics?.find(m => m.key === 'latency_p95')?.points}
                    color="#f59e0b"
                    active={selectedMetricKey === 'latency_p95'}
                    onClick={() => setSelectedMetricKey('latency_p95')}
                  />
                </SimpleGrid>

                <MetricDataTable series={appDetails.metrics?.metrics?.find(m => m.key === selectedMetricKey)} />

                <Text className={classes.sectionEyebrow}>최근 시스템 이벤트</Text>
                <div className={classes.surfaceCard}>
                  {appDetails.events.length === 0 ? (
                    <Text size="sm" c="dimmed" ta="center">최근 수집된 이벤트가 없습니다.</Text>
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
              </Stack>
            </Tabs.Panel>

            <Tabs.Panel value="deploy" p="xl">
              <Stack gap="lg">
                <div className={classes.surfaceCard}>
                  <Text fw={800} mb={4}>새 버전 배포</Text>
                  <Text size="sm" c="dimmed" mb="lg">대상 이미지 태그를 입력하여 즉시 배포를 시작합니다.</Text>
                  <Stack gap="md">
                    <TextInput
                      label="TARGET IMAGE TAG"
                      placeholder="e.g. v1.2.3"
                      value={deployImageTag}
                      onChange={(event) => setDeployImageTag(event.currentTarget.value)}
                      id="deploy_tag_input"
                    />
                    <Button
                      fullWidth
                      size="md"
                      color="lagoon.6"
                      loading={isDeploying}
                      disabled={!deployImageTag.trim()}
                      onClick={() => handleDeploy(deployImageTag.trim())}
                    >
                      배포 트리거 실행
                    </Button>
                  </Stack>
                </div>

                <div className={classes.surfaceCard}>
                  <Text className={classes.sectionEyebrow} mb="md">배포 진행률</Text>
                  <div className={classes.progressList}>
                    <div className={`${classes.progressItem} ${deploymentStageClass(appDetails.deployments.length > 0 ? 'complete' : 'pending', classes)}`}>
                      <div className={classes.progressMarker}><IconGitBranch size={16} /></div>
                      <div>
                        <Text className={classes.progressTitle}>Git Commit Pushed</Text>
                        <Text className={classes.progressDetail}>
                          {appDetails.deployments[0]
                            ? `${appDetails.deployments[0].imageTag} 버전 요청이 저장되었습니다.`
                            : '배포 이력이 아직 없습니다.'}
                        </Text>
                      </div>
                    </div>
                    <div className={`${classes.progressItem} ${deploymentStageClass(syncStageState(appDetails.syncStatus?.status), classes)}`}>
                      <div className={classes.progressMarker}><IconRefresh size={16} /></div>
                      <div>
                        <Text className={classes.progressTitle}>Flux Syncing</Text>
                        <Text className={classes.progressDetail}>
                          {appDetails.syncStatus?.message || '동기화 상태를 아직 수집하지 못했습니다.'}
                        </Text>
                      </div>
                    </div>
                    <div className={`${classes.progressItem} ${deploymentStageClass(rolloutStageState(appDetails.deployments[0]), classes)}`}>
                      <div className={classes.progressMarker}><IconBox size={16} /></div>
                      <div>
                        <Text className={classes.progressTitle}>Canary Monitoring</Text>
                        <Text className={classes.progressDetail}>
                          {rolloutStageMessage(appDetails.deployments[0])}
                        </Text>
                      </div>
                    </div>
                  </div>
                </div>
              </Stack>
            </Tabs.Panel>

            <Tabs.Panel value="history" p="xl">
              <Stack gap="md">
                <Table striped highlightOnHover>
                  <Table.Thead>
                    <Table.Tr>
                      <Table.Th>배포 ID</Table.Th>
                      <Table.Th>이미지 태그</Table.Th>
                      <Table.Th>상태</Table.Th>
                      <Table.Th>완료 시각</Table.Th>
                    </Table.Tr>
                  </Table.Thead>
                  <Table.Tbody>
                    {appDetails.deployments.map(d => (
                      <Table.Tr key={d.deploymentId}>
                        <Table.Td><Text size="xs" ff="monospace">{d.deploymentId.slice(0, 8)}</Text></Table.Td>
                        <Table.Td><Badge variant="outline" size="sm">{d.imageTag}</Badge></Table.Td>
                        <Table.Td><Badge color={d.status === 'Completed' ? 'green' : 'gray'}>{d.status}</Badge></Table.Td>
                        <Table.Td><Text size="xs" c="dimmed">{new Date(d.updatedAt).toLocaleString('ko-KR')}</Text></Table.Td>
                      </Table.Tr>
                    ))}
                  </Table.Tbody>
                </Table>
              </Stack>
            </Tabs.Panel>

            <Tabs.Panel value="rules" p="xl">
              <Stack gap="lg">
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
                      onChange={(event) =>
                        setRollbackPolicyDraft((current) => ({
                          ...current,
                          enabled: event.currentTarget.checked,
                        }))
                      }
                    />
                  </Group>
                  <SimpleGrid cols={2} spacing="md">
                    <NumberInput
                      label="최대 에러율 (%)"
                      value={rollbackPolicyDraft.maxErrorRate ?? undefined}
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
                    onClick={handleSaveRollbackPolicy}
                  >
                    사용자 정의 규칙 저장
                  </Button>
                </div>

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
                      disabled={!appDetails.deployments[0]}
                      onClick={handleAbortLatestDeployment}
                    >
                      배포 강제 중단
                    </Button>
                    <Button
                      variant="filled"
                      color="red"
                      size="xs"
                      loading={emergencyActionLoading === 'rollback'}
                      disabled={!appDetails.deployments[1]}
                      onClick={handleRollbackToPreviousRevision}
                    >
                      직전 버전 롤백
                    </Button>
                  </Group>
                </div>
              </Stack>
            </Tabs.Panel>
          </Tabs>
        </ScrollArea>
      </Drawer>

      {/* New Application Wizard Drawer */}
      <Drawer
        opened={wizardOpened}
        onClose={() => setWizardOpened(false)}
        position="right"
        size="520px"
        title={<Text fw={900} size="lg">새 애플리케이션 생성</Text>}
      >
        <ApplicationWizard
          environments={environments.map((e) => ({ id: e.id, name: e.name })) || []}
          allowedStrategies={projectPolicy?.allowedDeploymentStrategies || ['Standard', 'Canary']}
          initialState={{
            name: '',
            description: '',
            image: '',
            servicePort: 80,
            deploymentStrategy: 'Standard',
            environment: environments.find((environment) => environment.default)?.id || environments[0]?.id || 'dev',
            secrets: [{ key: '', value: '' }]
          }}
          onSubmit={handleCreateApp}
          onCancel={() => setWizardOpened(false)}
          submitting={false}
        />
      </Drawer>

      <Drawer
        opened={projectSettingsOpened}
        onClose={() => setProjectSettingsOpened(false)}
        position="right"
        size="640px"
        title={<Text fw={900} size="lg">{selectedProject?.name || '프로젝트'} 설정</Text>}
      >
        <Stack gap="lg">
          <div className={classes.surfaceCard}>
            <Text className={classes.sectionEyebrow} mb="md">연결된 저장소</Text>
            {repositories.length > 0 ? (
              <Table striped highlightOnHover>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>이름</Table.Th>
                    <Table.Th>설명</Table.Th>
                    <Table.Th>저장소 주소</Table.Th>
                    <Table.Th>바로가기</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {repositories.map((repository) => (
                    <Table.Tr key={repository.id}>
                      <Table.Td>{repository.name}</Table.Td>
                      <Table.Td>{repository.description || '-'}</Table.Td>
                      <Table.Td>
                        <Text size="sm" ff="monospace">{repository.url}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Button
                          variant="light"
                          color="lagoon.6"
                          size="xs"
                          rightSection={<IconExternalLink size={12} />}
                          onClick={() => window.open(repository.url, '_blank', 'noopener,noreferrer')}
                        >
                          열기
                        </Button>
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            ) : (
              <Text size="sm" c="dimmed">현재 프로젝트에 연결된 저장소가 없습니다.</Text>
            )}
          </div>

          <div className={classes.surfaceCard}>
            <Text className={classes.sectionEyebrow} mb="md">기본 정보</Text>
            <SimpleGrid cols={2} spacing="md">
              <div>
                <Text size="xs" c="dimmed" fw={700}>프로젝트 ID</Text>
                <Text size="sm" fw={800}>{selectedProject?.id || '-'}</Text>
              </div>
              <div>
                <Text size="xs" c="dimmed" fw={700}>네임스페이스</Text>
                <Text size="sm" fw={800}>{selectedProject?.namespace || '-'}</Text>
              </div>
            </SimpleGrid>
          </div>

          <div className={classes.surfaceCard}>
            <Text className={classes.sectionEyebrow} mb="md">운영 환경</Text>
            {environments.length > 0 ? (
              <Table striped highlightOnHover>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>이름</Table.Th>
                    <Table.Th>클러스터</Table.Th>
                    <Table.Th>반영 방식</Table.Th>
                    <Table.Th>기본</Table.Th>
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
            ) : (
              <Text size="sm" c="dimmed">운영 환경 정보를 아직 불러오지 못했습니다.</Text>
            )}
          </div>

          <div className={classes.surfaceCard}>
            <Text className={classes.sectionEyebrow} mb="md">배포 정책</Text>
            {projectPolicy ? (
              <SimpleGrid cols={2} spacing="md">
                <div>
                  <Text size="xs" c="dimmed" fw={700}>최소 복제본 수</Text>
                  <Text size="sm" fw={800}>{projectPolicy.minReplicas}</Text>
                </div>
                <div>
                  <Text size="xs" c="dimmed" fw={700}>프로브 필수</Text>
                  <Text size="sm" fw={800}>{projectPolicy.requiredProbes ? '예' : '아니오'}</Text>
                </div>
                <div>
                  <Text size="xs" c="dimmed" fw={700}>운영 환경 변경 요청 필수</Text>
                  <Text size="sm" fw={800}>{projectPolicy.prodPRRequired ? '예' : '아니오'}</Text>
                </div>
                <div>
                  <Text size="xs" c="dimmed" fw={700}>자동 롤백</Text>
                  <Text size="sm" fw={800}>{projectPolicy.autoRollbackEnabled ? '예' : '아니오'}</Text>
                </div>
                <div>
                  <Text size="xs" c="dimmed" fw={700}>허용 환경</Text>
                  <Text size="sm" fw={800}>
                    {projectPolicy.allowedEnvironments.length > 0
                      ? projectPolicy.allowedEnvironments.join(', ')
                      : '-'}
                  </Text>
                </div>
                <div>
                  <Text size="xs" c="dimmed" fw={700}>허용 배포 전략</Text>
                  <Text size="sm" fw={800}>
                    {projectPolicy.allowedDeploymentStrategies.length > 0
                      ? projectPolicy.allowedDeploymentStrategies
                          .map((strategy) => (strategy === 'Canary' ? '카나리아' : '표준 배포'))
                          .join(', ')
                      : '-'}
                  </Text>
                </div>
                <div style={{ gridColumn: '1 / -1' }}>
                  <Text size="xs" c="dimmed" fw={700}>허용 클러스터 대상</Text>
                  <Text size="sm" fw={800}>
                    {projectPolicy.allowedClusterTargets.length > 0
                      ? projectPolicy.allowedClusterTargets.join(', ')
                      : '-'}
                  </Text>
                </div>
              </SimpleGrid>
            ) : (
              <Text size="sm" c="dimmed">프로젝트 정책 정보를 아직 불러오지 못했습니다.</Text>
            )}
          </div>
        </Stack>
      </Drawer>
    </AppShell>
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

function formatInsightValue(series: MetricSeries[], key: string) {
  const item = findMetricSeries(series, key)
  const value = item ? latestMetricNumber(item) : null
  if (value === null) return '데이터 없음'
  switch (key) {
    case 'cpu_usage':
      return value < 1 ? `${(value * 1000).toFixed(1)} mCPU` : `${value.toFixed(2)} cores`
    case 'memory_usage':
      return `${value.toFixed(1)} MiB`
    case 'request_rate':
      return `${value.toFixed(1)} RPM`
    default:
      return value.toFixed(1)
  }
}

function latestMetricNumber(series: MetricSeries) {
  const value = [...series.points].reverse().find((point) => point.value !== null)?.value
  return value ?? null
}

function metricSeriesWidth(series: MetricSeries[], key: string) {
  const item = findMetricSeries(series, key)
  if (!item) return 0
  const values = item.points
    .map((point) => point.value)
    .filter((value): value is number => value !== null)
  if (values.length === 0) return 0
  const current = values.at(-1) ?? 0
  const max = Math.max(...values)
  if (max <= 0) return 0
  return Math.max(6, Math.min(100, (current / max) * 100))
}

function projectHealthColor(applications: ApplicationSummary[]) {
  if (applications.some((application) => application.syncStatus === 'Degraded')) return 'red'
  if (applications.some((application) => application.syncStatus === 'Syncing')) return 'yellow'
  if (applications.some((application) => application.syncStatus === 'Synced')) return 'green'
  return 'gray'
}

function projectHealthLabel(applications: ApplicationSummary[]) {
  if (applications.some((application) => application.syncStatus === 'Degraded')) return 'Issue'
  if (applications.some((application) => application.syncStatus === 'Syncing')) return 'Syncing'
  if (applications.some((application) => application.syncStatus === 'Synced')) return 'Healthy'
  return 'Unknown'
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
  return '배포 상태를 수집 중입니다.'
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
