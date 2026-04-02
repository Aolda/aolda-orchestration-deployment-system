import { useEffect, useState } from 'react'
import {
  Alert,
  Badge,
  Button,
  Card,
  Container,
  Divider,
  Grid,
  Group,
  Loader,
  NumberInput,
  Paper,
  Select,
  SimpleGrid,
  Stack,
  Switch,
  Text,
  TextInput,
  Textarea,
  Title,
  UnstyledButton,
} from '@mantine/core'
import { api, ApiError } from './api/client'
import type {
  ApplicationEvent,
  ApplicationMetricsResponse,
  ApplicationSummary,
  ChangeRecord,
  ClusterSummary,
  CreateChangeRequest,
  CurrentUser,
  DeploymentRecord,
  EnvironmentSummary,
  MetricSeries,
  ProjectPolicy,
  ProjectSummary,
  RollbackPolicy,
  SecretEntry,
  SyncStatus,
  SyncStatusResponse,
} from './types/api'
import classes from './App.module.css'

type CreateFormState = {
  name: string
  description: string
  image: string
  servicePort: number
  deploymentStrategy: 'Standard' | 'Canary'
  environment: string
  secrets: SecretEntry[]
}

type ProjectSnapshot = {
  applications: ApplicationSummary[]
  environments: EnvironmentSummary[]
  policies: ProjectPolicy
  clusters: ClusterSummary[]
}

type ApplicationSnapshot = {
  syncStatus: SyncStatusResponse
  metrics: ApplicationMetricsResponse
  deployments: DeploymentRecord[]
  events: ApplicationEvent[]
  rollbackPolicy: RollbackPolicy
}

function buildInitialCreateForm(
  environment = '',
  deploymentStrategy: 'Standard' | 'Canary' = 'Standard',
): CreateFormState {
  return {
    name: '',
    description: '',
    image: 'repo/my-app:v1',
    servicePort: 8080,
    deploymentStrategy,
    environment,
    secrets: [{ key: 'DATABASE_URL', value: '' }],
  }
}

const emptyRollbackPolicy: RollbackPolicy = {
  enabled: false,
}

