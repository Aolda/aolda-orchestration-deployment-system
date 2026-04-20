import { useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Badge,
  Button,
  Divider,
  Grid,
  Group,
  Select,
  SimpleGrid,
  Stack,
  Text,
  TextInput,
  Textarea,
  UnstyledButton,
} from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { IconAlertCircle, IconArrowRight, IconRefresh, IconSearch } from '@tabler/icons-react'
import type {
  ApplicationSummary,
  ChangeOperation,
  ChangeRecord,
  ChangeStatus,
  CreateChangeRequest,
  CurrentUser,
  EnvironmentSummary,
  ProjectSummary,
} from '../../types/api'
import { PageHeader } from '../../components/ui/PageHeader'
import { SurfaceCard } from '../../components/ui/SurfaceCard'
import { StatePanel } from '../../components/ui/StatePanel'
import classes from './ChangesWorkspace.module.css'

type ChangeActionKind = 'submit' | 'approve' | 'merge'
type CreateChangeOperation = Exclude<ChangeOperation, 'UpdatePolicies'>
type SortOption = 'updated_desc' | 'updated_asc' | 'created_desc'

type CreateChangeForm = {
  operation: CreateChangeOperation
  applicationId: string
  name: string
  description: string
  image: string
  servicePort: string
  deploymentStrategy: 'Rollout' | 'Canary'
  environment: string
  imageTag: string
  summary: string
}

type ChangesWorkspaceProps = {
  project?: ProjectSummary
  currentUser: CurrentUser | null
  applications: ApplicationSummary[]
  environments: EnvironmentSummary[]
  changes: ChangeRecord[]
  selectedChangeId: string | null
  onSelectChange: (changeId: string) => void
  onCreateChange: (body: CreateChangeRequest) => Promise<void>
  onTrackChange: (changeId: string) => Promise<void>
  onRefreshChanges: () => Promise<void>
  onSubmitChange: (changeId: string) => Promise<void>
  onApproveChange: (changeId: string) => Promise<void>
  onMergeChange: (changeId: string) => Promise<void>
  creatingChange: boolean
  refreshingChanges: boolean
  actionLoading: { changeId: string; action: ChangeActionKind } | null
  onOpenProjectChanges: () => void
}

const operationOptions: Array<{ value: CreateChangeOperation; label: string }> = [
  { value: 'Redeploy', label: '재배포' },
  { value: 'UpdateApplication', label: '앱 설정 변경' },
  { value: 'CreateApplication', label: '새 앱 생성' },
]

const statusOptions: Array<{ value: string; label: string }> = [
  { value: 'all', label: '전체 상태' },
  { value: 'Draft', label: 'Draft' },
  { value: 'Submitted', label: 'Submitted' },
  { value: 'Approved', label: 'Approved' },
  { value: 'Merged', label: 'Merged' },
]

const sortOptions: Array<{ value: SortOption; label: string }> = [
  { value: 'updated_desc', label: '최신 업데이트 순' },
  { value: 'created_desc', label: '최신 생성 순' },
  { value: 'updated_asc', label: '오래된 순' },
]

const initialCreateForm: CreateChangeForm = {
  operation: 'Redeploy',
  applicationId: '',
  name: '',
  description: '',
  image: '',
  servicePort: '',
  deploymentStrategy: 'Rollout',
  environment: '',
  imageTag: '',
  summary: '',
}

