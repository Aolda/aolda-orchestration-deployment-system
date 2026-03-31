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
  SimpleGrid,
  Stack,
  Text,
  TextInput,
  Textarea,
  Title,
  UnstyledButton,
} from '@mantine/core'
import { api, ApiError } from './api/client'
import type {
  ApplicationMetricsResponse,
  ApplicationSummary,
  CurrentUser,
  MetricSeries,
  ProjectSummary,
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
  secrets: SecretEntry[]
}

const initialCreateForm: CreateFormState = {
  name: '',
  description: '',
  image: 'repo/my-app:v1',
  servicePort: 8080,
  secrets: [{ key: 'DATABASE_URL', value: '' }],
}

function App() {
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [projects, setProjects] = useState<ProjectSummary[]>([])
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)
  const [applications, setApplications] = useState<ApplicationSummary[]>([])
  const [selectedApplicationId, setSelectedApplicationId] = useState<string | null>(
    null,
  )
  const [syncStatus, setSyncStatus] = useState<SyncStatusResponse | null>(null)
  const [metrics, setMetrics] = useState<ApplicationMetricsResponse | null>(null)
  const [bootstrapLoading, setBootstrapLoading] = useState(true)
  const [applicationsLoading, setApplicationsLoading] = useState(false)
  const [detailsLoading, setDetailsLoading] = useState(false)
  const [submittingCreate, setSubmittingCreate] = useState(false)
  const [submittingDeploy, setSubmittingDeploy] = useState(false)
  const [globalError, setGlobalError] = useState<string | null>(null)
  const [createForm, setCreateForm] = useState<CreateFormState>(initialCreateForm)
  const [imageTag, setImageTag] = useState('v2')

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
      setSelectedApplicationId(null)
      return
    }

    let cancelled = false

    ;(async () => {
      setApplicationsLoading(true)
      setGlobalError(null)

      try {
        const response = await api.getApplications(selectedProjectId)
        if (cancelled) {
          return
        }

        setApplications(response.items)
        setSelectedApplicationId((current) => {
          const stillPresent = response.items.some((item) => item.id === current)
          if (stillPresent) {
            return current
          }
          return response.items[0]?.id ?? null
        })
      } catch (error) {
        if (!cancelled) {
          setGlobalError(toErrorMessage(error))
        }
      } finally {
        if (!cancelled) {
          setApplicationsLoading(false)
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
      return
    }

    let cancelled = false

    ;(async () => {
      setDetailsLoading(true)

      try {
        const [syncResponse, metricsResponse] = await Promise.all([
          api.getSyncStatus(selectedApplicationId),
          api.getMetrics(selectedApplicationId),
        ])

        if (cancelled) {
          return
        }

        setSyncStatus(syncResponse)
        setMetrics(metricsResponse)
      } catch (error) {
        if (!cancelled) {
          setGlobalError(toErrorMessage(error))
        }
      } finally {
        if (!cancelled) {
          setDetailsLoading(false)
        }
      }
    })()

    return () => {
      cancelled = true
    }
  }, [selectedApplicationId])

  const selectedProject = projects.find((project) => project.id === selectedProjectId) ?? null
  const selectedApplication =
    applications.find((application) => application.id === selectedApplicationId) ?? null
  const canMutateSelectedProject = canMutateProject(selectedProject?.role)

  async function handleCreateApplication() {
    if (!selectedProjectId || !canMutateSelectedProject) {
      return
    }

    setSubmittingCreate(true)
    setGlobalError(null)

    try {
      const payload = {
        name: createForm.name.trim(),
        description: createForm.description.trim(),
        image: createForm.image.trim(),
        servicePort: createForm.servicePort,
        deploymentStrategy: 'Standard' as const,
        secrets: createForm.secrets.filter(
          (secret) => secret.key.trim() !== '' || secret.value.trim() !== '',
        ),
      }

      const created = await api.createApplication(selectedProjectId, payload)
      const appResponse = await api.getApplications(selectedProjectId)
      setApplications(appResponse.items)
      setSelectedApplicationId(created.id)
      setCreateForm({
        ...initialCreateForm,
        image: `${created.image.split(':')[0]}:v1`,
      })
    } catch (error) {
      setGlobalError(toErrorMessage(error))
    } finally {
      setSubmittingCreate(false)
    }
  }

  async function handleRedeploy() {
    if (!selectedApplicationId || !canMutateSelectedProject) {
      return
    }

    setSubmittingDeploy(true)
    setGlobalError(null)

    try {
      await api.createDeployment(selectedApplicationId, imageTag.trim())
      const [appResponse, syncResponse, metricsResponse] = await Promise.all([
        selectedProjectId ? api.getApplications(selectedProjectId) : Promise.resolve(null),
        api.getSyncStatus(selectedApplicationId),
        api.getMetrics(selectedApplicationId),
      ])

      if (appResponse) {
        setApplications(appResponse.items)
      }

      setSyncStatus(syncResponse)
      setMetrics(metricsResponse)
    } catch (error) {
      setGlobalError(toErrorMessage(error))
    } finally {
      setSubmittingDeploy(false)
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
                <Text className={classes.kicker}>AODS Phase 1 Portal</Text>
                <Title order={1} className={classes.heading}>
                  Git-backed deployment control for standard app rollouts
                </Title>
                <Text className={classes.lead}>
                  Projects come from <code>platform/projects.yaml</code>. App metadata stays
                  in the GitOps tree, secrets are staged outside git, and deployment state is
                  surfaced through a constrained sync and metrics view.
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

              <SimpleGrid cols={{ base: 1, sm: 3 }} spacing="md">
                <StatCard label="Accessible Projects" value={String(projects.length)} />
                <StatCard label="Applications In View" value={String(applications.length)} />
                <StatCard
                  label="Selected Sync Status"
                  value={syncStatus?.status ?? 'Unknown'}
                />
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
                  Authorized projects from GitHub-backed catalog
                </Title>
              </div>
              <Text className={classes.sectionMeta}>
                Source of truth remains the default branch.
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
                          <Text className={classes.cardBody}>
                            Strategy: {application.deploymentStrategy}
                          </Text>
                        </Card>
                      </UnstyledButton>
                    ))}

                    {!applicationsLoading && applications.length === 0 ? (
                      <Paper className={classes.emptyState} radius="lg">
                        <Text className={classes.emptyHeadline}>No apps yet</Text>
                        <Text className={classes.emptyBody}>
                          Create the first standard deployment for this project to populate
                          the GitOps tree.
                        </Text>
                      </Paper>
                    ) : null}
                  </Stack>
                </section>

                <section>
                  <Text className={classes.sectionEyebrow}>Create Application</Text>
                  <Title order={3} className={classes.sectionTitle}>
                    Standard deployment only
                  </Title>

                  <Paper className={classes.formPanel} radius="lg">
                    <Stack gap="md">
                      {!canMutateSelectedProject && selectedProject ? (
                        <Alert color="sand" variant="light" title="Read-only project">
                          {selectedProject.role} role users can inspect apps, but only deployers
                          and admins can create or redeploy them.
                        </Alert>
                      ) : null}
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

                      <Button
                        color="lagoon.6"
                        radius="md"
                        loading={submittingCreate}
                        disabled={!selectedProjectId || !canMutateSelectedProject}
                        onClick={handleCreateApplication}
                      >
                        Create application
                      </Button>
                    </Stack>
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

                        <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
                          <Card radius="lg" className={classes.detailCard}>
                            <Text className={classes.detailLabel}>Deployment Strategy</Text>
                            <Text className={classes.detailValue}>
                              {selectedApplication.deploymentStrategy}
                            </Text>
                          </Card>
                          <Card radius="lg" className={classes.detailCard}>
                            <Text className={classes.detailLabel}>Latest Sync Message</Text>
                            <Text className={classes.detailBody}>
                              {syncStatus?.message ?? 'Waiting for sync insight'}
                            </Text>
                          </Card>
                        </SimpleGrid>

                        <div>
                          <Group justify="space-between" align="center" mb="sm">
                            <Title order={3} className={classes.sectionTitle}>
                              Metrics Snapshot
                            </Title>
                            {detailsLoading ? <Loader size="sm" color="lagoon.5" /> : null}
                          </Group>
                          <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
                            {metrics?.metrics.map((series) => (
                              <MetricCard key={series.key} series={series} />
                            ))}
                          </SimpleGrid>
                        </div>

                        <Divider color="rgba(36, 57, 55, 0.12)" />

                        <div>
                          <Text className={classes.sectionEyebrow}>Standard Redeploy</Text>
                          <Group align="end" className={classes.deployRow}>
                            <TextInput
                              label="New image tag"
                              placeholder="v2"
                              value={imageTag}
                              disabled={!canMutateSelectedProject || !selectedApplication}
                              onChange={(event) => setImageTag(event.currentTarget.value)}
                              className={classes.deployInput}
                            />
                            <Button
                              color="lagoon.6"
                              radius="md"
                              loading={submittingDeploy}
                              disabled={!canMutateSelectedProject || !selectedApplication}
                              onClick={handleRedeploy}
                            >
                              Trigger redeploy
                            </Button>
                          </Group>
                        </div>
                      </Stack>
                    ) : (
                      <div className={classes.emptyState}>
                        <Text className={classes.emptyHeadline}>No application selected</Text>
                        <Text className={classes.emptyBody}>
                          Choose an application on the left to inspect sync status, metrics,
                          and redeploy controls.
                        </Text>
                      </div>
                    )}
                  </Paper>
                </section>
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

function toErrorMessage(error: unknown) {
  if (error instanceof ApiError) {
    return `${error.code}: ${error.message}`
  }

  if (error instanceof Error) {
    return error.message
  }

  return 'An unexpected error occurred.'
}

export default App