export default function App() {
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [projects, setProjects] = useState<ProjectSummary[]>([])
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)
  const [applications, setApplications] = useState<ApplicationSummary[]>([])
  const [selectedApplicationId, setSelectedApplicationId] = useState<string | null>(
    null,
  )
  const [environments, setEnvironments] = useState<EnvironmentSummary[]>([])
  const [clusters, setClusters] = useState<ClusterSummary[]>([])
  const [projectPolicies, setProjectPolicies] = useState<ProjectPolicy | null>(null)
  const [policyDraft, setPolicyDraft] = useState<ProjectPolicy | null>(null)
  const [syncStatus, setSyncStatus] = useState<SyncStatusResponse | null>(null)
  const [metrics, setMetrics] = useState<ApplicationMetricsResponse | null>(null)
  const [deployments, setDeployments] = useState<DeploymentRecord[]>([])
  const [events, setEvents] = useState<ApplicationEvent[]>([])
  const [rollbackPolicy, setRollbackPolicy] = useState<RollbackPolicy>(emptyRollbackPolicy)
  const [activeChange, setActiveChange] = useState<ChangeRecord | null>(null)
  const [bootstrapLoading, setBootstrapLoading] = useState(true)
  const [projectMetaLoading, setProjectMetaLoading] = useState(false)
  const [applicationsLoading, setApplicationsLoading] = useState(false)
  const [detailsLoading, setDetailsLoading] = useState(false)
  const [historyLoading, setHistoryLoading] = useState(false)
  const [submittingCreate, setSubmittingCreate] = useState(false)
  const [submittingDeploy, setSubmittingDeploy] = useState(false)
  const [submittingChange, setSubmittingChange] = useState(false)
  const [savingPolicies, setSavingPolicies] = useState(false)
  const [savingRollback, setSavingRollback] = useState(false)
  const [promotingDeploymentId, setPromotingDeploymentId] = useState<string | null>(null)
  const [abortingDeploymentId, setAbortingDeploymentId] = useState<string | null>(null)
  const [globalError, setGlobalError] = useState<string | null>(null)
  const [createForm, setCreateForm] = useState<CreateFormState>(buildInitialCreateForm())
  const [imageTag, setImageTag] = useState('v2')
  const [deployEnvironment, setDeployEnvironment] = useState('')

  useEffect(() => {
    let cancelled = false

    ;(async () => {
      setBootstrapLoading(true)
      setGlobalError(null)

      try {
        const [currentUser, projectResponse] = await Promise.all([
          api.getCurrentUser(),
          api.getProjects(),
        ])

        if (cancelled) {
          return
        }

        setUser(currentUser)
        setProjects(projectResponse.items)
        setSelectedProjectId(projectResponse.items[0]?.id ?? null)
      } catch (error) {
        if (!cancelled) {
          setGlobalError(toErrorMessage(error))
        }
      } finally {
        if (!cancelled) {
          setBootstrapLoading(false)
        }
      }
    })()

    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (!selectedProjectId) {
      setApplications([])
      setEnvironments([])
      setClusters([])
      setProjectPolicies(null)
      setPolicyDraft(null)
      setSelectedApplicationId(null)
      setActiveChange(null)
      setCreateForm(buildInitialCreateForm())
      return
    }

    let cancelled = false

    ;(async () => {
      setApplicationsLoading(true)
      setProjectMetaLoading(true)
      setGlobalError(null)
      setSelectedApplicationId(null)
      setSyncStatus(null)
      setMetrics(null)
      setDeployments([])
      setEvents([])
      setRollbackPolicy(emptyRollbackPolicy)

      try {
        const snapshot = await loadProjectSnapshot(selectedProjectId)
        if (cancelled) {
          return
        }

        applyProjectSnapshot(snapshot, {
          resetCreateForm: true,
        })
      } catch (error) {
        if (!cancelled) {
          setGlobalError(toErrorMessage(error))
        }
      } finally {
        if (!cancelled) {
          setApplicationsLoading(false)
          setProjectMetaLoading(false)
          setActiveChange(null)
        }
      }
    })()

    return () => {
      cancelled = true
    }
  }, [selectedProjectId])

  useEffect(() => {
    if (!selectedApplicationId) {
      setSyncStatus(null)
      setMetrics(null)
      setDeployments([])
      setEvents([])
      setRollbackPolicy(emptyRollbackPolicy)
      setDeployEnvironment(resolveDefaultEnvironment(environments)?.id ?? '')
      return
    }

    let cancelled = false

    ;(async () => {
      setDetailsLoading(true)
      setHistoryLoading(true)
      setGlobalError(null)

      try {
        const snapshot = await loadApplicationSnapshot(selectedApplicationId)
        if (cancelled) {
          return
        }

        setSyncStatus(snapshot.syncStatus)
        setMetrics(snapshot.metrics)
        setDeployments(snapshot.deployments)
        setEvents(snapshot.events)
        setRollbackPolicy(snapshot.rollbackPolicy)
        setDeployEnvironment(
          snapshot.deployments[0]?.environment ??
            resolveDefaultEnvironment(environments)?.id ??
            '',
        )
      } catch (error) {
        if (!cancelled) {
          setGlobalError(toErrorMessage(error))
        }
      } finally {
        if (!cancelled) {
          setDetailsLoading(false)
          setHistoryLoading(false)
        }
      }
    })()

    return () => {
      cancelled = true
    }
  }, [environments, selectedApplicationId])

  const selectedProject = projects.find((project) => project.id === selectedProjectId) ?? null
  const selectedApplication =
    applications.find((application) => application.id === selectedApplicationId) ?? null
  const canMutateSelectedProject = canMutateProject(selectedProject?.role)
  const canAdminSelectedProject = selectedProject?.role === 'admin'
  const selectedCreateEnvironment =
    environments.find((environment) => environment.id === createForm.environment) ??
    resolveDefaultEnvironment(environments) ??
    null
  const selectedDeployEnvironment =
    environments.find((environment) => environment.id === deployEnvironment) ??
    null
  const latestDeployment = deployments[0] ?? null
  const latestDeploymentEnvironment =
    latestDeployment?.environment ??
    selectedDeployEnvironment?.id ??
    resolveDefaultEnvironment(environments)?.id ??
    ''
  const latestCluster =
    clusters.find((cluster) => cluster.id === selectedDeployEnvironment?.clusterId) ?? null
  const activeChangeAppId = inferChangeApplicationId(activeChange)

  async function loadProjectSnapshot(projectId: string): Promise<ProjectSnapshot> {
    const [applicationResponse, environmentResponse, policiesResponse, clusterResponse] =
      await Promise.all([
        api.getApplications(projectId),
        api.getProjectEnvironments(projectId),
        api.getProjectPolicies(projectId),
        api.getClusters(),
      ])

    return {
      applications: applicationResponse.items,
      environments: environmentResponse.items,
      policies: policiesResponse,
      clusters: clusterResponse.items,
    }
  }

  async function loadApplicationSnapshot(applicationId: string): Promise<ApplicationSnapshot> {
    const [syncResponse, metricsResponse, deploymentResponse, eventsResponse, rollbackResponse] =
      await Promise.all([
        api.getSyncStatus(applicationId),
        api.getMetrics(applicationId),
        api.getDeployments(applicationId),
        api.getEvents(applicationId),
        api.getRollbackPolicy(applicationId),
      ])

    let deploymentItems = deploymentResponse.items
    if (deploymentItems[0]) {
      const latest = await api.getDeployment(applicationId, deploymentItems[0].deploymentId)
      deploymentItems = [latest, ...deploymentItems.slice(1)]
    }

    return {
      syncStatus: syncResponse,
      metrics: metricsResponse,
      deployments: deploymentItems,
      events: eventsResponse.items,
      rollbackPolicy: normalizeRollbackPolicy(rollbackResponse),
    }
  }

  function applyProjectSnapshot(
    snapshot: ProjectSnapshot,
    options: { preferredApplicationId?: string | null; resetCreateForm?: boolean } = {},
  ) {
    const defaultEnvironmentId = resolveDefaultEnvironment(snapshot.environments)?.id ?? ''
    const preferredStrategy = resolvePreferredStrategy(snapshot.policies)

    setApplications(snapshot.applications)
    setEnvironments(snapshot.environments)
    setClusters(snapshot.clusters)
    setProjectPolicies(snapshot.policies)
    setPolicyDraft(snapshot.policies)
    setSelectedApplicationId((current) => {
      if (
        options.preferredApplicationId &&
        snapshot.applications.some((item) => item.id === options.preferredApplicationId)
      ) {
        return options.preferredApplicationId
      }
      if (
        !options.preferredApplicationId &&
        current &&
        snapshot.applications.some((item) => item.id === current)
      ) {
        return current
      }
      return snapshot.applications[0]?.id ?? null
    })
    setCreateForm((current) => {
      if (options.resetCreateForm) {
        return buildInitialCreateForm(defaultEnvironmentId, preferredStrategy)
      }

      const nextEnvironment = snapshot.environments.some(
        (item) => item.id === current.environment,
      )
        ? current.environment
        : defaultEnvironmentId
      const nextStrategy = snapshot.policies.allowedDeploymentStrategies.includes(
        current.deploymentStrategy,
      )
        ? current.deploymentStrategy
        : preferredStrategy

      return {
        ...current,
        environment: nextEnvironment,
        deploymentStrategy: nextStrategy,
      }
    })
  }

  function applyApplicationSnapshot(snapshot: ApplicationSnapshot) {
    setSyncStatus(snapshot.syncStatus)
    setMetrics(snapshot.metrics)
    setDeployments(snapshot.deployments)
    setEvents(snapshot.events)
    setRollbackPolicy(snapshot.rollbackPolicy)
    setDeployEnvironment(
      snapshot.deployments[0]?.environment ?? resolveDefaultEnvironment(environments)?.id ?? '',
    )
  }

  async function refreshApplicationDetails(applicationId: string) {
    setDetailsLoading(true)
    setHistoryLoading(true)

    try {
      const snapshot = await loadApplicationSnapshot(applicationId)
      applyApplicationSnapshot(snapshot)
    } finally {
      setDetailsLoading(false)
      setHistoryLoading(false)
    }
  }

  async function refreshApplications(preferredApplicationId?: string | null) {
    if (!selectedProjectId) {
      return
    }

    setApplicationsLoading(true)

    try {
      const applicationResponse = await api.getApplications(selectedProjectId)
      setApplications(applicationResponse.items)
      setSelectedApplicationId((current) => {
        if (
          preferredApplicationId &&
          applicationResponse.items.some((item) => item.id === preferredApplicationId)
        ) {
          return preferredApplicationId
        }
        if (
          !preferredApplicationId &&
          current &&
          applicationResponse.items.some((item) => item.id === current)
        ) {
          return current
        }
        return applicationResponse.items[0]?.id ?? null
      })
    } finally {
      setApplicationsLoading(false)
    }
  }

  async function refreshPolicies() {
    if (!selectedProjectId) {
      return
    }

    setProjectMetaLoading(true)

    try {
      const policiesResponse = await api.getProjectPolicies(selectedProjectId)
      setProjectPolicies(policiesResponse)
      setPolicyDraft(policiesResponse)
    } finally {
      setProjectMetaLoading(false)
    }
  }

  async function createChange(body: CreateChangeRequest) {
    if (!selectedProjectId) {
      return null
    }

    const change = await api.createChange(selectedProjectId, body)
    setActiveChange(change)
    return change
  }

  async function handleCreateApplication() {
    if (!selectedProjectId || !canMutateSelectedProject) {
      return
    }

    const request = {
      name: createForm.name.trim(),
      description: createForm.description.trim(),
      image: createForm.image.trim(),
      servicePort: createForm.servicePort,
      deploymentStrategy: createForm.deploymentStrategy,
      environment: createForm.environment,
      secrets: createForm.secrets.filter(
        (secret) => secret.key.trim() !== '' || secret.value.trim() !== '',
      ),
    } as const

    setSubmittingCreate(true)
    setGlobalError(null)

    try {
      if (selectedCreateEnvironment?.writeMode === 'pull_request') {
        await createChange({
          operation: 'CreateApplication',
          ...request,
        })
        return
      }

      const created = await api.createApplication(selectedProjectId, request)
      await refreshApplications(created.id)
      await refreshApplicationDetails(created.id)
      setCreateForm(
        buildInitialCreateForm(
          resolveDefaultEnvironment(environments)?.id ?? '',
          resolvePreferredStrategy(projectPolicies),
        ),
      )
    } catch (error) {
      if (error instanceof ApiError && error.code === 'CHANGE_REVIEW_REQUIRED') {
        try {
          await createChange({
            operation: 'CreateApplication',
            ...request,
          })
        } catch (changeError) {
          setGlobalError(toErrorMessage(changeError))
        }
      } else {
        setGlobalError(toErrorMessage(error))
      }
    } finally {
      setSubmittingCreate(false)
    }
  }

  async function handleRedeploy() {
    if (!selectedApplicationId || !canMutateSelectedProject) {
      return
    }

    const environment = deployEnvironment || latestDeploymentEnvironment

    setSubmittingDeploy(true)
    setGlobalError(null)

    try {
      if (resolveEnvironmentWriteMode(environments, environment) === 'pull_request') {
        await createChange({
          operation: 'Redeploy',
          applicationId: selectedApplicationId,
          imageTag: imageTag.trim(),
          environment,
        })
        return
      }

      await api.createDeployment(selectedApplicationId, imageTag.trim(), environment)
      await Promise.all([
        refreshApplications(selectedApplicationId),
        refreshApplicationDetails(selectedApplicationId),
      ])
    } catch (error) {
      if (error instanceof ApiError && error.code === 'CHANGE_REVIEW_REQUIRED') {
        try {
          await createChange({
            operation: 'Redeploy',
            applicationId: selectedApplicationId,
            imageTag: imageTag.trim(),
            environment,
          })
        } catch (changeError) {
          setGlobalError(toErrorMessage(changeError))
        }
      } else {
        setGlobalError(toErrorMessage(error))
      }
    } finally {
      setSubmittingDeploy(false)
    }
  }

  async function handleSavePolicies() {
    if (!selectedProjectId || !policyDraft || !canAdminSelectedProject) {
      return
    }

    setSavingPolicies(true)
    setGlobalError(null)

    try {
      const updated = await api.updateProjectPolicies(selectedProjectId, policyDraft)
      setProjectPolicies(updated)
      setPolicyDraft(updated)
    } catch (error) {
      setGlobalError(toErrorMessage(error))
    } finally {
      setSavingPolicies(false)
    }
  }

  async function handleCreatePolicyChange() {
    if (!policyDraft || !canAdminSelectedProject) {
      return
    }

    setSubmittingChange(true)
    setGlobalError(null)

    try {
      await createChange({
        operation: 'UpdatePolicies',
        environment: resolvePreferredChangeEnvironment(environments),
        policies: policyDraft,
      })
    } catch (error) {
      setGlobalError(toErrorMessage(error))
    } finally {
      setSubmittingChange(false)
    }
  }

  async function handleChangeAction(action: 'submit' | 'approve' | 'merge') {
    if (!activeChange) {
      return
    }

    setSubmittingChange(true)
    setGlobalError(null)

    try {
      let updated: ChangeRecord

      switch (action) {
        case 'submit':
          updated = await api.submitChange(activeChange.id)
          break
        case 'approve':
          updated = await api.approveChange(activeChange.id)
          break
        default:
          updated = await api.mergeChange(activeChange.id)
          break
      }

      setActiveChange(updated)

      if (action === 'merge') {
        if (updated.operation === 'UpdatePolicies') {
          await refreshPolicies()
          return
        }

        const nextApplicationId = inferChangeApplicationId(updated)
        await refreshApplications(nextApplicationId)
        if (nextApplicationId) {
          setSelectedApplicationId(nextApplicationId)
          await refreshApplicationDetails(nextApplicationId)
        } else if (selectedApplicationId) {
          await refreshApplicationDetails(selectedApplicationId)
        }
      }
    } catch (error) {
      setGlobalError(toErrorMessage(error))
    } finally {
      setSubmittingChange(false)
    }
  }

  async function handlePromoteDeployment() {
    if (!selectedApplicationId || !latestDeployment || !canMutateSelectedProject) {
      return
    }

    setPromotingDeploymentId(latestDeployment.deploymentId)
    setGlobalError(null)

    try {
      await api.promoteDeployment(selectedApplicationId, latestDeployment.deploymentId)
      await refreshApplicationDetails(selectedApplicationId)
      await refreshApplications(selectedApplicationId)
    } catch (error) {
      setGlobalError(toErrorMessage(error))
    } finally {
      setPromotingDeploymentId(null)
    }
  }

  async function handleAbortDeployment() {
    if (!selectedApplicationId || !latestDeployment || !canMutateSelectedProject) {
      return
    }

    setAbortingDeploymentId(latestDeployment.deploymentId)
    setGlobalError(null)

    try {
      await api.abortDeployment(selectedApplicationId, latestDeployment.deploymentId)
      await refreshApplicationDetails(selectedApplicationId)
      await refreshApplications(selectedApplicationId)
    } catch (error) {
      setGlobalError(toErrorMessage(error))
    } finally {
      setAbortingDeploymentId(null)
    }
  }

  async function handleSaveRollbackPolicy() {
    if (!selectedApplicationId || !canMutateSelectedProject) {
      return
    }

    setSavingRollback(true)
    setGlobalError(null)

    try {
      const saved = await api.saveRollbackPolicy(selectedApplicationId, rollbackPolicy)
      setRollbackPolicy(normalizeRollbackPolicy(saved))
      await refreshApplicationDetails(selectedApplicationId)
    } catch (error) {
      setGlobalError(toErrorMessage(error))
    } finally {
      setSavingRollback(false)
    }
  }

  function updateSecret(index: number, field: keyof SecretEntry, value: string) {
    setCreateForm((current) => ({
      ...current,
      secrets: current.secrets.map((secret, itemIndex) =>
        itemIndex === index ? { ...secret, [field]: value } : secret,
      ),
    }))
  }

  function addSecretRow() {
    setCreateForm((current) => ({
      ...current,
      secrets: [...current.secrets, { key: '', value: '' }],
    }))
  }

  function removeSecretRow(index: number) {
    setCreateForm((current) => ({
      ...current,
      secrets: current.secrets.filter((_, itemIndex) => itemIndex !== index),
    }))
  }

  function updatePolicyDraft(
    field: keyof ProjectPolicy,
    value: boolean | number | string[] | Array<'Standard' | 'Canary'>,
  ) {
    setPolicyDraft((current) => {
      if (!current) {
        return current
      }
      return {
        ...current,
        [field]: value,
      }
    })
  }

  if (bootstrapLoading) {
    return (
      <div className={classes.loadingShell}>
        <Loader color="lagoon.5" size="lg" />
      </div>
    )
  }

  return (
    <div className={classes.page}>
      <Container size="xl" className={classes.container}>
        <Stack gap="xl">
          <Paper className={classes.masthead} radius="xl" shadow="sm">
            <div className={classes.mastheadGrid}>
              <div className={classes.mastheadCopy}>
                <Text className={classes.kicker}>AODS Phase 4 Portal</Text>
                <Title order={1} className={classes.heading}>
                  Git-backed rollout control with review flows and guardrails
                </Title>
                <Text className={classes.lead}>
                  Projects stay sourced from <code>platform/projects.yaml</code>, clusters from{' '}
                  <code>platform/clusters.yaml</code>, rollout state stays GitOps-first, and
                  operating risk is constrained through policies, canary control, and change
                  review gates.
                </Text>
                <Group gap="sm" className={classes.userGroup}>
                  <Badge size="lg" radius="sm" color="lagoon.6" variant="filled">
                    {user?.displayName || user?.username}
                  </Badge>
                  <Badge size="lg" radius="sm" color="sand.6" variant="light">
                    {user?.groups.length ?? 0} groups
                  </Badge>
                </Group>
              </div>

              <SimpleGrid cols={{ base: 1, sm: 4 }} spacing="md">
                <StatCard label="Accessible Projects" value={String(projects.length)} />
                <StatCard label="Clusters In Catalog" value={String(clusters.length)} />
                <StatCard label="Applications In View" value={String(applications.length)} />
                <StatCard label="Selected Sync Status" value={syncStatus?.status ?? 'Unknown'} />
              </SimpleGrid>
            </div>
          </Paper>

          {globalError ? (
            <Alert color="red" variant="light" title="Platform response">
              {globalError}
            </Alert>
          ) : null}

          <section>
            <Group justify="space-between" align="end" mb="md">
              <div>
                <Text className={classes.sectionEyebrow}>Project Catalog</Text>
                <Title order={2} className={classes.sectionTitle}>
                  Authorized projects from the GitHub-backed control catalog
                </Title>
              </div>
              <Text className={classes.sectionMeta}>
                Default branch remains the control plane source of truth.
              </Text>
            </Group>

            <SimpleGrid cols={{ base: 1, md: 3 }} spacing="md">
              {projects.map((project) => (
                <UnstyledButton
                  key={project.id}
                  onClick={() => setSelectedProjectId(project.id)}
                  className={classes.buttonReset}
                >
                  <Card
                    radius="lg"
                    className={
                      project.id === selectedProjectId
                        ? `${classes.selectCard} ${classes.selectCardActive}`
                        : classes.selectCard
                    }
                  >
                    <Group justify="space-between" align="start" mb="xs">
                      <div>
                        <Text className={classes.cardTitle}>{project.name}</Text>
                        <Text className={classes.cardMeta}>{project.namespace}</Text>
                      </div>
                      <Badge color={roleColor(project.role)} variant="light" radius="sm">
                        {project.role}
                      </Badge>
                    </Group>
                    <Text className={classes.cardBody}>{project.description}</Text>
                  </Card>
                </UnstyledButton>
              ))}
            </SimpleGrid>
          </section>

          {selectedProject ? (
            <section>
              <Group justify="space-between" align="end" mb="md">
                <div>
                  <Text className={classes.sectionEyebrow}>Project Guardrails</Text>
                  <Title order={2} className={classes.sectionTitle}>
                    Environments, clusters, and policy controls for {selectedProject.name}
                  </Title>
                </div>
                {projectMetaLoading ? <Loader size="sm" color="lagoon.5" /> : null}
              </Group>

              <Grid gutter="xl">
                <Grid.Col span={{ base: 12, lg: 4 }}>
                  <Paper className={classes.formPanel} radius="lg">
                    <Stack gap="md">
                      <div>
                        <Text className={classes.sectionEyebrow}>Environments</Text>
                        <Text className={classes.sectionMeta}>
                          Write mode and cluster targets per rollout lane.
                        </Text>
                      </div>
                      <Stack gap="sm">
                        {environments.map((environment) => {
                          const cluster = clusters.find(
                            (item) => item.id === environment.clusterId,
                          )
                          return (
                            <Card key={environment.id} radius="lg" className={classes.detailCard}>
                              <Group justify="space-between" align="start" mb="xs">
                                <div>
                                  <Text className={classes.cardTitle}>{environment.name}</Text>
                                  <Text className={classes.cardMeta}>
                                    {environment.id} · {cluster?.name ?? environment.clusterId}
                                  </Text>
                                </div>
                                <Group gap="xs">
                                  {environment.default ? (
                                    <Badge color="lagoon.6" variant="filled" radius="sm">
                                      default
                                    </Badge>
                                  ) : null}
                                  <Badge
                                    color={writeModeColor(environment.writeMode)}
                                    variant="light"
                                    radius="sm"
                                  >
                                    {environment.writeMode}
                                  </Badge>
                                </Group>
                              </Group>
                              <Text className={classes.cardBody}>
                                Cluster target: {environment.clusterId}
                              </Text>
                            </Card>
                          )
                        })}
                      </Stack>

                      <Divider color="rgba(36, 57, 55, 0.12)" />

                      <div>
                        <Text className={classes.sectionEyebrow}>Cluster Catalog</Text>
                        <Group gap="xs" mt="sm" className={classes.wrapRow}>
                          {clusters.map((cluster) => (
                            <Badge
                              key={cluster.id}
                              color={cluster.default ? 'lagoon.6' : 'gray.6'}
                              variant={cluster.default ? 'filled' : 'light'}
                              radius="sm"
                            >
                              {cluster.name}
                            </Badge>
                          ))}
                        </Group>
                      </div>
                    </Stack>
                  </Paper>
                </Grid.Col>

                <Grid.Col span={{ base: 12, lg: 8 }}>
                  <Paper className={classes.formPanel} radius="lg">
                    <Stack gap="md">
                      <div>
                        <Text className={classes.sectionEyebrow}>Policy Controls</Text>
                        <Text className={classes.sectionMeta}>
                          Project defaults constrain what future changes may do.
                        </Text>
                      </div>

                      {policyDraft ? (
                        <>
                          <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
                            <NumberInput
                              label="Minimum replicas"
                              min={1}
                              value={policyDraft.minReplicas}
                              disabled={!canAdminSelectedProject}
                              onChange={(value) =>
                                updatePolicyDraft('minReplicas', Math.max(Number(value) || 1, 1))
                              }
                            />
                            <TextInput
                              label="Allowed environments"
                              value={stringifyList(policyDraft.allowedEnvironments)}
                              disabled={!canAdminSelectedProject}
                              onChange={(event) =>
                                updatePolicyDraft(
                                  'allowedEnvironments',
                                  parseCommaList(event.currentTarget.value),
                                )
                              }
                            />
                            <TextInput
                              label="Allowed cluster targets"
                              value={stringifyList(policyDraft.allowedClusterTargets)}
                              disabled={!canAdminSelectedProject}
                              onChange={(event) =>
                                updatePolicyDraft(
                                  'allowedClusterTargets',
                                  parseCommaList(event.currentTarget.value),
                                )
                              }
                            />
                            <TextInput
                              label="Allowed strategies"
                              value={stringifyList(policyDraft.allowedDeploymentStrategies)}
                              disabled={!canAdminSelectedProject}
                              onChange={(event) =>
                                updatePolicyDraft(
                                  'allowedDeploymentStrategies',
                                  parseCommaList(event.currentTarget.value).filter(isStrategy),
                                )
                              }
                            />
                          </SimpleGrid>

                          <SimpleGrid cols={{ base: 1, md: 3 }} spacing="md">
                            <Switch
                              checked={policyDraft.requiredProbes}
                              disabled={!canAdminSelectedProject}
                              label="Require probes"
                              onChange={(event) =>
                                updatePolicyDraft('requiredProbes', event.currentTarget.checked)
                              }
                            />
                            <Switch
                              checked={policyDraft.prodPRRequired}
                              disabled={!canAdminSelectedProject}
                              label="Require PR for prod"
                              onChange={(event) =>
                                updatePolicyDraft('prodPRRequired', event.currentTarget.checked)
                              }
                            />
                            <Switch
                              checked={policyDraft.autoRollbackEnabled}
                              disabled={!canAdminSelectedProject}
                              label="Enable auto rollback"
                              onChange={(event) =>
                                updatePolicyDraft(
                                  'autoRollbackEnabled',
                                  event.currentTarget.checked,
                                )
                              }
                            />
                          </SimpleGrid>

                          {!canAdminSelectedProject ? (
                            <Alert color="sand" variant="light" title="Admin role required">
                              Only admins can change project policies. Deployers and viewers can
                              still inspect the current guardrails.
                            </Alert>
                          ) : null}

                          <Group className={classes.actionRow}>
                            <Button
                              color="lagoon.6"
                              radius="md"
                              loading={savingPolicies}
                              disabled={!canAdminSelectedProject}
                              onClick={handleSavePolicies}
                            >
                              Save policy directly
                            </Button>
                            <Button
                              color="sand.6"
                              variant="light"
                              radius="md"
                              loading={submittingChange}
                              disabled={!canAdminSelectedProject}
                              onClick={handleCreatePolicyChange}
                            >
                              Open policy change
                            </Button>
                          </Group>
                        </>
                      ) : (
                        <Text className={classes.emptyBody}>No policy data available yet.</Text>
                      )}
                    </Stack>
                  </Paper>
                </Grid.Col>
              </Grid>
            </section>
          ) : null}

          <Divider color="rgba(36, 57, 55, 0.12)" />

          <Grid gutter="xl">
            <Grid.Col span={{ base: 12, lg: 5 }}>
              <Stack gap="lg">
                <section>
                  <Group justify="space-between" align="end" mb="md">
                    <div>
                      <Text className={classes.sectionEyebrow}>Applications</Text>
                      <Title order={2} className={classes.sectionTitle}>
                        {selectedProject?.name ?? 'Select a project'}
                      </Title>
                    </div>
                    {applicationsLoading ? <Loader size="sm" color="lagoon.5" /> : null}
                  </Group>

                  <Stack gap="sm">
                    {applications.map((application) => (
                      <UnstyledButton
                        key={application.id}
                        onClick={() => setSelectedApplicationId(application.id)}
                        className={classes.buttonReset}
                      >
                        <Card
                          radius="lg"
                          className={
                            application.id === selectedApplicationId
                              ? `${classes.selectCard} ${classes.selectCardActive}`
                              : classes.selectCard
                          }
                        >
                          <Group justify="space-between" align="start" mb="xs">
                            <div>
                              <Text className={classes.cardTitle}>{application.name}</Text>
                              <Text className={classes.cardMeta}>{application.image}</Text>
                            </div>
                            <Badge
                              color={syncStatusColor(application.syncStatus)}
                              variant="light"
                              radius="sm"
                            >
                              {application.syncStatus}
                            </Badge>
                          </Group>
                          <Group gap="xs" className={classes.wrapRow}>
                            <Badge color="lagoon.6" variant="light" radius="sm">
                              {application.deploymentStrategy}
                            </Badge>
                            {activeChangeAppId === application.id && activeChange ? (
                              <Badge
                                color={changeStatusColor(activeChange.status)}
                                variant="light"
                                radius="sm"
                              >
                                change {activeChange.status}
                              </Badge>
                            ) : null}
                          </Group>
                        </Card>
                      </UnstyledButton>
                    ))}

                    {!applicationsLoading && applications.length === 0 ? (
                      <Paper className={classes.emptyState} radius="lg">
                        <Text className={classes.emptyHeadline}>No apps yet</Text>
                        <Text className={classes.emptyBody}>
                          Create the first standard or canary deployment to populate the GitOps
                          tree and deployment history.
                        </Text>
                      </Paper>
                    ) : null}
                  </Stack>
                </section>

                <section>
                  <Text className={classes.sectionEyebrow}>Create Application</Text>
                  <Title order={3} className={classes.sectionTitle}>
                    Direct deploy or open a reviewed change
                  </Title>

                  <Paper className={classes.formPanel} radius="lg">
                    <Stack gap="md">
                      {!canMutateSelectedProject && selectedProject ? (
                        <Alert color="sand" variant="light" title="Read-only project">
                          {selectedProject.role} role users can inspect apps, but only deployers
                          and admins can create or redeploy them.
                        </Alert>
                      ) : null}

                      <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
                        <TextInput
                          label="App name"
                          placeholder="my-app"
                          value={createForm.name}
                          disabled={!canMutateSelectedProject}
                          onChange={(event) => {
                            const value = event.currentTarget.value
                            setCreateForm((current) => ({
                              ...current,
                              name: value,
                            }))
                          }}
                        />
                        <Select
                          label="Environment"
                          data={environments.map((environment) => ({
                            value: environment.id,
                            label: `${environment.name} (${environment.writeMode})`,
                          }))}
                          value={createForm.environment}
                          disabled={!canMutateSelectedProject}
                          onChange={(value) =>
                            setCreateForm((current) => ({
                              ...current,
                              environment: value ?? '',
                            }))
                          }
                        />
                      </SimpleGrid>

                      <Textarea
                        label="Description"
                        placeholder="Internal service for batch processing"
                        autosize
                        minRows={2}
                        value={createForm.description}
                        disabled={!canMutateSelectedProject}
                        onChange={(event) => {
                          const value = event.currentTarget.value
                          setCreateForm((current) => ({
                            ...current,
                            description: value,
                          }))
                        }}
                      />

                      <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
                        <TextInput
                          label="Container image"
                          placeholder="repo/my-app:v1"
                          value={createForm.image}
                          disabled={!canMutateSelectedProject}
                          onChange={(event) => {
                            const value = event.currentTarget.value
                            setCreateForm((current) => ({
                              ...current,
                              image: value,
                            }))
                          }}
                        />
                        <Select
                          label="Deployment strategy"
                          data={allowedStrategies(projectPolicies).map((strategy) => ({
                            value: strategy,
                            label: strategy,
                          }))}
                          value={createForm.deploymentStrategy}
                          disabled={!canMutateSelectedProject}
                          onChange={(value) => {
                            if (!value || !isStrategy(value)) {
                              return
                            }
                            setCreateForm((current) => ({
                              ...current,
                              deploymentStrategy: value,
                            }))
                          }}
                        />
                      </SimpleGrid>

                      <NumberInput
                        label="Service port"
                        min={1}
                        max={65535}
                        value={createForm.servicePort}
                        disabled={!canMutateSelectedProject}
                        onChange={(value) =>
                          setCreateForm((current) => ({
                            ...current,
                            servicePort: Number(value) || 0,
                          }))
                        }
                      />

                      <div>
                        <Group justify="space-between" mb="xs">
                          <Text className={classes.secretLabel}>Secrets</Text>
                          <Button
                            size="xs"
                            variant="subtle"
                            color="lagoon.6"
                            disabled={!canMutateSelectedProject}
                            onClick={addSecretRow}
                          >
                            Add secret
                          </Button>
                        </Group>

                        <Stack gap="sm">
                          {createForm.secrets.map((secret, index) => (
                            <div key={`${index}-${secret.key}`} className={classes.secretRow}>
                              <TextInput
                                label="Key"
                                placeholder="DATABASE_URL"
                                value={secret.key}
                                disabled={!canMutateSelectedProject}
                                onChange={(event) =>
                                  updateSecret(index, 'key', event.currentTarget.value)
                                }
                              />
                              <Textarea
                                label="Value"
                                placeholder="postgres://..."
                                autosize
                                minRows={1}
                                value={secret.value}
                                disabled={!canMutateSelectedProject}
                                onChange={(event) =>
                                  updateSecret(index, 'value', event.currentTarget.value)
                                }
                              />
                              <Button
                                variant="light"
                                color="red"
                                onClick={() => removeSecretRow(index)}
                                disabled={
                                  !canMutateSelectedProject ||
                                  createForm.secrets.length === 1
                                }
                              >
                                Remove
                              </Button>
                            </div>
                          ))}
                        </Stack>
                      </div>

                      {selectedCreateEnvironment ? (
                        <Alert
                          color={
                            selectedCreateEnvironment.writeMode === 'pull_request'
                              ? 'sand'
                              : 'lagoon'
                          }
                          variant="light"
                          title={
                            selectedCreateEnvironment.writeMode === 'pull_request'
                              ? 'Reviewed change required'
                              : 'Direct push allowed'
                          }
                        >
                          {selectedCreateEnvironment.name} targets{' '}
                          {selectedCreateEnvironment.clusterId} and uses{' '}
                          {selectedCreateEnvironment.writeMode}.
                        </Alert>
                      ) : null}

                      <Button
                        color="lagoon.6"
                        radius="md"
                        loading={submittingCreate}
                        disabled={!selectedProjectId || !canMutateSelectedProject}
                        onClick={handleCreateApplication}
                      >
                        {selectedCreateEnvironment?.writeMode === 'pull_request'
                          ? 'Open application change'
                          : 'Create application'}
                      </Button>
                    </Stack>
                  </Paper>
                </section>

                <section>
                  <Text className={classes.sectionEyebrow}>Change Flow</Text>
                  <Title order={3} className={classes.sectionTitle}>
                    Draft, approve, and merge a selected change
                  </Title>

                  <Paper className={classes.formPanel} radius="lg">
                    {activeChange ? (
                      <Stack gap="md">
                        <Group justify="space-between" align="start">
                          <div>
                            <Text className={classes.cardTitle}>{activeChange.summary}</Text>
                            <Text className={classes.cardMeta}>
                              {activeChange.operation} · {activeChange.environment}
                            </Text>
                          </div>
                          <Badge
                            color={changeStatusColor(activeChange.status)}
                            variant="filled"
                            radius="sm"
                          >
                            {activeChange.status}
                          </Badge>
                        </Group>

                        <Group gap="xs" className={classes.wrapRow}>
                          <Badge
                            color={writeModeColor(activeChange.writeMode)}
                            variant="light"
                            radius="sm"
                          >
                            {activeChange.writeMode}
                          </Badge>
                          <Badge color="gray.6" variant="light" radius="sm">
                            created by {activeChange.createdBy}
                          </Badge>
                          {activeChange.approvedBy ? (
                            <Badge color="lagoon.6" variant="light" radius="sm">
                              approved by {activeChange.approvedBy}
                            </Badge>
                          ) : null}
                          {activeChange.mergedBy ? (
                            <Badge color="sand.6" variant="light" radius="sm">
                              merged by {activeChange.mergedBy}
                            </Badge>
                          ) : null}
                        </Group>

                        <div>
                          <Text className={classes.detailLabel}>Diff preview</Text>
                          <Stack gap="xs" mt="sm">
                            {activeChange.diffPreview.map((line) => (
                              <Text key={line} className={classes.monoLine}>
                                {line}
                              </Text>
                            ))}
                          </Stack>
                        </div>

                        <Group className={classes.actionRow}>
                          {activeChange.status === 'Draft' ? (
                            <Button
                              color="sand.6"
                              variant="light"
                              radius="md"
                              loading={submittingChange}
                              disabled={!canMutateSelectedProject}
                              onClick={() => handleChangeAction('submit')}
                            >
                              Submit change
                            </Button>
                          ) : null}

                          {activeChange.status === 'Submitted' ? (
                            <Button
                              color="lagoon.6"
                              radius="md"
                              loading={submittingChange}
                              disabled={!canAdminSelectedProject}
                              onClick={() => handleChangeAction('approve')}
                            >
                              Approve change
                            </Button>
                          ) : null}

                          {activeChange.status !== 'Merged' &&
                          (activeChange.writeMode === 'direct' ||
                            activeChange.status === 'Approved') ? (
                            <Button
                              color="coral.6"
                              variant="light"
                              radius="md"
                              loading={submittingChange}
                              disabled={!canMutateSelectedProject}
                              onClick={() => handleChangeAction('merge')}
                            >
                              Merge change
                            </Button>
                          ) : null}
                        </Group>
                      </Stack>
                    ) : (
                      <div className={classes.emptyState}>
                        <Text className={classes.emptyHeadline}>No active change selected</Text>
                        <Text className={classes.emptyBody}>
                          Open a change from application creation, redeploy, or policy editing to
                          drive the reviewed flow from here.
                        </Text>
                      </div>
                    )}
                  </Paper>
                </section>
              </Stack>
            </Grid.Col>

            <Grid.Col span={{ base: 12, lg: 7 }}>
              <Stack gap="lg">
                <section>
                  <Text className={classes.sectionEyebrow}>Application Detail</Text>
                  <Title order={2} className={classes.sectionTitle}>
                    {selectedApplication?.name ?? 'Pick an application'}
                  </Title>

                  <Paper className={classes.detailPanel} radius="lg">
                    {selectedApplication ? (
                      <Stack gap="lg">
                        <Group justify="space-between" align="start">
                          <div>
                            <Text className={classes.cardTitle}>{selectedApplication.name}</Text>
                            <Text className={classes.cardMeta}>{selectedApplication.image}</Text>
                          </div>
                          <Badge
                            color={syncStatusColor(syncStatus?.status ?? selectedApplication.syncStatus)}
                            variant="filled"
                            radius="sm"
                          >
                            {syncStatus?.status ?? selectedApplication.syncStatus}
                          </Badge>
                        </Group>

                        <SimpleGrid cols={{ base: 1, sm: 3 }} spacing="md">
                          <Card radius="lg" className={classes.detailCard}>
                            <Text className={classes.detailLabel}>Deployment Strategy</Text>
                            <Text className={classes.detailValue}>
                              {selectedApplication.deploymentStrategy}
                            </Text>
                          </Card>
                          <Card radius="lg" className={classes.detailCard}>
                            <Text className={classes.detailLabel}>Current Environment</Text>
                            <Text className={classes.detailValue}>
                              {latestDeploymentEnvironment || 'n/a'}
                            </Text>
                          </Card>
                          <Card radius="lg" className={classes.detailCard}>
                            <Text className={classes.detailLabel}>Latest Cluster</Text>
                            <Text className={classes.detailValue}>
                              {latestCluster?.name ?? latestCluster?.id ?? 'n/a'}
                            </Text>
                          </Card>
                        </SimpleGrid>

                        <Card radius="lg" className={classes.detailCard}>
                          <Text className={classes.detailLabel}>Latest Sync Message</Text>
                          <Text className={classes.detailBody}>
                            {syncStatus?.message ?? 'Waiting for sync insight'}
                          </Text>
                        </Card>

                        <div>
                          <Group justify="space-between" align="center" mb="sm">
                            <Title order={3} className={classes.sectionTitle}>
                              Metrics Snapshot
                            </Title>
                            {detailsLoading ? <Loader size="sm" color="lagoon.5" /> : null}
                          </Group>
                          {metrics?.metrics.length ? (
                            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
                              {metrics.metrics.map((series) => (
                                <MetricCard key={series.key} series={series} />
                              ))}
                            </SimpleGrid>
                          ) : (
                            <Paper className={classes.emptyState} radius="lg">
                              <Text className={classes.emptyHeadline}>No metric series yet</Text>
                              <Text className={classes.emptyBody}>
                                Prometheus is reachable, but this application does not have scrape
                                data yet.
                              </Text>
                            </Paper>
                          )}
                        </div>

                        <Divider color="rgba(36, 57, 55, 0.12)" />

                        <div>
                          <Text className={classes.sectionEyebrow}>Deployment Control</Text>
                          <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md" mb="md">
                            <Select
                              label="Target environment"
                              data={environments.map((environment) => ({
                                value: environment.id,
                                label: `${environment.name} (${environment.writeMode})`,
                              }))}
                              value={deployEnvironment}
                              disabled={!canMutateSelectedProject}
                              onChange={(value) => setDeployEnvironment(value ?? '')}
                            />
                            <TextInput
                              label="New image tag"
                              placeholder="v2"
                              value={imageTag}
                              disabled={!canMutateSelectedProject}
                              onChange={(event) => setImageTag(event.currentTarget.value)}
                            />
                          </SimpleGrid>
                          <Group className={classes.actionRow}>
                            <Button
                              color="lagoon.6"
                              radius="md"
                              loading={submittingDeploy}
                              disabled={!canMutateSelectedProject || !selectedApplication}
                              onClick={handleRedeploy}
                            >
                              {selectedDeployEnvironment?.writeMode === 'pull_request'
                                ? 'Open redeploy change'
                                : 'Trigger redeploy'}
                            </Button>
                            {selectedDeployEnvironment ? (
                              <Badge
                                color={writeModeColor(selectedDeployEnvironment.writeMode)}
                                variant="light"
                                radius="sm"
                              >
                                {selectedDeployEnvironment.name} uses{' '}
                                {selectedDeployEnvironment.writeMode}
                              </Badge>
                            ) : null}
                          </Group>
                        </div>

                        {selectedApplication.deploymentStrategy === 'Canary' && latestDeployment ? (
                          <>
                            <Divider color="rgba(36, 57, 55, 0.12)" />
                            <div>
                              <Text className={classes.sectionEyebrow}>Canary Rollout</Text>
                              <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md" mb="md">
                                <Card radius="lg" className={classes.detailCard}>
                                  <Text className={classes.detailLabel}>Rollout Phase</Text>
                                  <Text className={classes.detailValue}>
                                    {latestDeployment.rolloutPhase || latestDeployment.status}
                                  </Text>
                                </Card>
                                <Card radius="lg" className={classes.detailCard}>
                                  <Text className={classes.detailLabel}>Traffic Weight</Text>
                                  <Text className={classes.detailValue}>
                                    {latestDeployment.canaryWeight ?? 0}%
                                  </Text>
                                </Card>
                              </SimpleGrid>
                              <Card radius="lg" className={classes.detailCard}>
                                <Text className={classes.detailLabel}>Rollout Message</Text>
                                <Text className={classes.detailBody}>
                                  {latestDeployment.message || 'Rollout detail is not available yet.'}
                                </Text>
                              </Card>
                              <Group className={classes.actionRow} mt="md">
                                <Button
                                  color="lagoon.6"
                                  radius="md"
                                  loading={
                                    promotingDeploymentId === latestDeployment.deploymentId
                                  }
                                  disabled={!canMutateSelectedProject}
                                  onClick={handlePromoteDeployment}
                                >
                                  Promote canary
                                </Button>
                                <Button
                                  color="coral.6"
                                  variant="light"
                                  radius="md"
                                  loading={
                                    abortingDeploymentId === latestDeployment.deploymentId
                                  }
                                  disabled={!canMutateSelectedProject}
                                  onClick={handleAbortDeployment}
                                >
                                  Abort canary
                                </Button>
                              </Group>
                            </div>
                          </>
                        ) : null}

                        <Divider color="rgba(36, 57, 55, 0.12)" />

                        <div>
                          <Text className={classes.sectionEyebrow}>Rollback Policy</Text>
                          <Text className={classes.sectionMeta}>
                            Project auto rollback is{' '}
                            {projectPolicies?.autoRollbackEnabled ? 'enabled' : 'disabled'}.
                          </Text>
                          <SimpleGrid cols={{ base: 1, md: 3 }} spacing="md" mt="md">
                            <Switch
                              checked={rollbackPolicy.enabled}
                              disabled={!canMutateSelectedProject}
                              label="Enable policy"
                              onChange={(event) =>
                                setRollbackPolicy((current) => ({
                                  ...current,
                                  enabled: event.currentTarget.checked,
                                }))
                              }
                            />
                            <NumberInput
                              label="Max error rate (%)"
                              min={0}
                              decimalScale={2}
                              value={rollbackPolicy.maxErrorRate ?? ''}
                              disabled={!canMutateSelectedProject}
                              onChange={(value) =>
                                setRollbackPolicy((current) => ({
                                  ...current,
                                  maxErrorRate:
                                    value === '' ? undefined : Number(value) || undefined,
                                }))
                              }
                            />
                            <NumberInput
                              label="Min request rate"
                              min={0}
                              decimalScale={2}
                              value={rollbackPolicy.minRequestRate ?? ''}
                              disabled={!canMutateSelectedProject}
                              onChange={(value) =>
                                setRollbackPolicy((current) => ({
                                  ...current,
                                  minRequestRate:
                                    value === '' ? undefined : Number(value) || undefined,
                                }))
                              }
                            />
                          </SimpleGrid>
                          <NumberInput
                            label="Max p95 latency (ms)"
                            min={0}
                            value={rollbackPolicy.maxLatencyP95Ms ?? ''}
                            disabled={!canMutateSelectedProject}
                            onChange={(value) =>
                              setRollbackPolicy((current) => ({
                                ...current,
                                maxLatencyP95Ms:
                                  value === '' ? undefined : Number(value) || undefined,
                              }))
                            }
                            mt="md"
                          />
                          <Button
                            color="sand.6"
                            variant="light"
                            radius="md"
                            mt="md"
                            loading={savingRollback}
                            disabled={!canMutateSelectedProject}
                            onClick={handleSaveRollbackPolicy}
                          >
                            Save rollback policy
                          </Button>
                        </div>
                      </Stack>
                    ) : (
                      <div className={classes.emptyState}>
                        <Text className={classes.emptyHeadline}>No application selected</Text>
                        <Text className={classes.emptyBody}>
                          Choose an application on the left to inspect rollout state, metrics,
                          rollback policy, and change-managed redeploy controls.
                        </Text>
                      </div>
                    )}
                  </Paper>
                </section>

                <Grid gutter="xl">
                  <Grid.Col span={{ base: 12, lg: 6 }}>
                    <section>
                      <Group justify="space-between" align="center" mb="md">
                        <div>
                          <Text className={classes.sectionEyebrow}>Deployment History</Text>
                          <Title order={3} className={classes.sectionTitle}>
                            Timeline of rollout attempts
                          </Title>
                        </div>
                        {historyLoading ? <Loader size="sm" color="lagoon.5" /> : null}
                      </Group>

                      <Stack gap="sm">
                        {deployments.map((deployment, index) => (
                          <Card
                            key={deployment.deploymentId}
                            radius="lg"
                            className={
                              index === 0
                                ? `${classes.detailCard} ${classes.selectCardActive}`
                                : classes.detailCard
                            }
                          >
                            <Group justify="space-between" align="start" mb="xs">
                              <div>
                                <Text className={classes.cardTitle}>{deployment.imageTag}</Text>
                                <Text className={classes.cardMeta}>
                                  {deployment.environment} · {deployment.deploymentId}
                                </Text>
                              </div>
                              <Badge
                                color={deploymentStatusColor(deployment)}
                                variant="light"
                                radius="sm"
                              >
                                {deployment.rolloutPhase || deployment.status}
                              </Badge>
                            </Group>
                            <Text className={classes.cardBody}>
                              {deployment.message || deployment.image}
                            </Text>
                          </Card>
                        ))}

                        {!historyLoading && deployments.length === 0 ? (
                          <Paper className={classes.emptyState} radius="lg">
                            <Text className={classes.emptyHeadline}>No deployment history yet</Text>
                            <Text className={classes.emptyBody}>
                              The application history will appear here after create or redeploy.
                            </Text>
                          </Paper>
                        ) : null}
                      </Stack>
                    </section>
                  </Grid.Col>

                  <Grid.Col span={{ base: 12, lg: 6 }}>
                    <section>
                      <Group justify="space-between" align="center" mb="md">
                        <div>
                          <Text className={classes.sectionEyebrow}>Event Feed</Text>
                          <Title order={3} className={classes.sectionTitle}>
                            Audit-style application events
                          </Title>
                        </div>
                        {historyLoading ? <Loader size="sm" color="lagoon.5" /> : null}
                      </Group>

                      <Stack gap="sm">
                        {events.map((event) => (
                          <Card key={event.id} radius="lg" className={classes.detailCard}>
                            <Group justify="space-between" align="start" mb="xs">
                              <div>
                                <Text className={classes.cardTitle}>{event.type}</Text>
                                <Text className={classes.cardMeta}>
                                  {formatTimestamp(event.createdAt)}
                                </Text>
                              </div>
                            </Group>
                            <Text className={classes.cardBody}>{event.message}</Text>
                          </Card>
                        ))}

                        {!historyLoading && events.length === 0 ? (
                          <Paper className={classes.emptyState} radius="lg">
                            <Text className={classes.emptyHeadline}>No events yet</Text>
                            <Text className={classes.emptyBody}>
                              Create, deploy, promote, abort, and rollback policy actions will
                              append audit events here.
                            </Text>
                          </Paper>
                        ) : null}
                      </Stack>
                    </section>
                  </Grid.Col>
                </Grid>
              </Stack>
            </Grid.Col>
          </Grid>
        </Stack>
      </Container>
    </div>
  )
}