export function ChangesWorkspace({
  project,
  currentUser,
  applications,
  environments,
  changes,
  selectedChangeId,
  onSelectChange,
  onCreateChange,
  onTrackChange,
  onRefreshChanges,
  onSubmitChange,
  onApproveChange,
  onMergeChange,
  creatingChange,
  refreshingChanges,
  actionLoading,
  onOpenProjectChanges,
}: ChangesWorkspaceProps) {
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [environmentFilter, setEnvironmentFilter] = useState<string>('all')
  const [authorFilter, setAuthorFilter] = useState<string>('all')
  const [sortBy, setSortBy] = useState<SortOption>('updated_desc')
  const [trackedChangeId, setTrackedChangeId] = useState('')
  const [createForm, setCreateForm] = useState<CreateChangeForm>(initialCreateForm)

  const defaultEnvironmentId =
    environments.find((environment) => environment.default)?.id ?? environments[0]?.id ?? ''
  const defaultApplicationId = applications[0]?.id ?? ''

  const selectedChange = useMemo(
    () => changes.find((change) => change.id === selectedChangeId) ?? null,
    [changes, selectedChangeId],
  )
  const viewerMode = project?.role === 'viewer'

  const authorOptions = useMemo(() => {
    const uniqueAuthors = Array.from(new Set(changes.map((change) => change.createdBy))).sort()
    return [{ value: 'all', label: '전체 작성자' }, ...uniqueAuthors.map((author) => ({ value: author, label: author }))]
  }, [changes])

  const environmentOptions = useMemo(() => {
    return [
      { value: 'all', label: '전체 환경' },
      ...environments.map((environment) => ({
        value: environment.id,
        label: environment.name,
      })),
    ]
  }, [environments])

  const filteredChanges = useMemo(() => {
    const query = search.trim().toLowerCase()
    const next = changes.filter((change) => {
      if (statusFilter !== 'all' && change.status !== statusFilter) return false
      if (environmentFilter !== 'all' && change.environment !== environmentFilter) return false
      if (authorFilter !== 'all' && change.createdBy !== authorFilter) return false
      if (!query) return true
      return [
        change.id,
        change.summary,
        change.createdBy,
        change.applicationId ?? '',
        change.operation,
        ...change.diffPreview,
      ]
        .join(' ')
        .toLowerCase()
        .includes(query)
    })

    next.sort((left, right) => {
      if (sortBy === 'updated_asc') {
        return Date.parse(left.updatedAt) - Date.parse(right.updatedAt)
      }
      if (sortBy === 'created_desc') {
        return Date.parse(right.createdAt) - Date.parse(left.createdAt)
      }
      return Date.parse(right.updatedAt) - Date.parse(left.updatedAt)
    })

    return next
  }, [authorFilter, changes, environmentFilter, search, sortBy, statusFilter])

  useEffect(() => {
    if (!selectedChangeId && filteredChanges[0]) {
      onSelectChange(filteredChanges[0].id)
    }
  }, [filteredChanges, onSelectChange, selectedChangeId])

  const awaitingApprovalCount = changes.filter(
    (change) => change.writeMode === 'pull_request' && change.status === 'Submitted',
  ).length
  const readyToMergeCount = changes.filter((change) => isMergeReady(change)).length
  const lastUpdated = changes[0]?.updatedAt

  const handleTrackChange = async () => {
    const changeId = trackedChangeId.trim()
    if (!changeId) {
      notifications.show({
        title: '변경 ID 필요',
        message: '불러올 change ID를 입력하세요.',
        color: 'yellow',
      })
      return
    }

    try {
      await onTrackChange(changeId)
      setTrackedChangeId('')
    } catch {
      // notification handled upstream
    }
  }

  const handleCreateDraft = async () => {
    const normalizedForm = normalizeCreateChangeForm(
      createForm,
      defaultEnvironmentId,
      defaultApplicationId,
    )
    const validation = validateCreateChangeForm(normalizedForm)
    if (validation) {
      notifications.show({
        title: '입력 확인 필요',
        message: validation,
        color: 'yellow',
      })
      return
    }

    try {
      await onCreateChange(buildCreateChangeRequest(normalizedForm))
      setCreateForm((current) => ({
        ...initialCreateForm,
        environment: current.environment,
        applicationId: current.applicationId,
      }))
    } catch {
      // notification handled upstream
    }
  }

  const detailActionAvailability = selectedChange ? getActionAvailability(selectedChange, project?.role) : null

  return (
    <Stack gap="lg">
      <PageHeader
        eyebrow="변경 요청"
        title="변경 요청"
        description="프로젝트별 change draft를 추적하고, 제출, 승인, 반영 액션을 한 화면에서 다룹니다."
        actions={
          <Group gap="sm">
            <Button variant="light" color="gray" onClick={onOpenProjectChanges}>
              프로젝트 탭으로 돌아가기
            </Button>
            <Button
              variant="light"
              color="lagoon.6"
              leftSection={<IconRefresh size={16} />}
              onClick={() => {
                void onRefreshChanges()
              }}
              loading={refreshingChanges}
              disabled={!project}
            >
              추적 목록 새로고침
            </Button>
          </Group>
        }
      />

      <Alert color="lagoon.6" radius="lg" icon={<IconAlertCircle size={18} />}>
        프로젝트별 change 목록 API가 아직 없어 이 화면은 현재 세션에서 생성했거나 직접 불러온 change를 중심으로 동작합니다.
        목록 API가 열리면 같은 구조에 실제 프로젝트 히스토리를 연결할 수 있게 설계했습니다.
      </Alert>

      {project ? (
        <Group gap="xs">
          <Badge color="lagoon.6" variant="light" radius="sm">
            {project.name}
          </Badge>
          <Badge color="lagoon.8" variant="light" radius="sm">
            {formatProjectRole(project.role)}
          </Badge>
          <Badge color="gray" variant="light" radius="sm">
            {project.id}
          </Badge>
          {currentUser?.username ? (
            <Badge color="gray" variant="outline" radius="sm">
              현재 사용자 {currentUser.username}
            </Badge>
          ) : null}
        </Group>
      ) : (
        <StatePanel
          kind="empty"
          title="프로젝트를 선택해야 변경 요청 작업 공간이 열립니다"
          description="현재 프로젝트 문맥이 없어서 draft 생성과 change 추적을 시작할 수 없습니다."
        />
      )}

      <SimpleGrid cols={{ base: 1, sm: 2, xl: 4 }} spacing="md">
        <SurfaceCard>
          <Stack gap={4}>
            <Text size="xs" fw={700} c="dimmed">TRACKED CHANGES</Text>
            <Text className={classes.summaryValue}>{changes.length}</Text>
            <Text size="sm" c="lagoon.4">세션 또는 수동 불러오기 기준</Text>
          </Stack>
        </SurfaceCard>
        <SurfaceCard>
          <Stack gap={4}>
            <Text size="xs" fw={700} c="dimmed">AWAITING APPROVAL</Text>
            <Text className={classes.summaryValue}>{awaitingApprovalCount}</Text>
            <Text size="sm" c="lagoon.4">pull_request 환경에서 승인 대기</Text>
          </Stack>
        </SurfaceCard>
        <SurfaceCard>
          <Stack gap={4}>
            <Text size="xs" fw={700} c="dimmed">READY TO MERGE</Text>
            <Text className={classes.summaryValue}>{readyToMergeCount}</Text>
            <Text size="sm" c="lagoon.4">권한이 맞으면 바로 반영 가능</Text>
          </Stack>
        </SurfaceCard>
        <SurfaceCard>
          <Stack gap={4}>
            <Text size="xs" fw={700} c="dimmed">LAST UPDATED</Text>
            <Text className={classes.summaryValueSmall}>{lastUpdated ? formatTimestamp(lastUpdated) : '-'}</Text>
            <Text size="sm" c="lagoon.4">추적 중인 change 기준 최신 시각</Text>
          </Stack>
        </SurfaceCard>
      </SimpleGrid>

      <Grid gutter="lg" align="start">
        <Grid.Col span={{ base: 12, lg: 5 }}>
          <Stack gap="lg">
            <SurfaceCard>
              <Stack gap="md">
                <div>
                  <Text fw={800}>새 변경 요청 Draft</Text>
                  <Text size="sm" c="lagoon.4" mt={4}>
                    현재 UI에서는 Redeploy, UpdateApplication, CreateApplication 세 가지 유형만 빠르게 생성합니다.
                  </Text>
                </div>

                {viewerMode ? (
                  <StatePanel
                    kind="forbidden"
                    withCard={false}
                    title="viewer 역할은 change draft를 생성할 수 없습니다"
                    description="세부 내용은 조회할 수 있지만 draft 생성, 제출, 반영은 deployer 이상 권한에서만 가능합니다."
                  />
                ) : null}

                {environments.length === 0 ? (
                  <StatePanel
                    kind="empty"
                    withCard={false}
                    title="대상 환경이 없습니다"
                    description="프로젝트에 환경이 정의되어야 변경 요청 draft를 생성할 수 있습니다."
                  />
                ) : null}

                {!viewerMode && createForm.operation !== 'CreateApplication' && applications.length === 0 ? (
                  <StatePanel
                    kind="empty"
                    withCard={false}
                    title="대상 애플리케이션이 없습니다"
                    description="Redeploy와 UpdateApplication은 기존 애플리케이션이 있어야 생성할 수 있습니다."
                  />
                ) : null}

                <Select
                  label="작업 유형"
                  value={createForm.operation}
                  onChange={(value) => {
                    if (!value) return
                    setCreateForm((current) => ({
                      ...current,
                      operation: value as CreateChangeOperation,
                    }))
                  }}
                  data={operationOptions}
                  allowDeselect={false}
                />

                <Select
                  label="대상 환경"
                  value={createForm.environment || defaultEnvironmentId}
                  onChange={(value) => setCreateForm((current) => ({ ...current, environment: value ?? '' }))}
                  data={environments.map((environment) => ({
                    value: environment.id,
                    label: `${environment.name} · ${environment.writeMode === 'pull_request' ? '변경 요청' : '직접 반영'}`,
                  }))}
                  placeholder="환경 선택"
                  allowDeselect={false}
                  disabled={environments.length === 0}
                />

                {createForm.operation !== 'CreateApplication' ? (
                  <Select
                    label="대상 애플리케이션"
                    value={createForm.applicationId || defaultApplicationId}
                    onChange={(value) => setCreateForm((current) => ({ ...current, applicationId: value ?? '' }))}
                    data={applications.map((application) => ({
                      value: application.id,
                      label: `${application.name} · ${application.image}`,
                    }))}
                    placeholder="애플리케이션 선택"
                    searchable
                    disabled={applications.length === 0}
                  />
                ) : null}

                {createForm.operation === 'Redeploy' ? (
                  <TextInput
                    label="새 이미지 태그"
                    placeholder="예: v1.2.4"
                    value={createForm.imageTag}
                    onChange={(event) => setCreateForm((current) => ({ ...current, imageTag: event.target.value }))}
                  />
                ) : null}

                {createForm.operation === 'CreateApplication' ? (
                  <>
                    <TextInput
                      label="애플리케이션 이름"
                      placeholder="예: payment-api"
                      value={createForm.name}
                      onChange={(event) => setCreateForm((current) => ({ ...current, name: event.target.value }))}
                    />
                    <TextInput
                      label="이미지"
                      placeholder="예: ghcr.io/aolda/payment-api:v1.0.0"
                      value={createForm.image}
                      onChange={(event) => setCreateForm((current) => ({ ...current, image: event.target.value }))}
                    />
                    <TextInput
                      label="서비스 포트"
                      placeholder="예: 8080"
                      value={createForm.servicePort}
                      onChange={(event) => setCreateForm((current) => ({ ...current, servicePort: event.target.value }))}
                    />
                    <Select
                      label="배포 전략"
                      value={createForm.deploymentStrategy}
                      onChange={(value) => {
                        setCreateForm((current) => ({
                          ...current,
                          deploymentStrategy: value === 'Canary' ? 'Canary' : 'Rollout',
                        }))
                      }}
                      data={[
                        { value: 'Rollout', label: 'Rollout' },
                        { value: 'Canary', label: 'Canary' },
                      ]}
                      allowDeselect={false}
                    />
                  </>
                ) : null}

                {createForm.operation === 'UpdateApplication' ? (
                  <>
                    <Textarea
                      label="설명 변경"
                      placeholder="변경하려는 설명이 있으면 입력"
                      minRows={2}
                      value={createForm.description}
                      onChange={(event) => setCreateForm((current) => ({ ...current, description: event.target.value }))}
                    />
                    <TextInput
                      label="서비스 포트 변경"
                      placeholder="예: 9090"
                      value={createForm.servicePort}
                      onChange={(event) => setCreateForm((current) => ({ ...current, servicePort: event.target.value }))}
                    />
                  </>
                ) : null}

                <Textarea
                  label="변경 요약"
                  placeholder="예: 프로덕션 이미지 태그를 v1.2.4로 올립니다."
                  minRows={2}
                  value={createForm.summary}
                  onChange={(event) => setCreateForm((current) => ({ ...current, summary: event.target.value }))}
                />

                <Button
                  color="lagoon.6"
                  onClick={() => {
                    void handleCreateDraft()
                  }}
                  loading={creatingChange}
                  disabled={!project || viewerMode || environments.length === 0}
                >
                  Draft 생성
                </Button>
              </Stack>
            </SurfaceCard>

            <SurfaceCard>
              <Stack gap="md">
                <div>
                  <Text fw={800}>기존 Change 불러오기</Text>
                  <Text size="sm" c="lagoon.4" mt={4}>
                    목록 API가 없기 때문에 알고 있는 change ID를 직접 추가해 추적할 수 있습니다.
                  </Text>
                </div>
                <Group align="end">
                  <TextInput
                    className={classes.flexInput}
                    label="Change ID"
                    placeholder="예: chg_01HXYZ"
                    value={trackedChangeId}
                    onChange={(event) => setTrackedChangeId(event.target.value)}
                    leftSection={<IconSearch size={16} />}
                  />
                  <Button
                    variant="light"
                    color="lagoon.6"
                    onClick={() => {
                      void handleTrackChange()
                    }}
                    disabled={!project}
                  >
                    불러오기
                  </Button>
                </Group>
              </Stack>
            </SurfaceCard>

            <SurfaceCard>
              <Stack gap="md">
                <div>
                  <Text fw={800}>Tracked Changes</Text>
                  <Text size="sm" c="lagoon.4" mt={4}>
                    상태, 환경, 작성자 기준으로 탐색한 뒤 상세 패널에서 실제 액션을 실행할 수 있습니다.
                  </Text>
                </div>

                <div className={classes.filterGrid}>
                  <TextInput
                    label="검색"
                    placeholder="요약, ID, 앱 ID 검색"
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                  />
                  <Select
                    label="상태"
                    value={statusFilter}
                    onChange={(value) => setStatusFilter(value ?? 'all')}
                    data={statusOptions}
                    allowDeselect={false}
                  />
                  <Select
                    label="환경"
                    value={environmentFilter}
                    onChange={(value) => setEnvironmentFilter(value ?? 'all')}
                    data={environmentOptions}
                    allowDeselect={false}
                  />
                  <Select
                    label="작성자"
                    value={authorFilter}
                    onChange={(value) => setAuthorFilter(value ?? 'all')}
                    data={authorOptions}
                    allowDeselect={false}
                  />
                  <Select
                    label="정렬"
                    value={sortBy}
                    onChange={(value) => setSortBy((value as SortOption) ?? 'updated_desc')}
                    data={sortOptions}
                    allowDeselect={false}
                  />
                </div>

                {filteredChanges.length > 0 ? (
                  <Stack gap="sm">
                    {filteredChanges.map((change) => (
                      <UnstyledButton
                        key={change.id}
                        onClick={() => onSelectChange(change.id)}
                        className={`${classes.changeItem} ${
                          selectedChangeId === change.id ? classes.changeItemActive : ''
                        }`}
                      >
                        <Group justify="space-between" align="start" mb={8}>
                          <div>
                            <Text fw={800}>{change.summary}</Text>
                            <Text size="sm" c="lagoon.4">
                              {change.id}
                            </Text>
                          </div>
                          <Badge color={statusColor(change.status)} variant="light" radius="sm">
                            {change.status}
                          </Badge>
                        </Group>
                        <Group gap="xs">
                          <Badge color="gray" variant="outline" radius="sm">
                            {operationLabel(change.operation)}
                          </Badge>
                          <Badge color="gray" variant="outline" radius="sm">
                            {change.environment}
                          </Badge>
                          <Badge
                            color={change.writeMode === 'pull_request' ? 'indigo' : 'teal'}
                            variant="light"
                            radius="sm"
                          >
                            {change.writeMode === 'pull_request' ? '변경 요청' : '직접 반영'}
                          </Badge>
                        </Group>
                        <Text size="sm" c="lagoon.4" mt={10}>
                          작성자 {change.createdBy} · 업데이트 {formatTimestamp(change.updatedAt)}
                        </Text>
                      </UnstyledButton>
                    ))}
                  </Stack>
                ) : (
                  <StatePanel
                    kind="empty"
                    withCard={false}
                    title="검색 조건에 맞는 change가 없습니다"
                    description="필터를 조정하거나 Draft를 생성하거나 change ID로 직접 불러오세요."
                  />
                )}
              </Stack>
            </SurfaceCard>
          </Stack>
        </Grid.Col>

        <Grid.Col span={{ base: 12, lg: 7 }}>
          <SurfaceCard>
            {selectedChange ? (
              <Stack gap="lg">
                <div>
                  <Group justify="space-between" align="start">
                    <div>
                      <Text fw={900} size="xl">
                        {selectedChange.summary}
                      </Text>
                      <Text c="lagoon.4" mt={6}>
                        {selectedChange.id}
                      </Text>
                    </div>
                    <Group gap="xs">
                      <Badge color={statusColor(selectedChange.status)} variant="light" radius="sm">
                        {selectedChange.status}
                      </Badge>
                      <Badge
                        color={selectedChange.writeMode === 'pull_request' ? 'indigo' : 'teal'}
                        variant="light"
                        radius="sm"
                      >
                        {selectedChange.writeMode === 'pull_request' ? '변경 요청' : '직접 반영'}
                      </Badge>
                    </Group>
                  </Group>
                </div>

                <div className={classes.detailGrid}>
                  <DetailField label="작업 유형" value={operationLabel(selectedChange.operation)} />
                  <DetailField label="대상 환경" value={selectedChange.environment} />
                  <DetailField label="작성자" value={selectedChange.createdBy} />
                  <DetailField label="마지막 업데이트" value={formatTimestamp(selectedChange.updatedAt)} />
                  <DetailField label="애플리케이션 ID" value={selectedChange.applicationId || '-'} />
                  <DetailField label="생성 시각" value={formatTimestamp(selectedChange.createdAt)} />
                  <DetailField label="승인자" value={selectedChange.approvedBy || '-'} />
                  <DetailField label="반영자" value={selectedChange.mergedBy || '-'} />
                </div>

                <Divider />

                <Stack gap="sm">
                  <Text fw={800}>요청 본문</Text>
                  <Stack gap={6}>
                    {describeRequest(selectedChange).map((line) => (
                      <Text key={line} size="sm" c="lagoon.4">
                        {line}
                      </Text>
                    ))}
                  </Stack>
                </Stack>

                <Divider />

                <Stack gap="sm">
                  <Text fw={800}>Diff Preview</Text>
                  {selectedChange.diffPreview.length > 0 ? (
                    <pre className={classes.diffBlock}>
                      {selectedChange.diffPreview.join('\n')}
                    </pre>
                  ) : (
                    <StatePanel
                      kind="empty"
                      withCard={false}
                      title="diff preview가 없습니다"
                      description="이 change에는 비교 가능한 변경 라인이 아직 생성되지 않았습니다."
                    />
                  )}
                </Stack>

                <Divider />

                <Stack gap="sm">
                  <Text fw={800}>Action Bar</Text>
                  <Group gap="sm">
                    <Button
                      variant="light"
                      color="lagoon.6"
                      onClick={() => {
                        void onSubmitChange(selectedChange.id)
                      }}
                      disabled={!detailActionAvailability?.canSubmit}
                      loading={
                        actionLoading?.changeId === selectedChange.id && actionLoading.action === 'submit'
                      }
                    >
                      제출
                    </Button>
                    <Button
                      variant="light"
                      color="violet"
                      onClick={() => {
                        void onApproveChange(selectedChange.id)
                      }}
                      disabled={!detailActionAvailability?.canApprove}
                      loading={
                        actionLoading?.changeId === selectedChange.id && actionLoading.action === 'approve'
                      }
                    >
                      승인
                    </Button>
                    <Button
                      color="lagoon.8"
                      rightSection={<IconArrowRight size={16} />}
                      onClick={() => {
                        void onMergeChange(selectedChange.id)
                      }}
                      disabled={!detailActionAvailability?.canMerge}
                      loading={actionLoading?.changeId === selectedChange.id && actionLoading.action === 'merge'}
                    >
                      반영
                    </Button>
                  </Group>
                  <Text size="sm" c="lagoon.4">
                    {detailActionAvailability?.message ?? '현재 상태에서 실행 가능한 액션을 선택하세요.'}
                  </Text>
                </Stack>
              </Stack>
            ) : (
              <StatePanel
                kind="empty"
                withCard={false}
                title="선택된 change가 없습니다"
                description="왼쪽 목록에서 change를 선택하면 diff preview와 제출, 승인, 반영 액션이 열립니다."
              />
            )}
          </SurfaceCard>
        </Grid.Col>
      </Grid>
    </Stack>
  )
}

function buildCreateChangeRequest(form: CreateChangeForm): CreateChangeRequest {
  const request: CreateChangeRequest = {
    operation: form.operation,
    environment: form.environment,
    summary: form.summary.trim() || undefined,
  }

  if (form.operation === 'Redeploy') {
    request.applicationId = form.applicationId
    request.imageTag = form.imageTag.trim()
    return request
  }

  if (form.operation === 'UpdateApplication') {
    request.applicationId = form.applicationId
    request.description = form.description.trim() || undefined
    request.servicePort = toOptionalNumber(form.servicePort)
    return request
  }

  request.name = form.name.trim()
  request.description = form.description.trim() || undefined
  request.image = form.image.trim()
  request.servicePort = toOptionalNumber(form.servicePort)
  request.deploymentStrategy = form.deploymentStrategy
  return request
}

function normalizeCreateChangeForm(
  form: CreateChangeForm,
  defaultEnvironmentId: string,
  defaultApplicationId: string,
) {
  return {
    ...form,
    environment: form.environment || defaultEnvironmentId,
    applicationId:
      form.operation === 'CreateApplication'
        ? form.applicationId
        : form.applicationId || defaultApplicationId,
  }
}

function validateCreateChangeForm(form: CreateChangeForm) {
  if (!form.environment.trim()) {
    return '대상 환경을 선택하세요.'
  }

  switch (form.operation) {
    case 'Redeploy':
      if (!form.applicationId.trim()) {
        return '재배포 대상 애플리케이션을 선택하세요.'
      }
      if (!form.imageTag.trim()) {
        return '새 이미지 태그를 입력하세요.'
      }
      return null
    case 'UpdateApplication':
      if (!form.applicationId.trim()) {
        return '변경할 애플리케이션을 선택하세요.'
      }
      if (!form.description.trim() && !form.servicePort.trim()) {
        return '설명 또는 서비스 포트 중 하나 이상 변경값을 입력하세요.'
      }
      return null
    case 'CreateApplication':
      if (!form.name.trim()) {
        return '애플리케이션 이름을 입력하세요.'
      }
      if (!form.image.trim()) {
        return '이미지를 입력하세요.'
      }
      if (!form.servicePort.trim()) {
        return '서비스 포트를 입력하세요.'
      }
      return null
    default:
      return '지원되지 않는 작업 유형입니다.'
  }
}