type StatCardProps = {
  label: string
  value: string
}

function StatCard({ label, value }: StatCardProps) {
  return (
    <Card radius="lg" className={classes.statCard}>
      <Text className={classes.statLabel}>{label}</Text>
      <Text className={classes.statValue}>{value}</Text>
    </Card>
  )
}

type MetricCardProps = {
  series: MetricSeries
}

function MetricCard({ series }: MetricCardProps) {
  return (
    <Card radius="lg" className={classes.metricCard}>
      <Text className={classes.detailLabel}>{series.label}</Text>
      <Text className={classes.metricValue}>{latestMetricValue(series)}</Text>
      <Text className={classes.detailBody}>
        Last sample: {series.points.at(-1)?.timestamp.slice(11, 16) ?? 'n/a'}
      </Text>
    </Card>
  )
}

function roleColor(role: ProjectSummary['role']) {
  switch (role) {
    case 'admin':
      return 'coral.6'
    case 'deployer':
      return 'lagoon.6'
    default:
      return 'sand.6'
  }
}

function canMutateProject(role: ProjectSummary['role'] | undefined) {
  return role === 'deployer' || role === 'admin'
}

function syncStatusColor(status: SyncStatus) {
  switch (status) {
    case 'Synced':
      return 'lagoon.6'
    case 'Syncing':
      return 'sand.6'
    case 'Degraded':
      return 'coral.6'
    default:
      return 'gray.6'
  }
}