function describeRequest(change: ChangeRecord) {
  const lines = [
    `operation: ${change.operation}`,
    `environment: ${change.environment}`,
    `writeMode: ${change.writeMode}`,
  ]

  if (change.request?.applicationId) {
    lines.push(`applicationId: ${change.request.applicationId}`)
  }
  if (change.request?.name) {
    lines.push(`name: ${change.request.name}`)
  }
  if (change.request?.image) {
    lines.push(`image: ${change.request.image}`)
  }
  if (change.request?.imageTag) {
    lines.push(`imageTag: ${change.request.imageTag}`)
  }
  if (change.request?.servicePort) {
    lines.push(`servicePort: ${change.request.servicePort}`)
  }
  if (change.request?.deploymentStrategy) {
    lines.push(`deploymentStrategy: ${change.request.deploymentStrategy}`)
  }
  if (change.request?.description) {
    lines.push(`description: ${change.request.description}`)
  }
  return lines
}

function getActionAvailability(change: ChangeRecord, role?: ProjectSummary['role']) {
  const canDeploy = role === 'admin' || role === 'deployer'
  const canAdmin = role === 'admin'

  if (change.status === 'Draft') {
    return {
      canSubmit: canDeploy,
      canApprove: false,
      canMerge: false,
      message: canDeploy
        ? 'Draft는 먼저 제출해야 합니다.'
        : '제출 권한이 없어 Draft를 다음 단계로 넘길 수 없습니다.',
    }
  }

  if (change.status === 'Submitted') {
    if (change.writeMode === 'pull_request') {
      return {
        canSubmit: false,
        canApprove: canAdmin,
        canMerge: false,
        message: canAdmin
          ? 'pull_request 환경은 승인 후에만 반영할 수 있습니다.'
          : '이 change는 관리자 승인 후에만 반영할 수 있습니다.',
      }
    }

    return {
      canSubmit: false,
      canApprove: canAdmin,
      canMerge: canDeploy,
      message: canDeploy
        ? '직접 반영 환경이라 제출 후 바로 merge가 가능합니다.'
        : '반영 권한이 없어 merge를 실행할 수 없습니다.',
    }
  }

  if (change.status === 'Approved') {
    return {
      canSubmit: false,
      canApprove: false,
      canMerge: canDeploy,
      message: canDeploy
        ? '승인이 끝났으므로 반영을 진행할 수 있습니다.'
        : '승인은 끝났지만 반영 권한이 없습니다.',
    }
  }

  return {
    canSubmit: false,
    canApprove: false,
    canMerge: false,
    message: '이미 반영이 끝난 change입니다.',
  }
}