function changeStatusColor(status: ChangeRecord['status']) {
  switch (status) {
    case 'Merged':
      return 'lagoon.6'
    case 'Approved':
      return 'coral.6'
    case 'Submitted':
      return 'sand.6'
    default:
      return 'gray.6'
  }
}

function writeModeColor(mode: EnvironmentSummary['writeMode']) {
  return mode === 'pull_request' ? 'sand.6' : 'lagoon.6'
}

function deploymentStatusColor(deployment: DeploymentRecord) {
  if (deployment.syncStatus) {
    return syncStatusColor(deployment.syncStatus)
  }
  if (deployment.status === 'Aborted') {
    return 'coral.6'
  }
  if (deployment.status === 'Promoted') {
    return 'lagoon.6'
  }
  return 'sand.6'
}

function latestMetricValue(series: MetricSeries) {
  const latest = [...series.points].reverse().find((point) => point.value !== null)
  if (!latest?.value && latest?.value !== 0) {
    return 'No data'
  }

  if (series.unit === '%') {
    return `${latest.value.toFixed(2)}${series.unit}`
  }

  if (series.unit === 'cores') {
    return `${latest.value.toFixed(2)} ${series.unit}`
  }

  return `${Math.round(latest.value)} ${series.unit}`
}

function resolveDefaultEnvironment(environments: EnvironmentSummary[]) {
  return environments.find((environment) => environment.default) ?? environments[0] ?? null
}

function resolvePreferredStrategy(policy: ProjectPolicy | null) {
  if (!policy || policy.allowedDeploymentStrategies.length === 0) {
    return 'Standard' as const
  }
  if (policy.allowedDeploymentStrategies.includes('Standard')) {
    return 'Standard' as const
  }
  return policy.allowedDeploymentStrategies[0] === 'Canary' ? 'Canary' : 'Standard'
}

function allowedStrategies(policy: ProjectPolicy | null) {
  if (!policy || policy.allowedDeploymentStrategies.length === 0) {
    return ['Standard', 'Canary'] as const
  }
  return policy.allowedDeploymentStrategies.filter(isStrategy)
}

function parseCommaList(value: string) {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function stringifyList(values: string[]) {
  return values.join(', ')
}

function normalizeRollbackPolicy(policy: RollbackPolicy | null | undefined): RollbackPolicy {
  return {
    enabled: policy?.enabled ?? false,
    maxErrorRate: policy?.maxErrorRate,
    maxLatencyP95Ms: policy?.maxLatencyP95Ms,
    minRequestRate: policy?.minRequestRate,
  }
}

function resolveEnvironmentWriteMode(environments: EnvironmentSummary[], environmentId: string) {
  return environments.find((environment) => environment.id === environmentId)?.writeMode ?? 'direct'
}

function resolvePreferredChangeEnvironment(environments: EnvironmentSummary[]) {
  return (
    environments.find((environment) => environment.writeMode === 'pull_request')?.id ??
    resolveDefaultEnvironment(environments)?.id ??
    'prod'
  )
}

function inferChangeApplicationId(change: ChangeRecord | null) {
  if (!change) {
    return null
  }
  if (change.applicationId) {
    return change.applicationId
  }
  if (change.operation === 'CreateApplication' && change.request?.name) {
    return `${change.projectId}__${change.request.name}`
  }
  return null
}

function isStrategy(value: string): value is 'Standard' | 'Canary' {
  return value === 'Standard' || value === 'Canary'
}

function formatTimestamp(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
}

function toErrorMessage(error: unknown) {
  if (error instanceof ApiError) {
    return `${error.code}: ${error.message}`
  }

  if (error instanceof Error) {
    return error.message
  }

  return 'An unexpected error occurred.'
}