function isMergeReady(change: ChangeRecord) {
  if (change.writeMode === 'pull_request') {
    return change.status === 'Approved'
  }
  return change.status === 'Submitted' || change.status === 'Approved'
}

function operationLabel(operation: ChangeOperation) {
  switch (operation) {
    case 'CreateApplication':
      return '새 앱 생성'
    case 'UpdateApplication':
      return '앱 설정 변경'
    case 'Redeploy':
      return '재배포'
    case 'UpdatePolicies':
      return '정책 변경'
    default:
      return operation
  }
}

function statusColor(status: ChangeStatus) {
  switch (status) {
    case 'Draft':
      return 'gray'
    case 'Submitted':
      return 'yellow'
    case 'Approved':
      return 'violet'
    case 'Merged':
      return 'green'
    default:
      return 'gray'
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

function formatTimestamp(value: string) {
  const timestamp = Date.parse(value)
  if (Number.isNaN(timestamp)) {
    return value
  }
  return new Intl.DateTimeFormat('ko-KR', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(timestamp))
}

function toOptionalNumber(value: string) {
  if (value.trim() === '') return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

function DetailField({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <Text size="xs" fw={700} c="dimmed" mb={4}>
        {label}
      </Text>
      <Text size="sm" fw={800}>
        {value}
      </Text>
    </div>
  )
}
